package digitalocean

import (
	"context"
<<<<<<< HEAD
	"fmt"
=======
>>>>>>> 8efa963 (feat: enable multiple providers at the same time)
	"github.com/digitalocean/godo"
)

func (p *Provider) createFirewall(ctx context.Context, allowedIPs []string) (*godo.Firewall, error) {
	req := &godo.FirewallRequest{
<<<<<<< HEAD
		Name: fmt.Sprintf("%s-firewall", p.petriTag),
		Tags: []string{p.petriTag},
=======
		Name: state.PetriTag,
		Tags: []string{state.PetriTag},
>>>>>>> 8efa963 (feat: enable multiple providers at the same time)
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
					Tags:      []string{p.petriTag},
					Addresses: allowedIPs,
				},
			},
			{
				Protocol:  "udp",
				PortRange: "0",
				Sources: &godo.Sources{
					Tags:      []string{p.petriTag},
					Addresses: allowedIPs,
				},
			},
		},
	}

	firewall, res, err := p.doClient.Firewalls.Create(ctx, req)

	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return firewall, nil
}
