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
	petritypes "github.com/skip-mev/petri/types"
	"github.com/skip-mev/petri/util"
	"time"
)

type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

type InteractingWallet struct {
	wallet petritypes.WalletI

	chain          petritypes.ChainI
	encodingConfig EncodingConfig
}

func NewInteractingWallet(network petritypes.ChainI, wallet petritypes.WalletI, encodingConfig EncodingConfig) *InteractingWallet {
	return &InteractingWallet{
		wallet:         wallet,
		chain:          network,
		encodingConfig: encodingConfig,
	}
}

func (w *InteractingWallet) CreateAndBroadcastTx(ctx context.Context, blocking bool, gas uint64, fees sdk.Coins, msgs ...sdk.Msg) (*sdk.TxResponse, error) {
	tx, err := w.CreateTx(ctx, gas, fees, msgs...)

	if err != nil {
		return nil, err
	}

	if !blocking {
		return w.BroadcastTx(ctx, tx)
	}

	checkTxResp, err := w.BroadcastTx(ctx, tx)

	if err != nil {
		return nil, err
	}

	if checkTxResp.Code != 0 {
		return checkTxResp, fmt.Errorf("checkTx for the transaction failed with error code: %d", checkTxResp.Code)
	}

	txResp, err := w.getTxResponse(ctx, checkTxResp.TxHash)

	if err != nil {
		return nil, err
	}

	return &txResp, nil
}

func (w *InteractingWallet) CreateTx(ctx context.Context, gas uint64, fees sdk.Coins, msgs ...sdk.Msg) (sdk.Tx, error) {
	privateKey, err := w.wallet.PrivateKey()

	if err != nil {
		return nil, err
	}

	publicKey, err := w.wallet.PublicKey()

	if err != nil {
		return nil, err
	}

	accInfo, err := w.Account(ctx)

	if err != nil {
		return nil, err
	}

	txFactory := w.encodingConfig.TxConfig.NewTxBuilder()

	err = txFactory.SetMsgs(msgs...)

	if err != nil {
		return nil, err
	}

	txFactory.SetGasLimit(gas)
	txFactory.SetFeeAmount(fees)
	txFactory.SetMemo("")

	err = txFactory.SetSignatures(signing.SignatureV2{
		PubKey: publicKey,
		Data: &signing.SingleSignatureData{
			SignMode:  signing.SignMode(w.encodingConfig.TxConfig.SignModeHandler().DefaultMode()),
			Signature: nil,
		},
	})

	if err != nil {
		return nil, err
	}

	signerData := xauthsigning.SignerData{
		ChainID:       w.chain.GetConfig().ChainId,
		AccountNumber: accInfo.GetAccountNumber(),
		Sequence:      accInfo.GetSequence(),
	}

	sigV2, err := clienttx.SignWithPrivKey(
		ctx,
		signing.SignMode(w.encodingConfig.TxConfig.SignModeHandler().DefaultMode()),
		signerData,
		txFactory,
		privateKey,
		w.encodingConfig.TxConfig,
		accInfo.GetSequence(),
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

func (w *InteractingWallet) BroadcastTx(ctx context.Context, tx sdk.Tx) (*sdk.TxResponse, error) {
	txBytes, err := w.chain.GetTxConfig().TxEncoder()(tx)

	if err != nil {
		return nil, err
	}

	cc, err := w.chain.GetGPRCClient()

	if err != nil {
		return nil, err
	}

	txClient := txtypes.NewServiceClient(cc)

	if err != nil {
		return nil, err
	}

	res, err := txClient.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})

	if err != nil {
		return nil, err
	}

	return res.TxResponse, nil
}

func (w *InteractingWallet) Account(ctx context.Context) (*authtypes.BaseAccount, error) {
	cc, err := w.chain.GetGPRCClient()
	if err != nil {
		return nil, err
	}

	defer cc.Close()

	authClient := authtypes.NewQueryClient(cc)

	res, err := authClient.Account(ctx, &authtypes.QueryAccountRequest{
		Address: w.wallet.FormattedAddress(),
	})

	if err != nil {
		return nil, err
	}

	var acc authtypes.BaseAccount

	err = w.encodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)

	if err != nil {
		return nil, err
	}

	return &acc, nil
}

func (w *InteractingWallet) getTxResponse(ctx context.Context, txHash string) (sdk.TxResponse, error) {
	var txResp sdk.TxResponse

	cc, err := w.chain.GetTMClient()

	if err != nil {
		return sdk.TxResponse{}, err
	}

	clientCtx := client.Context{Client: cc}
	err = util.WaitForCondition(ctx, time.Second*60, time.Second*1, func() (bool, error) {
		res, err := authtx.QueryTx(clientCtx, txHash)

		if err != nil {
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
