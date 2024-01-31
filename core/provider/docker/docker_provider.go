package docker

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"sync"

	"github.com/docker/docker/client"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"
)

var _ provider.Provider = (*Provider)(nil)

const (
	providerLabelName = "petri-provider"
)

type Provider struct {
	logger            *zap.Logger
	dockerClient      *client.Client
	name              string
	dockerNetworkID   string
	dockerNetworkName string
	networkMu         sync.RWMutex
	listeners         map[string]Listeners
}

// NewDockerProvider creates a provider that implements the Provider interface for Docker. It uses the default
// Docker client options unless provided with additional options
func NewDockerProvider(ctx context.Context, logger *zap.Logger, providerName string, dockerOpts ...client.Opt) (*Provider, error) {
	dockerClient, err := client.NewClientWithOpts(dockerOpts...)
	if err != nil {
		return nil, err
	}

	_, err = dockerClient.Ping(ctx)

	if err != nil {
		return nil, err
	}

	dockerProvider := &Provider{
		dockerClient: dockerClient,
		listeners:    map[string]Listeners{},
		logger:       logger.Named("docker_provider"),
		name:         providerName,
	}

	dockerProvider.dockerNetworkName = fmt.Sprintf("petri-network-%s", util.RandomString(5))
	networkID, err := dockerProvider.createNetwork(ctx, dockerProvider.dockerNetworkName)
	if err != nil {
		return nil, err
	}

	dockerProvider.dockerNetworkID = networkID

	return dockerProvider, nil
}

func (p *Provider) Teardown(ctx context.Context) error {
	p.logger.Info("tearing down Docker provider")
	if err := p.teardownTasks(ctx); err != nil {
		return err
	}

	if err := p.teardownVolumes(ctx); err != nil {
		return err
	}

	if err := p.destroyNetwork(ctx, p.dockerNetworkID); err != nil {
		return err
	}

	return nil
}
