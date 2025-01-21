package e2e

import (
	"context"
	"flag"
	"os"
	"testing"
	"time"

	"github.com/skip-mev/petri/core/v2/tests/e2e"

	"github.com/skip-mev/petri/cosmos/v2/node"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean"
	"github.com/skip-mev/petri/core/v2/types"
	cosmoschain "github.com/skip-mev/petri/cosmos/v2/chain"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

var (
	defaultChainConfig = types.ChainConfig{
		Denom:         "stake",
		Decimals:      6,
		NumValidators: 1,
		NumNodes:      1,
		BinaryName:    "/usr/bin/simd",
		Image: provider.ImageDefinition{
			Image: "interchainio/simapp:latest",
			UID:   "1000",
			GID:   "1000",
		},
		GasPrices:    "0.0005stake",
		Bech32Prefix: "cosmos",
		HomeDir:      "/gaia",
		CoinType:     "118",
		ChainId:      "stake-1",
	}

	defaultChainOptions = types.ChainOptions{
		NodeCreator: node.CreateNode,
		WalletConfig: types.WalletConfig{
			SigningAlgorithm: string(hd.Secp256k1.Name()),
			Bech32Prefix:     "cosmos",
			HDPath:           hd.CreateHDPath(118, 0, 0),
			DerivationFn:     hd.Secp256k1.Derive(),
			GenerationFn:     hd.Secp256k1.Generate(),
		},
		NodeOptions: types.NodeOptions{
			NodeDefinitionModifier: func(def provider.TaskDefinition, nodeConfig types.NodeConfig) provider.TaskDefinition {
				doConfig := digitalocean.DigitalOceanTaskConfig{
					"size":     "s-2vcpu-4gb",
					"region":   "ams3",
					"image_id": os.Getenv("DO_IMAGE_ID"),
				}
				def.ProviderSpecificConfig = doConfig
				return def
			},
		},
	}

	numTestChains = flag.Int("num-chains", 3, "number of chains to create for concurrent testing")
	numNodes      = flag.Int("num-nodes", 1, "number of nodes per chain")
	numValidators = flag.Int("num-validators", 1, "number of validators per chain")
)

func TestDOE2E(t *testing.T) {
	if !flag.Parsed() {
		flag.Parse()
	}

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	doToken := os.Getenv("DO_API_TOKEN")
	if doToken == "" {
		logger.Fatal("DO_API_TOKEN environment variable not set")
	}

	imageID := os.Getenv("DO_IMAGE_ID")
	if imageID == "" {
		logger.Fatal("DO_IMAGE_ID environment variable not set")
	}

	externalIP, err := e2e.GetExternalIP()
	logger.Info("External IP", zap.String("address", externalIP))
	require.NoError(t, err)

	p, err := digitalocean.NewProvider(ctx, logger, "digitalocean_provider", doToken, []string{externalIP}, nil)
	require.NoError(t, err)

	chains := make([]*cosmoschain.Chain, *numTestChains)

	// Create first half of chains
	defaultChainConfig.NumNodes = *numNodes
	defaultChainConfig.NumValidators = *numValidators
	e2e.CreateChainsConcurrently(ctx, t, logger, p, 0, *numTestChains/2, chains, defaultChainConfig, defaultChainOptions)

	// Restore provider before creating second half of chains
	serializedProvider, err := p.SerializeProvider(ctx)
	require.NoError(t, err)
	restoredProvider, err := digitalocean.RestoreProvider(ctx, doToken, serializedProvider, nil, nil)
	require.NoError(t, err)

	// Restore the existing chains with the restored provider
	restoredChains := make([]*cosmoschain.Chain, *numTestChains)
	for i := 0; i < *numTestChains/2; i++ {
		chainState, err := chains[i].Serialize(ctx, restoredProvider)
		require.NoError(t, err)

		restoredChain, err := cosmoschain.RestoreChain(ctx, logger, restoredProvider, chainState, node.RestoreNode)
		require.NoError(t, err)

		require.Equal(t, chains[i].GetConfig(), restoredChain.GetConfig())
		require.Equal(t, len(chains[i].GetValidators()), len(restoredChain.GetValidators()))

		restoredChains[i] = restoredChain
	}

	// Create second half of chains with restored provider
	e2e.CreateChainsConcurrently(ctx, t, logger, restoredProvider, *numTestChains/2, *numTestChains, restoredChains, defaultChainConfig, defaultChainOptions)

	// Test and teardown half the chains individually
	for i := 0; i < *numTestChains/2; i++ {
		originalChain := restoredChains[i]
		validators := originalChain.GetValidators()
		nodes := originalChain.GetNodes()

		for _, validator := range validators {
			e2e.AssertNodeRunning(t, ctx, validator)
		}

		for _, node := range nodes {
			e2e.AssertNodeRunning(t, ctx, node)
		}

		err = originalChain.WaitForBlocks(ctx, 2)
		require.NoError(t, err)

		// Test individual chain teardown
		err = originalChain.Teardown(ctx)
		require.NoError(t, err)

		// wait for status to update on DO client side
		time.Sleep(15 * time.Second)

		for _, validator := range validators {
			e2e.AssertNodeShutdown(t, ctx, validator)
		}

		for _, node := range nodes {
			e2e.AssertNodeShutdown(t, ctx, node)
		}
	}

	// Test the remaining chains but let the provider teardown handle their cleanup
	remainingChains := make([]*cosmoschain.Chain, 0)
	for i := *numTestChains / 2; i < *numTestChains; i++ {
		originalChain := restoredChains[i]
		remainingChains = append(remainingChains, originalChain)
		validators := originalChain.GetValidators()
		nodes := originalChain.GetNodes()
		for _, validator := range validators {
			e2e.AssertNodeRunning(t, ctx, validator)
		}
		for _, node := range nodes {
			e2e.AssertNodeRunning(t, ctx, node)
		}

		err = originalChain.WaitForBlocks(ctx, 2)
		require.NoError(t, err)
	}

	require.NoError(t, restoredProvider.Teardown(ctx))
	// wait for status to update on DO client side
	time.Sleep(15 * time.Second)

	// Verify all remaining chains are properly torn down
	for _, chain := range remainingChains {
		validators := chain.GetValidators()
		nodes := chain.GetNodes()

		for _, validator := range validators {
			e2e.AssertNodeShutdown(t, ctx, validator)
		}

		for _, node := range nodes {
			e2e.AssertNodeShutdown(t, ctx, node)
		}
	}
}
