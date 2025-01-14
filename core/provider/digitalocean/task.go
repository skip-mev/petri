package digitalocean

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"path"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"
)

func (p *Provider) CreateTask(ctx context.Context, logger *zap.Logger, definition provider.TaskDefinition) (string, error) {
	if err := definition.ValidateBasic(); err != nil {
		return "", fmt.Errorf("failed to validate task definition: %w", err)
	}

	if definition.ProviderSpecificConfig == nil {
		return "", fmt.Errorf("digitalocean specific config is nil for %s", definition.Name)
	}

	_, ok := definition.ProviderSpecificConfig.(DigitalOceanTaskConfig)
	if !ok {
		return "", fmt.Errorf("could not cast digitalocean specific config")
	}

	logger = logger.Named("digitalocean_provider")

	logger.Info("creating droplet", zap.String("name", definition.Name))

	droplet, err := p.CreateDroplet(ctx, definition)
	if err != nil {
		return "", err
	}

	p.droplets.Store(droplet.Name, droplet)

	ip, err := p.GetIP(ctx, droplet.Name)
	if err != nil {
		return "", err
	}

	logger.Info("droplet created", zap.String("name", droplet.Name), zap.String("ip", ip))

	dockerClient, err := p.getDropletDockerClient(ctx, droplet.Name)
	defer dockerClient.Close() // nolint

	if err != nil {
		return "", err
	}

	_, _, err = dockerClient.ImageInspectWithRaw(ctx, definition.Image.Image)
	if err != nil {
		logger.Info("image not found, pulling", zap.String("image", definition.Image.Image))
		if err := p.pullImage(ctx, dockerClient, definition.Image.Image); err != nil {
			return "", err
		}
	}

	createdContainer, err := dockerClient.ContainerCreate(ctx, &container.Config{
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
		return "", err
	}

	p.containers.Store(droplet.Name, createdContainer.ID)

	return droplet.Name, nil
}

func (p *Provider) StartTask(ctx context.Context, taskName string) error {
	dockerClient, err := p.getDropletDockerClient(ctx, taskName)
	if err != nil {
		return err
	}

	defer dockerClient.Close() // nolint

	containerID, ok := p.containers.Load(taskName)
	if !ok {
		return fmt.Errorf("could not find container for %s with ID %s", taskName, containerID)
	}

	err = dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return err
	}

	err = util.WaitForCondition(ctx, time.Second*300, time.Millisecond*100, func() (bool, error) {
		status, err := p.GetTaskStatus(ctx, taskName)
		if err != nil {
			return false, err
		}

		if status == provider.TASK_RUNNING {
			return true, nil
		}

		return false, nil
	})

	return err
}

func (p *Provider) StopTask(ctx context.Context, taskName string) error {
	dockerClient, err := p.getDropletDockerClient(ctx, taskName)
	if err != nil {
		return err
	}

	defer dockerClient.Close() // nolint

	containerID, ok := p.containers.Load(taskName)
	if !ok {
		return fmt.Errorf("could not find container for %s with ID %s", taskName, containerID)
	}

	return dockerClient.ContainerStop(ctx, containerID, container.StopOptions{})
}

func (p *Provider) DestroyTask(ctx context.Context, taskName string) error {
	logger := p.logger.With(zap.String("task", taskName))
	logger.Info("deleting task")

	err := p.deleteDroplet(ctx, taskName)
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) GetTaskStatus(ctx context.Context, taskName string) (provider.TaskStatus, error) {
	droplet, err := p.getDroplet(ctx, taskName, false)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	if droplet.Status != "active" {
		return provider.TASK_STOPPED, nil
	}

	dockerClient, err := p.getDropletDockerClient(ctx, taskName)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	defer dockerClient.Close()

	id, ok := p.containers.Load(taskName)

	if !ok {
		return provider.TASK_STATUS_UNDEFINED, fmt.Errorf("could not find container for %s with ID %s", taskName, id)
	}

	container, err := dockerClient.ContainerInspect(ctx, id)
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

func (p *Provider) WriteFile(ctx context.Context, taskName string, relPath string, content []byte) error {
	absPath := path.Join("/docker_volumes", relPath)

	sshClient, err := p.getDropletSSHClient(ctx, taskName)
	if err != nil {
		return err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return err
	}

	defer sftpClient.Close()

	err = sftpClient.MkdirAll(path.Dir(absPath))
	if err != nil {
		return err
	}

	file, err := sftpClient.Create(absPath)
	if err != nil {
		return err
	}

	_, err = file.Write(content)
	if err != nil {
		return err
	}

	return nil
}

func (p *Provider) ReadFile(ctx context.Context, taskName string, relPath string) ([]byte, error) {
	absPath := path.Join("/docker_volumes", relPath)

	sshClient, err := p.getDropletSSHClient(ctx, taskName)
	if err != nil {
		return nil, err
	}

	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return nil, err
	}

	defer sftpClient.Close()

	fs := sftpfs.New(sftpClient)

	content, err := afero.ReadFile(fs, absPath)
	if err != nil {
		return nil, err
	}

	return content, nil
}

