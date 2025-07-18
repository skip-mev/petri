package node_test

import (
	"context"
	"testing"

	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	"github.com/skip-mev/petri/core/v3/types"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

const idAlphabet = "abcdefghijklqmnoqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890"

var defaultChainConfig = types.ChainConfig{
	Denom:         "stake",
	Decimals:      6,
	NumValidators: 1,
	NumNodes:      0,
	BinaryName:    "/usr/bin/simd",
	Image: provider.ImageDefinition{
		Image: "ghcr.io/cosmos/simapp:v0.47",
		UID:   "1000",
		GID:   "1000",
	},
	Bech32Prefix:         "cosmos",
	HomeDir:              "/gaia",
	CoinType:             "118",
	ChainId:              "stake-1",
	UseGenesisSubCommand: true,
	AppConfig: types.Toml{
		"minimum-gas-prices": "0.0005stake",
		"grpc": types.Toml{
			"address": "0.0.0.0:9090",
		},
		"api": types.Toml{
			"enable":  true,
			"swagger": true,
			"address": "tcp://0.0.0.0:1317",
		},
		"telemetry": types.Toml{
			"enabled":                   true,
			"prometheus-retention-time": 3600,
		},
	},
	ConsensusConfig: types.Toml{
		"log_level": "info",
		"p2p": types.Toml{
			"allow_duplicate_ip": true,
			"addr_book_strict":   false,
		},
		"consensus": types.Toml{
			"timeout_commit":  "2s",
			"timeout_propose": "2s",
		},
		"instrumentation": types.Toml{
			"prometheus": true,
		},
		"rpc": types.Toml{
			"laddr":           "tcp://0.0.0.0:26657",
			"allowed_origins": []string{"*"},
		},
	},
	ClientConfig: types.Toml{
		"chain-id":        "stake-1",
		"keyring-backend": "test",
		"output":          "text",
		"node":            "http://localhost:26657",
		"broadcast-mode":  "sync",
	},
}

func TestNodeLifecycle(t *testing.T) {
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

	n, err := node.CreateNode(ctx, logger, p, types.NodeConfig{
		Name:        "test",
		Index:       0,
		ChainConfig: chainConfig,
	}, types.NodeOptions{})
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

func TestNodeSerialization(t *testing.T) {
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

	n, err := node.CreateNode(ctx, logger, p, types.NodeConfig{
		Name:        "test",
		Index:       0,
		ChainConfig: chainConfig,
	}, types.NodeOptions{})
	require.NoError(t, err)
	defer func(n types.NodeI, ctx context.Context) {
		require.NoError(t, n.Stop(ctx))
	}(n, ctx)

	status, err := n.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_STOPPED, status)

	err = n.Start(ctx)
	require.NoError(t, err)

	state, err := n.Serialize(ctx, p)
	require.NoError(t, err)
	require.NotEmpty(t, state)

	n2, err := node.RestoreNode(ctx, logger, state, p)
	require.NoError(t, err)

	status, err = n2.GetStatus(ctx)
	require.NoError(t, err)
	require.Equal(t, provider.TASK_RUNNING, status)
}
