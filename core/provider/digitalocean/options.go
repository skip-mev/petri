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

func WithDockerClients(clients map[string]clients.DockerClient) func(*Provider) {
	return func(p *Provider) {
		p.dockerClientOverrides = clients
	}
}

func WithTelemetry(telemetry TelemetrySettings) func(*Provider) {
	return func(p *Provider) {
		p.telemetrySettings = &telemetry
	}
}
