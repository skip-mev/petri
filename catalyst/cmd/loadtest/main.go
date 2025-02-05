package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/cosmos/cosmos-sdk/crypto/keys/secp256k1"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	"go.uber.org/zap"
	"gopkg.in/yaml.v3"

	loadtesttypes "github.com/skip-mev/catalyst/internal/types"
	"github.com/skip-mev/catalyst/loadtest"
)

type Config struct {
	ChainID             string                      `yaml:"chain_id"`
	BlockGasLimitTarget float64                     `yaml:"block_gas_limit_target"`
	Runtime             string                      `yaml:"runtime"`
	NumOfBlocks         int                         `yaml:"num_of_blocks"`
	NodesAddresses      []loadtesttypes.NodeAddress `yaml:"nodes_addresses"`
	PrivateKeys         []string                    `yaml:"private_keys"` // Base64 encoded private keys
	GasDenom            string                      `yaml:"gas_denom"`
	Bech32Prefix        string                      `yaml:"bech32_prefix"`
	Msgs                []loadtesttypes.LoadTestMsg `yaml:"msgs"`
}

func main() {
	logger, _ := zap.NewProduction()
	defer logger.Sync()

	configPath := flag.String("config", "", "Path to load test configuration file")
	flag.Parse()

	if *configPath == "" {
		logger.Fatal("Config file path is required")
	}

	configData, err := os.ReadFile(*configPath)
	if err != nil {
		logger.Fatal("Failed to read config file", zap.Error(err))
	}

	var config Config
	if err := yaml.Unmarshal(configData, &config); err != nil {
		logger.Fatal("Failed to parse config file", zap.Error(err))
	}

	duration, err := time.ParseDuration(config.Runtime)
	if err != nil {
		logger.Fatal("Failed to parse runtime duration", zap.Error(err))
	}

	var privKeys []types.PrivKey
	for _, keyStr := range config.PrivateKeys {
		privKey, err := parsePrivateKey(keyStr)
		if err != nil {
			logger.Fatal("Failed to parse private key", zap.Error(err))
		}
		privKeys = append(privKeys, privKey)
	}

	spec := loadtesttypes.LoadTestSpec{
		ChainID:             config.ChainID,
		BlockGasLimitTarget: config.BlockGasLimitTarget,
		Runtime:             duration,
		NumOfBlocks:         config.NumOfBlocks,
		NodesAddresses:      config.NodesAddresses,
		PrivateKeys:         privKeys,
		GasDenom:            config.GasDenom,
		Bech32Prefix:        config.Bech32Prefix,
		Msgs:                config.Msgs,
	}

	ctx := context.Background()
	test, err := loadtest.New(ctx, spec)
	if err != nil {
		logger.Fatal("Failed to create test", zap.Error(err))
	}

	_, err = test.Run(ctx)
	if err != nil {
		logger.Fatal("Failed to run load test", zap.Error(err))
	}
}

func parsePrivateKey(keyStr string) (types.PrivKey, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(keyStr)
	if err != nil {
		return nil, fmt.Errorf("failed to decode base64 private key: %w", err)
	}

	privKey := &secp256k1.PrivKey{Key: keyBytes}
	return privKey, nil
}
