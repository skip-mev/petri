package digitalocean

import (
	"context"
	"errors"
	"fmt"
	"github.com/skip-mev/petri/core/v3/provider/clients"
	"go.uber.org/zap"
	"golang.org/x/oauth2/clientcredentials"
	"strings"
	"tailscale.com/client/tailscale"
	"tailscale.com/ipn/ipnstate"
	"tailscale.com/tsnet"
	"time"
)

type TailscaleSettings struct {
	AuthKey     string
	Tags        []string
	Server      clients.TailscaleServer
	LocalClient clients.TailscaleLocalClient
}

func (ts *TailscaleSettings) GetCommand(hostname string) string {
	prefixedTags := make([]string, len(ts.Tags))

	for i, tag := range ts.Tags {
		prefixedTags[i] = fmt.Sprintf("tag:%s", tag)
	}

	command := []string{
		"tailscale",
		"up",
		"--ssh",
		"--authkey",
		fmt.Sprintf("\"%s\"", ts.AuthKey),
		"--hostname",
		hostname,
	}

	if len(prefixedTags) > 0 {
		command = append(command, "--advertise-tags", strings.Join(prefixedTags, ","))
	}

	return strings.Join(command, " ")
}

func (ts *TailscaleSettings) ValidateBasic() error {
	if ts.AuthKey == "" {
		return errors.New("auth key cannot be empty")
	}

	if ts.Server == nil {
		return errors.New("tailscale server cannot be nil")
	}

	if ts.LocalClient == nil {
		return errors.New("tailscale client cannot be nil")
	}

	if len(ts.Tags) == 0 {
		return errors.New("tags cannot be empty")
	}

	return nil
}

func (t *Task) getTailscalePeer(ctx context.Context) (*ipnstate.PeerStatus, error) {
	status, err := t.tailscaleSettings.LocalClient.Status(ctx)

	if err != nil {
		return nil, err
	}

	hostname := t.GetState().TailscaleHostname

	t.logger.Info("getTailscalePeer", zap.String("hostname", hostname), zap.Any("peers", status.Peer))

	for _, peer := range status.Peer {
		if peer.HostName == hostname {
			return peer, nil
		}
	}

	return nil, fmt.Errorf("no Tailscale peer found for hostname: %s", hostname)
}

func (t *Task) getTailscaleIp(ctx context.Context) (string, error) {
	self, err := t.getTailscalePeer(ctx)

	if err != nil {
		return "", err
	}

	for _, tailscaleIp := range self.TailscaleIPs {
		if tailscaleIp.Is4() {
			return tailscaleIp.String(), nil
		}
	}

	return "", errors.New("no IPv4 Tailscale address found")
}

func GenerateTailscaleAuthKey(ctx context.Context, oauthSecret string, tags []string) (string, error) {
	prefixedTags := make([]string, len(tags))

	for i, tag := range tags {
		prefixedTags[i] = fmt.Sprintf("tag:%s", tag)
	}

	baseURL := "https://api.tailscale.com"

	credentials := clientcredentials.Config{
		ClientSecret: oauthSecret,
		TokenURL:     baseURL + "/api/v2/oauth/token",
	}

	tsClient := tailscale.NewClient("-", nil)
	tailscale.I_Acknowledge_This_API_Is_Unstable = true
	tsClient.UserAgent = "tailscale-cli"
	tsClient.HTTPClient = credentials.Client(ctx)
	tsClient.BaseURL = baseURL

	caps := tailscale.KeyCapabilities{
		Devices: tailscale.KeyDeviceCapabilities{
			Create: tailscale.KeyDeviceCreateCapabilities{
				Reusable:      false,
				Ephemeral:     true,
				Preauthorized: true,
				Tags:          prefixedTags,
			},
		},
	}
	authkey, _, err := tsClient.CreateKey(ctx, caps)
	if err != nil {
		return "", err
	}
	return authkey, nil
}

func SetupTailscale(ctx context.Context, serverOauthSecret, nodeAuthKey, hostname string, serverTags, nodeTags []string) (TailscaleSettings, error) {
	authKey, err := GenerateTailscaleAuthKey(ctx, serverOauthSecret, serverTags)
	if err != nil {
		return TailscaleSettings{}, err
	}

	ts := tsnet.Server{
		AuthKey:   authKey,
		Ephemeral: true,
		Hostname:  hostname,
	}

	if err := ts.Start(); err != nil {
		return TailscaleSettings{}, err
	}

	lc, err := ts.LocalClient()
	if err != nil {
		return TailscaleSettings{}, err
	}

	for {
		status, err := lc.Status(ctx)
		if err != nil {
			return TailscaleSettings{}, err
		}

		if status.BackendState == "Running" {
			break
		}

		time.Sleep(1 * time.Second)
	}

	tailscaleSettings := TailscaleSettings{
		AuthKey:     nodeAuthKey,
		Tags:        nodeTags,
		Server:      &ts,
		LocalClient: lc,
	}

	return tailscaleSettings, nil
}
