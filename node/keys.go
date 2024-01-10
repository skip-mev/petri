package node

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	petritypes "github.com/skip-mev/petri/types"
	"github.com/skip-mev/petri/util"
	"github.com/skip-mev/petri/wallet"
	"go.uber.org/zap"
)

func (n *Node) CreateWallet(ctx context.Context, name string) (petritypes.WalletI, error) {
	n.logger.Info("creating wallet", zap.String("name", name))
	coinType, err := hd.NewParamsFromPath(n.chain.GetConfig().HDPath)

	if err != nil {
		return nil, err
	}

	keyWallet, err := wallet.NewGeneratedWallet(name, n.chain.GetConfig().Bech32Prefix, coinType) // todo: fix this to depend on WalletI

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
	n.logger.Info("recovering wallet", zap.String("name", name), zap.String("mnemonic", mnemonic))
	chainConfig := n.chain.GetConfig()

	command := []string{
		"sh",
		"-c",
		fmt.Sprintf(`echo %q | %s keys add %s --recover --keyring-backend %s --coin-type %s --home %s --output json`, mnemonic, chainConfig.BinaryName, name, keyring.BackendTest, chainConfig.CoinType, chainConfig.HomeDir),
	}

	_, _, err := n.RunCommand(ctx, command)

	return err
}

func (n *Node) KeyBech32(ctx context.Context, name, bech string) (string, error) {
	chainConfig := n.chain.GetConfig()

	command := []string{chainConfig.BinaryName,
		"keys", "show", name, "-a", "--keyring-backend", keyring.BackendTest, "--home", chainConfig.HomeDir,
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
