package cosmosutil

import (
	"context"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
)

// bank queries

func (c *ChainClient) Balances(ctx context.Context, address string) (sdk.Coins, error) {
	bankClient, err := c.getBankClient()

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

	if err != nil {
		return nil, err
	}

	return balances, nil
}

func (c *ChainClient) Balance(ctx context.Context, address, denom string) (sdk.Coin, error) {
	bankClient, err := c.getBankClient()

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

func (c *ChainClient) DenomMetadata(ctx context.Context, denom string) (banktypes.Metadata, error) {
	bankClient, err := c.getBankClient()

	if err != nil {
		return banktypes.Metadata{}, err
	}

	res, err := bankClient.DenomMetadata(ctx, &banktypes.QueryDenomMetadataRequest{
		Denom: denom,
	})

	if err != nil {
		return banktypes.Metadata{}, err
	}

	return res.Metadata, nil
}

func (c *ChainClient) DenomsMetadata(ctx context.Context) ([]banktypes.Metadata, error) {
	bankClient, err := c.getBankClient()

	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var metadatas []banktypes.Metadata

	for {
		res, err := bankClient.DenomsMetadata(ctx, &banktypes.QueryDenomsMetadataRequest{})

		if err != nil {
			return nil, err
		}

		metadatas = append(metadatas, res.Metadatas...)

		nextToken = res.Pagination.GetNextKey()

		if nextToken == nil {
			break
		}
	}

	return metadatas, nil
}

func (c *ChainClient) TotalSupplyAll(ctx context.Context) (sdk.Coins, error) {
	bankClient, err := c.getBankClient()

	if err != nil {
		return nil, err
	}

	var nextToken []byte
	var supplies sdk.Coins

	for {
		res, err := bankClient.TotalSupply(ctx, &banktypes.QueryTotalSupplyRequest{})

		if err != nil {
			return nil, err
		}

		supplies = append(supplies, res.Supply...)

		nextToken = res.Pagination.GetNextKey()

		if nextToken == nil {
			break
		}
	}

	return supplies, nil
}

func (c *ChainClient) BankTotalSupplySingle(ctx context.Context, denom string) (sdk.Coin, error) {
	bankClient, err := c.getBankClient()

	if err != nil {
		return sdk.Coin{}, err
	}

	res, err := bankClient.SupplyOf(ctx, &banktypes.QuerySupplyOfRequest{
		Denom: denom,
	})

	if err != nil {
		return sdk.Coin{}, err
	}

	return res.Amount, nil
}

// bank transactions

func (c *ChainClient) BankSend(ctx context.Context, user InteractingWallet, toAddress string, amount sdk.Coins, blocking bool) (*sdk.TxResponse, error) {
	msg := banktypes.NewMsgSend(sdk.AccAddress(user.wallet.FormattedAddress()), sdk.AccAddress(toAddress), amount)

	txResp, err := user.CreateAndBroadcastTx(ctx, true, 0, sdk.Coins{}, msg)

	if err != nil {
		return nil, err
	}

	return txResp, nil
}
