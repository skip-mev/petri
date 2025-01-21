package e2e

import (
	"context"
	"flag"
	"fmt"
	"sync"
	"testing"

	"github.com/skip-mev/petri/cosmos/v2/node"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/docker"
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
		GasPrices:            "0.0005stake",
		Bech32Prefix:         "cosmos",
		HomeDir:              "/gaia",
		CoinType:             "118",
		ChainId:              "stake-1",
		UseGenesisSubCommand: false,
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
	}

	numTestChains = flag.Int("num-chains", 3, "number of chains to create for concurrent testing")
	numNodes      = flag.Int("num-nodes", 1, "number of nodes per chain")
	numValidators = flag.Int("num-validators", 1, "number of validators per chain")
)

func TestDockerE2E(t *testing.T) {
	if !flag.Parsed() {
		flag.Parse()
	}

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()

	p, err := docker.CreateProvider(ctx, logger, "docker_provider")
	require.NoError(t, err)

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

	var wg sync.WaitGroup
	chainErrors := make(chan error, *numTestChains*2)
	chains := make([]*cosmoschain.Chain, *numTestChains)

	// Create first half of chains
	for i := 0; i < *numTestChains/2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			chainConfig := defaultChainConfig
			chainConfig.ChainId = fmt.Sprintf("chain-%d", index)
			chainConfig.NumNodes = *numNodes
			chainConfig.NumValidators = *numValidators
			c, err := cosmoschain.CreateChain(ctx, logger, p, chainConfig, defaultChainOptions)
			if err != nil {
				t.Logf("Chain creation error: %v", err)
				chainErrors <- fmt.Errorf("failed to create chain %d: %w", index, err)
				return
			}
			if err := c.Init(ctx, defaultChainOptions); err != nil {
				t.Logf("Chain creation error: %v", err)
				chainErrors <- fmt.Errorf("failed to init chain %d: %w", index, err)
				return
			}
			chains[index] = c
		}(i)
	}
	wg.Wait()
	require.Empty(t, chainErrors)

	serializedProvider, err := p.SerializeProvider(ctx)
	require.NoError(t, err)
	restoredProvider, err := docker.RestoreProvider(ctx, logger, serializedProvider)
	require.NoError(t, err)

	// Create second half of chains with restored provider
	for i := *numTestChains / 2; i < *numTestChains; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			chainConfig := defaultChainConfig
			chainConfig.ChainId = fmt.Sprintf("chain-%d", index)
			chainConfig.NumNodes = *numNodes
			chainConfig.NumValidators = *numValidators
			c, err := cosmoschain.CreateChain(ctx, logger, restoredProvider, chainConfig, defaultChainOptions)
			if err != nil {
				t.Logf("Chain creation error: %v", err)
				chainErrors <- fmt.Errorf("failed to create chain %d: %w", index, err)
				return
			}
			if err := c.Init(ctx, defaultChainOptions); err != nil {
				t.Logf("Chain creation error: %v", err)
				chainErrors <- fmt.Errorf("failed to init chain %d: %w", index, err)
				return
			}
			chains[index] = c
		}(i)
	}
	wg.Wait()
	require.Empty(t, chainErrors)

	// Serialize and restore all chains with the restored provider
	restoredChains := make([]*cosmoschain.Chain, *numTestChains)
	for i := 0; i < *numTestChains; i++ {
		chainState, err := chains[i].Serialize(ctx, restoredProvider)
		require.NoError(t, err)

		restoredChain, err := cosmoschain.RestoreChain(ctx, logger, restoredProvider, chainState, node.RestoreNode)
		require.NoError(t, err)

		require.Equal(t, chains[i].GetConfig(), restoredChain.GetConfig())
		require.Equal(t, len(chains[i].GetValidators()), len(restoredChain.GetValidators()))

		restoredChains[i] = restoredChain
	}

	// Test and teardown half the chains individually
	for i := 0; i < *numTestChains/2; i++ {
		originalChain := restoredChains[i]
		validators := originalChain.GetValidators()
		nodes := originalChain.GetNodes()

		for _, validator := range validators {
			status, err := validator.GetStatus(ctx)
			require.NoError(t, err)
			require.Equal(t, provider.TASK_RUNNING, status)

			ip, err := validator.GetIP(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, ip)

			testFile := "test.txt"
			testContent := []byte("test content")
			err = validator.WriteFile(ctx, testFile, testContent)
			require.NoError(t, err)

			readContent, err := validator.ReadFile(ctx, testFile)
			require.NoError(t, err)
			require.Equal(t, testContent, readContent)
		}

		for _, node := range nodes {
			status, err := node.GetStatus(ctx)
			require.NoError(t, err)
			require.Equal(t, provider.TASK_RUNNING, status)

			ip, err := node.GetIP(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, ip)
		}

		err = originalChain.WaitForBlocks(ctx, 2)
		require.NoError(t, err)

		// Test individual chain teardown
		err = originalChain.Teardown(ctx)
		require.NoError(t, err)

		for _, validator := range validators {
			status, err := validator.GetStatus(ctx)
			logger.Info("validator status", zap.Any("", status))
			require.Error(t, err)
			require.Equal(t, provider.TASK_STATUS_UNDEFINED, status, "validator task should report undefined as container isn't available")

			_, err = validator.GetIP(ctx)
			require.Error(t, err, "validator IP should not be accessible after teardown")
		}

		for _, node := range nodes {
			status, err := node.GetStatus(ctx)
			logger.Info("node status", zap.Any("", status))
			require.Error(t, err)
			require.Equal(t, provider.TASK_STATUS_UNDEFINED, status, "node task should report undefined as container isn't available")

			_, err = node.GetIP(ctx)
			require.Error(t, err, "node IP should not be accessible after teardown")
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
			status, err := validator.GetStatus(ctx)
			require.NoError(t, err)
			require.Equal(t, provider.TASK_RUNNING, status)

			ip, err := validator.GetIP(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, ip)

			testFile := "test.txt"
			testContent := []byte("test content")
			err = validator.WriteFile(ctx, testFile, testContent)
			require.NoError(t, err)

			readContent, err := validator.ReadFile(ctx, testFile)
			require.NoError(t, err)
			require.Equal(t, testContent, readContent)
		}
		for _, node := range nodes {
			status, err := node.GetStatus(ctx)
			require.NoError(t, err)
			require.Equal(t, provider.TASK_RUNNING, status)

			ip, err := node.GetIP(ctx)
			require.NoError(t, err)
			require.NotEmpty(t, ip)
		}

		err = originalChain.WaitForBlocks(ctx, 2)
		require.NoError(t, err)
	}

	require.NoError(t, restoredProvider.Teardown(ctx))
	// Verify all remaining chains are properly torn down
	for _, chain := range remainingChains {
		validators := chain.GetValidators()
		nodes := chain.GetNodes()

		for _, validator := range validators {
			status, err := validator.GetStatus(ctx)
			logger.Info("validator status after provider teardown", zap.Any("", status))
			require.Error(t, err)
			require.Equal(t, provider.TASK_STATUS_UNDEFINED, status, "validator task should report undefined as container isn't available")

			_, err = validator.GetIP(ctx)
			require.Error(t, err, "validator IP should not be accessible after teardown")
		}

		for _, node := range nodes {
			status, err := node.GetStatus(ctx)
			logger.Info("node status after provider teardown", zap.Any("", status))
			require.Error(t, err)
			require.Equal(t, provider.TASK_STATUS_UNDEFINED, status, "node task should report undefined as container isn't available")

			_, err = node.GetIP(ctx)
			require.Error(t, err, "node IP should not be accessible after teardown")
		}
	}
}
