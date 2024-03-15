package digitalocean

import (
	"time"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"github.com/digitalocean/godo"
	"golang.org/x/crypto/ssh"
	"io"
	"net"
	"net/http"
	"strings"
)

func makeSSHKeyPair() (string, string, string, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return "", "", "", err
	}

	// generate and write private key as PEM
	var privKeyBuf strings.Builder

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return "", "", "", err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return "", "", "", err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return pubKeyBuf.String(), privKeyBuf.String(), ssh.FingerprintLegacyMD5(pub), nil
}

func getUserIPs(ctx context.Context) (ips []string, err error) {
	ipv4s, err := getUserIPForNetwork(ctx, TCP4)

	errs := make([]error, 0)

	if err == nil {
		ips = append(ips, ipv4s...)
	} else {
		errs = append(errs, err)
	}

	ipv6s, err := getUserIPForNetwork(ctx, TCP6)
	if err == nil {
		ips = append(ips, ipv6s...)
	} else {
		errs = append(errs, err)
	}

	if len(ips) == 0 {
		err = fmt.Errorf("failed to get user IP: %v", errs)
	}

	return ips, nil
}

type Network string

const (
	TCP6 Network = "tcp6"
	TCP4 Network = "tcp4"
)

func getUserIPForNetwork(ctx context.Context, network Network) ([]string, error) {
	client := &http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				dialer := net.Dialer{}
				return dialer.DialContext(ctx, string(network), "https://ifconfig.io")
			},
		},
		Timeout: 10 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", "https://ifconfig.io", nil)
	if err != nil {
		return nil, err
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	ips := make([]string, 0)
	ips = append(ips, strings.Trim(string(body), "\n"))

	return ips, nil
}


func (p *Provider) createSSHKey(ctx context.Context, pubKey string) (*godo.Key, error) {
	req := &godo.KeyCreateRequest{PublicKey: pubKey, Name: fmt.Sprintf("%s-key", p.petriTag)}

	key, res, err := p.doClient.Keys.Create(ctx, req)

	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return key, nil
}
