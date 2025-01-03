package docker

import (
	"bytes"
	"context"
	"fmt"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/pkg/stdcopy"
	"github.com/docker/go-connections/nat"
	"github.com/skip-mev/petri/core/v2/util"
	"sync"
	"github.com/skip-mev/petri/core/v2/provider"
	"go.uber.org/zap"
	"time"
)

type TaskState struct {
	Id         string                  `json:"id"`
	Name       string                  `json:"name"`
	Volume     *VolumeState            `json:"volumes"`
	Definition provider.TaskDefinition `json:"definition"`
	Status     provider.TaskStatus     `json:"status"`
}

type VolumeState struct {
	Name string `json:"name"`
	Size string `json:"size"`
}

type Task struct {
	state    *TaskState
	stateMu  sync.Mutex
	provider *Provider
}

var _ provider.TaskI = (*Task)(nil)

func (t *Task) Start(ctx context.Context) error {
	t.provider.logger.Info("starting task", zap.String("id", t.state.Id))

	err := t.provider.dockerClient.ContainerStart(ctx, t.state.Id, container.StartOptions{})

	if err != nil {
		return err
	}

	if err := t.WaitForStatus(ctx, 1*time.Second, provider.TASK_RUNNING); err != nil {
		return err
	}

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	t.state.Status = provider.TASK_RUNNING

	return nil
}

func (t *Task) Stop(ctx context.Context) error {
	t.provider.logger.Info("stopping task", zap.String("id", t.state.Id))

	err := t.provider.dockerClient.ContainerStop(ctx, t.state.Id, container.StopOptions{})

	if err != nil {
		return err
	}

	if err := t.WaitForStatus(ctx, 1*time.Second, provider.TASK_STOPPED); err != nil {
		return err
	}

	t.stateMu.Lock()
	defer t.stateMu.Unlock()

	t.state.Status = provider.TASK_STOPPED

	return nil
}

func (t *Task) Destroy(ctx context.Context) error {
	t.provider.logger.Info("destroying task", zap.String("id", t.state.Id))

	err := t.provider.dockerClient.ContainerRemove(ctx, t.state.Id, container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	})

	if err != nil {
		return err
	}

	if err := t.provider.removeTask(ctx, t.state.Id); err != nil {
		return err
	}

	return nil
}

func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	t.provider.logger.Debug("getting external address", zap.String("id", t.state.Id))

	dockerContainer, err := t.provider.dockerClient.ContainerInspect(ctx, t.state.Id)

	if err != nil {
		return "", fmt.Errorf("failed to inspect container: %w", err)
	}

	portBindings, ok := dockerContainer.NetworkSettings.Ports[nat.Port(fmt.Sprintf("%s/tcp", port))]

	if !ok || len(portBindings) == 0 {
		return "", fmt.Errorf("port %s not found", port)
	}

	return fmt.Sprintf("0.0.0.0:%s", portBindings[0].HostPort), nil
}

func (t *Task) GetIP(ctx context.Context) (string, error) {
	t.provider.logger.Debug("getting IP", zap.String("id", t.state.Id))

	dockerContainer, err := t.provider.dockerClient.ContainerInspect(ctx, t.state.Id)
	if err != nil {
		return "", err
	}

	ip := dockerContainer.NetworkSettings.Networks[t.provider.state.NetworkName].IPAMConfig.IPv4Address

	return ip, nil
}

func (t *Task) WaitForStatus(ctx context.Context, interval time.Duration, desiredStatus provider.TaskStatus) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			status, err := t.GetStatus(ctx)
			if err != nil {
				return err
			}

			if status == desiredStatus {
				return nil
			}
			time.Sleep(interval)
		}
	}
}

