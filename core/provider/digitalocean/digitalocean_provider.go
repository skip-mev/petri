package digitalocean

import (
	"fmt"
	"github.com/digitalocean/godo"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"
	"go.uber.org/zap"
)

import (
	"context"
)

var _ provider.Provider = (*Provider)(nil)

const (
	providerLabelName = "petri-provider"
)

type Provider struct {
	logger   *zap.Logger
	name     string
	doClient *godo.Client
	petriTag string

	userIPs []string

	sshPubKey, sshPrivKey, sshFingerprint string

	droplets   map[string]*godo.Droplet
	containers map[string]string
}

// NewDigitalOceanProvider creates a provider that implements the Provider interface for DigitalOcean.
// Token is the DigitalOcean API token
func NewDigitalOceanProvider(ctx context.Context, logger *zap.Logger, providerName string, token string) (*Provider, error) {
	doClient := godo.NewFromToken(token)

	sshPubKey, sshPrivKey, sshFingerprint, err := makeSSHKeyPair()

	if err != nil {
		return nil, err
	}

	userIPs, err := getUserIPs(ctx)

	if err != nil {
		return nil, err
	}

	userIPs = append(userIPs, "77.175.129.184")

	digitalOceanProvider := &Provider{
		logger:   logger.Named("digitalocean_provider"),
		name:     providerName,
		doClient: doClient,
		petriTag: fmt.Sprintf("petri-droplet-%s", util.RandomString(5)),

		userIPs: userIPs,

		droplets:   map[string]*godo.Droplet{},
		containers: map[string]string{},

		sshPubKey:      sshPubKey,
		sshPrivKey:     sshPrivKey,
		sshFingerprint: sshFingerprint,
	}

	_, err = digitalOceanProvider.createTag(ctx, digitalOceanProvider.petriTag)

	if err != nil {
		return nil, err
	}

	_, err = digitalOceanProvider.createFirewall(ctx, userIPs)

	if err != nil {
		return nil, err
	}

	_, err = digitalOceanProvider.createSSHKey(ctx, sshPubKey)

	if err != nil {
		return nil, err
	}

	return digitalOceanProvider, nil
}

func (p *Provider) Teardown(ctx context.Context) error {
	p.logger.Info("tearing down Docker provider")

	if err := p.teardownTasks(ctx); err != nil {
		return err
	}

	return nil
}

// TODO: Implement teardownTasks
func (p *Provider) teardownTasks(ctx context.Context) error {
	return nil
}
