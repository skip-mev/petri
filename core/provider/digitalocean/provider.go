package digitalocean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"tailscale.com/tsnet"

	"github.com/digitalocean/godo"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"

	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/clients"
	"github.com/skip-mev/petri/core/v3/util"
)

var _ provider.ProviderI = (*Provider)(nil)

const (
	providerLabelName = "petri-provider"
)

type ProviderState struct {
	TaskStates map[string]*TaskState `json:"task_states"` // map of task ids to the corresponding task state
	Name       string                `json:"name"`
	PetriTag   string                `json:"petri_tag"`
	UserIPs    []string              `json:"user_ips"`
	SSHKeyPair *SSHKeyPair           `json:"ssh_key_pair"`
	FirewallID string                `json:"firewall_id"`
}

type Provider struct {
	state   *ProviderState
	stateMu sync.Mutex

	logger                *zap.Logger
	doClient              DoClient
	tailscaleServer       *tsnet.Server
	tailscaleTags         []string
	tailscaleNodeAuthkey  string
	dockerClientOverrides map[string]clients.DockerClient // map of droplet name to docker clients
}

func NewProvider(ctx context.Context, providerName, token string, opts ...func(*Provider)) (*Provider, error) {
	if token == "" {
		return nil, errors.New("a non-empty token must be passed when creating a DigitalOcean provider")
	}

	doClient := NewGodoClient(token)
	return NewProviderWithClient(ctx, providerName, doClient, opts...)
}

