package docker

import (
	"context"
	"fmt"
<<<<<<< HEAD
=======
	"github.com/cilium/ipam/service/ipallocator"
	"github.com/skip-mev/petri/core/v2/util"
	"net"
	"sync"

>>>>>>> 7ca1fb6 (feat(docker): statically allocate a network and IP addresses)
	"go.uber.org/zap"
	"sync"

	"github.com/docker/docker/client"

	"github.com/skip-mev/petri/core/v2/provider"
)

var _ provider.Provider = (*Provider)(nil)

const (
	providerLabelName = "petri-provider"
)

type Provider struct {
	logger                 *zap.Logger
	dockerClient           *client.Client
	name                   string
	dockerNetworkID        string
	dockerNetworkName      string
	dockerNetworkAllocator *ipallocator.Range
	networkMu              sync.Mutex
	listeners              map[string]Listeners
	builderImageName       string
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
		dockerClient:     dockerClient,
		listeners:        map[string]Listeners{},
		logger:           logger.Named("docker_provider"),
		name:             providerName,
		builderImageName: "busybox:latest",
	}

	dockerProvider.dockerNetworkName = fmt.Sprintf("petri-network-%s", util.RandomString(5))
	network, err := dockerProvider.createNetwork(ctx, dockerProvider.dockerNetworkName)
	if err != nil {
		return nil, err
	}

	dockerProvider.dockerNetworkID = network.ID

	_, cidrMask, err := net.ParseCIDR(network.IPAM.Config[0].Subnet)

	if err != nil {
		return nil, err
	}

	dockerProvider.dockerNetworkAllocator, err = ipallocator.NewCIDRRange(cidrMask)
	if err != nil {
		return nil, err
	}

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
