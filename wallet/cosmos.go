package wallet

import (
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
)

type CosmosWallet struct {
	mnemonic     string
	address      []byte
	keyName      string
	bech32Prefix string
}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  math.Int
}

func NewWallet(keyname string, address []byte, mnemonic string, bech32Prefix string) *CosmosWallet {
	return &CosmosWallet{
		mnemonic:     mnemonic,
		address:      address,
		keyName:      keyname,
		bech32Prefix: bech32Prefix,
	}
}

func NewGeneratedWallet(keyname string, bech32Prefix string) (*CosmosWallet, error) {
	entropy, err := bip39.NewEntropy(128)

	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)

	if err != nil {
		return nil, err
	}

	hdPath := hd.CreateHDPath(118, 0, 0)

	derivedPrivKey, err := hd.Secp256k1.Derive()(mnemonic, "", hdPath.String())

	if err != nil {
		return nil, err
	}

	privKey := hd.Secp256k1.Generate()(derivedPrivKey)

	address := privKey.PubKey().Address()

	return NewWallet(keyname, address, mnemonic, bech32Prefix), nil
}

func (w *CosmosWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *CosmosWallet) FormattedAddress() string {
	return types.MustBech32ifyAddressBytes(w.bech32Prefix, w.address)
}

// Get mnemonic, only used for relayer wallets
func (w *CosmosWallet) Mnemonic() string {
	return w.mnemonic
}

// Get Address with chain's prefix
func (w *CosmosWallet) Address() []byte {
	return w.address
}

func (w *CosmosWallet) FormattedAddressWithPrefix(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.address)
}
