package main

import (
	"context"
	"os"

	petritypes "github.com/skip-mev/petri/core/v2/types"
	"github.com/skip-mev/petri/cosmos/v2/chain"
	"github.com/skip-mev/petri/cosmos/v2/node"

	"github.com/skip-mev/petri/core/v2/provider"
	"github.com/skip-mev/petri/core/v2/provider/digitalocean"
	"go.uber.org/zap"
)

func main() {
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

	sshKeyPair, err := digitalocean.MakeSSHKeyPair()
	if err != nil {
		logger.Fatal("failed to create SSH key pair", zap.Error(err))
	}

	doProvider, err := digitalocean.NewProvider(ctx, logger, "cosmos-hub", doToken, []string{}, sshKeyPair)
	if err != nil {
		logger.Fatal("failed to create DigitalOcean provider", zap.Error(err))
	}

	chainConfig := petritypes.ChainConfig{
		ChainId:       "cosmoshub-4",
		NumValidators: 1,
		NumNodes:      1,
		BinaryName:    "gaiad",
		Denom:         "uatom",
		Decimals:      6,
		GasPrices:     "0.0025uatom",
		Image: provider.ImageDefinition{
			Image: "ghcr.io/cosmos/gaia:v21.0.1",
			UID:   "1000",
			GID:   "1000",
		},
		HomeDir:              "/root/.gaia",
		Bech32Prefix:         "cosmos",
		CoinType:             "118",
		UseGenesisSubCommand: true,
		NodeCreator:          node.CreateNode,
		NodeDefinitionModifier: func(def provider.TaskDefinition, nodeConfig petritypes.NodeConfig) provider.TaskDefinition {
			doConfig := digitalocean.DigitalOceanTaskConfig{
				"size":     "s-2vcpu-4gb",
				"region":   "ams3",
				"image_id": imageID,
			}
			def.ProviderSpecificConfig = doConfig
			return def
		},
	}

	logger.Info("Creating chain")
	cosmosChain, err := chain.CreateChain(ctx, logger, doProvider, chainConfig)
	if err != nil {
		logger.Fatal("failed to create chain", zap.Error(err))
	}

	logger.Info("Initializing chain")
	err = cosmosChain.Init(ctx)
	if err != nil {
		logger.Fatal("failed to initialize chain", zap.Error(err))
	}

	logger.Info("Waiting for chain to produce blocks")
	err = cosmosChain.WaitForBlocks(ctx, 1)
	if err != nil {
		logger.Fatal("failed waiting for blocks", zap.Error(err))
	}

	logger.Info("Chain is successfully running!")
}
