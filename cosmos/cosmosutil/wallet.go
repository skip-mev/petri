package cosmosutil

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/client"
	clienttx "github.com/cosmos/cosmos-sdk/client/tx"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	xauthsigning "github.com/cosmos/cosmos-sdk/x/auth/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	petritypes "github.com/skip-mev/petri/general/v2/types"
	"github.com/skip-mev/petri/general/v2/util"
	"strings"
	"time"
)

type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

type InteractingWallet struct {
	petritypes.WalletI

	chain          petritypes.ChainI
	encodingConfig EncodingConfig
}

func NewInteractingWallet(network petritypes.ChainI, wallet petritypes.WalletI, encodingConfig EncodingConfig) *InteractingWallet {
	return &InteractingWallet{
		WalletI:        wallet,
		chain:          network,
		encodingConfig: encodingConfig,
	}
}

func (w *InteractingWallet) CreateAndBroadcastTx(ctx context.Context, blocking bool, gas int64, fees sdk.Coins, timeoutHeight uint64, memo string, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	tx, err := w.CreateSignedTx(ctx, gas, fees, timeoutHeight, memo, msgs...)

	if err != nil {
		return nil, err
	}

	return w.BroadcastTx(ctx, tx, blocking)
}

func (w *InteractingWallet) CreateSignedTx(ctx context.Context, gas int64, fees sdk.Coins, timeoutHeight uint64, memo string, msgs ...sdk.Msg) (sdk.Tx, error) {
	tx, err := w.CreateTx(ctx, gas, fees, timeoutHeight, memo, msgs...)

	if err != nil {
		return nil, err
	}

	acc, err := w.Account(ctx)

	if err != nil {
		return nil, err
	}

	return w.SignTx(ctx, tx, acc.GetAccountNumber(), acc.GetSequence())
}

func (w *InteractingWallet) CreateTx(ctx context.Context, gas int64, fees sdk.Coins, timeoutHeight uint64, memo string, msgs ...sdk.Msg) (sdk.Tx, error) {
	txFactory := w.encodingConfig.TxConfig.NewTxBuilder()

	err := txFactory.SetMsgs(msgs...)

	if err != nil {
		return nil, err
	}

	txFactory.SetGasLimit(uint64(gas))
	txFactory.SetFeeAmount(fees)
	txFactory.SetMemo(memo)
	txFactory.SetTimeoutHeight(timeoutHeight)

	return txFactory.GetTx(), nil
}

func (w *InteractingWallet) SignTx(ctx context.Context, tx sdk.Tx, accNum, sequence uint64) (sdk.Tx, error) {
	privateKey, err := w.PrivateKey()

	if err != nil {
		return nil, err
	}

	publicKey, err := w.PublicKey()

	if err != nil {
		return nil, err
	}

	txFactory, err := w.encodingConfig.TxConfig.WrapTxBuilder(tx)

	if err != nil {
		return nil, err
	}

	err = txFactory.SetSignatures(signing.SignatureV2{
		PubKey: publicKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(w.encodingConfig.TxConfig.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
		Sequence: sequence,
	})

	if err != nil {
		return nil, err
	}

	signerData := xauthsigning.SignerData{
		ChainID:       w.chain.GetConfig().ChainId,
		AccountNumber: accNum,
		Sequence:      sequence,
		PubKey:        publicKey,
	}

	sigV2, err := clienttx.SignWithPrivKey(
		ctx,
		signing.SignMode(w.encodingConfig.TxConfig.SignModeHandler().DefaultMode()),
		signerData,
		txFactory,
		privateKey,
		w.encodingConfig.TxConfig,
		sequence,
	)

	if err != nil {
		return nil, err
	}

	err = txFactory.SetSignatures(sigV2)

	if err != nil {
		return nil, err
	}

	return txFactory.GetTx(), nil
}

func (w *InteractingWallet) BroadcastTx(ctx context.Context, tx sdk.Tx, blocking bool) (*sdk.TxResponse, error) {
	txBytes, err := w.chain.GetTxConfig().TxEncoder()(tx)

	if err != nil {
		return nil, err
	}

	cc, err := w.chain.GetGRPCClient(ctx)

	if err != nil {
		return nil, err
	}

	txClient := txtypes.NewServiceClient(cc)

	if err != nil {
		return nil, err
	}

	checkTxResp, err := txClient.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})

	if err != nil {
		return checkTxResp.TxResponse, err
	}

	if checkTxResp.TxResponse.Code != 0 {
		return checkTxResp.TxResponse, fmt.Errorf("checkTx for the transaction failed with error code: %d", checkTxResp.TxResponse.Code)
	}

	if !blocking {
		return checkTxResp.TxResponse, nil
	}

	if err != nil {
		return nil, err
	}

	txResp, err := w.getTxResponse(ctx, checkTxResp.TxResponse.TxHash)

	if err != nil {
		return nil, err
	}

	return &txResp, nil
}

func (w *InteractingWallet) Account(ctx context.Context) (authtypes.AccountI, error) {
	cc, err := w.chain.GetGRPCClient(ctx)
	if err != nil {
		return nil, err
	}

	defer cc.Close()

	authClient := authtypes.NewQueryClient(cc)

	res, err := authClient.Account(ctx, &authtypes.QueryAccountRequest{
		Address: w.FormattedAddress(),
	})

	if err != nil {
		return nil, err
	}

	var acc authtypes.AccountI

	err = w.encodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)

	if err != nil {
		return nil, err
	}

	return acc, nil
}

func (w *InteractingWallet) getTxResponse(ctx context.Context, txHash string) (sdk.TxResponse, error) {
	var txResp sdk.TxResponse

	cc, err := w.chain.GetTMClient(ctx)

	if err != nil {
		return sdk.TxResponse{}, err
	}

	clientCtx := client.Context{Client: cc, TxConfig: w.chain.GetTxConfig(), InterfaceRegistry: w.chain.GetInterfaceRegistry()}

	err = util.WaitForCondition(ctx, time.Second*60, time.Second*1, func() (bool, error) {
		res, err := authtx.QueryTx(clientCtx, txHash)

		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				return false, nil
			}

			return false, err
		}

		txResp = *res

		return true, nil
	})

	if err != nil {
		return sdk.TxResponse{}, err
	}

	return txResp, nil
}
