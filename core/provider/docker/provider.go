package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"

	"github.com/skip-mev/petri/core/v2/provider"

	"github.com/cilium/ipam/service/ipallocator"
	"github.com/docker/docker/api/types/network"
	"go.uber.org/zap"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"

	"github.com/docker/docker/client"
)

type ProviderState struct {
	TaskStates map[string]*TaskState `json:"task_states"`

	Name string `json:"name"`

	NetworkID      string `json:"network_id"`
	NetworkName    string `json:"network_name"`
	NetworkCIDR    string `json:"network_cidr"`
	NetworkGateway string `json:"network_gateway"`

	BuilderImageName string `json:"builder_image_name"`
}

type Provider struct {
	state   *ProviderState
	stateMu sync.Mutex

	dockerClient           provider.DockerClient
	dockerNetworkAllocator *ipallocator.Range
	networkMu              sync.Mutex
	logger                 *zap.Logger
}

var _ provider.ProviderI = (*Provider)(nil)

func CreateProvider(ctx context.Context, logger *zap.Logger, providerName string) (*Provider, error) {
	dockerClient, err := client.NewClientWithOpts()
	if err != nil {
		return nil, err
	}

	_, err = dockerClient.Ping(ctx)
	if err != nil {
		return nil, err
	}

	if providerName == "" {
		return nil, fmt.Errorf("provider name cannot be empty")
	}

	state := ProviderState{
		Name:             providerName,
		BuilderImageName: "busybox:latest",
		NetworkName:      fmt.Sprintf("petri-network-%s", providerName),
		TaskStates:       make(map[string]*TaskState),
	}

	dockerProvider := &Provider{
		dockerClient: dockerClient,
		state:        &state,
		logger:       logger,
	}

	network, err := dockerProvider.initNetwork(ctx)
	if err != nil {
		return nil, err
	}

	dockerProvider.state.NetworkID = network.ID

	if len(network.IPAM.Config) == 0 {
		return nil, fmt.Errorf("network does not have an IPAM config")
	}

	_, cidrMask, err := net.ParseCIDR(network.IPAM.Config[0].Subnet)
	if err != nil {
		return nil, err
	}

	dockerProvider.state.NetworkCIDR = cidrMask.String()
	dockerProvider.state.NetworkGateway = network.IPAM.Config[0].Gateway

	dockerProvider.dockerNetworkAllocator, err = ipallocator.NewCIDRRange(cidrMask)

	if err := dockerProvider.dockerNetworkAllocator.Allocate(net.ParseIP(network.IPAM.Config[0].Gateway)); err != nil {
		return nil, fmt.Errorf("failed to allocate gateway ip: %w", err)
	}

	if err != nil {
		return nil, err
	}

	return dockerProvider, nil
}

func RestoreProvider(ctx context.Context, logger *zap.Logger, state []byte) (*Provider, error) {
	var providerState ProviderState

	err := json.Unmarshal(state, &providerState)

	if err != nil {
		return nil, err
	}

	dockerProvider := &Provider{
		state:  &providerState,
		logger: logger,
	}

	dockerClient, err := provider.NewDockerClient("")
	if err != nil {
		return nil, err
	}

	_, err = dockerClient.Ping(ctx)

	if err != nil {
		return nil, err
	}

	dockerProvider.dockerClient = dockerClient
	_, cidrMask, err := net.ParseCIDR(providerState.NetworkCIDR)

	if err != nil {
		return nil, fmt.Errorf("failed to parse cidr mask from state: %w", err)
	}

	dockerProvider.dockerNetworkAllocator, err = ipallocator.NewCIDRRange(cidrMask)

	if err != nil {
		return nil, fmt.Errorf("failed to create ip allocator from state: %w", err)
	}

	if err := dockerProvider.dockerNetworkAllocator.Allocate(net.ParseIP(providerState.NetworkGateway)); err != nil {
		return nil, fmt.Errorf("failed to allocate gateway ip: %w", err)
	}

	for _, task := range providerState.TaskStates {
		if err := dockerProvider.dockerNetworkAllocator.Allocate(net.ParseIP(task.IpAddress)); err != nil {
			return nil, fmt.Errorf("failed to restore ip allocator state: %w", err)
		}
	}

	if err := dockerProvider.ensureNetwork(ctx); err != nil {
		return nil, fmt.Errorf("failed to reconcilliate the docker network: %w", err)
	}

	return dockerProvider, nil
}

