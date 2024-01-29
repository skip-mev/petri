package node

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/skip-mev/petri/types/v2"
	"github.com/skip-mev/petri/util/v2"
	"github.com/skip-mev/petri/wallet/v2"
	"go.uber.org/zap"
)

func (n *Node) CreateWallet(ctx context.Context, name string, walletConfig types.WalletConfig) (types.WalletI, error) {
	n.logger.Info("creating wallet", zap.String("name", name))

	keyWallet, err := wallet.NewGeneratedWallet(name, walletConfig) // todo: fix this to depend on WalletI

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

	_, _, _, err := n.RunCommand(ctx, command)

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

	stdout, stderr, _, err := n.Task.RunCommand(ctx, command)
	n.logger.Debug("show key", zap.String("name", name), zap.String("stdout", stdout), zap.String("stderr", stderr))

	if err != nil {
		return "", fmt.Errorf("failed to show key %q (stderr=%q): %w", name, stderr, err)
	}

	return util.CleanDockerOutput(stdout), nil
}
