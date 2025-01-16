package digitalocean

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/digitalocean/godo"
	"golang.org/x/crypto/ssh"
)

type SSHKeyPair struct {
	PublicKey   string
	PrivateKey  string
	Fingerprint string
}

func ParseSSHKeyPair(privKey []byte) (*SSHKeyPair, error) {
	block, _ := pem.Decode(privKey)

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}

	// generate and write public key
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	var pubKeyBuf strings.Builder
	pubKeyBuf.Write(ssh.MarshalAuthorizedKey(pub))

	return &SSHKeyPair{
		PublicKey:   pubKeyBuf.String(),
		PrivateKey:  string(privKey),
		Fingerprint: ssh.FingerprintLegacyMD5(pub),
	}, nil
}

func MakeSSHKeyPair() (*SSHKeyPair, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 1024)
	if err != nil {
		return nil, err
	}

	// generate and write private key as PEM
	var privKeyBuf bytes.Buffer

	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(&privKeyBuf, privateKeyPEM); err != nil {
		return nil, err
	}

	return ParseSSHKeyPair(privKeyBuf.Bytes())
}

func getUserIPs(ctx context.Context) (ips []string, err error) {
	res, err := http.Get("https://ifconfig.io")
	if err != nil {
		return ips, err
	}

	defer res.Body.Close()

	ifconfigIoIp, err := io.ReadAll(res.Body)
	if err != nil {
		return ips, err
	}

	ips = append(ips, strings.Trim(string(ifconfigIoIp), "\n"))

	res, err = http.Get("https://ipinfo.io/ip")
	if err != nil {
		return ips, err
	}

	defer res.Body.Close()

	ifconfigCoIp, err := io.ReadAll(res.Body)
	if err != nil {
		return ips, err
	}

	ips = append(ips, strings.Trim(string(ifconfigCoIp), "\n"))

	return removeDuplicateStr(ips), nil
}

func (p *Provider) createSSHKey(ctx context.Context, pubKey string) (*godo.Key, error) {
	req := &godo.KeyCreateRequest{PublicKey: pubKey, Name: fmt.Sprintf("%s-key", p.state.petriTag)}

	key, res, err := p.doClient.CreateKey(ctx, req)
	if err != nil {
		return nil, err
	}

	if res.StatusCode > 299 || res.StatusCode < 200 {
		return nil, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	return key, nil
}
