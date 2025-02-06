package digitalocean

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"

	"github.com/digitalocean/godo"
	"go.uber.org/zap"
	"golang.org/x/crypto/ssh"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/clients"
	"github.com/skip-mev/petri/core/v3/util"

	"strconv"

	_ "embed"
)

func (p *Provider) CreateDroplet(ctx context.Context, definition provider.TaskDefinition) (*godo.Droplet, error) {
	if err := definition.ValidateBasic(); err != nil {
		return nil, fmt.Errorf("failed to validate task definition: %w", err)
	}

	var doConfig DigitalOceanTaskConfig = definition.ProviderSpecificConfig

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

	return droplet, nil
}

func (t *Task) waitForSSHClient(ctx context.Context) error {
	start := time.Now()

	err := util.WaitForCondition(ctx, time.Second*600, time.Millisecond*300, func() (bool, error) {
		_, err := t.getDropletSSHClient(ctx)

		if err != nil {
			if strings.Contains(err.Error(), "connection refused") {
				t.logger.Debug("connection refused", zap.String("task", t.GetState().Name))
				return false, nil
			}
			return false, err
		}

		return true, nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to wait for ssh in droplet to become active")
	}

	end := time.Now()

	t.logger.Info("droplet's ssh daemon is ready after", zap.String("name", t.GetState().Name), zap.Duration("startup_time", end.Sub(start)))

	return nil
}

func (t *Task) waitForDockerStart(ctx context.Context) error {
	start := time.Now()

	err := util.WaitForCondition(ctx, time.Second*600, time.Millisecond*300, func() (bool, error) {
		d, err := t.getDroplet(ctx)
		if err != nil {
			return false, err
		}

		if d.Status != "active" {
			t.logger.Debug("droplet is not active", zap.String("status", d.Status), zap.String("task", t.GetState().Name))
			return false, nil
		}

		ip, err := t.GetIP(ctx)

		if err != nil || ip == "" {
			t.logger.Debug("task does not have ipv4 address", zap.Error(err), zap.String("task", t.GetState().Name))
			return false, err
		}

		if t.dockerClient == nil {
			t.dockerClient, err = clients.NewDockerClient(ip, t.getDialFunc())
			if err != nil {
				t.logger.Error("failed to create docker client", zap.Error(err))
				return false, err
			}
		}

		_, err = t.dockerClient.Ping(ctx)
		if err != nil {
			t.logger.Debug("docker client is not ready", zap.Error(err), zap.String("task", t.GetState().Name))
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		return errors.Wrap(err, "failed to wait for docker in droplet to become active")
	}

	end := time.Now()

	t.logger.Info("droplet's docker daemon is ready after", zap.String("name", t.GetState().Name), zap.Duration("startup_time", end.Sub(start)))

	return nil
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

func (t *Task) getDropletSSHClient(ctx context.Context) (*ssh.Client, error) {
	taskName := t.GetState().Name

	if _, err := t.getDroplet(ctx); err != nil {
		return nil, fmt.Errorf("droplet %s does not exist", taskName)
	}

	if t.sshClient != nil {
		status, _, err := t.sshClient.SendRequest("ping", true, []byte("ping"))

		if err == nil && status {
			return t.sshClient, nil
		}
	}

	d, err := t.getDroplet(ctx)
	if err != nil {
		return nil, err
	}

	ip, err := d.PublicIPv4()

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

	client, err := ssh.Dial("tcp", fmt.Sprintf("%s:22", ip), sshConfig)
	if err != nil {
		return nil, err
	}

	return client, nil
}
