package docker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/pkg/stdcopy"
	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v2/util"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/go-connections/nat"

	"github.com/skip-mev/petri/core/v2/provider"
)

func (p *Provider) CreateTask(ctx context.Context, logger *zap.Logger, definition provider.TaskDefinition) (string, error) {
	if err := definition.ValidateBasic(); err != nil {
		return "", fmt.Errorf("failed to validate task definition: %w", err)
	}

	logger = logger.Named("docker_provider")

	if err := p.pullImage(ctx, definition.Image.Image); err != nil {
		return "", err
	}

	portSet := convertTaskDefinitionPortsToPortSet(definition)
	portBindings, listeners, err := p.GeneratePortBindings(portSet)
	if err != nil {
		return "", fmt.Errorf("failed to allocate task ports: %v", err)
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
			return "", fmt.Errorf("failed to create dataDir: %v", err)
		}

		volumeMount := mount.Mount{
			Type:   mount.TypeVolume,
			Source: volumeName,
			Target: definition.DataDir,
		}

		logger.Debug("setting volume owner", zap.String("name", volumeName), zap.String("uid", definition.Image.UID), zap.String("gid", definition.Image.GID))

		if err = p.SetVolumeOwner(ctx, volumeName, definition.Image.UID, definition.Image.GID); err != nil {
			return "", fmt.Errorf("failed to set volume owner: %v", err)
		}

		mounts = []mount.Mount{volumeMount}
	}

	logger.Debug("creating container", zap.String("name", definition.Name), zap.String("image", definition.Image.Image))

	// network map is volatile, so we need to mutex update it
	p.networkMu.Lock()
	ip, err := p.dockerNetworkAllocator.AllocateNext()
	p.networkMu.Unlock()

	if err != nil {
		return "", err
	}

	createdContainer, err := p.dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      definition.Image.Image,
		Entrypoint: definition.Entrypoint,
		Cmd:        definition.Command,
		Tty:        false,
		Hostname:   definition.Name,
		Labels: map[string]string{
			providerLabelName: p.name,
		},
		Env: convertEnvMapToList(definition.Environment),
	}, &container.HostConfig{
		Mounts:          mounts,
		PortBindings:    portBindings,
		PublishAllPorts: true,
		NetworkMode:     container.NetworkMode(p.dockerNetworkName),
	}, &network.NetworkingConfig{
		EndpointsConfig: map[string]*network.EndpointSettings{
			p.dockerNetworkName: {
				IPAMConfig: &network.EndpointIPAMConfig{
					IPv4Address: ip.String(),
				},
			},
		},
	}, nil, definition.ContainerName)
	if err != nil {
		listeners.CloseAll()
		return "", err
	}

	// network map is volatile, so we need to mutex update it
	p.networkMu.Lock()
	p.listeners[createdContainer.ID] = listeners
	p.networkMu.Unlock()

	return createdContainer.ID, nil
}

func (p *Provider) pullImage(ctx context.Context, imageName string) error {
	_, _, err := p.dockerClient.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		p.logger.Info("image not found, pulling", zap.String("image", imageName))
		resp, err := p.dockerClient.ImagePull(ctx, imageName, image.PullOptions{})
		if err != nil {
			return err
		}
		defer resp.Close()

		// throw away the image pull stdout response
		_, err = io.Copy(io.Discard, resp)
		return err
	}
	return nil
}

func (p *Provider) StartTask(ctx context.Context, id string) error {
	p.logger.Info("starting task", zap.String("id", id))
	p.networkMu.Lock()
	defer p.networkMu.Unlock()

	if _, ok := p.listeners[id]; !ok {
		return fmt.Errorf("task port listeners %s not found", id)
	}

	p.listeners[id].CloseAll()

	err := p.dockerClient.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return err
	}

	for {
		status, err := p.GetTaskStatus(ctx, id)
		if err != nil {
			return err
		}

		if status == provider.TASK_RUNNING {
			return nil
		}
		time.Sleep(time.Millisecond * 100)
	}
}

