package main

import (
	"context"
	"fmt"
	"os/signal"
	"syscall"
	"testing"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/hd"
	cryptotypes "github.com/cosmos/cosmos-sdk/crypto/types"
	"github.com/skip-mev/petri/core/v3/provider"
	"github.com/skip-mev/petri/core/v3/provider/docker"
	petritypes "github.com/skip-mev/petri/core/v3/types"
	"github.com/skip-mev/petri/cosmos/v3/chain"
	"github.com/skip-mev/petri/cosmos/v3/node"
	"go.uber.org/zap"

	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/loadtest"
)

var (
	defaultChainConfig = petritypes.ChainConfig{
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

	defaultChainOptions = petritypes.ChainOptions{
		NodeCreator: node.CreateNode,
		ModifyGenesis: chain.ModifyGenesis([]chain.GenesisKV{
			{
				Key:   "consensus_params.block.max_gas",
				Value: "1330000",
			},
		}),
		WalletConfig: petritypes.WalletConfig{
			SigningAlgorithm: string(hd.Secp256k1.Name()),
			Bech32Prefix:     "cosmos",
			HDPath:           hd.CreateHDPath(118, 0, 0),
			DerivationFn:     hd.Secp256k1.Derive(),
			GenerationFn:     hd.Secp256k1.Generate(),
		},
	}
)

func TestPetriIntegration(t *testing.T) {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger, _ := zap.NewDevelopment()

	p, err := docker.CreateProvider(ctx, logger, "docker_provider")
	if err != nil {
		t.Fatal("Provider creation error", zap.Error(err))
		return
	}

	defer func() {
		err := p.Teardown(ctx)
		if err != nil {
			t.Logf("Failed to teardown provider: %v", err)
		}
	}()

	c, err := chain.CreateChain(ctx, logger, p, defaultChainConfig, defaultChainOptions)
	if err != nil {
		t.Fatal("Chain creation error", zap.Error(err))
	}
	err = c.Init(ctx, defaultChainOptions)
	if err != nil {
		t.Fatal("Failed to init chain", zap.Error(err))
	}
	err = c.WaitForStartup(ctx)
	if err != nil {
		t.Fatal("Failed to wait for chain startup", zap.Error(err))
	}

	// Add a delay to ensure the node is fully ready
	time.Sleep(5 * time.Second)

	k1, err := c.CreateWallet(ctx, "key1", defaultChainOptions.WalletConfig)
	if err != nil {
		t.Fatal("Failed to create key1", zap.Error(err))
	}
	k2, err := c.CreateWallet(ctx, "key2", defaultChainOptions.WalletConfig)
	if err != nil {
		t.Fatal("Failed to create key2", zap.Error(err))
	}
	k1PrivKey, err := k1.PrivateKey()
	if err != nil {
		t.Fatal("Failed to get k1 private key", zap.Error(err))
	}
	k2PrivKey, err := k2.PrivateKey()
	if err != nil {
		t.Fatal("Failed to get k2 private key", zap.Error(err))
	}

	faucetWallet := c.GetFaucetWallet()
	logger.Info("Faucet wallet", zap.String("address", faucetWallet.FormattedAddress()))

	node := c.GetNodes()[0]
	command := []string{
		defaultChainConfig.BinaryName,
		"tx", "bank", "send",
		faucetWallet.FormattedAddress(),
		k1.FormattedAddress(),
		"1000000000stake",
		"--chain-id", defaultChainConfig.ChainId,
		"--keyring-backend", "test",
		"--fees", "100stake",
		"--yes",
		"--home", defaultChainConfig.HomeDir,
	}
	stdout, stderr, exitCode, err := node.RunCommand(ctx, command)
	if err != nil || exitCode != 0 {
		t.Fatal("Failed to fund wallet 1", zap.Error(err), zap.String("stderr", stderr))
	}
	logger.Debug("Fund wallet 1 response", zap.String("stdout", stdout))

	// Wait for first send to complete to avoid nonce issues
	time.Sleep(10 * time.Second)

	command = []string{
		defaultChainConfig.BinaryName,
		"tx", "bank", "send",
		faucetWallet.FormattedAddress(),
		k2.FormattedAddress(),
		"1000000000stake",
		"--chain-id", defaultChainConfig.ChainId,
		"--keyring-backend", "test",
		"--fees", "100stake",
		"--yes",
		"--home", defaultChainConfig.HomeDir,
	}
	stdout, stderr, exitCode, err = node.RunCommand(ctx, command)
	if err != nil || exitCode != 0 {
		t.Fatal("Failed to fund wallet 2", zap.Error(err), zap.String("stderr", stderr))
	}
	logger.Debug("Fund wallet 2 response", zap.String("stdout", stdout))

	// Wait for transactions to be included in a block
	time.Sleep(10 * time.Second)

	grpcAddress, err := c.GetNodes()[0].GetExternalAddress(ctx, "9090")
	if err != nil {
		t.Fatal("Failed to get node grpc address", zap.Error(err))
	}
	rpcAddress, err := c.GetNodes()[0].GetExternalAddress(ctx, "26657")
	if err != nil {
		t.Fatal("Failed to get node rpc address", zap.Error(err))
	}

	logger.Info("Node addresses",
		zap.String("grpc", grpcAddress),
		zap.String("rpc", rpcAddress))

	spec := loadtesttypes.LoadTestSpec{
		ChainID:             defaultChainConfig.ChainId,
		BlockGasLimitTarget: 0.3,
		Runtime:             1 * time.Minute,
		NumOfBlocks:         20,
		NodesAddresses: []loadtesttypes.NodeAddress{
			{
				GRPC: grpcAddress,
				RPC:  "http://" + rpcAddress,
			},
		},
		PrivateKeys:  []cryptotypes.PrivKey{k1PrivKey, k2PrivKey},
		GasDenom:     defaultChainConfig.Denom,
		Bech32Prefix: defaultChainConfig.Bech32Prefix,
	}

	test, err := loadtest.New(ctx, spec)
	if err != nil {
		t.Fatal("Failed to create test", zap.Error(err))
	}

	result, err := test.Run(ctx)
	if err != nil {
		t.Fatal("Failed to run load test", zap.Error(err))
	}

	fmt.Printf("Load test results: %+v\n", result)
}
