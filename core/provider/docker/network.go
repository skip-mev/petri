package docker

import (
	"context"
	"fmt"
<<<<<<< HEAD
<<<<<<< HEAD
=======
=======
	"net"

>>>>>>> 889bebb (fix: imports)
	"github.com/docker/docker/api/types/network"
>>>>>>> 7ca1fb6 (feat(docker): statically allocate a network and IP addresses)
	"go.uber.org/zap"

	"github.com/docker/docker/api/types"
	"github.com/docker/go-connections/nat"
)

type Listeners []net.Listener

func (p *Provider) createNetwork(ctx context.Context, networkName string) (types.NetworkResource, error) {
	p.logger.Info("creating network", zap.String("name", networkName))
	networkResponse, err := p.dockerClient.NetworkCreate(ctx, networkName, types.NetworkCreate{
		Scope:  "local",
		Driver: "bridge",
		Options: map[string]string{ // https://docs.docker.com/engine/reference/commandline/network_create/#bridge-driver-options
			"com.docker.network.bridge.enable_icc":           "true",
			"com.docker.network.bridge.enable_ip_masquerade": "true",
			"com.docker.network.bridge.host_binding_ipv4":    "0.0.0.0",
			"com.docker.network.driver.mtu":                  "1500",
		},
		Internal:   false,
		Attachable: false,
		Ingress:    false,
		Labels: map[string]string{
			providerLabelName: p.name,
		},
		IPAM: &network.IPAM{
			Driver: "default",
			Config: []network.IPAMConfig{
				{
					Subnet:  "192.192.192.0/24",
					Gateway: "192.192.192.1",
				},
			},
		},
	})
	if err != nil {
		return types.NetworkResource{}, err
	}

	networkInfo, err := p.dockerClient.NetworkInspect(ctx, networkResponse.ID, types.NetworkInspectOptions{})
	if err != nil {
		return types.NetworkResource{}, err
	}

	return networkInfo, nil
}

func (p *Provider) destroyNetwork(ctx context.Context, networkID string) error {
	p.logger.Info("destroying network", zap.String("id", networkID))
	if err := p.dockerClient.NetworkRemove(ctx, networkID); err != nil {
		return err
	}

	return nil
}

func (l Listeners) CloseAll() {
	for _, listener := range l {
		listener.Close()
	}
}

// openListenerOnFreePort opens the next free port
func (p *Provider) openListenerOnFreePort() (*net.TCPListener, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, err
	}

	p.networkMu.Lock()
	defer p.networkMu.Unlock()
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return nil, err
	}

	return l, nil
}

// nextAvailablePort generates a docker PortBinding by finding the next available port.
// The listener will be closed in the case of an error, otherwise it will be left open.
// This allows multiple nextAvailablePort calls to find multiple available ports
// before closing them so they are available for the PortBinding.
func (p *Provider) nextAvailablePort() (nat.PortBinding, *net.TCPListener, error) {
	l, err := p.openListenerOnFreePort()
	if err != nil {
		l.Close()
		return nat.PortBinding{}, nil, err
	}

	return nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: fmt.Sprint(l.Addr().(*net.TCPAddr).Port),
	}, l, nil
}

// GeneratePortBindings will find open ports on the local
// machine and create a PortBinding for every port in the portSet.
func (p *Provider) GeneratePortBindings(portSet nat.PortSet) (nat.PortMap, Listeners, error) {
	m := make(nat.PortMap)
	listeners := make(Listeners, 0, len(portSet))

	for port := range portSet {
		pb, l, err := p.nextAvailablePort()
		if err != nil {
			listeners.CloseAll()
			return nat.PortMap{}, nil, err
		}
		listeners = append(listeners, l)
		m[port] = []nat.PortBinding{pb}
	}

	return m, listeners, nil
}
