package apps

import (
	"context"
	"fmt"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/digitalocean"
)

type LoadBalancerDomain struct {
	Domain   string `json:"domain"`
	IP       string `json:"ip"`
	Protocol string `json:"protocol"`
}

type LoadBalancerDefinition struct {
	ProviderSpecificOptions map[string]string
	Domains                 []LoadBalancerDomain
	SSLCertificate          []byte
	SSLKey                  []byte
}

const CaddyHttpDomainTemplate = `%s {
	log
	
	handle {
		reverse_proxy %s {
			header_up Host {http.request.host}
            header_up X-Real-IP {http.request.remote.host}
            header_up X-Forwarded-For {http.request.remote.host}
            header_up X-Forwarded-Port {http.request.port}
            header_up X-Forwarded-Proto {http.request.scheme}
		}
	}

	tls /caddy/cert.pem /caddy/key.pem 
}
`

const CaddyGrpcDomainTemplate = `%s {
	log

	handle {
		reverse_proxy %s {
			header_up Host {http.request.host}
            header_up X-Real-IP {http.request.remote.host}
            header_up X-Forwarded-For {http.request.remote.host}
            header_up X-Forwarded-Port {http.request.port}
            header_up X-Forwarded-Proto {http.request.scheme}
			transport http {
				# Use HTTP/2 cleartext for gRPC
				versions h2c
			}
		}
	}

	tls /caddy/cert.pem /caddy/key.pem 
}
`

// LaunchLoadBalancer only supports the DigitalOcean provider
func LaunchLoadBalancer(ctx context.Context, p *digitalocean.Provider, rootDomain string, definition LoadBalancerDefinition) (provider.TaskI, error) {
	task, err := p.CreateTask(ctx, provider.TaskDefinition{
		Name: "loadbalancer",
		Image: provider.ImageDefinition{
			Image: "caddy:2-alpine",
			UID:   "0",
			GID:   "0",
		},
		Ports:      []string{},
		DataDir:    "/caddy",
		Entrypoint: []string{"caddy", "run", "--config", "/caddy/Caddyfile"},

		ProviderSpecificConfig: definition.ProviderSpecificOptions,
	})

	if err != nil {
		return nil, err
	}

	if err := task.WriteFile(ctx, "cert.pem", definition.SSLCertificate); err != nil {
		return nil, err
	}

	if err := task.WriteFile(ctx, "key.pem", definition.SSLKey); err != nil {
		return nil, err
	}

	caddyConfig := ""

	for _, domain := range definition.Domains {
		fullDomain := fmt.Sprintf("%s.%s", domain.Domain, rootDomain)
		var template string
		if domain.Protocol == "http" {
			template = CaddyHttpDomainTemplate
		} else if domain.Protocol == "grpc" {
			template = CaddyGrpcDomainTemplate
		}

		caddyConfig += fmt.Sprintf(template, fullDomain, domain.IP)
	}

	if err := task.WriteFile(ctx, "Caddyfile", []byte(caddyConfig)); err != nil {
		return nil, err
	}

	if err := task.Start(ctx); err != nil {
		return nil, err
	}

	// hack: until we figure out how to best handle returning addresses in providers
	doTask, ok := task.(*digitalocean.Task)
	if !ok {
		return nil, fmt.Errorf("task is not a DigitalOcean task")
	}

	droplet, err := doTask.GetDroplet(ctx)

	if err != nil {
		return nil, err
	}

	ip, err := droplet.PublicIPv4()

	if err != nil {
		return nil, err
	}

	domains := make(map[string]string, len(definition.Domains))

	for _, domain := range definition.Domains {
		domains[domain.Domain] = ip
	}

	if err := p.CreateDomains(ctx, domains); err != nil {
		return nil, err
	}

	return task, nil
}
