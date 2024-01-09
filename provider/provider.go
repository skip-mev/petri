package provider

import (
	"context"
	"go.uber.org/zap"
)

type TaskStatus int

const (
	TASK_STATUS_UNDEFINED TaskStatus = iota
	TASK_RUNNING
	TASK_STOPPED
	TASK_PAUSED
)

type Task struct {
	Provider Provider

	ID         string
	Definition TaskDefinition
	Sidecars   []*Task

	logger *zap.Logger

	PreStart func(context.Context, *Task) error
	PostStop func(context.Context, *Task) error
}

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

	RunCommand(context.Context, string, []string) (string, int, error)
}
