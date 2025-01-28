package wallet

import (
	"context"

	"github.com/cosmos/cosmos-sdk/client"
	txclient "github.com/cosmos/cosmos-sdk/client/tx"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
)

// Signer handles key management and signing
type Signer struct {
	privKey      cryptotypes.PrivKey
	bech32Prefix string
}

func NewSigner(privKey cryptotypes.PrivKey, bech32Prefix string) *Signer {
	return &Signer{
		privKey:      privKey,
		bech32Prefix: bech32Prefix,
	}
}

func (s *Signer) FormattedAddress() string {
	return sdk.MustBech32ifyAddressBytes(s.bech32Prefix, sdk.AccAddress(s.privKey.PubKey().Address()))
}

func (s *Signer) SignTx(signerData xauthsigning.SignerData, txBuilder client.TxBuilder, txConfig client.TxConfig) (signing.SignatureV2, error) {
	return txclient.SignWithPrivKey(
		context.Background(),
		signing.SignMode(txConfig.SignModeHandler().DefaultMode()),
		signerData,
		txBuilder,
		s.privKey,
		txConfig,
		signerData.Sequence,
	)
}

func (s *Signer) Address() []byte {
	return s.privKey.PubKey().Address()
}

func (s *Signer) PrivateKey() cryptotypes.PrivKey {
	return s.privKey
}

func (s *Signer) PublicKey() cryptotypes.PubKey {
	return s.privKey.PubKey()
}
