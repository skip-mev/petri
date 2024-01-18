package loadtest

import (
	"context"
	"cosmossdk.io/math"
	"encoding/base64"
	"encoding/binary"
	"github.com/cometbft/cometbft/test/loadtime/payload"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/cosmosutil/v2"
	petritypes "github.com/skip-mev/petri/types/v2"
	petriutil "github.com/skip-mev/petri/util/v2"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type DefaultClient struct {
	loader           *cosmosutil.InteractingWallet
	chainClient      *cosmosutil.ChainClient
	seq, accNum      uint64
	gasSettings      petritypes.GasSettings
	msgs             []sdk.Msg
	p                *payload.Payload
	batchID          int
	skipSeqIncrement bool
}

func NewDefaultClient(loader *cosmosutil.InteractingWallet, chainClient *cosmosutil.ChainClient, msgs []sdk.Msg, gasSettings petritypes.GasSettings, p *payload.Payload, skipSequenceIncrement bool) *DefaultClient {
	return &DefaultClient{
		loader:           loader,
		chainClient:      chainClient,
		p:                p,
		batchID:          -1,
		skipSeqIncrement: skipSequenceIncrement,
		gasSettings:      gasSettings,
		msgs:             msgs,
	}
}

func (c *DefaultClient) GenerateTx() ([]byte, error) {
	// update padding to be unique (for this client)
	padding := make([]byte, 64)
	binary.BigEndian.PutUint64(padding, c.seq)

	bz, err := proto.Marshal(&payload.Payload{
		Id:          c.p.Id,
		Rate:        c.p.Rate,
		Size:        c.p.Size,
		Connections: c.p.Connections,
		Time:        timestamppb.Now(),
		Padding:     padding,
	})
	if err != nil {
		return nil, err
	}

	memo := base64.StdEncoding.EncodeToString(bz) + "/" + petriutil.RandomString(10)

	tx, err := c.loader.CreateTx(
		context.Background(),
		c.gasSettings.Gas,
		sdk.NewCoins(sdk.NewInt64Coin(c.chainClient.Chain.GetConfig().Denom, math.NewInt(c.gasSettings.PricePerGas).Mul(math.NewInt(c.gasSettings.Gas)).Int64())),
		0,
		memo,
		c.msgs...,
	)

	if err != nil {
		return nil, err
	}

	signedTx, err := c.loader.SignTx(context.Background(), tx, c.accNum, c.seq)

	if err != nil {
		return nil, err
	}

	bz, err = c.chainClient.Chain.GetTxConfig().TxEncoder()(signedTx)

	if err != nil {
		return nil, err
	}

	// increment sequence after successful tx-generation (if necessary)
	if !c.skipSeqIncrement {
		c.seq++
	}

	return bz, nil
}
