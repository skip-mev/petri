package digitalocean

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"

	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"
)

var _ provider.ProviderI = (*Provider)(nil)

const (
	providerLabelName = "petri-provider"
	sshPort           = "2375"
)

type ProviderState struct {
	TaskStates map[int]*TaskState `json:"task_states"` // map of task ids to the corresponding task state
	Name       string             `json:"name"`
	petriTag   string
	userIPs    []string
	sshKeyPair *SSHKeyPair
	firewallID string
}

type Provider struct {
	state   *ProviderState
	stateMu sync.Mutex

	logger        *zap.Logger
	doClient      DoClient
	dockerClients map[string]DockerClient // map of droplet ip address to docker clients
}

// NewProvider creates a provider that implements the Provider interface for DigitalOcean.
// Token is the DigitalOcean API token
func NewProvider(ctx context.Context, logger *zap.Logger, providerName string, token string, additionalUserIPS []string, sshKeyPair *SSHKeyPair) (*Provider, error) {
	doClient := NewGodoClient(token)
	return NewProviderWithClient(ctx, logger, providerName, doClient, nil, additionalUserIPS, sshKeyPair)
}

// NewProviderWithClient creates a provider with custom digitalocean/docker client implementation.
// This is primarily used for testing.
func NewProviderWithClient(ctx context.Context, logger *zap.Logger, providerName string, doClient DoClient, dockerClients map[string]DockerClient, additionalUserIPS []string, sshKeyPair *SSHKeyPair) (*Provider, error) {
	if sshKeyPair == nil {
		newSshKeyPair, err := MakeSSHKeyPair()
		if err != nil {
			return nil, err
		}
		sshKeyPair = newSshKeyPair
	}

	userIPs, err := getUserIPs(ctx)
	if err != nil {
		return nil, err
	}

	userIPs = append(userIPs, additionalUserIPS...)

	if dockerClients == nil {
		dockerClients = make(map[string]DockerClient)
	}

	digitalOceanProvider := &Provider{
		logger:        logger.Named("digitalocean_provider"),
		doClient:      doClient,
		dockerClients: dockerClients,
		state: &ProviderState{
			TaskStates: make(map[int]*TaskState),
			userIPs:    userIPs,
			Name:       providerName,
			sshKeyPair: sshKeyPair,
			petriTag:   fmt.Sprintf("petri-droplet-%s", util.RandomString(5)),
		},
	}

	_, err = digitalOceanProvider.createTag(ctx, digitalOceanProvider.state.petriTag)
	if err != nil {
		return nil, err
	}

	firewall, err := digitalOceanProvider.createFirewall(ctx, userIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %w", err)
	}

	digitalOceanProvider.state.firewallID = firewall.ID

	//TODO(Zygimantass): TOCTOU issue
	if key, _, err := doClient.GetKeyByFingerprint(ctx, sshKeyPair.Fingerprint); err != nil || key == nil {
		_, err = digitalOceanProvider.createSSHKey(ctx, sshKeyPair.PublicKey)
		if err != nil {
			if !strings.Contains(err.Error(), "422") {
				return nil, err
			}
		}
	}

	return digitalOceanProvider, nil
}

