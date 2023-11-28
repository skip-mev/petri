package provider

import (
	"context"
)

type Task struct {
	Provider Provider

	ID         string
	Definition TaskDefinition
	Sidecars   []*Task

	PreStart *func(context.Context, *Task) error
	PostStop *func(context.Context, *Task) error
}

type Provider interface {
	CreateTask(context.Context, TaskDefinition) (string, error)
	StartTask(context.Context, string) error
	StopTask(context.Context, string) error
	DestroyTask(context.Context, string) error

	WriteFile(context.Context, string, string, []byte) error
	ReadFile(context.Context, string, string) ([]byte, error)
	DownloadDir(context.Context, string, string, string) error

	GetIP(context.Context, string) (string, error)

	RunCommand(context.Context, string, []string) (string, int, error)
}
