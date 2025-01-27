package digitalocean

import (
	"context"
	"fmt"

	"github.com/digitalocean/godo"
)

func (p *Provider) createFirewall(ctx context.Context, allowedIPs []string) (*godo.Firewall, error) {
	state := p.GetState()
	req := &godo.FirewallRequest{
		Name: fmt.Sprintf("%s-firewall", state.PetriTag),
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
		InboundRules: []godo.InboundRule{
			{
				Protocol:  "tcp",
				PortRange: "0",
				Sources: &godo.Sources{
					Tags:      []string{state.PetriTag},
					Addresses: allowedIPs,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "0",
				Sources: &godo.Sources{
					Tags:      []string{state.PetriTag},
					Addresses: allowedIPs,
				},
			},
		},
	}

	return p.doClient.CreateFirewall(ctx, req)
}
