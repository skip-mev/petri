package digitalocean

import (
	"context"
	"github.com/digitalocean/godo"
)

func (p *Provider) createFirewall(ctx context.Context) (*godo.Firewall, error) {
	state := p.GetState()
	req := &godo.FirewallRequest{
		Name: state.PetriTag,
		Tags: []string{state.PetriTag},
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
	}

	return p.doClient.CreateFirewall(ctx, req)
}
