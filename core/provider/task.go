package provider

import (
	"context"
	"fmt"
	"golang.org/x/sync/errgroup"
	"sync"

	"go.uber.org/zap"
)

// CreateTask creates a task structure and sets up its underlying workload on a provider, including sidecars if there are any in the definition
func CreateTask(ctx context.Context, logger *zap.Logger, provider Provider, definition TaskDefinition) (*Task, error) {
	if err := definition.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate task definition: %w", err)
	}

	task := &Task{
		Provider:   provider,
		Definition: definition,
		mu:         sync.RWMutex{},
		logger:     logger.Named("task"),
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	sidecarTasks := make([]*Task, 0)

	var eg errgroup.Group

	eg.Go(func() error {
		id, err := provider.CreateTask(ctx, logger, definition)
		if err != nil {
			return err
		}
		
		task.ID = id
		
		return nil
	})
	
	if err := eg.Wait(); err != nil {
		return nil, err
	}
	
	task.Sidecars = sidecarTasks
	return task, nil
}

// Start starts the underlying task's workload including its sidecars if startSidecars is set to true.
// This method does not take a lock on the provider, hence 2 threads may simultaneously call Start on the same task,
// this is not thread-safe: PLEASE DON'T DO THAT.
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
	t.mu.Lock()
	defer t.mu.Unlock()

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
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.Provider.WriteFile(ctx, t.ID, path, bz)
}

// ReadFile returns a file's contents in the task's volume at a relative path
func (t *Task) ReadFile(ctx context.Context, path string) ([]byte, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Provider.ReadFile(ctx, t.ID, path)
}

// DownloadDir downloads a directory from the task's volume at path relPath to a local path localPath
func (t *Task) DownloadDir(ctx context.Context, relPath, localPath string) error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Provider.DownloadDir(ctx, t.ID, relPath, localPath)
}

// GetIP returns the task's IP
func (t *Task) GetIP(ctx context.Context) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Provider.GetIP(ctx, t.ID)
}

// GetExternalAddress returns the external address for a specific task port in format host:port.
// Providers choose the protocol to return the port for themselves.
func (t *Task) GetExternalAddress(ctx context.Context, port string) (string, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Provider.GetExternalAddress(ctx, t.ID, port)
}

// RunCommand executes a shell command on the task's workload, returning stdout/stderr, the exit code and an error if there is one
func (t *Task) RunCommand(ctx context.Context, command []string) (string, string, int, error) {
	status, err := t.Provider.GetTaskStatus(ctx, t.ID)
	if err != nil {
		t.logger.Error("failed to get task status", zap.Error(err), zap.Any("definition", t.Definition))
		return "", "", 0, err
	}

	if status == TASK_RUNNING {
		t.mu.Lock()
		defer t.mu.Unlock()
		t.logger.Info("running command", zap.Strings("command", command), zap.String("status", "running"))
		return t.Provider.RunCommand(ctx, t.ID, command)
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	t.logger.Info("running command", zap.Strings("command", command), zap.String("status", "not running"))
	return t.Provider.RunCommandWhileStopped(ctx, t.ID, t.Definition, command)
}

// GetStatus returns the task's underlying workload's status
func (t *Task) GetStatus(ctx context.Context) (TaskStatus, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.Provider.GetTaskStatus(ctx, t.ID)
}

// Destroy destroys the task's underlying workload, including it's sidecars if destroySidecars is set to true
func (t *Task) Destroy(ctx context.Context, destroySidecars bool) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if destroySidecars {
	}

	err := t.Provider.DestroyTask(ctx, t.ID)
	if err != nil {
		return err
	}

	return nil
}

// SetPreStart sets a task's hook function that gets called right before the task's underlying workload is about to be started
func (t *Task) SetPreStart(f func(context.Context, *Task) error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.PreStart = f
}

// SetPostStop sets a task's hook function that gets called right after the task's underlying workload is stopped
func (t *Task) SetPostStop(f func(context.Context, *Task) error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.PostStop = f
}
