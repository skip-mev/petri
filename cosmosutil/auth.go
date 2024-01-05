package cosmosutil

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
)

func (c *ChainClient) Account(ctx context.Context, address string) (sdk.AccountI, error) {
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

	var acc authtypes.BaseAccount

	err = c.EncodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)

	if err != nil {
		return nil, err
	}

	return &acc, nil
}
