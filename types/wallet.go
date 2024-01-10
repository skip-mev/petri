package types

import "github.com/cosmos/cosmos-sdk/crypto/hd"

type WalletConfig struct {
	DerivationFn     hd.DeriveFn
	GenerationFn     hd.GenerateFn
	Bech32Prefix     string
	HDPath           *hd.BIP44Params
	SigningAlgorithm string
}
