package digitalocean

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

func (p *Provider) createFirewall(ctx context.Context, allowedIPs []string) (*godo.Firewall, error) {
	req := &godo.FirewallRequest{
		Name: fmt.Sprintf("%s-firewall", p.state.petriTag),
		Tags: []string{p.state.petriTag},
		OutboundRules: []godo.OutboundRule{
			{
				Protocol:  "tcp",
				PortRange: "0",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0"},
				},
			},
			{
				Protocol:  "udp",
				PortRange: "0",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0"},
				},
			},
			{
				Protocol:  "icmp",
				PortRange: "0",
				Destinations: &godo.Destinations{
					Addresses: []string{"0.0.0.0/0"},
				},
			},
		},
		InboundRules: []godo.InboundRule{
			{
				Protocol:  "tcp",
				PortRange: "0",
				Sources: &godo.Sources{
					Tags:      []string{p.state.petriTag},
					Addresses: allowedIPs,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "0",
				Sources: &godo.Sources{
					Tags:      []string{p.state.petriTag},
					Addresses: allowedIPs,
				},
			},
		},
	}

	firewall, res, err := p.doClient.CreateFirewall(ctx, req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return firewall, nil
}
