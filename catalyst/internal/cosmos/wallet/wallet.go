package wallet

import (
	"context"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	"github.com/skip-mev/catalyst/internal/types"
)

// InteractingWallet represents a wallet that can interact with the chain
type InteractingWallet struct {
	signer *Signer
	pool   *types.ClientPool
}

// NewInteractingWallet creates a new wallet
func NewInteractingWallet(privKey cryptotypes.PrivKey, bech32Prefix string, pool *types.ClientPool) *InteractingWallet {
	return &InteractingWallet{
		signer: NewSigner(privKey, bech32Prefix),
		pool:   pool,
	}
}

// CreateAndBroadcastTx creates and broadcasts a transaction
func (w *InteractingWallet) CreateAndBroadcastTx(ctx context.Context, gas uint64, fees sdk.Coins, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	client := w.pool.GetClient()

	acc, err := client.GetAccount(ctx, w.signer.FormattedAddress())
	if err != nil {
		return nil, err
	}

	tx, err := w.CreateSignedTx(ctx, client, gas, fees, acc.GetSequence(), acc.GetAccountNumber(), msgs...)
	if err != nil {
		return nil, err
	}

	txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
	if err != nil {
		return nil, err
	}

	txResp, err := client.BroadcastTx(ctx, txBytes)
	if err != nil {
		return nil, err
	}
	return txResp, nil
}

// CreateSignedTx creates and signs a transaction
func (w *InteractingWallet) CreateSignedTx(ctx context.Context, client types.ChainI, gas uint64, fees sdk.Coins, sequence, accountNumber uint64, msgs ...sdk.Msg) (sdk.Tx, error) {
	encodingConfig := client.GetEncodingConfig()

	// Create transaction
	txBuilder := encodingConfig.TxConfig.NewTxBuilder()
	if err := txBuilder.SetMsgs(msgs...); err != nil {
		return nil, err
	}

	txBuilder.SetGasLimit(gas)
	txBuilder.SetFeeAmount(fees)

	// Sign transaction
	signerData := xauthsigning.SignerData{
		ChainID:       client.GetChainID(),
		AccountNumber: accountNumber,
		Sequence:      sequence,
		PubKey:        w.signer.PublicKey(),
	}

	sigV2, err := w.signer.SignTx(signerData, txBuilder, encodingConfig.TxConfig)
	if err != nil {
		return nil, err
	}

	if err := txBuilder.SetSignatures(sigV2); err != nil {
		return nil, err
	}

	return txBuilder.GetTx(), nil
}

// FormattedAddress returns the Bech32 formatted address for the wallet
func (w *InteractingWallet) FormattedAddress() string {
	return w.signer.FormattedAddress()
}

// Address returns the raw address bytes for the wallet
func (w *InteractingWallet) Address() []byte {
	return w.signer.Address()
}

func (w *InteractingWallet) GetClient() types.ChainI {
	return w.pool.GetClient()
}
