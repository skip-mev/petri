package node_test

import (
	"context"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/docker"
	"github.com/skip-mev/petri/core/v2/types"
	"github.com/skip-mev/petri/cosmos/v2/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
	"testing"
)

const idAlphabet = "abcdefghijklqmnoqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var defaultChainConfig = types.ChainConfig{
	Denom:         "stake",
	Decimals:      6,
	NumValidators: 1,
	NumNodes:      0,
	BinaryName:    "/simd/simd",
	Image: provider.ImageDefinition{
		Image: "cosmossdk/simd:latest",
		UID:   "1000",
		GID:   "1000",
	},
	GasPrices:            "0.0005stake",
	Bech32Prefix:         "stake",
	HomeDir:              "/gaia",
	CoinType:             "118",
	ChainId:              "stake-1",
	WalletConfig:         types.WalletConfig{},
	UseGenesisSubCommand: true,
	NodeCreator:          node.CreateNode,
}

func TestNodeLifecycle(t *testing.T) {
	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate(idAlphabet, 10)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)
	defer func(p provider.ProviderI, ctx context.Context) {
		require.NoError(t, p.Teardown(ctx))
	}(p, ctx)

	n, err := node.CreateNode(ctx, logger, p, types.NodeConfig{
		Name:        "test",
		Index:       0,
		ChainConfig: defaultChainConfig,
	})
	require.NoError(t, err)
	defer func(n types.NodeI, ctx context.Context) {
		require.NoError(t, n.Stop(ctx))
	}(n, ctx)

	status, err := n.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_STOPPED, status)

	err = n.Start(ctx)
	require.NoError(t, err)
}