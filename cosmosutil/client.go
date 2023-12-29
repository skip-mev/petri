package cosmosutil

import (
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	petritypes "github.com/skip-mev/petri/types"
)

type ChainClient struct {
	Chain          petritypes.ChainI
	EncodingConfig EncodingConfig
}

// todo: remove the nolints

//nolint:unused
func (c *ChainClient) getAuthClient() (authtypes.QueryClient, error) {
	cc, err := c.Chain.GetGPRCClient()

	if err != nil {
		return nil, err
	}

	return authtypes.NewQueryClient(cc), nil
}

//nolint:unused
func (c *ChainClient) getBankClient() (banktypes.QueryClient, error) {
	cc, err := c.Chain.GetGPRCClient()

	if err != nil {
		return nil, err
	}

	return banktypes.NewQueryClient(cc), nil
}
