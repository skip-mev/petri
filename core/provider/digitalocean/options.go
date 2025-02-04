package digitalocean

import (
	"github.com/skip-mev/petri/core/v3/provider/clients"
	"go.uber.org/zap"
	"tailscale.com/tsnet"
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
		p.dockerClientOverrides = clients
	}
}

func WithAdditionalIPs(ips []string) func(*Provider) {
	return func(p *Provider) {
		p.state.UserIPs = append(p.state.UserIPs, ips...)
	}
}

func WithTailscale(server *tsnet.Server, nodeAuthkey string, tailscaleTags []string) func(*Provider) {
	return func(p *Provider) {
		p.tailscaleServer = server
		p.tailscaleNodeAuthkey = nodeAuthkey
		p.tailscaleTags = tailscaleTags
	}
}
