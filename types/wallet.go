package types

import "github.com/cosmos/cosmos-sdk/crypto/hd"

// WalletConfig is a configuration for a Cosmos SDK type wallet
type WalletConfig struct {
	DerivationFn     hd.DeriveFn     // DerivationFn is the function to derive a seed from a mnemonic
	GenerationFn     hd.GenerateFn   // GenerateFn is the function to derive a private key from a seed
	Bech32Prefix     string          // Bech32Prefix is the default prefix that should be used for formatting addresses
	HDPath           *hd.BIP44Params // HDPath is the default HD path to use for deriving keys
	SigningAlgorithm string          // SigningAlgorithm is the default signing algorithm to use
}
