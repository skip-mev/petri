package provider

import (
	"context"
	"go.uber.org/zap"
	"sync"
)

// TaskStatus defines the status of a task's underlying workload
type TaskStatus int

const (
	TASK_STATUS_UNDEFINED TaskStatus = iota
	TASK_RUNNING
	TASK_STOPPED
	TASK_PAUSED
)

// Task is a stateful object that holds the underlying workload's details and tracks the workload's lifecycle
type Task struct {
	Provider Provider

	ID         string
	Definition TaskDefinition
	Sidecars   []*Task

	logger   *zap.Logger
	mu       sync.RWMutex
	PreStart func(context.Context, *Task) error
	PostStop func(context.Context, *Task) error
}

// Provider is the representation of any infrastructure provider that can handle
// running arbitrary Docker workloads
type Provider interface {
	CreateTask(context.Context, *zap.Logger, TaskDefinition) (string, error)
	StartTask(context.Context, string) error
	StopTask(context.Context, string) error
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