func (p *Provider) CreateTask(ctx context.Context, definition provider.TaskDefinition) (provider.TaskI, error) {
	if err := definition.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate task definition: %w", err)
	}

	if definition.ProviderSpecificConfig == nil {
		return nil, fmt.Errorf("digitalocean specific config is nil for %s", definition.Name)
	}

	var doConfig DigitalOceanTaskConfig
	doConfig, ok := definition.ProviderSpecificConfig.(DigitalOceanTaskConfig)
	if !ok {
		return nil, fmt.Errorf("invalid provider specific config type for %s", definition.Name)
	}

	if err := doConfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("could not cast digitalocean specific config: %w", err)
	}

	p.logger.Info("creating droplet", zap.String("name", definition.Name))

	droplet, err := p.CreateDroplet(ctx, definition)
	if err != nil {
		return nil, err
	}

	ip, err := droplet.PublicIPv4()
	if err != nil {
		return nil, err
	}

	p.logger.Info("droplet created", zap.String("name", droplet.Name), zap.String("ip", ip))

	if p.dockerClients[ip] == nil {
		p.dockerClients[ip], err = NewDockerClient(fmt.Sprintf("tcp://%s:%s", ip, sshPort))
		if err != nil {
			return nil, err
		}
	}

	_, _, err = p.dockerClients[ip].ImageInspectWithRaw(ctx, definition.Image.Image)
	if err != nil {
		p.logger.Info("image not found, pulling", zap.String("image", definition.Image.Image))
		err = pullImage(ctx, p.dockerClients[ip], p.logger, definition.Image.Image)
		if err != nil {
			return nil, err
		}
	}

	_, err = p.dockerClients[ip].ContainerCreate(ctx, &container.Config{
		Image:      definition.Image.Image,
		Entrypoint: definition.Entrypoint,
		Cmd:        definition.Command,
		Tty:        false,
		Hostname:   definition.Name,
		Labels: map[string]string{
			providerLabelName: p.state.Name,
		},
		Env: convertEnvMapToList(definition.Environment),
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/docker_volumes",
				Target: definition.DataDir,
			},
		},
		NetworkMode: container.NetworkMode("host"),
	}, nil, nil, definition.ContainerName)
	if err != nil {
		return nil, err
	}

	taskState := &TaskState{
		ID:           droplet.ID,
		Name:         definition.Name,
		Definition:   definition,
		Status:       provider.TASK_STOPPED,
		ProviderName: p.state.Name,
	}

	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	p.state.TaskStates[taskState.ID] = taskState

	return &Task{
		state:        taskState,
		provider:     p,
		sshKeyPair:   p.state.sshKeyPair,
		logger:       p.logger.With(zap.String("task", definition.Name)),
		doClient:     p.doClient,
		dockerClient: p.dockerClients[ip],
	}, nil
}

func (p *Provider) SerializeProvider(context.Context) ([]byte, error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	bz, err := json.Marshal(p.state)

	return bz, err
}

func (p *Provider) DeserializeProvider(context.Context) ([]byte, error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	bz, err := json.Marshal(p.state)

	return bz, err
}

func (p *Provider) SerializeTask(ctx context.Context, task provider.TaskI) ([]byte, error) {
	if _, ok := task.(*Task); !ok {
		return nil, fmt.Errorf("task is not a Docker task")
	}

	doTask := task.(*Task)

	bz, err := json.Marshal(doTask.state)

	if err != nil {
		return nil, err
	}

	return bz, nil
}

func (p *Provider) DeserializeTask(ctx context.Context, bz []byte) (provider.TaskI, error) {
	var taskState TaskState

	err := json.Unmarshal(bz, &taskState)
	if err != nil {
		return nil, err
	}

	task := &Task{
		state: &taskState,
	}

	return task, nil
}

func (p *Provider) Teardown(ctx context.Context) error {
	p.logger.Info("tearing down DigitalOcean provider")

	if err := p.teardownTasks(ctx); err != nil {
		return err
	}
	if err := p.teardownFirewall(ctx); err != nil {
		return err
	}
	if err := p.teardownSSHKey(ctx); err != nil {
		return err
	}
	if err := p.teardownTag(ctx); err != nil {
		return err
	}
	return nil
}

func (p *Provider) teardownTasks(ctx context.Context) error {
	res, err := p.doClient.DeleteDropletByTag(ctx, p.state.petriTag)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownFirewall(ctx context.Context) error {
	res, err := p.doClient.DeleteFirewall(ctx, p.state.firewallID)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownSSHKey(ctx context.Context) error {
	res, err := p.doClient.DeleteKeyByFingerprint(ctx, p.state.sshKeyPair.Fingerprint)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownTag(ctx context.Context) error {
	res, err := p.doClient.DeleteTag(ctx, p.state.petriTag)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) removeTask(_ context.Context, taskID int) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	delete(p.state.TaskStates, taskID)

	return nil
}
