package digitalocean

import (
	"context"
	"fmt"
	"github.com/skip-mev/petri/core/v2/provider/clients"
	"time"

	"github.com/pkg/errors"

	"github.com/digitalocean/godo"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/util"

	"strconv"

	_ "embed"
)

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

	state := p.GetState()
	req := &godo.DropletCreateRequest{
		Name:   fmt.Sprintf("%s-%s", state.PetriTag, definition.Name),
		Region: doConfig["region"],
		Size:   doConfig["size"],
		Image: godo.DropletCreateImage{
			ID: int(imageId),
		},
		SSHKeys: []godo.DropletCreateSSHKey{
			{
				Fingerprint: state.SSHKeyPair.Fingerprint,
			},
		},
		Tags: []string{state.PetriTag},
	}

	droplet, err := p.doClient.CreateDroplet(ctx, req)
	if err != nil {
		return nil, err
	}

	start := time.Now()

	err = util.WaitForCondition(ctx, time.Second*600, time.Millisecond*300, func() (bool, error) {
		d, err := p.doClient.GetDroplet(ctx, droplet.ID)
		if err != nil {
			return false, err
		}

		if d.Status != "active" {
			return false, nil
		}

		ip, err := d.PublicIPv4()
		if err != nil {
			return false, err
		}

		if p.dockerClients[ip] == nil {
			dockerClient, err := clients.NewDockerClient(fmt.Sprintf("tcp://%s:%s", ip, dockerPort))
			if err != nil {
				p.logger.Error("failed to create docker client", zap.Error(err))
				return false, err
			}
			p.dockerClients[ip] = dockerClient
		}

		_, err = p.dockerClients[ip].Ping(ctx)
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

	p.logger.Info("droplet is ready after", zap.String("droplet_name", droplet.Name), zap.Duration("startup_time", end.Sub(start)))

	return droplet, nil
}

func (t *Task) deleteDroplet(ctx context.Context) error {
	droplet, err := t.getDroplet(ctx)
	if err != nil {
		return err
	}

	return t.doClient.DeleteDropletByID(ctx, droplet.ID)
}

func (t *Task) getDroplet(ctx context.Context) (*godo.Droplet, error) {
	dropletId, err := strconv.Atoi(t.GetState().ID)
	if err != nil {
		return nil, err
	}
	return t.doClient.GetDroplet(ctx, dropletId)
}

func (t *Task) getDropletSSHClient(ctx context.Context, taskName string) (*ssh.Client, error) {
	if _, err := t.getDroplet(ctx); err != nil {
		return nil, fmt.Errorf("droplet %s does not exist", taskName)
	}

	if t.sshClient != nil {
		status, _, err := t.sshClient.SendRequest("ping", true, []byte("ping"))

		if err == nil && status {
			return t.sshClient, nil
		}
	}

	ip, err := t.GetIP(ctx)
	if err != nil {
		return nil, err
	}

	parsedSSHKey, err := ssh.ParsePrivateKey([]byte(t.GetState().SSHKeyPair.PrivateKey))
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

	return client, nil
}
