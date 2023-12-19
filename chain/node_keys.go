package chain

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	cryptocodec "github.com/cosmos/cosmos-sdk/crypto/codec"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/skip-mev/petri/util"
	"github.com/skip-mev/petri/wallet"
	"os"
	"path"
)

func (n *Node) CreateWallet(ctx context.Context, name string) (*wallet.CosmosWallet, error) {
	keyWallet, err := wallet.NewGeneratedWallet(name, n.chain.Config.Bech32Prefix)

	if err != nil {
		return nil, err
	}

	err = n.RecoverKey(ctx, name, keyWallet.Mnemonic())

	if err != nil {
		return nil, err
	}

	return keyWallet, nil
}

func (n *Node) RecoverKey(ctx context.Context, name, mnemonic string) error {
	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s keys add %s --recover --keyring-backend %s --coin-type %s --home %s --output json`, mnemonic, n.chain.Config.BinaryName, name, keyring.BackendTest, n.chain.Config.CoinType, n.chain.Config.HomeDir),
	}

	_, _, err := n.RunCommand(ctx, command)
	return err
}

func (n *Node) KeyBech32(ctx context.Context, name, bech string) (string, error) {
	command := []string{n.chain.Config.BinaryName,
		"keys", "show", name, "-a", "--keyring-backend", keyring.BackendTest, "--home", n.chain.Config.HomeDir,
	}

	if bech != "" {
		command = append(command, "--bech", bech)
	}

	stdout, stderr, err := n.Task.RunCommand(ctx, command)

	if err != nil {
		return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
	}

	return util.CleanDockerOutput(stdout), nil
}

func (n *Node) GetLocalKeyring(ctx context.Context, containerKeyringDir, localDirectory string) (keyring.Keyring, error) {
	localDirectory = path.Join(localDirectory)

	err := n.Task.DownloadDir(ctx, containerKeyringDir, path.Join(localDirectory, "keyring-test"))
	if err != nil {
		return nil, err
	}

	registry := codectypes.NewInterfaceRegistry()
	cryptocodec.RegisterInterfaces(registry)
	cdc := codec.NewProtoCodec(registry)

	return keyring.New("", keyring.BackendTest, localDirectory, os.Stdin, cdc)
}
