package digitalocean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

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
	dockerPort        = "2375"
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

	logger        *zap.Logger
	doClient      DoClient
	dockerClients map[string]clients.DockerClient // map of droplet ip address to docker clients
}

// NewProvider creates a provider with custom digitalocean/docker client implementation.
// This is primarily used for testing.
func NewProvider(
	ctx context.Context,
	logger *zap.Logger,
	providerName string,
	opts ...func(*Provider),
) (*Provider, error) {
	petriTag := fmt.Sprintf("petri-droplet-%s", util.RandomString(5))
	digitalOceanProvider := &Provider{
		logger: logger.Named("digitalocean_provider"),
		state: &ProviderState{
			TaskStates: make(map[string]*TaskState),
			Name:       providerName,
			PetriTag:   petriTag,
		},
	}

	for _, opt := range opts {
		opt(digitalOceanProvider)
	}

	if digitalOceanProvider.doClient == nil {
		return nil, errors.New("digital ocean client is nil, please use either WithDigitalOceanToken or WithDigitalOceanClient")
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

	if digitalOceanProvider.dockerClients == nil {
		digitalOceanProvider.dockerClients = make(map[string]clients.DockerClient)
	}

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

	var doConfig DigitalOceanTaskConfig
	doConfig = definition.ProviderSpecificConfig

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

	dockerClient := p.dockerClients[ip]
	if dockerClient == nil {
		dockerClient, err = clients.NewDockerClient(ip)
		if err != nil {
			return nil, err
		}
	}

	_, _, err = dockerClient.ImageInspectWithRaw(ctx, definition.Image.Image)
	if err != nil {
		p.logger.Info("image not found, pulling", zap.String("image", definition.Image.Image))
		if err = dockerClient.ImagePull(ctx, p.logger, definition.Image.Image, image.PullOptions{}); err != nil {
			return nil, err
		}
	}

	state := p.GetState()

	err = util.WaitForCondition(ctx, 30*time.Second, 1*time.Second, func() (bool, error) {
		_, err := dockerClient.ContainerCreate(ctx, &container.Config{
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
			NetworkMode: container.NetworkMode("host"),
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

	taskState := &TaskState{
		ID:           strconv.Itoa(droplet.ID),
		Name:         definition.Name,
		Definition:   definition,
		Status:       provider.TASK_STOPPED,
		ProviderName: state.Name,
		SSHKeyPair:   state.SSHKeyPair,
	}

	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	p.state.TaskStates[taskState.ID] = taskState

	return &Task{
		state:        taskState,
		removeTask:   p.removeTask,
		logger:       p.logger.With(zap.String("task", definition.Name)),
		doClient:     p.doClient,
		dockerClient: dockerClient,
	}, nil
}

func (p *Provider) SerializeProvider(context.Context) ([]byte, error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	bz, err := json.Marshal(p.state)

	return bz, err
}

func RestoreProvider(ctx context.Context, logger *zap.Logger, token string, state []byte, doClient DoClient, dockerClients map[string]clients.DockerClient) (*Provider, error) {
	if doClient == nil && token == "" {
		return nil, errors.New("a valid token or digital ocean client must be passed when restoring the provider")
	}
	var providerState ProviderState

	err := json.Unmarshal(state, &providerState)
	if err != nil {
		return nil, err
	}

	if dockerClients == nil {
		dockerClients = make(map[string]clients.DockerClient)
	}

	if logger == nil {
		logger = zap.L()
	}

	digitalOceanProvider := &Provider{
		state:         &providerState,
		dockerClients: dockerClients,
		logger:        logger.Named("digitalocean_provider"),
	}

	if doClient != nil {
		digitalOceanProvider.doClient = doClient
	} else {
		digitalOceanProvider.doClient = NewGodoClient(token)
	}

	for _, taskState := range providerState.TaskStates {
		id, err := strconv.Atoi(taskState.ID)
		if err != nil {
			return nil, err
		}

		droplet, err := digitalOceanProvider.doClient.GetDroplet(ctx, id)
		if err != nil {
			return nil, fmt.Errorf("failed to get droplet for task state: %w", err)
		}

		ip, err := droplet.PublicIPv4()
		if err != nil {
			return nil, fmt.Errorf("failed to get droplet IP: %w", err)
		}

		if digitalOceanProvider.dockerClients[ip] == nil {
			dockerClient, err := clients.NewDockerClient(ip)
			if err != nil {
				return nil, fmt.Errorf("failed to create docker client: %w", err)
			}
			digitalOceanProvider.dockerClients[ip] = dockerClient
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
		state:      &taskState,
		removeTask: p.removeTask,
	}

	if err := p.initializeDeserializedTask(ctx, task); err != nil {
		return nil, err
	}

	return task, nil
}

func (p *Provider) initializeDeserializedTask(ctx context.Context, task *Task) error {
	taskState := task.GetState()
	task.logger = p.logger.With(zap.String("task", taskState.Name))
	task.doClient = p.doClient

	droplet, err := task.getDroplet(ctx)
	if err != nil {
		return fmt.Errorf("failed to get droplet for task initialization: %w", err)
	}

	ip, err := droplet.PublicIPv4()
	if err != nil {
		return fmt.Errorf("failed to get droplet IP: %w", err)
	}

	if p.dockerClients[ip] == nil {
		dockerClient, err := clients.NewDockerClient(ip)
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		p.dockerClients[ip] = dockerClient
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
