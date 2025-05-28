package e2e

import (
	"context"
	"flag"
	"testing"

	"github.com/skip-mev/petri/core/v3/util"

	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	"github.com/skip-mev/petri/core/v3/types"
	cosmoschain "github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"github.com/skip-mev/petri/cosmos/v3/tests/e2e"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	evmChainConfig = types.ChainConfig{
		Denom:         "atest",
		Decimals:      6,
		NumValidators: 1,
		NumNodes:      1,
		BinaryName:    "gaiad",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/cosmos/gaia:na-build-arm64",
			UID:   "1025",
			GID:   "1025",
		},
		GasPrices:            "0.0005atest",
		Bech32Prefix:         "cosmos",
		HomeDir:              "/gaia",
		CoinType:             "118",
		ChainId:              "cosmos_22222-1",
		UseGenesisSubCommand: true,
	}

	evmChainOptions = types.ChainOptions{
		NodeCreator: node.CreateNode,
		WalletConfig: types.WalletConfig{
			SigningAlgorithm: string(hd.Secp256k1.Name()),
			Bech32Prefix:     "cosmos",
			HDPath:           hd.CreateHDPath(118, 0, 0),
			DerivationFn:     hd.Secp256k1.Derive(),
			GenerationFn:     hd.Secp256k1.Generate(),
		},
		ModifyGenesis: cosmoschain.ModifyGenesis([]cosmoschain.GenesisKV{
			{
				Key:   "app_state.staking.params.bond_denom",
				Value: "atest",
			},
			{
				Key:   "app_state.gov.deposit_params.min_deposit.0.denom",
				Value: "atest",
			},
			{
				Key:   "app_state.gov.params.min_deposit.0.denom",
				Value: "atest",
			},
			{
				Key:   "app_state.evm.params.evm_denom",
				Value: "atest",
			},
			{
				Key:   "app_state.mint.params.mint_denom",
				Value: "atest",
			},
			{
				Key: "app_state.bank.denom_metadata",
				Value: []map[string]interface{}{
					{
						"description": "The native staking token for evmd.",
						"denom_units": []map[string]interface{}{
							{
								"denom":    "atest",
								"exponent": 0,
								"aliases":  []string{"attotest"},
							},
							{
								"denom":    "test",
								"exponent": 18,
								"aliases":  []string{},
							},
						},
						"base":     "atest",
						"display":  "test",
						"name":     "Test Token",
						"symbol":   "TEST",
						"uri":      "",
						"uri_hash": "",
					},
				},
			},
			{
				Key: "app_state.evm.params.active_static_precompiles",
				Value: []string{
					"0x0000000000000000000000000000000000000100",
					"0x0000000000000000000000000000000000000400",
					"0x0000000000000000000000000000000000000800",
					"0x0000000000000000000000000000000000000801",
					"0x0000000000000000000000000000000000000802",
					"0x0000000000000000000000000000000000000803",
					"0x0000000000000000000000000000000000000804",
					"0x0000000000000000000000000000000000000805",
				},
			},
			{
				Key:   "app_state.erc20.params.native_precompiles",
				Value: []string{"0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE"},
			},
			{
				Key: "app_state.erc20.token_pairs",
				Value: []map[string]interface{}{
					{
						"contract_owner": 1,
						"erc20_address":  "0xEeeeeEeeeEeEeeEeEeEeeEEEeeeeEeeeeeeeEEeE",
						"denom":          "atest",
						"enabled":        true,
					},
				},
			},
			{
				Key:   "consensus.params.block.max_gas",
				Value: "75000000",
			},
		}),
	}
)

const (
	evmChainIDFmt = "cosmos_22222-%d"
)

func TestGaiaEvmE2E(t *testing.T) {
	if !flag.Parsed() {
		flag.Parse()
	}

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	defer func() {
		dockerClient, err := client.NewClientWithOpts()
		if err != nil {
			t.Logf("Failed to create Docker client for volume cleanup: %v", err)
			return
		}
		_, err = dockerClient.VolumesPrune(ctx, filters.Args{})
		if err != nil {
			t.Logf("Failed to prune volumes: %v", err)
		}
	}()

	providerName := util.RandomString(5)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)

	chains := make([]*cosmoschain.Chain, *numTestChains)

	evmChainConfig.NumNodes = *numNodes
	evmChainConfig.NumValidators = *numValidators
	e2e.CreateChainsConcurrently(ctx, t, logger, p, 0, 1, chains,
		evmChainConfig, evmChainIDFmt, evmChainOptions)
	require.NoError(t, p.Teardown(ctx))
}
