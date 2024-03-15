package digitalocean

import (
	"fmt"
	"github.com/digitalocean/godo"
	"github.com/puzpuzpuz/xsync/v3"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"
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

	droplets   *xsync.MapOf[string, *godo.Droplet]
	containers *xsync.MapOf[string, string]
	sshClients *xsync.MapOf[string, *ssh.Client]

	firewallID string
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

	digitalOceanProvider := &Provider{
		logger:   logger.Named("digitalocean_provider"),
		name:     providerName,
		doClient: doClient,
		petriTag: fmt.Sprintf("petri-droplet-%s", util.RandomString(5)),

		userIPs: userIPs,

		droplets:   xsync.NewMapOf[string, *godo.Droplet](),
		containers: xsync.NewMapOf[string, string](),
		sshClients: xsync.NewMapOf[string, *ssh.Client](),

		sshPubKey:      sshPubKey,
		sshPrivKey:     sshPrivKey,
		sshFingerprint: sshFingerprint,
	}

	logger.Info("petri tag", zap.String("tag", digitalOceanProvider.petriTag))

	_, err = digitalOceanProvider.createTag(ctx, digitalOceanProvider.petriTag)

	if err != nil {
		return nil, err
	}

	firewall, err := digitalOceanProvider.createFirewall(ctx, userIPs)

	if err != nil {
		return nil, fmt.Errorf("failed to create firewall: %w", err)
	}

	digitalOceanProvider.firewallID = firewall.ID
	_, err = digitalOceanProvider.createSSHKey(ctx, sshPubKey)

	if err != nil {
		return nil, err
	}

	return digitalOceanProvider, nil
}

func (p *Provider) Teardown(ctx context.Context) error {
	p.logger.Info("tearing down DigitalOcean provider")

	if err := p.teardownTasks(ctx); err != nil {
		return err
	}
	if err := p.teardownFirewall(ctx); err != nil {
		return err
	}
	if err := p.teardownSSHKey(ctx); err != nil {
		return err
	}
	if err := p.teardownTag(ctx); err != nil {
		return err
	}

	return nil
}

func (p *Provider) teardownTasks(ctx context.Context) error {
	res, err := p.doClient.Droplets.DeleteByTag(ctx, p.petriTag)

	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownFirewall(ctx context.Context) error {
	res, err := p.doClient.Firewalls.Delete(ctx, p.firewallID)

	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownSSHKey(ctx context.Context) error {
	res, err := p.doClient.Keys.DeleteByFingerprint(ctx, p.sshFingerprint)

	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) teardownTag(ctx context.Context) error {
	res, err := p.doClient.Tags.Delete(ctx, p.petriTag)

	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}
