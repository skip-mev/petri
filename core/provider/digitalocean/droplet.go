package digitalocean

import (
	"context"
	"fmt"
	"time"

	"github.com/pkg/errors"

	"github.com/digitalocean/godo"
	dockerclient "github.com/docker/docker/client"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/util"

	"strconv"

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

	doConfig := definition.ProviderSpecificConfig.(DigitalOceanTaskConfig)

	if err := doConfig.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("could not cast digitalocean specific config: %w", err)
	}

	imageId, err := strconv.ParseInt(doConfig["image_id"], 10, 64)

	if err != nil {
		return nil, fmt.Errorf("failed to parse image ID: %w", err)
	}

	req := &godo.DropletCreateRequest{
		Name:   fmt.Sprintf("%s-%s", p.petriTag, definition.Name),
		Region: doConfig["region"],
		Size:   doConfig["size"],
		Image: godo.DropletCreateImage{
			ID: int(imageId),
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				Fingerprint: p.sshKeyPair.Fingerprint,
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
	droplet, err := p.getDroplet(ctx, name)

	if err != nil {
		return err
	}

	res, err := p.doClient.Droplets.Delete(ctx, droplet.ID)
	if err != nil {
		return err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return nil
}

func (p *Provider) getDroplet(ctx context.Context, name string) (*godo.Droplet, error) {
	// TODO(Zygimantass): this change assumes that all Petri droplets are unique by name
	// which should be technically true, but there might be edge cases where it's not.
	droplets, res, err := p.doClient.Droplets.ListByName(ctx, name, nil)

	if err != nil {
		return nil, err
	}

	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	if len(droplets) == 0 {
		return nil, fmt.Errorf("could not find droplet")
	}

	return &droplets[0], nil
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
	if _, err := p.getDroplet(ctx, taskName); err != nil {
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

	parsedSSHKey, err := ssh.ParsePrivateKey([]byte(p.sshKeyPair.PrivateKey))
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
