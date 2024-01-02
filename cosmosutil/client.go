package cosmosutil

import (
	"context"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	govtypes "github.com/cosmos/cosmos-sdk/x/gov/types/v1"
	petritypes "github.com/skip-mev/petri/types"
)

type ChainClient struct {
	Chain          petritypes.ChainI
	EncodingConfig EncodingConfig
}

// todo: remove the nolints

//nolint:unused
func (c *ChainClient) getAuthClient(ctx context.Context) (authtypes.QueryClient, error) {
	cc, err := c.Chain.GetGRPCClient(ctx)

	if err != nil {
		return nil, err
	}

	return authtypes.NewQueryClient(cc), nil
}

//nolint:unused
func (c *ChainClient) getBankClient(ctx context.Context) (banktypes.QueryClient, error) {
	cc, err := c.Chain.GetGRPCClient(ctx)

	if err != nil {
		return nil, err
	}

	return banktypes.NewQueryClient(cc), nil
}

//nolint:unused
func (c *ChainClient) getGovClient(ctx context.Context) (govtypes.QueryClient, error) {
	cc, err := c.Chain.GetGRPCClient(ctx)

	if err != nil {
		return nil, err
	}

	return govtypes.NewQueryClient(cc), nil
}
