package digitalocean

import (
	"context"
	"fmt"
	"github.com/pkg/errors"
	"time"

	"github.com/digitalocean/godo"
	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/util"

	_ "embed"
)

// nolint
//
//go:embed files/docker-cloud-init.yaml
var dockerCloudInit string

func (p *Provider) CreateDroplet(ctx context.Context, definition provider.TaskDefinition) (*godo.Droplet, error) {
	if err := definition.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate task definition: %w", err)
	}

	doConfig, ok := definition.ProviderSpecificConfig.(DigitalOceanTaskConfig)

	if !ok {
		return nil, fmt.Errorf("could not cast provider specific config to DigitalOceanConfig")
	}
	req := &godo.DropletCreateRequest{
		Name:   fmt.Sprintf("%s-%s", p.petriTag, definition.Name),
		Region: doConfig.Region,
		Size:   doConfig.Size,
		Image: godo.DropletCreateImage{
			ID: doConfig.ImageID,
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				Fingerprint: p.sshFingerprint,
			},
		},
		Tags: []string{p.petriTag},
	}

	droplet, res, err := p.doClient.Droplets.Create(ctx, req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	start := time.Now()

	err = util.WaitForCondition(ctx, time.Second*600, time.Millisecond*300, func() (bool, error) {
		d, _, err := p.doClient.Droplets.Get(ctx, droplet.ID)
		if err != nil {
			return false, err
		}

		if d.Status != "active" {
			return false, nil
		}

		ip, err := d.PublicIPv4()
		if err != nil {
			return false, nil
		}

		dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(fmt.Sprintf("tcp://%s:2375", ip)))
		if err != nil {
			p.logger.Error("failed to create docker client", zap.Error(err))
			return false, err
		}

		_, err = dockerClient.Ping(ctx)
		if err != nil {
			return false, nil
		}

		p.logger.Info("droplet is active", zap.Duration("after", time.Since(start)), zap.String("task", definition.Name))
		droplet = d
		return true, nil
	})
	if err != nil {
		return nil, errors.Wrap(err, "failed to wait for droplet to become active")
	}

	end := time.Now()

	p.logger.Info("droplet %s is ready after %s", zap.String("name", droplet.Name), zap.Duration("took", end.Sub(start)))

	return droplet, nil
}

func (p *Provider) deleteDroplet(ctx context.Context, name string) error {
	cachedDroplet, ok := p.droplets.Load(name)

	if !ok {
		return fmt.Errorf("could not find droplet %s", name)
	}

	res, err := p.doClient.Droplets.Delete(ctx, cachedDroplet.ID)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) getDroplet(ctx context.Context, name string, returnOnCacheHit bool) (*godo.Droplet, error) {
	cachedDroplet, ok := p.droplets.Load(name)
	if !ok {
		return nil, fmt.Errorf("could not find droplet %s", name)
	}

	if ok && returnOnCacheHit {
		return cachedDroplet, nil
	}

	droplet, res, err := p.doClient.Droplets.Get(ctx, cachedDroplet.ID)
	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return droplet, nil
}

func (p *Provider) getDropletDockerClient(ctx context.Context, taskName string) (*dockerclient.Client, error) {
	ip, err := p.GetIP(ctx, taskName)
	if err != nil {
		return nil, err
	}

	dockerClient, err := dockerclient.NewClientWithOpts(dockerclient.WithHost(fmt.Sprintf("tcp://%s:2375", ip)))
	if err != nil {
		return nil, err
	}

	return dockerClient, nil
}

func (p *Provider) getDropletSSHClient(ctx context.Context, taskName string) (*ssh.Client, error) {
	if _, ok := p.droplets.Load(taskName); !ok {
		return nil, fmt.Errorf("droplet %s does not exist", taskName)
	}

	if client, ok := p.sshClients.Load(taskName); ok {
		status, _, err := client.SendRequest("ping", true, []byte("ping"))

		if err == nil && status {
			return client, nil
		}
	}

	ip, err := p.GetIP(ctx, taskName)
	if err != nil {
		return nil, err
	}

	parsedSSHKey, err := ssh.ParsePrivateKey([]byte(p.sshPrivKey))
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User: "root",
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(parsedSSHKey),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	if err != nil {
		return nil, err
	}

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), sshConfig)
	if err != nil {
		return nil, err
	}

	p.sshClients.Store(taskName, client)

	return client, nil
}
