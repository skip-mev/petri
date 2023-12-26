package wallet

import (
	"cosmossdk.io/math"
	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	petritypes "github.com/skip-mev/petri/types"
)

type CosmosWallet struct {
	mnemonic     string
	privKey      cryptotypes.PrivKey
	keyName      string
	bech32Prefix string
}

var _ petritypes.WalletI = &CosmosWallet{}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  math.Int
}

func NewWallet(keyname string, mnemonic string, bech32Prefix string, hdPath *hd.BIP44Params) (*CosmosWallet, error) {
	derivedPrivKey, err := hd.Secp256k1.Derive()(mnemonic, "", hdPath.String())

	if err != nil {
		return nil, err
	}

	privKey := hd.Secp256k1.Generate()(derivedPrivKey)

	return &CosmosWallet{
		mnemonic:     mnemonic,
		privKey:      privKey,
		keyName:      keyname,
		bech32Prefix: bech32Prefix,
	}, nil
}

func NewGeneratedWallet(keyname string, bech32Prefix string, hdPath *hd.BIP44Params) (*CosmosWallet, error) {
	entropy, err := bip39.NewEntropy(128)

	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)

	if err != nil {
		return nil, err
	}

	return NewWallet(keyname, mnemonic, bech32Prefix, hdPath)
}

func (w *CosmosWallet) KeyName() string {
	return w.keyName
}

// Get formatted address, passing in a prefix
func (w *CosmosWallet) FormattedAddress() string {
	return types.MustBech32ifyAddressBytes(w.bech32Prefix, w.privKey.PubKey().Address())
}

func (w *CosmosWallet) Address() []byte {
	return w.privKey.PubKey().Address()
}

func (w *CosmosWallet) FormattedAddressWithPrefix(prefix string) string {
	return types.MustBech32ifyAddressBytes(prefix, w.privKey.PubKey().Address())
}

func (w *CosmosWallet) PublicKey() (cryptotypes.PubKey, error) {
	return w.privKey.PubKey(), nil
}

func (w *CosmosWallet) PrivateKey() (cryptotypes.PrivKey, error) {
	return w.privKey, nil
}

func (w *CosmosWallet) SigningAlgo() string {
	return "secp256k1"
}

func (w *CosmosWallet) Mnemonic() string {
	return w.mnemonic
}
