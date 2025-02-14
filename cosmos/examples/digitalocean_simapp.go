package examples

import (
	"context"
	"github.com/skip-mev/petri/core/v3/provider/digitalocean"
	"io"
	"net/http"
	"os"
	"strings"
	"tailscale.com/tsnet"

	"github.com/cosmos/cosmos-sdk/crypto/hd"

	"github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"

	"github.com/skip-mev/petri/core/v3/provider"
	petritypes "github.com/skip-mev/petri/core/v3/types"
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

	clientAuthKey := os.Getenv("TS_CLIENT_AUTH_KEY")
	if clientAuthKey == "" {
		logger.Fatal("TS_CLIENT_AUTH_KEY environment variable not set")
	}

	serverOauthSecret := os.Getenv("TS_SERVER_OAUTH_SECRET")
	if serverOauthSecret == "" {
		logger.Fatal("TS_SERVER_AUTH_KEY environment variable not set")
	}

	serverAuthKey, err := digitalocean.GenerateTailscaleAuthKey(ctx, serverOauthSecret, []string{"tag:petri-e2e"})
	if err != nil {
		logger.Fatal("failed to generate Tailscale auth key", zap.Error(err))
	}

	tsServer := tsnet.Server{
		AuthKey:   serverAuthKey,
		Ephemeral: true,
		Hostname:  "petri-e2e",
	}

	localClient, err := tsServer.LocalClient()
	if err != nil {
		logger.Fatal("failed to create local client", zap.Error(err))
	}

	tailscaleSettings := digitalocean.TailscaleSettings{
		AuthKey:     clientAuthKey,
		Server:      &tsServer,
		Tags:        []string{"petri-e2e"},
		LocalClient: localClient,
	}

	doProvider, err := digitalocean.NewProvider(
		ctx,
		"cosmos-hub",
		doToken,
		tailscaleSettings,
		digitalocean.WithLogger(logger),
	)

	if err != nil {
		logger.Fatal("failed to create DigitalOcean provider", zap.Error(err))
	}

	chainConfig := petritypes.ChainConfig{
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

	chainOptions := petritypes.ChainOptions{
		NodeCreator: node.CreateNode,
		NodeOptions: petritypes.NodeOptions{
			NodeDefinitionModifier: func(def provider.TaskDefinition, nodeConfig petritypes.NodeConfig) provider.TaskDefinition {
				doConfig := digitalocean.DigitalOceanTaskConfig{
					"size":     "s-2vcpu-4gb",
					"region":   "ams3",
					"image_id": imageID,
				}
				def.ProviderSpecificConfig = doConfig
				return def
			},
		},
		WalletConfig: petritypes.WalletConfig{
			SigningAlgorithm: string(hd.Secp256k1.Name()),
			Bech32Prefix:     "cosmos",
			HDPath:           hd.CreateHDPath(118, 0, 0),
			DerivationFn:     hd.Secp256k1.Derive(),
			GenerationFn:     hd.Secp256k1.Generate(),
		},
	}

	logger.Info("Creating chain")
	cosmosChain, err := chain.CreateChain(ctx, logger, doProvider, chainConfig, chainOptions)
	if err != nil {
		logger.Fatal("failed to create chain", zap.Error(err))
	}

	logger.Info("Initializing chain")
	err = cosmosChain.Init(ctx, chainOptions)
	if err != nil {
		logger.Fatal("failed to initialize chain", zap.Error(err))
	}

	logger.Info("Chain is successfully running! Waiting for chain to produce blocks")
	err = cosmosChain.WaitForBlocks(ctx, 1)
	if err != nil {
		logger.Fatal("failed waiting for blocks", zap.Error(err))
	}

	// Comment out section below if you want to persist your Digital Ocean resources
	logger.Info("Chain has successfully produced required number of blocks. Tearing down Digital Ocean resources.")
	err = doProvider.Teardown(ctx)
	if err != nil {
		logger.Fatal("failed to teardown provider", zap.Error(err))
	}

	logger.Info("All Digital Ocean resources created have been successfully deleted!")
}

func getExternalIP() (string, error) {
	resp, err := http.Get("https://ifconfig.me")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	ip, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(ip)), nil
}
