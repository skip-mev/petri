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
		Image: "interchainio/simapp:latest",
		UID:   "1000",
		GID:   "1000",
	},
	GasPrices:            "0.0005stake",
	Bech32Prefix:         "cosmos",
	HomeDir:              "/gaia",
	CoinType:             "118",
	ChainId:              "stake-1",
	UseGenesisSubCommand: false,
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

func TestChainLifecycle(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	c, err := chain.CreateChain(ctx, logger, p, defaultChainConfig, defaultChainOptions)
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

	c, err := chain.CreateChain(ctx, logger, p2, defaultChainConfig, defaultChainOptions)
	require.NoError(t, err)

	require.NoError(t, c.Init(ctx, defaultChainOptions))
	require.Len(t, c.GetValidators(), 4)
	require.Len(t, c.GetNodes(), 0)

	require.NoError(t, c.WaitForStartup(ctx))

	state, err := c.Serialize(ctx, p)
	require.NoError(t, err)

	require.NotEmpty(t, state)

	c2, err := chain.RestoreChain(ctx, logger, p2, state, node.RestoreNode)
	require.NoError(t, err)

	require.Equal(t, c.GetConfig(), c2.GetConfig())
	require.Equal(t, len(c.GetValidators()), len(c2.GetValidators()))

	if !t.Failed() {
		require.NoError(t, c.Teardown(ctx))
	}
}

func TestGenesisModifier(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)
	chainName := gonanoid.MustGenerate(idAlphabet, 64)

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

	c, err := chain.CreateChain(ctx, logger, p, defaultChainConfig, chainOpts)
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
