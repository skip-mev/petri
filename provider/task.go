package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/skip-mev/petri/util"
)

func CreateTask(ctx context.Context, provider Provider, definition TaskDefinition) (*Task, error) {
	sidecarTasks := make([]*Task, 0)

	for _, sidecar := range definition.Sidecars {
		if len(sidecar.Sidecars) > 0 {
			return nil, errors.New("sidecar cannot have sidecar")
		}

		id, err := provider.CreateTask(ctx, sidecar)
		if err != nil {
			return nil, err
		}

		sidecarTasks = append(sidecarTasks, &Task{
			Provider:   provider,
			Definition: sidecar,
			ID:         id,
			Sidecars:   make([]*Task, 0),
		})
	}

	id, err := provider.CreateTask(ctx, definition)
	if err != nil {
		return nil, err
	}

	return &Task{
		Definition: definition,
		Provider:   provider,
		ID:         id,
		Sidecars:   sidecarTasks,
	}, nil
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
	return t.Provider.WriteFile(ctx, fmt.Sprintf("%s-data", t.Definition.Name), path, bz)
}

func (t *Task) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return t.Provider.ReadFile(ctx, fmt.Sprintf("%s-data", t.Definition.Name), path)
}

func (t *Task) DownloadDir(ctx context.Context, relPath, localPath string) error {
	return t.Provider.DownloadDir(ctx, fmt.Sprintf("%s-data", t.Definition.Name), relPath, localPath)
}

func (t *Task) GetIP(ctx context.Context) (string, error) {
	return t.Provider.GetIP(ctx, t.ID)
}

func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	return t.Provider.GetExternalAddress(ctx, t.ID, port)
}

func (t *Task) RunCommand(ctx context.Context, command []string) (string, int, error) {
	status, err := t.Provider.GetTaskStatus(ctx, t.ID)
	if err != nil {
		return "", 0, err
	}

	if status == TASK_RUNNING {
		return t.Provider.RunCommand(ctx, t.ID, command)
	}

	modifiedDefinition := t.Definition // todo(zygimantass): make sure this deepcopies the struct instead of modifiying it

	modifiedDefinition.Entrypoint = []string{"sh", "-c"}
	modifiedDefinition.Command = []string{"sleep 36000"}
	modifiedDefinition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", t.Definition.Name, util.RandomString(5), time.Now().Unix())
	modifiedDefinition.Ports = []string{}

	task, err := t.Provider.CreateTask(ctx, modifiedDefinition)
	if err != nil {
		return "", 0, err
	}

	err = t.Provider.StartTask(ctx, task)
	defer t.Provider.DestroyTask(ctx, task)

	if err != nil {
		return "", 0, err
	}

	stdout, exitCode, err := t.Provider.RunCommand(ctx, task, command)
	if err != nil {
		return "", 0, err
	}

	return stdout, exitCode, nil
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
