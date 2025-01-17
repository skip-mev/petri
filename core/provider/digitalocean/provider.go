package digitalocean

import (
	"context"
	"encoding/json"
	"errors"
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
	PetriTag   string             `json:"petri_tag"`
	UserIPs    []string           `json:"user_ips"`
	SSHKeyPair *SSHKeyPair        `json:"ssh_key_pair"`
	FirewallID string             `json:"firewall_id"`
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
			UserIPs:    userIPs,
			Name:       providerName,
			SSHKeyPair: sshKeyPair,
			PetriTag:   fmt.Sprintf("petri-droplet-%s", util.RandomString(5)),
		},
	}

	_, err = digitalOceanProvider.createTag(ctx, digitalOceanProvider.state.PetriTag)
	if err != nil {
		return nil, err
	}

	firewall, err := digitalOceanProvider.createFirewall(ctx, userIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %w", err)
	}

	digitalOceanProvider.state.FirewallID = firewall.ID

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
		sshKeyPair:   p.state.SSHKeyPair,
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

func RestoreProvider(ctx context.Context, token string, state []byte, doClient DoClient, dockerClients map[string]DockerClient) (*Provider, error) {
	if doClient == nil && token == "" {
		return nil, errors.New("a valid token or digital ocean client must be passed when restoring the provider")
	}
	var providerState ProviderState

	err := json.Unmarshal(state, &providerState)
	if err != nil {
		return nil, err
	}

	if dockerClients == nil {
		dockerClients = make(map[string]DockerClient)
	}

	digitalOceanProvider := &Provider{
		state:         &providerState,
		dockerClients: dockerClients,
		logger:        zap.L().Named("digitalocean_provider"),
	}

	if doClient != nil {
		digitalOceanProvider.doClient = doClient
	} else {
		digitalOceanProvider.doClient = NewGodoClient(token)
	}

	for _, taskState := range providerState.TaskStates {
		droplet, _, err := digitalOceanProvider.doClient.GetDroplet(ctx, taskState.ID)
		if err != nil {
			return nil, fmt.Errorf("failed to get droplet for task state: %w", err)
		}

		ip, err := droplet.PublicIPv4()
		if err != nil {
			return nil, fmt.Errorf("failed to get droplet IP: %w", err)
		}

		if digitalOceanProvider.dockerClients[ip] == nil {
			client, err := NewDockerClient(fmt.Sprintf("tcp://%s:%s", ip, sshPort))
			if err != nil {
				return nil, fmt.Errorf("failed to create docker client: %w", err)
			}
			digitalOceanProvider.dockerClients[ip] = client
		}
	}

	return digitalOceanProvider, nil
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

	if err := p.initializeDeserializedTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (p *Provider) initializeDeserializedTask(ctx context.Context, task *Task) error {
	task.logger = p.logger.With(zap.String("task", task.state.Name))
	task.sshKeyPair = p.state.SSHKeyPair
	task.doClient = p.doClient
	task.provider = p

	droplet, err := task.getDroplet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get droplet for task initialization: %w", err)
	}

	ip, err := droplet.PublicIPv4()
	if err != nil {
		return fmt.Errorf("failed to get droplet IP: %w", err)
	}

	if p.dockerClients[ip] == nil {
		client, err := NewDockerClient(fmt.Sprintf("tcp://%s:%s", ip, sshPort))
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		p.dockerClients[ip] = client
	}

	task.dockerClient = p.dockerClients[ip]
	return nil
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
	res, err := p.doClient.DeleteDropletByTag(ctx, p.state.PetriTag)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownFirewall(ctx context.Context) error {
	res, err := p.doClient.DeleteFirewall(ctx, p.state.FirewallID)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownSSHKey(ctx context.Context) error {
	res, err := p.doClient.DeleteKeyByFingerprint(ctx, p.state.SSHKeyPair.Fingerprint)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownTag(ctx context.Context) error {
	res, err := p.doClient.DeleteTag(ctx, p.state.PetriTag)
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
