package loadtest

import (
	"context"
	"github.com/cometbft/cometbft/test/loadtime/payload"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/module"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"
	"github.com/skip-mev/petri/cosmosutil"
	petritypes "github.com/skip-mev/petri/types"
	"github.com/skip-mev/petri/wallet"
)

type GenerateMsgs func(senderAddress []byte) ([]sdk.Msg, petritypes.GasSettings, error)

type DefaultClientFactory struct {
	chain            petritypes.ChainI
	chainClient      *cosmosutil.ChainClient
	seeder           *cosmosutil.InteractingWallet
	walletConfig     petritypes.WalletConfig
	encodingConfig   cosmosutil.EncodingConfig
	amtToSend        int64
	id               []byte
	skipSeqIncrement bool
	msgGenerator     GenerateMsgs
}

type ClientFactoryConfig struct {
	Chain                 petritypes.ChainI
	Seeder                *cosmosutil.InteractingWallet
	WalletConfig          petritypes.WalletConfig
	AmountToSend          int64
	SkipSequenceIncrement bool
	EncodingConfig        cosmosutil.EncodingConfig
	MsgGenerator          GenerateMsgs
}

func NewDefaultClientFactory(cfg ClientFactoryConfig, mbm module.BasicManager) (*DefaultClientFactory, error) {
	return &DefaultClientFactory{
		chain:            cfg.Chain,
		chainClient:      &cosmosutil.ChainClient{Chain: cfg.Chain},
		seeder:           cfg.Seeder,
		amtToSend:        cfg.AmountToSend,
		skipSeqIncrement: cfg.SkipSequenceIncrement,
		walletConfig:     cfg.WalletConfig,
		encodingConfig:   cfg.EncodingConfig,
		msgGenerator:     cfg.MsgGenerator,
	}, nil
}

func (f *DefaultClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	// create a new private-key
	loaderWallet, err := wallet.NewGeneratedWallet("seed", f.walletConfig)
	if err != nil {
		return nil, err
	}

	interactingLoaderWallet := cosmosutil.NewInteractingWallet(f.chain, loaderWallet, f.encodingConfig)

	_, err = f.chainClient.BankSend(context.Background(), *f.seeder, interactingLoaderWallet.Address(), sdk.NewCoins(sdk.NewInt64Coin(f.chain.GetConfig().Denom, f.amtToSend)), petritypes.GasSettings{
		Gas:         200000,
		GasDenom:    f.chain.GetConfig().Denom,
		PricePerGas: 0, // todo(Zygimantass): get gas settings
	}, true)

	msgs, gasSettings, err := f.msgGenerator(interactingLoaderWallet.Address())

	if err != nil {
		return nil, err
	}

	// create the CosmosDefaultClient
	return NewDefaultClient(interactingLoaderWallet, f.chainClient, msgs, gasSettings, &payload.Payload{
		Connections: uint64(cfg.Connections),
		Id:          f.id,
		Size:        uint64(cfg.Size),
		Rate:        uint64(cfg.Rate),
	}, f.skipSeqIncrement), nil
}

func (f *DefaultClientFactory) ValidateConfig(_ loadtest.Config) error {
	return nil
}