func (p *Provider) CreateTask(ctx context.Context, definition provider.TaskDefinition) (provider.TaskI, error) {
	if err := definition.ValidateBasic(); err != nil {
		return &Task{}, fmt.Errorf("failed to validate task definition: %w", err)
	}

	taskState := &TaskState{
		Name:             definition.Name,
		Definition:       definition,
		BuilderImageName: p.state.BuilderImageName,
	}

	logger := p.logger.Named("docker_provider")

	if err := provider.PullImage(ctx, p.dockerClient, logger, definition.Image.Image); err != nil {
		return nil, err
	}

	portSet := convertTaskDefinitionPortsToPortSet(definition)
	portBindings, err := p.GeneratePortBindings(portSet)
	if err != nil {
		return nil, fmt.Errorf("failed to allocate task ports: %v", err)
	}

	var mounts []mount.Mount

	logger.Debug("creating task", zap.String("name", definition.Name), zap.String("image", definition.Image.Image))

	if definition.DataDir != "" {
		volumeName := fmt.Sprintf("%s-data", definition.Name)

		logger.Debug("creating volume", zap.String("name", volumeName))

		_, err = p.CreateVolume(ctx, provider.VolumeDefinition{
			Name:      volumeName,
			Size:      "10GB",
			MountPath: definition.DataDir,
		})

		if err != nil {
			return nil, fmt.Errorf("failed to create dataDir: %v", err)
		}

		volumeMount := mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: definition.DataDir,
		}

		logger.Debug("setting volume owner", zap.String("name", volumeName), zap.String("uid", definition.Image.UID), zap.String("gid", definition.Image.GID))

		if err = p.SetVolumeOwner(ctx, volumeName, definition.Image.UID, definition.Image.GID); err != nil {
			return nil, fmt.Errorf("failed to set volume owner: %v", err)
		}

		mounts = []mount.Mount{volumeMount}

		taskState.Volume = &VolumeState{
			Name: volumeName,
			Size: "10GB",
		}
	}

	logger.Debug("creating container", zap.String("name", definition.Name), zap.String("image", definition.Image.Image))

	ip, err := p.nextAvailableIP()
	if err != nil {
		return nil, err
	}

	createdContainer, err := p.dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      definition.Image.Image,
		Entrypoint: definition.Entrypoint,
		Cmd:        definition.Command,
		Tty:        false,
		Hostname:   definition.Name,
		Labels: map[string]string{
			providerLabelName: p.state.Name,
		},
		Env:          convertEnvMapToList(definition.Environment),
		ExposedPorts: portSet,
	}, &container.HostConfig{
		Mounts:       mounts,
		PortBindings: portBindings,
		NetworkMode:  container.NetworkMode(p.state.NetworkName),
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			p.state.NetworkName: {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: ip,
				},
			},
		},
	}, nil, definition.ContainerName)

	if err != nil {
		return nil, err
	}

	taskState.Id = createdContainer.ID
	taskState.Status = provider.TASK_STOPPED
	taskState.NetworkName = p.state.NetworkName
	taskState.ProviderName = p.state.Name
	taskState.IpAddress = ip

	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	p.state.TaskStates[taskState.Id] = taskState

	return &Task{
		state:        taskState,
		logger:       p.logger.With(zap.String("task", definition.Name)),
		dockerClient: p.dockerClient,
		removeTask:   p.removeTask,
	}, nil
}

func (p *Provider) SerializeProvider(context.Context) ([]byte, error) {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	bz, err := json.Marshal(p.state)

	return bz, err
}

func (p *Provider) SerializeTask(ctx context.Context, task provider.TaskI) ([]byte, error) {
	if _, ok := task.(*Task); !ok {
		return nil, fmt.Errorf("task is not a Docker task")
	}

	dockerTask := task.(*Task)

	bz, err := json.Marshal(dockerTask.state)

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
		state:        &taskState,
		logger:       p.logger.With(zap.String("task", taskState.Name)),
		dockerClient: p.dockerClient,
		removeTask:   p.removeTask,
	}

	if err := task.ensureTask(ctx); err != nil {
		return nil, err
	}

	return task, nil
}

func (p *Provider) removeTask(_ context.Context, taskID string) error {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()

	delete(p.state.TaskStates, taskID)

	return nil
}

func (p *Provider) Teardown(ctx context.Context) error {
	p.logger.Info("tearing down Docker provider")

	for _, task := range p.state.TaskStates {
		if err := p.dockerClient.ContainerRemove(ctx, task.Id, container.RemoveOptions{
			Force: true,
		}); err != nil {
			return err
		}
	}

	if err := p.teardownVolumes(ctx); err != nil {
		return err
	}

	if err := p.destroyNetwork(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Provider) GetState() ProviderState {
	p.stateMu.Lock()
	defer p.stateMu.Unlock()
	return *p.state
}
