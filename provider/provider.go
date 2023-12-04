package provider

import (
	"context"
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

	PreStart func(context.Context, *Task) error
	PostStop func(context.Context, *Task) error
}

type Provider interface {
	CreateTask(context.Context, TaskDefinition) (string, error)
	StartTask(context.Context, string) error
	StopTask(context.Context, string) error
	DestroyTask(context.Context, string) error

	GetTaskStatus(context.Context, string) (TaskStatus, error)

	WriteFile(context.Context, string, string, []byte) error
	ReadFile(context.Context, string, string) ([]byte, error)
	DownloadDir(context.Context, string, string, string) error

	GetIP(context.Context, string) (string, error)
	GetHostname(context.Context, string) (string, error)

	RunCommand(context.Context, string, []string) (string, int, error)
}
