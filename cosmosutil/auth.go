package cosmosutil

import (
	"context"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

// Account fetches the Cosmos SDK account from a provided full node
func (c *ChainClient) Account(ctx context.Context, address string) (authtypes.AccountI, error) {
	authClient, err := c.getAuthClient(ctx)

	if err != nil {
		return nil, err
	}

	res, err := authClient.Account(ctx, &authtypes.QueryAccountRequest{
		Address: address,
	})

	if err != nil {
		return nil, err
	}

	var acc authtypes.AccountI

	err = c.EncodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)

	if err != nil {
		return nil, err
	}

	return acc, nil
}
