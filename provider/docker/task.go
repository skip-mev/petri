package docker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/skip-mev/petri/provider"
	"io"
	"time"
)

func (p *Provider) CreateTask(ctx context.Context, definition provider.TaskDefinition) (string, error) {
	_, _, err := p.dockerClient.ImageInspectWithRaw(ctx, definition.Image.Image)

	if err != nil {
		if err := p.pullImage(ctx, definition); err != nil {
			return "", err
		}
	}

	portBindings, err := convertTaskDefinitionPortsToNat(definition)

	if err != nil {
		return "", fmt.Errorf("failed to convert task ports to bindings: %v", err)
	}

	var mounts []mount.Mount

	if definition.DataDir != "" {
		volumeName := fmt.Sprintf("%s-data", definition.Name)
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

		if err = p.SetVolumeOwner(ctx, volumeName, definition.Image.UID, definition.Image.GID); err != nil {
			return "", fmt.Errorf("failed to set volume owner: %v", err)
		}

		mounts = []mount.Mount{volumeMount}
	}

	createdContainer, err := p.dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      definition.Image.Image,
		Entrypoint: definition.Entrypoint,
		Cmd:        definition.Command,
		Tty:        false,
	}, &container.HostConfig{
		Mounts:       mounts,
		PortBindings: portBindings,
	}, nil, nil, definition.ContainerName)

	if err != nil {
		return "", err
	}

	return createdContainer.ID, nil
}

func (p *Provider) pullImage(ctx context.Context, definition provider.TaskDefinition) error {
	resp, err := p.dockerClient.ImagePull(ctx, definition.Image.Image, types.ImagePullOptions{})

	if err != nil {
		return err
	}

	defer resp.Close()
	// throw away the image pull stdout response
	_, err = io.Copy(io.Discard, resp)

	if err != nil {
		return err
	}
	return nil
}

func (p *Provider) StartTask(ctx context.Context, id string) error {
	err := p.dockerClient.ContainerStart(ctx, id, types.ContainerStartOptions{})

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
		time.Sleep(time.Second)
	}
}

func (p *Provider) StopTask(ctx context.Context, id string) error {
	err := p.dockerClient.ContainerStop(ctx, id, container.StopOptions{})

	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) DestroyTask(ctx context.Context, id string) error {
	err := p.dockerClient.ContainerRemove(ctx, id, types.ContainerRemoveOptions{
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

func (p *Provider) RunCommand(ctx context.Context, id string, command []string) (string, int, error) {
	exec, err := p.dockerClient.ContainerExecCreate(ctx, id, types.ExecConfig{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})

	if err != nil {
		return "", 0, err
	}

	resp, err := p.dockerClient.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})

	if err != nil {
		return "", 0, err
	}

	defer resp.Close()

	stdout, err := io.ReadAll(resp.Reader)

	if err != nil {
		return "", 0, err
	}

	execInspect, err := p.dockerClient.ContainerExecInspect(ctx, exec.ID)

	if err != nil {
		return "", 0, err
	}

	return string(stdout), execInspect.ExitCode, nil
}

func (p *Provider) GetIP(ctx context.Context, id string) (string, error) {
	container, err := p.dockerClient.ContainerInspect(ctx, id)

	if err != nil {
		return "", err
	}

	return container.NetworkSettings.IPAddress, nil
}

func (p *Provider) GetHostname(ctx context.Context, id string) (string, error) {
	container, err := p.dockerClient.ContainerInspect(ctx, id)

	if err != nil {
		return "", err
	}

	hostname := bytes.TrimPrefix([]byte(container.Name), []byte("\\"))

	return string(hostname), nil
}
