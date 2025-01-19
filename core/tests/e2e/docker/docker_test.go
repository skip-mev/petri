package e2e

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/skip-mev/petri/cosmos/v2/node"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	gonanoid "github.com/matoous/go-nanoid/v2"
	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean"
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

	numTestChains = flag.Int("num-chains", 1, "number of chains to create for concurrent testing")
	numNodes      = flag.Int("num-nodes", 1, "number of nodes per chain")
	numValidators = flag.Int("num-validators", 1, "number of validators per chain")
)

func TestDockerE2E(t *testing.T) {
	if !flag.Parsed() {
		flag.Parse()
	}

	ctx := context.Background()
	logger, _ := zap.NewDevelopment()
	providerName := gonanoid.MustGenerate("abcdefghijklqmnoqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890", 10)

	p, err := docker.CreateProvider(ctx, logger, providerName)
	require.NoError(t, err)

	defer func() {
		require.NoError(t, p.Teardown(ctx))
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

	for i := 0; i < *numTestChains; i++ {
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

	for i := 0; i < *numTestChains/2; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			if err := chains[index].Teardown(ctx); err != nil {
				chainErrors <- fmt.Errorf("failed to teardown chain %d: %w", index, err)
			}
		}(i)
	}
	wg.Wait()
	require.Empty(t, chainErrors)

	serializedProvider, err := p.SerializeProvider(ctx)
	require.NoError(t, err)
	_, err = docker.RestoreProvider(ctx, logger, serializedProvider)
	require.NoError(t, err)

	for i := *numTestChains / 2; i < *numTestChains; i++ {
		originalChain := chains[i]
		validators := originalChain.GetValidators()
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

		err = originalChain.WaitForBlocks(ctx, 2)
		require.NoError(t, err)

		err = originalChain.Teardown(ctx)
		require.NoError(t, err)

		for _, validator := range validators {
			status, err := validator.GetStatus(ctx)
			require.Error(t, err)
			require.Equal(t, provider.TASK_STATUS_UNDEFINED, status)
		}
	}
}
