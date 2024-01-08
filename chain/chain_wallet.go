package chain

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/types"
	petritypes "github.com/skip-mev/petri/types"
	"github.com/skip-mev/petri/wallet"
)

func (c *Chain) BuildWallet(ctx context.Context, keyName, mnemonic string) (petritypes.WalletI, error) {
	// if mnemonic is empty, we just generate a wallet
	if mnemonic == "" {
		return c.CreateWallet(ctx, keyName)
	}

	if err := c.RecoverKey(ctx, keyName, mnemonic); err != nil {
		return nil, fmt.Errorf("failed to recover key with name %q on chain %s: %w", keyName, c.Config.ChainId, err)
	}

	coinType, err := hd.NewParamsFromPath(c.Config.HDPath)

	if err != nil {
		return nil, err
	}

	return wallet.NewWallet(keyName, mnemonic, c.Config.Bech32Prefix, coinType)
}

func (c *Chain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.GetFullNode().RecoverKey(ctx, keyName, mnemonic)
}

func (c *Chain) CreateWallet(ctx context.Context, keyName string) (petritypes.WalletI, error) {
	return c.GetFullNode().CreateWallet(ctx, keyName)
}

func (c *Chain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.GetFullNode().KeyBech32(ctx, keyName, "acc")
	if err != nil {
		return nil, err
	}

	return types.GetFromBech32(b32Addr, c.Config.Bech32Prefix)
}
