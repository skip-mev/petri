package client

import (
	"context"

	"github.com/skip-mev/catalyst/internal/types"

	"cosmossdk.io/math"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// Balances returns the bank module token balances for a given address
func (c *Chain) Balances(ctx context.Context, address string) (sdk.Coins, error) {
	bankClient, err := c.getBankClient(ctx)
	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var balances sdk.Coins

	for {
		res, err := bankClient.AllBalances(ctx, &banktypes.QueryAllBalancesRequest{
			Address: address,
			Pagination: &query.PageRequest{
				Key: nextToken,
			},
		})
		if err != nil {
			return nil, err
		}

		balances = append(balances, res.Balances...)

		nextToken = res.Pagination.GetNextKey()

		if nextToken == nil {
			break
		}
	}

	return balances, nil
}

// Balances returns the bank module token balance for a given address and denom
func (c *Chain) Balance(ctx context.Context, address, denom string) (sdk.Coin, error) {
	bankClient, err := c.getBankClient(ctx)
	if err != nil {
		return sdk.Coin{}, err
	}

	res, err := bankClient.Balance(ctx, &banktypes.QueryBalanceRequest{
		Address: address,
		Denom:   denom,
	})
	if err != nil {
		return sdk.Coin{}, err
	}

	if res.Balance == nil {
		return sdk.Coin{}, nil
	}

	return *res.Balance, nil
}

// // BankSend sends tokens from the given user to another address
// func (c *Chain) BankSend(ctx context.Context, user wallet.InteractingWallet, toAddress []byte, amount sdk.Coins, gasSettings types.GasSettings, blocking bool) (*sdk.TxResponse, error) {
// 	fromAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(user.Address()))
// 	if err != nil {
// 		return nil, err
// 	}

// 	toAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(toAddress))
// 	if err != nil {
// 		return nil, err
// 	}

// 	msg := banktypes.NewMsgSend(fromAccAddress, toAccAddress, amount)

// 	return user.CreateAndBroadcastTx(ctx, uint64(gasSettings.Gas), GetFeeAmountsFromGasSettings(gasSettings), blocking, msg)
// }

// GetFeeAmountsFromGasSettings returns the fee amounts from the gas settings
func GetFeeAmountsFromGasSettings(gasSettings types.GasSettings) sdk.Coins {
	return sdk.NewCoins(sdk.NewCoin(gasSettings.GasDenom, math.NewInt(gasSettings.Gas).Mul(math.NewInt(gasSettings.PricePerGas))))
}
