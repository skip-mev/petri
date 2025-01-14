package digitalocean

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net"
	"path"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/api/types/mount"
	dockerclient "github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/pkg/errors"
	"github.com/pkg/sftp"
	"github.com/spf13/afero"
	"github.com/spf13/afero/sftpfs"
	"go.uber.org/zap"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/util"
)

type TaskState struct {
	ID           int                     `json:"id"`
	Name         string                  `json:"name"`
	Definition   provider.TaskDefinition `json:"definition"`
	Status       provider.TaskStatus     `json:"status"`
	ProviderName string                  `json:"provider_name"`
}

type Task struct {
	state   *TaskState
	stateMu sync.Mutex

	provider     *Provider
	logger       *zap.Logger
	sshKeyPair   *SSHKeyPair
	sshClient    *ssh.Client
	doClient     DoClient
	dockerClient DockerClient
}

var _ provider.TaskI = (*Task)(nil)

func (t *Task) Start(ctx context.Context) error {
	containers, err := t.dockerClient.ContainerList(ctx, container.ListOptions{
		Limit: 1,
	})

	if err != nil {
		return fmt.Errorf("failed to retrieve containers: %w", err)
	}

	if len(containers) != 1 {
		return fmt.Errorf("could not find container for %s", t.state.Name)
	}

	containerID := containers[0].ID

	err = t.dockerClient.ContainerStart(ctx, containerID, container.StartOptions{})
	if err != nil {
		return err
	}

	err = util.WaitForCondition(ctx, time.Second*300, time.Millisecond*100, func() (bool, error) {
		status, err := t.GetStatus(ctx)
		if err != nil {
			return false, err
		}

		if status == provider.TASK_RUNNING {
			t.stateMu.Lock()
			defer t.stateMu.Unlock()

			t.state.Status = provider.TASK_RUNNING
			return true, nil
		}

		return false, nil
	})

	t.logger.Info("Final task status after start", zap.Any("status", t.state.Status))
	return err
}

func (t *Task) Stop(ctx context.Context) error {
	containers, err := t.dockerClient.ContainerList(ctx, container.ListOptions{
		Limit: 1,
	})

	if err != nil {
		return fmt.Errorf("failed to retrieve containers: %w", err)
	}

	if len(containers) != 1 {
		return fmt.Errorf("could not find container for %s", t.state.Name)
	}

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	t.state.Status = provider.TASK_STOPPED
	return t.dockerClient.ContainerStop(ctx, containers[0].ID, container.StopOptions{})
}

func (t *Task) Initialize(ctx context.Context) error {
	panic("implement me")
}

func (t *Task) Modify(ctx context.Context, definition provider.TaskDefinition) error {
	panic("implement me")
}

func (t *Task) Destroy(ctx context.Context) error {
	logger := t.logger.With(zap.String("task", t.state.Name))
	logger.Info("deleting task")
	defer t.dockerClient.Close()

	err := t.deleteDroplet(ctx)
	if err != nil {
		return err
	}

	// TODO(nadim-az): remove reference to provider in Task struct
	if err := t.provider.removeTask(ctx, t.state.ID); err != nil {
		return err
	}
	return nil
}

func (t *Task) GetState() TaskState {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return *t.state
}

func (t *Task) GetDefinition() provider.TaskDefinition {
	return t.GetState().Definition
}

func (t *Task) GetStatus(ctx context.Context) (provider.TaskStatus, error) {
	droplet, err := t.getDroplet(ctx)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	if droplet.Status != "active" {
		return provider.TASK_STOPPED, nil
	}

	containers, err := t.dockerClient.ContainerList(ctx, container.ListOptions{
		Limit: 1,
	})

	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, fmt.Errorf("failed to retrieve containers: %w", err)
	}

	if len(containers) != 1 {
		return provider.TASK_STATUS_UNDEFINED, fmt.Errorf("could not find container for %s", t.state.Name)
	}

	c, err := t.dockerClient.ContainerInspect(ctx, containers[0].ID)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	switch state := c.State.Status; state {
	case "created":
		return provider.TASK_STOPPED, nil
	case "running":
		return provider.TASK_RUNNING, nil
	case "paused":
		return provider.TASK_PAUSED, nil
	case "removing":
		return provider.TASK_STOPPED, nil
	case "restarting":
		return provider.TASK_RESTARTING, nil
	case "exited":
		return provider.TASK_STOPPED, nil
	case "dead":
		return provider.TASK_STOPPED, nil
	}

	return provider.TASK_STATUS_UNDEFINED, nil
}