// NewProviderWithClient creates a DigitalOcean provider given an existing DigitalOcean client
// with additional options to configure behaviour.
func NewProviderWithClient(ctx context.Context, providerName string, doClient DoClient, opts ...func(*Provider)) (*Provider, error) {
	if doClient == nil {
		return nil, errors.New("a valid digital ocean client must be passed when creating a provider")
	}

	petriTag := fmt.Sprintf("petri-droplet-%s", util.RandomString(5))
	digitalOceanProvider := &Provider{
		doClient: doClient,
		state: &ProviderState{
			TaskStates: make(map[string]*TaskState),
			Name:       providerName,
			PetriTag:   petriTag,
		},
	}

	for _, opt := range opts {
		opt(digitalOceanProvider)
	}

	if digitalOceanProvider.logger == nil {
		digitalOceanProvider.logger = zap.NewNop()
	}

	if digitalOceanProvider.state.SSHKeyPair == nil {
		sshKeyPair, err := MakeSSHKeyPair()
		if err != nil {
			return nil, err
		}

		digitalOceanProvider.state.SSHKeyPair = sshKeyPair
	}

	userIPs, err := getUserIPs(ctx)
	if err != nil {
		return nil, err
	}

	digitalOceanProvider.state.UserIPs = append(digitalOceanProvider.state.UserIPs, userIPs...)

	_, err = digitalOceanProvider.createTag(ctx, petriTag)
	if err != nil {
		return nil, err
	}

	firewall, err := digitalOceanProvider.createFirewall(ctx, userIPs)
	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %w", err)
	}

	digitalOceanProvider.state.FirewallID = firewall.ID

	//TODO(Zygimantass): TOCTOU issue
	if key, err := digitalOceanProvider.doClient.GetKeyByFingerprint(ctx, digitalOceanProvider.state.SSHKeyPair.Fingerprint); err != nil || key == nil {
		_, err = digitalOceanProvider.createSSHKey(ctx, digitalOceanProvider.state.SSHKeyPair.PublicKey)
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

	var doConfig DigitalOceanTaskConfig = definition.ProviderSpecificConfig

	if err := doConfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("could not cast digitalocean specific config: %w", err)
	}

	p.logger.Info("creating droplet", zap.String("name", definition.Name))

	droplet, err := p.CreateDroplet(ctx, definition)
	if err != nil {
		return nil, err
	}

	p.logger.Info("droplet created", zap.String("name", droplet.Name))

	state := p.GetState()

	taskState := &TaskState{
		ID:               strconv.Itoa(droplet.ID),
		Name:             definition.Name,
		Definition:       definition,
		Status:           provider.TASK_STOPPED,
		ProviderName:     state.Name,
		SSHKeyPair:       state.SSHKeyPair,
		TailscaleEnabled: p.tailscaleServer != nil,
	}

	p.stateMu.Lock()
	p.state.TaskStates[taskState.ID] = taskState
	p.stateMu.Unlock()

	task := &Task{
		state:           taskState,
		removeTask:      p.removeTask,
		logger:          p.logger.With(zap.String("task", definition.Name)),
		doClient:        p.doClient,
		tailscaleServer: p.tailscaleServer,
	}

	if p.tailscaleServer != nil {
		if err := task.waitForSSHClient(ctx); err != nil {
			return nil, fmt.Errorf("failed to wait for docker start: %w", err)
		}

		_, err = task.launchTailscale(ctx, p.tailscaleNodeAuthkey, p.tailscaleTags)
		if err != nil {
			return nil, fmt.Errorf("failed to launch tailscale server: %w", err)
		}
	}

	ip, err := task.GetIP(ctx)

	if err != nil {
		return nil, err
	}

	task.dockerClient = p.getDockerClientOverride(task.GetState().Name)

	if task.dockerClient == nil {
		task.dockerClient, err = clients.NewDockerClient(ip, p.getDialFunc())

		if err != nil {
			return nil, fmt.Errorf("failed to create docker client: %w", err)
		}
	}

	if err := task.waitForDockerStart(ctx); err != nil {
		return nil, fmt.Errorf("failed to wait for docker start: %w", err)
	}

	_, _, err = task.dockerClient.ImageInspectWithRaw(ctx, definition.Image.Image)
	if err != nil {
		p.logger.Info("image not found, pulling", zap.String("image", definition.Image.Image))
		if err = task.dockerClient.ImagePull(ctx, p.logger, definition.Image.Image, image.PullOptions{}); err != nil {
			return nil, err
		}
	}

	err = util.WaitForCondition(ctx, 30*time.Second, 1*time.Second, func() (bool, error) {
		_, err := task.dockerClient.ContainerCreate(ctx, &container.Config{
			Image:      definition.Image.Image,
			Entrypoint: definition.Entrypoint,
			Cmd:        definition.Command,
			Tty:        false,
			Hostname:   definition.Name,
			Labels: map[string]string{
				providerLabelName: state.Name,
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
			NetworkMode: "host",
		}, nil, nil, definition.ContainerName)

		if err != nil {
			if client.IsErrConnectionFailed(err) {
				p.logger.Warn("connection failed while creating container, will retry", zap.Error(err))
				return false, nil
			}
			return false, err
		}

		return true, nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create container after retries: %w", err)
	}

	return task, nil
}

func (p *Provider) SerializeProvider(context.Context) ([]byte, error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	bz, err := json.Marshal(p.state)

	return bz, err
}

func RestoreProvider(ctx context.Context, state []byte, token string, opts ...func(*Provider)) (*Provider, error) {
	if token == "" {
		return nil, errors.New("a non-empty token must be passed when restoring a DigitalOcean provider")
	}

	doClient := NewGodoClient(token)
	return RestoreProviderWithClient(ctx, state, doClient, opts...)
}

func RestoreProviderWithClient(_ context.Context, state []byte, doClient DoClient, opts ...func(*Provider)) (*Provider, error) {
	if doClient == nil {
		return nil, errors.New("a valid digital ocean client must be passed when restoring the provider")
	}

	var providerState ProviderState

	err := json.Unmarshal(state, &providerState)
	if err != nil {
		return nil, err
	}

	digitalOceanProvider := &Provider{
		state:    &providerState,
		doClient: doClient,
	}

	for _, opt := range opts {
		opt(digitalOceanProvider)
	}

	if digitalOceanProvider.logger == nil {
		digitalOceanProvider.logger = zap.NewNop()
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
		state:      &taskState,
		removeTask: p.removeTask,
	}

	if err := p.initializeDeserializedTask(task); err != nil {
		return nil, err
	}

	return task, nil
}

func (p *Provider) initializeDeserializedTask(task *Task) error {
	taskState := task.GetState()
	task.logger = p.logger.With(zap.String("task", taskState.Name))
	task.doClient = p.doClient
	task.dockerClient = p.getDockerClientOverride(task.GetState().Name)

	if task.dockerClient == nil {
		ip, err := task.GetIP(context.Background())
		if err != nil {
			return err
		}

		task.dockerClient, err = clients.NewDockerClient(ip, p.getDialFunc())
		if err != nil {
			return err
		}
	}

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
	return p.doClient.DeleteDropletByTag(ctx, p.GetState().PetriTag)
}

func (p *Provider) teardownFirewall(ctx context.Context) error {
	return p.doClient.DeleteFirewall(ctx, p.GetState().FirewallID)
}

func (p *Provider) teardownSSHKey(ctx context.Context) error {
	return p.doClient.DeleteKeyByFingerprint(ctx, p.GetState().SSHKeyPair.Fingerprint)
}

func (p *Provider) teardownTag(ctx context.Context) error {
	return p.doClient.DeleteTag(ctx, p.GetState().PetriTag)
}

func (p *Provider) removeTask(_ context.Context, taskID string) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	delete(p.state.TaskStates, taskID)

	return nil
}

func (p *Provider) createTag(ctx context.Context, tagName string) (*godo.Tag, error) {
	req := &godo.TagCreateRequest{
		Name: tagName,
	}

	return p.doClient.CreateTag(ctx, req)
}

func (p *Provider) GetState() ProviderState {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return *p.state
}

func (p *Provider) getDialFunc() func(ctx context.Context, network, address string) (net.Conn, error) {
	if p.tailscaleServer == nil {
		return nil
	}

	return p.tailscaleServer.Dial
}

func (p *Provider) getDockerClientOverride(task string) clients.DockerClient {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	if dockerClient, ok := p.dockerClientOverrides[task]; ok {
		return dockerClient
	}

	return nil
}
