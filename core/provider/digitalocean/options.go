package digitalocean

import "github.com/skip-mev/petri/core/v3/provider/clients"

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

func WithDigitalOceanClient(client DoClient) func(*Provider) {
	return func(p *Provider) {
		p.doClient = client
	}
}

func WithAdditionalIPs(ips []string) func(*Provider) {
	return func(p *Provider) {
		p.state.UserIPs = append(p.state.UserIPs, ips...)
	}
}

func WithDigitalOceanToken(token string) func(*Provider) {
	return func(p *Provider) {
		doClient := NewGodoClient(token)
		p.doClient = doClient
	}
}