func (t *Task) GetStatus(ctx context.Context) (provider.TaskStatus, error) {
	containerJSON, err := t.provider.dockerClient.ContainerInspect(ctx, t.state.Id)
	if err != nil {
		return provider.TASK_STATUS_UNDEFINED, err
	}

	switch state := containerJSON.State.Status; state {
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

func (t *Task) Modify(ctx context.Context, td provider.TaskDefinition) error {
	panic("unimplemented")
}

func (t *Task) RunCommand(ctx context.Context, cmd []string) (string, string, int, error) {
	status, err := t.GetStatus(ctx)
	if err != nil {
		return "", "", 0, err
	}

	if status != provider.TASK_RUNNING {
		return t.runCommandWhileStopped(ctx, cmd)
	}

	return t.runCommand(ctx, cmd)
}

func (t *Task) runCommand(ctx context.Context, cmd []string) (string, string, int, error) {
	t.provider.logger.Debug("running command", zap.String("id", t.state.Id), zap.Strings("command", cmd))

	exec, err := t.provider.dockerClient.ContainerExecCreate(ctx, t.state.Id, container.ExecOptions{
		AttachStdout: true,
		AttachStderr: true,
		Cmd:          cmd,
	})
	if err != nil {
		if buf, err := t.provider.dockerClient.ContainerLogs(ctx, t.state.Id, container.LogsOptions{
			ShowStdout: true,
			ShowStderr: true,
		}); err == nil {
			defer buf.Close()
			var out bytes.Buffer
			_, _ = out.ReadFrom(buf)
			fmt.Println(out.String())
		}

		return "", "", 0, err
	}

	resp, err := t.provider.dockerClient.ContainerExecAttach(ctx, exec.ID, container.ExecAttachOptions{})
	if err != nil {
		return "", "", 0, err
	}

	defer resp.Close()

	lastExitCode := 0

	//TODO(Zygimantass): talk with Eric about best practices here
	ticker := time.NewTicker(1 * time.Second)

	//TODO(Zygimantass): i hate this
loop:
	for {
		select {
		case <-ctx.Done():
			return "", "", lastExitCode, ctx.Err()
		case <-ticker.C:
			execInspect, err := t.provider.dockerClient.ContainerExecInspect(ctx, exec.ID)
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

	if err != nil {
		t.provider.logger.Error("failed to wait for exec", zap.Error(err), zap.String("id", t.state.Id))
		return "", "", lastExitCode, err
	}

	var stdout, stderr bytes.Buffer
	_, err = stdcopy.StdCopy(&stdout, &stderr, resp.Reader)
	if err != nil {
		return "", "", lastExitCode, err
	}

	return stdout.String(), stderr.String(), lastExitCode, nil
}

func (t *Task) runCommandWhileStopped(ctx context.Context, cmd []string) (string, string, int, error) {
	definition := t.GetState().Definition
	if err := definition.ValidateBasic(); err != nil {
		return "", "", 0, fmt.Errorf("failed to validate task definition: %w", err)
	}

	t.provider.logger.Debug("running command while stopped", zap.String("id", t.state.Id), zap.Strings("command", cmd))

	status, err := t.GetStatus(ctx)
	if err != nil {
		return "", "", 0, err
	}

	if status == provider.TASK_RUNNING {
		return t.RunCommand(ctx, cmd)
	}

	definition.Entrypoint = []string{"/bin/sh", "-c"}
	definition.Command = []string{"sleep 36000"}
	definition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", definition.Name, util.RandomString(5), time.Now().Unix())
	definition.Ports = []string{}

	task, err := t.provider.CreateTask(ctx, definition)
	if err != nil {
		return "", "", 0, err
	}

	err = task.Start(ctx)
	defer task.Destroy(ctx) // nolint:errcheck

	if err != nil {
		return "", "", 0, err
	}

	stdout, stderr, exitCode, err := task.RunCommand(ctx, cmd)
	if err != nil {
		return "", "", 0, err
	}

	return stdout, stderr, exitCode, nil
}

func (t *Task) GetState() TaskState {
	t.stateMu.Lock()
	defer t.stateMu.Unlock()
	return *t.state
}

func (t *Task) ensureTask(ctx context.Context) error {
	state := t.GetState()

	dockerContainer, err := t.provider.dockerClient.ContainerInspect(ctx, state.Id)
	if err != nil {
		return fmt.Errorf("failed to inspect container: %w", err)
	}

	actualStatus, err := t.GetStatus(ctx)

	if err != nil {
		return fmt.Errorf("failed to get task status: %w", err)
	}

	if actualStatus != state.Status {
		return fmt.Errorf("task status mismatch, expected: %d, got: %d", state.Status, actualStatus)
	}

	// ref: https://github.com/moby/moby/issues/6705
	if dockerContainer.Name != fmt.Sprintf("/%s", state.Name) {
		return fmt.Errorf("task name mismatch, expected: %s, got: %s", state.Name, dockerContainer.Name)
	}

	if err := t.ensureVolume(ctx); err != nil {
		return err
	}

	return nil
}

func (t *Task) ensureVolume(ctx context.Context) error {
	if t.state.Volume == nil {
		return nil
	}

	volume, err := t.provider.dockerClient.VolumeInspect(ctx, t.state.Volume.Name)
	if err != nil {
		return fmt.Errorf("failed to inspect volume: %w", err)
	}

	if volume.Name != t.state.Volume.Name {
		return fmt.Errorf("volume name mismatch, expected: %s, got: %s", t.state.Volume.Name, volume.Name)
	}

	return nil
}

func (t *Task) GetDefinition() provider.TaskDefinition {
	return t.GetState().Definition
}