func (t *Task) WriteFile(ctx context.Context, relPath string, content []byte) error {
	absPath := path.Join("/docker_volumes", relPath)

	sshClient, err := t.getDropletSSHClient(ctx, t.state.Name)
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

func (t *Task) ReadFile(ctx context.Context, relPath string) ([]byte, error) {
	absPath := path.Join("/docker_volumes", relPath)

	sshClient, err := t.getDropletSSHClient(ctx, t.state.Name)
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

func (t *Task) DownloadDir(ctx context.Context, s string, s2 string) error {
	panic("implement me")
}

func (t *Task) GetIP(ctx context.Context) (string, error) {
	droplet, err := t.getDroplet(ctx)

	if err != nil {
		return "", err
	}

	return droplet.PublicIPv4()
}

func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	ip, err := t.GetIP(ctx)
	if err != nil {
		return "", err
	}

	return net.JoinHostPort(ip, port), nil
}

func (t *Task) RunCommand(ctx context.Context, command []string) (string, string, int, error) {
	containers, err := t.dockerClient.ContainerList(ctx, container.ListOptions{
		Limit: 1,
	})

	if err != nil {
		return "", "", 0, fmt.Errorf("failed to retrieve containers: %w", err)
	}

	if len(containers) != 1 {
		return "", "", 0, fmt.Errorf("could not find container for %s", t.state.Name)
	}

	id := containers[0].ID

	t.logger.Debug("running command", zap.String("id", id), zap.Strings("command", command))

	exec, err := t.dockerClient.ContainerExecCreate(ctx, id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          command,
	})
	if err != nil {
		return "", "", 0, err
	}

	resp, err := t.dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	ticker := time.NewTicker(100 * time.Millisecond)

loop:
	for {
		select {
		case <-ctx.Done():
			return "", "", lastExitCode, ctx.Err()
		case <-ticker.C:
			execInspect, err := t.dockerClient.ContainerExecInspect(ctx, exec.ID)
			if err != nil {
				return "", "", lastExitCode, err
			}

			if execInspect.Running {
				continue
			}

			lastExitCode = execInspect.ExitCode

			break loop
		}
	}

	var stdout, stderr bytes.Buffer

	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", lastExitCode, err
	}

	return stdout.String(), stderr.String(), lastExitCode, nil
}

func (t *Task) RunCommandWhileStopped(ctx context.Context, cmd []string) (string, string, int, error) {
	if err := t.state.Definition.ValidateBasic(); err != nil {
		return "", "", 0, fmt.Errorf("failed to validate task definition: %w", err)
	}

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	t.state.Definition.Entrypoint = []string{"sh", "-c"}
	t.state.Definition.Command = []string{"sleep 36000"}
	t.state.Definition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", t.state.Definition.Name, util.RandomString(5), time.Now().Unix())
	t.state.Definition.Ports = []string{}

	createdContainer, err := t.dockerClient.ContainerCreate(ctx, &container.Config{
		Image:      t.state.Definition.Image.Image,
		Entrypoint: t.state.Definition.Entrypoint,
		Cmd:        t.state.Definition.Command,
		Tty:        false,
		Hostname:   t.state.Definition.Name,
		Labels: map[string]string{
			providerLabelName: t.state.ProviderName,
		},
		Env: convertEnvMapToList(t.state.Definition.Environment),
	}, &container.HostConfig{
		Mounts: []mount.Mount{
			{
				Type:   mount.TypeBind,
				Source: "/docker_volumes",
				Target: t.state.Definition.DataDir,
			},
		},
		NetworkMode: container.NetworkMode("host"),
	}, nil, nil, t.state.Definition.ContainerName)
	if err != nil {
		t.logger.Error("failed to create container", zap.Error(err), zap.String("taskName", t.state.Name))
		return "", "", 0, err
	}

	t.logger.Debug("container created successfully", zap.String("id", createdContainer.ID), zap.String("taskName", t.state.Name))

	defer func() {
		if _, err := t.dockerClient.ContainerInspect(ctx, createdContainer.ID); err != nil && dockerclient.IsErrNotFound(err) {
			// container was auto-removed, no need to remove it again
			return
		}

		if err := t.dockerClient.ContainerRemove(ctx, createdContainer.ID, container.RemoveOptions{Force: true}); err != nil {
			t.logger.Error("failed to remove container", zap.Error(err), zap.String("taskName", t.state.Name), zap.String("id", createdContainer.ID))
		}
	}()

	if err := startContainerWithBlock(ctx, t.dockerClient, createdContainer.ID); err != nil {
		t.logger.Error("failed to start container", zap.Error(err), zap.String("taskName", t.state.Name))
		return "", "", 0, err
	}

	t.logger.Debug("container started successfully", zap.String("id", createdContainer.ID), zap.String("taskName", t.state.Name))

	// wait for container start
	exec, err := t.dockerClient.ContainerExecCreate(ctx, createdContainer.ID, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		t.logger.Error("failed to create exec", zap.Error(err), zap.String("taskName", t.state.Name))
		return "", "", 0, err
	}

	resp, err := t.dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		t.logger.Error("failed to attach to exec", zap.Error(err), zap.String("taskName", t.state.Name))
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	ticker := time.NewTicker(100 * time.Millisecond)

loop:
	for {
		select {
		case <-ctx.Done():
			return "", "", lastExitCode, ctx.Err()
		case <-ticker.C:
			execInspect, err := t.dockerClient.ContainerExecInspect(ctx, exec.ID)
			if err != nil {
				return "", "", lastExitCode, err
			}

			if execInspect.Running {
				continue
			}

			lastExitCode = execInspect.ExitCode

			break loop
		}
	}

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", 0, err
	}

	return stdout.String(), stderr.String(), lastExitCode, err
}

func startContainerWithBlock(ctx context.Context, dockerClient DockerClient, containerID string) error {
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

func pullImage(ctx context.Context, dockerClient DockerClient, logger *zap.Logger, img string) error {
	logger.Info("pulling image", zap.String("image", img))
	resp, err := dockerClient.ImagePull(ctx, img, image.PullOptions{})
	if err != nil {
		return errors.Wrap(err, "failed to pull docker image")
	}

	defer resp.Close()
	// throw away the image pull stdout response
	_, err = io.Copy(io.Discard, resp)
	if err != nil {
		return errors.Wrap(err, "failed to pull docker image")
	}
	return nil
}
