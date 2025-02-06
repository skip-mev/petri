package digitalocean

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/skip-mev/petri/core/v3/util"
	"go.uber.org/zap"
	"tailscale.com/ipn/ipnstate"
)

func (t *Task) launchTailscale(ctx context.Context, authKey string, tags []string) (string, error) {
	prefixedTags := []string{}

	for _, tag := range tags {
		prefixedTags = append(prefixedTags, fmt.Sprintf("tag:%s", tag))
	}

	command := []string{
		"tailscale",
		"up",
		"--authkey",
		authKey,
	}

	if len(prefixedTags) > 0 {
		command = append(command, "--advertise-tags")
		command = append(command, strings.Join(prefixedTags, ","))
	}

	stdout, stderr, exitCode, err := t.runCommandOnDroplet(ctx, command)

	if err != nil {
		return "", err
	}

	if exitCode != 0 {
		return "", fmt.Errorf("tailscale up failed (exit code=%d): %s", exitCode, stderr)
	}

	t.logger.Debug("tailscale up", zap.String("stdout", stdout), zap.String("stderr", stderr))

	var status *ipnstate.Status

	err = util.WaitForCondition(ctx, time.Second*30, time.Second*5, func() (bool, error) {
		status, err = t.getTailscaleStatus(ctx)
		if err != nil {
			return false, err
		}

		return status.BackendState == "Running", nil
	})

	if err != nil {
		return "", err
	}

	return t.getTailscaleIp(ctx)
}

func (t *Task) getTailscaleStatus(ctx context.Context) (*ipnstate.Status, error) {
	stdout, stderr, exitCode, err := t.runCommandOnDroplet(ctx, []string{"tailscale", "status", "--json"})

	if err != nil {
		return nil, err
	}

	if exitCode != 0 {
		return nil, fmt.Errorf("tailscale status failed (exit code=%d): %s", exitCode, stderr)
	}

	t.logger.Debug("tailscale status", zap.String("stdout", stdout), zap.String("stderr", stderr))

	var status ipnstate.Status

	if err := json.Unmarshal([]byte(strings.Trim(stdout, "\n")), &status); err != nil {
		return nil, err
	}

	return &status, nil
}

func (t *Task) getTailscaleIp(ctx context.Context) (string, error) {
	status, err := t.getTailscaleStatus(ctx)

	if err != nil {
		return "", err
	}

	var ip string

	for _, tailscaleIp := range status.TailscaleIPs {
		if !tailscaleIp.Is4() {
			continue
		}

		ip = tailscaleIp.String()

		break
	}

	if ip == "" {
		return "", errors.New("no IPv4 Tailscale address found")
	}

	t.logger.Info("tailscale ips", zap.Any("ips", status.TailscaleIPs))
	return status.TailscaleIPs[0].String(), nil
}
