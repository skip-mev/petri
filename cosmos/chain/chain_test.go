package chain_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	"github.com/skip-mev/petri/core/v3/types"
	"github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const idAlphabet = "abcdefghijklqmnoqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var defaultChainConfig = types.ChainConfig{
	Denom:         "stake",
	Decimals:      6,
	NumValidators: 4,
	NumNodes:      0,
	BinaryName:    "/usr/bin/simd",
	Image: provider.ImageDefinition{
		Image: "ghcr.io/cosmos/simapp:v0.47",
		UID:   "1000",
		GID:   "1000",
	},
	GasPrices:            "0.0005stake",
	Bech32Prefix:         "cosmos",
	HomeDir:              "/gaia",
	CoinType:             "118",
	ChainId:              "stake-1",
	UseGenesisSubCommand: true,
}

var defaultChainOptions = types.ChainOptions{
	NodeCreator: node.CreateNode,
	WalletConfig: types.WalletConfig{
		SigningAlgorithm: string(hd.Secp256k1.Name()),
		Bech32Prefix:     "cosmos",
		HDPath:           hd.CreateHDPath(118, 0, 0),
		DerivationFn:     hd.Secp256k1.Derive(),
		GenerationFn:     hd.Secp256k1.Generate(),
	},
}

var evmChainConfig = types.ChainConfig{
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

var evmChainOptions = types.ChainOptions{
	NodeCreator: node.CreateNode,
	WalletConfig: types.WalletConfig{
		SigningAlgorithm: string(hd.Secp256k1.Name()),
		Bech32Prefix:     "cosmos",
		HDPath:           hd.CreateHDPath(118, 0, 0),
		DerivationFn:     hd.Secp256k1.Derive(),
		GenerationFn:     hd.Secp256k1.Generate(),
	},
	ModifyGenesis: chain.ModifyGenesis([]chain.GenesisKV{
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

func TestChainLifecycle(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)
	chainName := gonanoid.MustGenerate(idAlphabet, 5)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	chainConfig := defaultChainConfig
	chainConfig.Name = chainName

	c, err := chain.CreateChain(ctx, logger, p, chainConfig, defaultChainOptions)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx, defaultChainOptions))
	require.Len(t, c.GetValidators(), 4)
	require.Len(t, c.GetNodes(), 0)

	time.Sleep(1 * time.Second)

	require.NoError(t, c.WaitForBlocks(ctx, 5))

	require.NoError(t, c.Teardown(ctx))
}

func TestChainSerialization(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)
	chainName := gonanoid.MustGenerate(idAlphabet, 5)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)

	pState, err := p.SerializeProvider(ctx)
	require.NoError(t, err)

	p2, err := docker.RestoreProvider(ctx, logger, pState)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		if !t.Failed() {
			require.NoError(t, p.Teardown(ctx))
		}
	}(p2, ctx)

	chainConfig := defaultChainConfig
	chainConfig.Name = chainName

	c, err := chain.CreateChain(ctx, logger, p2, chainConfig, defaultChainOptions)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx, defaultChainOptions))
	require.Len(t, c.GetValidators(), 4)
	require.Len(t, c.GetNodes(), 0)

	require.NoError(t, c.WaitForStartup(ctx))

	state, err := c.Serialize(ctx, p)
	require.NoError(t, err)

	require.NotEmpty(t, state)

	c2, err := chain.RestoreChain(ctx, logger, p2, state, node.RestoreNode, defaultChainOptions.WalletConfig)
	require.NoError(t, err)

	require.Equal(t, c.GetConfig(), c2.GetConfig())
	require.Equal(t, len(c.GetValidators()), len(c2.GetValidators()))
	require.Equal(t, len(c.GetValidatorWallets()), len(c2.GetValidatorWallets()))
	require.Equal(t, c.GetFaucetWallet(), c2.GetFaucetWallet())

	if !t.Failed() {
		require.NoError(t, c.Teardown(ctx))
	}
}

func TestGenesisModifier(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)
	chainName := gonanoid.MustGenerate(idAlphabet, 5)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	chainOpts := defaultChainOptions
	chainOpts.ModifyGenesis = chain.ModifyGenesis([]chain.GenesisKV{
		{
			Key:   "app_state.gov.params.min_deposit.0.denom",
			Value: chainName,
		},
	})

	chainConfig := defaultChainConfig
	chainConfig.Name = chainName

	c, err := chain.CreateChain(ctx, logger, p, chainConfig, chainOpts)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx, chainOpts))
	require.Len(t, c.GetValidators(), 4)
	require.Len(t, c.GetNodes(), 0)

	time.Sleep(1 * time.Second)

	require.NoError(t, c.WaitForBlocks(ctx, 1))

	cometIp, err := c.GetValidators()[0].GetExternalAddress(ctx, "26657")
	require.NoError(t, err)

	resp, err := http.Get(fmt.Sprintf("http://%s/genesis", cometIp))
	require.NoError(t, err)
	defer func(resp *http.Response) {
		require.NoError(t, resp.Body.Close())
	}(resp)

	bz, err := io.ReadAll(resp.Body)
	fmt.Println(string(bz))
	require.NoError(t, err)

	require.Contains(t, string(bz), chainName)
}

func TestGaiaEvm(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)
	chainName := gonanoid.MustGenerate(idAlphabet, 5)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	chainConfig := evmChainConfig
	chainConfig.Name = chainName

	c, err := chain.CreateChain(ctx, logger, p, chainConfig, evmChainOptions)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx, evmChainOptions))
	require.Len(t, c.GetValidators(), 1)
	require.Len(t, c.GetNodes(), 1)

	time.Sleep(1 * time.Second)

	require.NoError(t, c.WaitForBlocks(ctx, 2))

	require.NoError(t, c.Teardown(ctx))
}
