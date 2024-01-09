package cosmosutil

import (
	"context"
	"fmt"
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

	var acc sdk.AccountI

	err = c.EncodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)

	if err != nil {
		return nil, err
	}

	p, ok := acc.(*authtypes.BaseAccount)

	if !ok {
		return nil, fmt.Errorf("failed converting account to baseaccount")
	}

	return p, nil
}
