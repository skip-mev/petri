package chain

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/cosmos/v2/wallet"
	"github.com/skip-mev/petri/general/v2/types"
)

// BuildWallet creates a wallet in the first available full node's keystore. If a mnemonic is not specificied,
// a random one will be generated

func (c *Chain) BuildWallet(ctx context.Context, keyName, mnemonic string, walletConfig types.WalletConfig) (types.WalletI, error) {
	// if mnemonic is empty, we just generate a wallet
	if mnemonic == "" {
		return c.CreateWallet(ctx, keyName)
	}

	if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
		return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.Config.ChainId, err)
	}

	return wallet.NewWallet(keyName, mnemonic, walletConfig)
}

// RecoverKey recovers a wallet in the first available full node's keystore using a provided mnemonic

func (c *Chain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.GetFullNode().RecoverKey(ctx, keyName, mnemonic)
}

// CreateWallet creates a wallet in the first available full node's keystore using a randomly generated mnemonic

func (c *Chain) CreateWallet(ctx context.Context, keyName string) (types.WalletI, error) {
	return c.GetFullNode().CreateWallet(ctx, keyName, c.Config.WalletConfig)
}

// GetAddress returns a Bech32 formatted address for a given key in the first available full node's keystore

func (c *Chain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.GetFullNode().KeyBech32(ctx, keyName, "acc")
	if err != nil {
		return nil, err
	}

	return sdk.GetFromBech32(b32Addr, c.Config.Bech32Prefix)
}
