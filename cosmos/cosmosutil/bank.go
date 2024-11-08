package cosmosutil

import (
	"context"
	"encoding/hex"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/cosmos/cosmos-sdk/types/query"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"

	"github.com/skip-mev/petri/core/v2/types"
)

// bank queries

// Balances returns the bank module token balances for a given address
func (c *ChainClient) Balances(ctx context.Context, address string) (sdk.Coins, error) {
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

	if err != nil {
		return nil, err
	}

	return balances, nil
}

// Balances returns the bank module token balance for a given address and denom
func (c *ChainClient) Balance(ctx context.Context, address, denom string) (sdk.Coin, error) {
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

// DenomMetadata returns the bank module token metadata for a given denom
func (c *ChainClient) DenomMetadata(ctx context.Context, denom string) (banktypes.Metadata, error) {
	bankClient, err := c.getBankClient(ctx)
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

// DenomsMetadata returns the bank module token metadata for all denoms
func (c *ChainClient) DenomsMetadata(ctx context.Context) ([]banktypes.Metadata, error) {
	bankClient, err := c.getBankClient(ctx)
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

// TotalSupplyAll returns the total supply of all tokens in the bank module
func (c *ChainClient) TotalSupplyAll(ctx context.Context) (sdk.Coins, error) {
	bankClient, err := c.getBankClient(ctx)
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

// TotalSupplySingle returns the total supply of a single token in the bank module
func (c *ChainClient) BankTotalSupplySingle(ctx context.Context, denom string) (sdk.Coin, error) {
	bankClient, err := c.getBankClient(ctx)
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

// BankSend sends tokens from the given user to another address
func (c *ChainClient) BankSend(ctx context.Context, user InteractingWallet, toAddress []byte, amount sdk.Coins, gasSettings types.GasSettings, blocking bool) (*sdk.TxResponse, error) {
	fromAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(user.Address()))
	if err != nil {
		return nil, err
	}

	toAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(toAddress))
	if err != nil {
		return nil, err
	}

	msg := banktypes.NewMsgSend(fromAccAddress, toAccAddress, amount)

	return user.CreateAndBroadcastTx(ctx, blocking, gasSettings.Gas, GetFeeAmountsFromGasSettings(gasSettings), 0, "", msg)
}