func (p *Provider) StopTask(ctx context.Context, id string) error {
	p.logger.Info("stopping task", zap.String("id", id))
	err := p.dockerClient.ContainerStop(ctx, id, container.StopOptions{})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) DestroyTask(ctx context.Context, id string) error {
	p.logger.Info("destroying task", zap.String("id", id))
	err := p.dockerClient.ContainerRemove(ctx, id, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) GetTaskStatus(ctx context.Context, id string) (provider.TaskStatus, error) {
	container, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	switch state := container.State.Status; state {
	case "created":
		return provider.TASK_STOPPED, nil
	case "running":
		return provider.TASK_RUNNING, nil
	case "paused":
		return provider.TASK_PAUSED, nil
	case "restarting":
		return provider.TASK_RUNNING, nil // todo(zygimantass): is this sane?
	case "removing":
		return provider.TASK_STOPPED, nil
	case "exited":
		return provider.TASK_STOPPED, nil
	case "dead":
		return provider.TASK_STOPPED, nil
	}

	return provider.TASK_STATUS_UNDEFINED, nil
}

func (p *Provider) RunCommand(ctx context.Context, id string, command []string) (string, string, int, error) {
	p.logger.Debug("running command", zap.String("id", id), zap.Strings("command", command))

	exec, err := p.dockerClient.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return "", "", 0, err
	}

	resp, err := p.dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	err = util.WaitForCondition(ctx, 10*time.Second, 100*time.Millisecond, func() (bool, error) {
		execInspect, err := p.dockerClient.ContainerExecInspect(ctx, exec.ID)
		if err != nil {
			return false, err
		}

		if execInspect.Running {
			return false, nil
		}

		lastExitCode = execInspect.ExitCode

		return true, nil
	})

	if err != nil {
		p.logger.Error("failed to wait for exec", zap.Error(err), zap.String("id", id))
		return "", "", lastExitCode, err
	}

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", lastExitCode, err
	}

	return stdout.String(), stderr.String(), lastExitCode, nil
}

func (p *Provider) RunCommandWhileStopped(ctx context.Context, id string, definition provider.TaskDefinition, command []string) (string, string, int, error) {
	if err := definition.ValidateBasic(); err != nil {
		return "", "", 0, fmt.Errorf("failed to validate task definition: %w", err)
	}

	p.logger.Debug("running command while stopped", zap.String("id", id), zap.Strings("command", command))

	status, err := p.GetTaskStatus(ctx, id)
	if err != nil {
		return "", "", 0, err
	}

	if status == provider.TASK_RUNNING {
		return p.RunCommand(ctx, id, command)
	}

	definition.Entrypoint = []string{"sh", "-c"}
	definition.Command = []string{"sleep 36000"}
	definition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", definition.Name, util.RandomString(5), time.Now().Unix())
	definition.Ports = []string{}

	task, err := p.CreateTask(ctx, p.logger, definition)
	if err != nil {
		return "", "", 0, err
	}

	err = p.StartTask(ctx, task)
	defer p.DestroyTask(ctx, task) // nolint:errcheck

	if err != nil {
		return "", "", 0, err
	}

	stdout, stderr, exitCode, err := p.RunCommand(ctx, task, command)
	if err != nil {
		return "", "", 0, err
	}

	return stdout, stderr, exitCode, nil
}

func (p *Provider) GetIP(ctx context.Context, id string) (string, error) {
	p.logger.Debug("getting IP", zap.String("id", id))

	container, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}

	ip := container.NetworkSettings.Networks[p.dockerNetworkName].IPAMConfig.IPv4Address

	return ip, nil
}

func (p *Provider) GetExternalAddress(ctx context.Context, id string, port string) (string, error) {
	p.logger.Debug("getting external address", zap.String("id", id), zap.String("port", port))

	container, err := p.dockerClient.ContainerInspect(ctx, id)
	if err != nil {
		return "", err
	}

	portBindings, ok := container.NetworkSettings.Ports[nat.Port(fmt.Sprintf("%s/tcp", port))]

	if !ok || len(portBindings) == 0 {
		return "", fmt.Errorf("could not find port %s", port)
	}

	ip := portBindings[0].HostIP

	return net.JoinHostPort(ip, portBindings[0].HostPort), nil
}

func (p *Provider) teardownTasks(ctx context.Context) error {
	p.logger.Info("tearing down tasks")

	containers, err := p.dockerClient.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filters.NewArgs(filters.Arg("label", fmt.Sprintf("%s=%s", providerLabelName, p.name))),
	})
	if err != nil {
		return err
	}

	for _, filteredContainer := range containers {
		err := p.DestroyTask(ctx, filteredContainer.ID)
		if err != nil {
			return err
		}
	}

	return nil
}
