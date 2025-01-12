package digitalocean

import (
	"context"

	"github.com/digitalocean/godo"
)

// DoClient defines the interface for DigitalOcean API operations used by the provider
type DoClient interface {
	// Droplet operations
	CreateDroplet(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error)
	GetDroplet(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error)
	DeleteDropletByTag(ctx context.Context, tag string) (*godo.Response, error)
	DeleteDropletByID(ctx context.Context, id int) (*godo.Response, error)

	// Firewall operations
	CreateFirewall(ctx context.Context, req *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error)
	DeleteFirewall(ctx context.Context, firewallID string) (*godo.Response, error)

	// SSH Key operations
	CreateKey(ctx context.Context, req *godo.KeyCreateRequest) (*godo.Key, *godo.Response, error)
	DeleteKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Response, error)
	GetKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Key, *godo.Response, error)

	// Tag operations
	CreateTag(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error)
	DeleteTag(ctx context.Context, tag string) (*godo.Response, error)
}

// godoClient implements the DoClient interface using the actual godo.Client
type godoClient struct {
	*godo.Client
}

func NewGodoClient(token string) DoClient {
	return &godoClient{Client: godo.NewFromToken(token)}
}

// Droplet operations
func (c *godoClient) CreateDroplet(ctx context.Context, req *godo.DropletCreateRequest) (*godo.Droplet, *godo.Response, error) {
	return c.Droplets.Create(ctx, req)
}

func (c *godoClient) GetDroplet(ctx context.Context, dropletID int) (*godo.Droplet, *godo.Response, error) {
	return c.Droplets.Get(ctx, dropletID)
}

func (c *godoClient) DeleteDropletByTag(ctx context.Context, tag string) (*godo.Response, error) {
	return c.Droplets.DeleteByTag(ctx, tag)
}

func (c *godoClient) DeleteDropletByID(ctx context.Context, id int) (*godo.Response, error) {
	return c.Droplets.Delete(ctx, id)
}

// Firewall operations
func (c *godoClient) CreateFirewall(ctx context.Context, req *godo.FirewallRequest) (*godo.Firewall, *godo.Response, error) {
	return c.Firewalls.Create(ctx, req)
}

func (c *godoClient) DeleteFirewall(ctx context.Context, firewallID string) (*godo.Response, error) {
	return c.Firewalls.Delete(ctx, firewallID)
}

// SSH Key operations
func (c *godoClient) CreateKey(ctx context.Context, req *godo.KeyCreateRequest) (*godo.Key, *godo.Response, error) {
	return c.Keys.Create(ctx, req)
}

func (c *godoClient) DeleteKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Response, error) {
	return c.Keys.DeleteByFingerprint(ctx, fingerprint)
}

func (c *godoClient) GetKeyByFingerprint(ctx context.Context, fingerprint string) (*godo.Key, *godo.Response, error) {
	return c.Keys.GetByFingerprint(ctx, fingerprint)
}

// Tag operations
func (c *godoClient) CreateTag(ctx context.Context, req *godo.TagCreateRequest) (*godo.Tag, *godo.Response, error) {
	return c.Tags.Create(ctx, req)
}

func (c *godoClient) DeleteTag(ctx context.Context, tag string) (*godo.Response, error) {
	return c.Tags.Delete(ctx, tag)
}
