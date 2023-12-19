package chain

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/wallet"
)

func (c *Chain) BuildWallet(ctx context.Context, keyName, mnemonic string) (*wallet.CosmosWallet, error) {
	// if mnemonic is empty, we just generate a wallet
	if mnemonic == "" {
		return c.CreateWallet(ctx, keyName)
	}

	if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
		return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.Config.ChainId, err)
	}

	addrBytes, err := c.GetAddress(ctx, keyName)
	if err != nil {
		return nil, fmt.Errorf("failed to get account address for key %q on chain %s: %w", keyName, c.Config.ChainId, err)
	}

	return wallet.NewWallet(keyName, addrBytes, mnemonic, c.Config.Bech32Prefix), nil
}

func (c *Chain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.GetFullNode().RecoverKey(ctx, keyName, mnemonic)
}

func (c *Chain) CreateWallet(ctx context.Context, keyName string) (*wallet.CosmosWallet, error) {
	return c.GetFullNode().CreateWallet(ctx, keyName)
}

func (c *Chain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.GetFullNode().KeyBech32(ctx, keyName, "acc")
	if err != nil {
		return nil, err
	}

	return types.GetFromBech32(b32Addr, c.Config.Bech32Prefix)
}
