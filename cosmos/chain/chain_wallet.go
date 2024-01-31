package chain

import (
	"context"
	"fmt"
	sdk "github.com/cosmos/cosmos-sdk/types"
<<<<<<< HEAD
<<<<<<< HEAD:chain/chain_wallet.go
	"github.com/skip-mev/petri/types"
	"github.com/skip-mev/petri/wallet"
=======
	"github.com/skip-mev/petri/cosmos/v2/wallet"
	"github.com/skip-mev/petri/general/v2/types"
>>>>>>> cd1f05b (chore: move everything inside of two packages):cosmos/chain/chain_wallet.go
=======
	"github.com/skip-mev/petri/core/v2/types"
	"github.com/skip-mev/petri/cosmos/v2/wallet"
>>>>>>> d34ae41 (fix: general -> core)
)

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

func (c *Chain) RecoverKey(ctx context.Context, keyName, mnemonic string) error {
	return c.GetFullNode().RecoverKey(ctx, keyName, mnemonic)
}

func (c *Chain) CreateWallet(ctx context.Context, keyName string) (types.WalletI, error) {
	return c.GetFullNode().CreateWallet(ctx, keyName, c.Config.WalletConfig)
}

func (c *Chain) GetAddress(ctx context.Context, keyName string) ([]byte, error) {
	b32Addr, err := c.GetFullNode().KeyBech32(ctx, keyName, "acc")
	if err != nil {
		return nil, err
	}

	return sdk.GetFromBech32(b32Addr, c.Config.Bech32Prefix)
}
