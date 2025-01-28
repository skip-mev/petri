package digitalocean

import (
	"github.com/skip-mev/petri/core/v3/provider/clients"
	"go.uber.org/zap"
)

func WithLogger(logger *zap.Logger) func(*Provider) {
	return func(p *Provider) {
		p.logger = logger
	}
}

func WithSSHKeyPair(pair SSHKeyPair) func(*Provider) {
	return func(p *Provider) {
		p.state.SSHKeyPair = &pair
	}
}

func WithDockerClients(clients map[string]clients.DockerClient) func(*Provider) {
	return func(p *Provider) {
		p.dockerClients = clients
	}
}

func WithAdditionalIPs(ips []string) func(*Provider) {
	return func(p *Provider) {
		p.state.UserIPs = append(p.state.UserIPs, ips...)
	}
}