func (p *Provider) DownloadDir(ctx context.Context, s string, s2 string, s3 string) error {
	panic("implement me")
}

func (p *Provider) GetIP(ctx context.Context, taskName string) (string, error) {
	droplet, err := p.getDroplet(ctx, taskName, true)
	if err != nil {
		return "", err
	}

	return droplet.PublicIPv4()
}

func (p *Provider) GetExternalAddress(ctx context.Context, taskName string, port string) (string, error) {
	ip, err := p.GetIP(ctx, taskName)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(ip, port), nil
}

func (p *Provider) RunCommand(ctx context.Context, taskName string, command []string) (string, string, int, error) {
	dockerClient, err := p.getDropletDockerClient(ctx, taskName)
	if err != nil {
		return "", "", 0, err
	}

	defer dockerClient.Close()

	id, ok := p.containers.Load(taskName)

	if !ok {
		return "", "", 0, fmt.Errorf("could not find container for %s with ID %s", taskName, id)
	}

	p.logger.Debug("running command", zap.String("id", id), zap.Strings("command", command))

	exec, err := dockerClient.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return "", "", 0, err
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	err = util.WaitForCondition(ctx, 10*time.Second, 100*time.Millisecond, func() (bool, error) {
		execInspect, err := dockerClient.ContainerExecInspect(ctx, exec.ID)
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
		p.logger.Error("failed to wait for exec", zap.Error(err), zap.String("taskName", taskName))
		return "", "", lastExitCode, err
	}

	var stdout, stderr bytes.Buffer

	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", lastExitCode, err
	}

	return stdout.String(), stderr.String(), lastExitCode, nil
}

func (p *Provider) RunCommandWhileStopped(ctx context.Context, taskName string, definition provider.TaskDefinition, command []string) (string, string, int, error) {
	if err := definition.ValidateBasic(); err != nil {
		return "", "", 0, fmt.Errorf("failed to validate task definition: %w", err)
	}

	dockerClient, err := p.getDropletDockerClient(ctx, taskName)
	if err != nil {
		p.logger.Error("failed to get docker client", zap.Error(err), zap.String("taskName", taskName))
		return "", "", 0, err
	}

	definition.Entrypoint = []string{"sh", "-c"}
	definition.Command = []string{"sleep 36000"}
	definition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", definition.Name, util.RandomString(5), time.Now().Unix())
	definition.Ports = []string{}

	createdContainer, err := dockerClient.ContainerCreate(ctx, &container.Config{
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
		p.logger.Error("failed to create container", zap.Error(err), zap.String("taskName", taskName))
		return "", "", 0, err
	}

	defer func() {
		if _, err := dockerClient.ContainerInspect(ctx, createdContainer.ID); err != nil && dockerclient.IsErrNotFound(err) {
			// auto-removed, but not detected as autoremoved
			return
		}

		if err := dockerClient.ContainerRemove(ctx, createdContainer.ID, container.RemoveOptions{Force: true}); err != nil {
			p.logger.Error("failed to remove container", zap.Error(err), zap.String("taskName", taskName), zap.String("id", createdContainer.ID))
		}
	}()

	if err := startContainerWithBlock(ctx, dockerClient, createdContainer.ID); err != nil {
		p.logger.Error("failed to start container", zap.Error(err), zap.String("taskName", taskName))
		return "", "", 0, err
	}

	// wait for container start
	exec, err := dockerClient.ContainerExecCreate(ctx, createdContainer.ID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		p.logger.Error("failed to create exec", zap.Error(err), zap.String("taskName", taskName))
		return "", "", 0, err
	}

	resp, err := dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		p.logger.Error("failed to attach to exec", zap.Error(err), zap.String("taskName", taskName))
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	err = util.WaitForCondition(ctx, 10*time.Second, 100*time.Millisecond, func() (bool, error) {
		execInspect, err := dockerClient.ContainerExecInspect(ctx, exec.ID)
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
		p.logger.Error("failed to wait for exec", zap.Error(err), zap.String("taskName", taskName))
		return "", "", lastExitCode, err
	}

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", 0, err
	}

	return stdout.String(), stderr.String(), lastExitCode, err
}

func startContainerWithBlock(ctx context.Context, dockerClient *dockerclient.Client, containerID string) error {
	// start container
	if err := dockerClient.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return err
	}

	// cancel container after a minute
	waitCtx, cancel := context.WithTimeout(ctx, 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(100 * time.Millisecond)
	for {
		select {
		case <-waitCtx.Done():
			return fmt.Errorf("error waiting for container to start: %v", waitCtx.Err())
		case <-ticker.C:
			container, err := dockerClient.ContainerInspect(ctx, containerID)
			if err != nil {
				return err
			}

			// if the container is running, we're done
			if container.State.Running {
				return nil
			}

			if container.State.Status == "exited" && container.State.ExitCode != 0 {
				return fmt.Errorf("container exited with status %d", container.State.ExitCode)
			}
		}
	}
}

func (p *Provider) pullImage(ctx context.Context, dockerClient *dockerclient.Client, img string) error {
	p.logger.Info("pulling image", zap.String("image", img))
	resp, err := dockerClient.ImagePull(ctx, img, image.PullOptions{})
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
