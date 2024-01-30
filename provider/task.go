package provider

import (
	"context"
	"errors"
	"go.uber.org/zap"
	"time"

	"github.com/skip-mev/petri/util"
)

// CreateTask creates a task structure and sets up its underlying workload on a provider, including sidecars if there are any in the definition
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

// Start starts the underlying task's workload including its sidecars if startSidecars is set to true
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

// Stop stops the underlying task's workload including its sidecars if stopSidecars is set to true
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

// WriteFile writes to a file in the task's volume at a relative path
func (t *Task) WriteFile(ctx context.Context, path string, bz []byte) error {
	return t.Provider.WriteFile(ctx, fmt.Sprintf("%s-data", t.Definition.Name), path, bz)
}

// ReadFile returns a file's contents in the task's volume at a relative path
func (t *Task) ReadFile(ctx context.Context, path string) ([]byte, error) {
	return t.Provider.ReadFile(ctx, fmt.Sprintf("%s-data", t.Definition.Name), path)
}

// DownloadDir downloads a directory from the task's volume at path relPath to a local path localPath
func (t *Task) DownloadDir(ctx context.Context, relPath, localPath string) error {
	return t.Provider.DownloadDir(ctx, fmt.Sprintf("%s-data", t.Definition.Name), relPath, localPath)
}

// GetIP returns the task's IP
func (t *Task) GetIP(ctx context.Context) (string, error) {
	return t.Provider.GetIP(ctx, t.ID)
}

<<<<<<< HEAD:provider/task.go
=======
// GetExternalAddress returns the external address for a specific task port in format host:port.
// Providers choose the protocol to return the port for themselves.
>>>>>>> d3ea03a (chore(doc): add godocs everywhere.):core/provider/task.go
func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	return t.Provider.GetExternalAddress(ctx, t.ID, port)
}

// RunCommand executes a shell command on the task's workload, returning stdout/stderr, the exit code and an error if there is one
func (t *Task) RunCommand(ctx context.Context, command []string) (string, string, int, error) {
	status, err := t.Provider.GetTaskStatus(ctx, t.ID)
	if err != nil {
		return "", "", 0, err
	}

	if status == TASK_RUNNING {
		return t.Provider.RunCommand(ctx, t.ID, command)
	}

	modifiedDefinition := t.Definition // todo(zygimantass): make sure this deepcopies the struct instead of modifiying it

	modifiedDefinition.Entrypoint = []string{"sh", "-c"}
	modifiedDefinition.Command = []string{"sleep 36000"}
	modifiedDefinition.ContainerName = fmt.Sprintf("%s-executor-%s-%d", t.Definition.Name, util.RandomString(5), time.Now().Unix())
	modifiedDefinition.Ports = []string{}

	task, err := t.Provider.CreateTask(ctx, t.logger, modifiedDefinition)
	if err != nil {
		return "", "", 0, err
	}

	err = t.Provider.StartTask(ctx, task)
	defer t.Provider.DestroyTask(ctx, task) // nolint:errcheck

	if err != nil {
		return "", "", 0, err
	}

	stdout, stderr, exitCode, err := t.Provider.RunCommand(ctx, task, command)
	if err != nil {
		return "", "", 0, err
	}

	return stdout, stderr, exitCode, nil
}

// GetStatus returns the task's underlying workload's status
func (t *Task) GetStatus(ctx context.Context) (TaskStatus, error) {
	return t.Provider.GetTaskStatus(ctx, t.ID)
}

// Destroy destroys the task's underlying workload, including it's sidecars if destroySidecars is set to true
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

// SetPreStart sets a task's hook function that gets called right before the task's underlying workload is about to be started
func (t *Task) SetPreStart(f func(context.Context, *Task) error) {
	t.PreStart = f
}

// SetPostStop sets a task's hook function that gets called right after the task's underlying workload is stopped
func (t *Task) SetPostStop(f func(context.Context, *Task) error) {
	t.PostStop = f
}
