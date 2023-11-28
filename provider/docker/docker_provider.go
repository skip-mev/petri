package docker

import (
	"context"
	"sync"

	"github.com/docker/docker/client"
	"github.com/skip-mev/petri/provider"
)

var _ provider.Provider = (*Provider)(nil)

type Provider struct {
	dockerClient *client.Client
	taskStates   sync.Map
	volumes      []string
}

func NewDockerProvider(ctx context.Context, dockerOpts ...client.Opt) (*Provider, error) {
	dockerClient, err := client.NewClientWithOpts(dockerOpts...)
	if err != nil {
		return nil, err
	}

	_, err = dockerClient.Ping(ctx)

	if err != nil {
		return nil, err
	}

	return &Provider{
		dockerClient: dockerClient,
		taskStates:   sync.Map{},
	}, nil
}
