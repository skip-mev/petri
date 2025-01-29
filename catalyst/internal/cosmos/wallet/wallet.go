package wallet

import (
	"context"
	"fmt"
	"strings"
	"time"

	sdkclient "github.com/cosmos/cosmos-sdk/client"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	"github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/internal/util"
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
func (w *InteractingWallet) CreateAndBroadcastTx(ctx context.Context, gas uint64, fees sdk.Coins, blocking bool, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	// Get a client from the pool and use it consistently throughout the transaction
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

	// Broadcast the transaction
	txResp, err := client.BroadcastTx(ctx, txBytes)
	if err != nil {
		return nil, err
	}

	if txResp.Code != 0 {
		return txResp, fmt.Errorf("checkTx failed: %s", txResp.RawLog)
	}

	if !blocking {
		return txResp, nil
	}

	// Wait for transaction to be included in block and get full response
	return w.getTxResponse(ctx, client, txResp.TxHash)
}

func (w *InteractingWallet) getTxResponse(ctx context.Context, client types.ChainI, txHash string) (*sdk.TxResponse, error) {
	var txResp *sdk.TxResponse

	cometClient := client.GetCometClient(ctx)

	clientCtx := sdkclient.Context{}.
		WithClient(cometClient).
		WithTxConfig(client.GetEncodingConfig().TxConfig).
		WithInterfaceRegistry(client.GetEncodingConfig().InterfaceRegistry)

	err := util.WaitForCondition(ctx, time.Second*60, time.Millisecond*100, func() (bool, error) {
		res, err := authtx.QueryTx(clientCtx, txHash)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return false, nil
			}
			return false, err
		}

		txResp = res
		return true, nil
	})
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

	chainID := client.GetChainID()
	if chainID == "" {
		return nil, fmt.Errorf("chain ID cannot be empty")
	}

	pubKey := w.signer.PublicKey()
	err := txBuilder.SetSignatures(signing.SignatureV2{
		PubKey: pubKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(encodingConfig.TxConfig.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: sequence,
	})
	if err != nil {
		return nil, err
	}

	// Sign transaction
	signerData := xauthsigning.SignerData{
		ChainID:       chainID,
		AccountNumber: accountNumber,
		Sequence:      sequence,
		PubKey:        pubKey,
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
