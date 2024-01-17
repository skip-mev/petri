package wallet

import (
	"cosmossdk.io/math"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/go-bip39"
	petritypes "github.com/skip-mev/petri/types/v2"
)

type CosmosWallet struct {
	mnemonic     string
	privKey      cryptotypes.PrivKey
	keyName      string
	bech32Prefix string
	signingAlgo  string
}

var _ petritypes.WalletI = &CosmosWallet{}

type WalletAmount struct {
	Address string
	Denom   string
	Amount  math.Int
}

func NewWallet(keyname string, mnemonic string, config petritypes.WalletConfig) (*CosmosWallet, error) {
	derivedPrivKey, err := config.DerivationFn(mnemonic, "", config.HDPath.String())

	if err != nil {
		return nil, err
	}

	privKey := config.GenerationFn(derivedPrivKey)

	return &CosmosWallet{
		mnemonic:     mnemonic,
		privKey:      privKey,
		keyName:      keyname,
		signingAlgo:  config.SigningAlgorithm,
		bech32Prefix: config.Bech32Prefix,
	}, nil
}

func NewGeneratedWallet(keyname string, config petritypes.WalletConfig) (*CosmosWallet, error) {
	entropy, err := bip39.NewEntropy(128)

	if err != nil {
		return nil, err
	}

	mnemonic, err := bip39.NewMnemonic(entropy)

	if err != nil {
		return nil, err
	}

	return NewWallet(keyname, mnemonic, config)
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
	return w.signingAlgo
}

func (w *CosmosWallet) Mnemonic() string {
	return w.mnemonic
}
