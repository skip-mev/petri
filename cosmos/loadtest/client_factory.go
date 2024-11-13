package loadtest

import (
	"context"
	"fmt"

	"github.com/cometbft/cometbft/test/loadtime/payload"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/informalsystems/tm-load-test/pkg/loadtest"

	petritypes "github.com/skip-mev/petri/core/v2/types"
	"github.com/skip-mev/petri/cosmos/v2/cosmosutil"
	"github.com/skip-mev/petri/cosmos/v2/wallet"
)

// GenerateMsgs is a function that generates messages given an address for the DefaultClient
type GenerateMsgs func(senderAddress []byte) ([]sdk.Msg, petritypes.GasSettings, error)

// DefaultClientFactory is an implementation of the ClientFactory interface that creates DefaultClients
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

// ClientFactoryConfig is a struct that packs all the necessary information for creating a new DefaultClientFactory
type ClientFactoryConfig struct {
	Chain                 petritypes.ChainI
	Seeder                *cosmosutil.InteractingWallet
	WalletConfig          petritypes.WalletConfig
	AmountToSend          int64
	SkipSequenceIncrement bool
	EncodingConfig        cosmosutil.EncodingConfig
	MsgGenerator          GenerateMsgs
}

func NewDefaultClientFactory(cfg ClientFactoryConfig) *DefaultClientFactory {
	return &DefaultClientFactory{
		chain:            cfg.Chain,
		chainClient:      &cosmosutil.ChainClient{Chain: cfg.Chain},
		seeder:           cfg.Seeder,
		amtToSend:        cfg.AmountToSend,
		skipSeqIncrement: cfg.SkipSequenceIncrement,
		walletConfig:     cfg.WalletConfig,
		encodingConfig:   cfg.EncodingConfig,
		msgGenerator:     cfg.MsgGenerator,
	}
}

// NewClient implements the ClientFactory's interface for creating new clients.
// In this case, it'll create the DefaultClient
func (f *DefaultClientFactory) NewClient(cfg loadtest.Config) (loadtest.Client, error) {
	// create a new private-key
	loaderWallet, err := wallet.NewGeneratedWallet("seed", f.walletConfig)
	if err != nil {
		return nil, err
	}

	interactingLoaderWallet := cosmosutil.NewInteractingWallet(f.chain, loaderWallet, f.encodingConfig)

	resp, err := f.chainClient.BankSend(context.Background(), *f.seeder, interactingLoaderWallet.Address(), sdk.NewCoins(sdk.NewInt64Coin(f.chain.GetConfig().Denom, f.amtToSend)), petritypes.GasSettings{
		Gas:         200000,
		GasDenom:    f.chain.GetConfig().Denom,
		PricePerGas: 1,
	}, true)
	if err != nil {
		return nil, fmt.Errorf("error seeding account %s from seeder %s: %v, resp: %v", interactingLoaderWallet.FormattedAddress(), f.seeder.FormattedAddress(), err, resp)
	}

	msgs, gasSettings, err := f.msgGenerator(interactingLoaderWallet.Address())
	if err != nil {
		return nil, err
	}

	acc, err := interactingLoaderWallet.Account(context.Background())
	if err != nil {
		return nil, fmt.Errorf("error in client initialization: sender account was not created %s", interactingLoaderWallet.FormattedAddress())
	}

	return NewDefaultClient(interactingLoaderWallet, f.chainClient, msgs, acc.GetSequence(), acc.GetAccountNumber(), gasSettings, &payload.Payload{
		Connections: uint64(cfg.Connections),
		Id:          f.id,
		Size:        uint64(cfg.Size),
		Rate:        uint64(cfg.Rate),
	}, f.skipSeqIncrement), nil
}

// ValidateConfig for the DefaultClientFactory is a no-op
func (f *DefaultClientFactory) ValidateConfig(_ loadtest.Config) error {
	return nil
}
