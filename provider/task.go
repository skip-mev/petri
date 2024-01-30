package provider

import (
	"context"
	"errors"
	"fmt"
	"go.uber.org/zap"
)

func CreateTask(ctx context.Context, logger *zap.Logger, provider Provider, definition TaskDefinition) (*Task, error) {
	task := &Task{
		Provider:   provider,
		Definition: definition,
		logger:     logger.Named("task"),
	}

	sidecarTasks := make([]*Task, 0)

	for _, sidecar := range definition.Sidecars {
		if len(sidecar.Sidecars) > 0 {
			return nil, errors.New("sidecar cannot have sidecar")
		}

		id, err := provider.CreateTask(ctx, task.logger, sidecar)
		if err != nil {
			return nil, err
		}

		sidecarTasks = append(sidecarTasks, &Task{
			Provider:   provider,
			Definition: sidecar,
			ID:         id,
			Sidecars:   make([]*Task, 0),
			logger:     task.logger,
		})
	}

	task.Sidecars = sidecarTasks

	id, err := provider.CreateTask(ctx, logger, definition)
	if err != nil {
		return nil, err
	}

	task.ID = id

	return task, nil
}

func (t *Task) Start(ctx context.Context, startSidecars bool) error {
	if startSidecars {
		for _, sidecar := range t.Sidecars {
			err := sidecar.Start(ctx, startSidecars)
			if err != nil {
				return err
			}
		}
	}

	if t.PreStart != nil {
		err := t.PreStart(ctx, t)
		if err != nil {
			return err
		}
	}

	err := t.Provider.StartTask(ctx, t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) Stop(ctx context.Context, stopSidecars bool) error {
	if stopSidecars {
		for _, sidecar := range t.Sidecars {
			err := sidecar.Stop(ctx, stopSidecars)
			if err != nil {
				return err
			}
		}
	}

	err := t.Provider.StopTask(ctx, t.ID)

	if t.PostStop != nil {
		err := t.PostStop(ctx, t)
		if err != nil {
			return err
		}
	}

	if err != nil {
		return err
	}

	return nil
}

func (t *Task) WriteFile(ctx context.Context, path string, bz []byte) error {
	return t.Provider.WriteFile(ctx, t.ID, path, bz)
}

func (t *Task) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return t.Provider.ReadFile(ctx, t.ID, path)
}

func (t *Task) DownloadDir(ctx context.Context, relPath, localPath string) error {
	return t.Provider.DownloadDir(ctx, t.ID, relPath, localPath)
}

func (t *Task) GetIP(ctx context.Context) (string, error) {
	return t.Provider.GetIP(ctx, t.ID)
}

// GetExternalAddress returns the external address for a specific task port in format host:port.
// Providers choose the protocol to return the port for themselves.

func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	return t.Provider.GetExternalAddress(ctx, t.ID, port)
}

func (t *Task) RunCommand(ctx context.Context, command []string) (string, string, int, error) {
	status, err := t.Provider.GetTaskStatus(ctx, t.ID)
	if err != nil {
		return "", "", 0, err
	}

	if status == TASK_RUNNING {
		return t.Provider.RunCommand(ctx, t.ID, command)
	}

	return t.Provider.RunCommandWhileStopped(ctx, t.ID, t.Definition, command)
}

func (t *Task) GetStatus(ctx context.Context) (TaskStatus, error) {
	return t.Provider.GetTaskStatus(ctx, t.ID)
}

func (t *Task) Destroy(ctx context.Context, destroySidecars bool) error {
	if destroySidecars {
		for _, sidecar := range t.Sidecars {
			err := sidecar.Destroy(ctx, destroySidecars)
			if err != nil {
				return err
			}
		}
	}

	err := t.Provider.DestroyTask(ctx, t.ID)
	if err != nil {
		return err
	}

	return nil
}

func (t *Task) SetPreStart(f func(context.Context, *Task) error) {
	t.PreStart = f
}

func (t *Task) SetPostStop(f func(context.Context, *Task) error) {
	t.PostStop = f
}
