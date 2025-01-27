package provider

import (
	"context"
	"sync"

	"go.uber.org/zap"
)

// TaskStatus defines the status of a task's underlying workload
type TaskStatus int

// RemoveTaskFunc is a callback function type for removing a task from its provider
type RemoveTaskFunc func(ctx context.Context, taskID string) error

const (
	TASK_STATUS_UNDEFINED TaskStatus = iota
	TASK_RUNNING
	TASK_STOPPED
	TASK_PAUSED
	TASK_RESTARTING
)

// Task is a stateful object that holds the underlying workload's details and tracks the workload's lifecycle
type Task struct {
	Provider Provider

	ID         string
	Definition TaskDefinition

	logger   *zap.Logger
	mu       sync.RWMutex
	PreStart func(context.Context, *Task) error
	PostStop func(context.Context, *Task) error
}

type TaskI interface {
	Start(context.Context) error
	Stop(context.Context) error
	Destroy(context.Context) error

	GetDefinition() TaskDefinition

	GetStatus(context.Context) (TaskStatus, error)

	Modify(context.Context, TaskDefinition) error

	WriteFile(context.Context, string, []byte) error
	ReadFile(context.Context, string) ([]byte, error)
	DownloadDir(context.Context, string, string) error

	GetIP(context.Context) (string, error)
	GetExternalAddress(context.Context, string) (string, error)

	RunCommand(context.Context, []string) (string, string, int, error)
}

type ProviderI interface {
	CreateTask(context.Context, TaskDefinition) (TaskI, error) // should this create a TaskI or the resources behind it?
	SerializeTask(context.Context, TaskI) ([]byte, error)
	DeserializeTask(context.Context, []byte) (TaskI, error)

	Teardown(context.Context) error

	SerializeProvider(context.Context) ([]byte, error)
}

// Provider is the representation of any infrastructure provider that can handle
// running arbitrary Docker workloads
type Provider interface {
	CreateTask(context.Context, *zap.Logger, TaskDefinition) (string, error)
	StartTask(context.Context, string) error
	StopTask(context.Context, string) error
	ModifyTask(context.Context, string, TaskDefinition) error
	DestroyTask(context.Context, string) error

	GetTaskStatus(context.Context, string) (TaskStatus, error)

	WriteFile(context.Context, string, string, []byte) error
	ReadFile(context.Context, string, string) ([]byte, error)
	DownloadDir(context.Context, string, string, string) error

	GetIP(context.Context, string) (string, error)
	GetExternalAddress(context.Context, string, string) (string, error)

	Teardown(context.Context) error

	RunCommand(context.Context, string, []string) (string, string, int, error)
	RunCommandWhileStopped(context.Context, string, TaskDefinition, []string) (string, string, int, error)
}
