package main

import (
	"context"
	"encoding/base64"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
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
}

func main() {
	logger, _ := zap.NewDevelopment()
	defer logger.Sync()

	configPath := flag.String("config", "", "Path to the YAML configuration file")
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
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	test, err := loadtest.New(ctx, spec)
	if err != nil {
		logger.Fatal("Failed to create load test", zap.Error(err))
	}

	result, err := test.Run(ctx)
	if err != nil {
		logger.Fatal("Failed to run load test", zap.Error(err))
	}

	fmt.Println("Load Test Results:")
	fmt.Println("=================")
	fmt.Println()

	fmt.Println("General Statistics:")
	fmt.Println("-----------------")
	fmt.Printf("Total Transactions: %d\n", result.TotalTransactions)
	fmt.Printf("Successful Transactions: %d\n", result.SuccessfulTransactions)
	fmt.Printf("Failed Transactions: %d\n", result.FailedTransactions)
	fmt.Printf("Average Gas Per Transaction: %d\n", result.AvgGasPerTransaction)
	fmt.Printf("Average Block Gas Utilization: %.2f%%\n", result.AvgBlockGasUtilization*100)
	fmt.Printf("Blocks Processed: %d\n", result.BlocksProcessed)
	fmt.Println()

	fmt.Println("Timing:")
	fmt.Println("-------")
	fmt.Printf("Start Time: %s\n", result.StartTime.Format(time.RFC3339))
	fmt.Printf("End Time: %s\n", result.EndTime.Format(time.RFC3339))
	fmt.Printf("Total Runtime: %s\n", result.Runtime)

	if len(result.BroadcastErrors) > 0 {
		fmt.Println()
		fmt.Println("Broadcast Errors:")
		fmt.Println("----------------")
		for i, err := range result.BroadcastErrors {
			fmt.Printf("%d. Block Height: %d, Tx Hash: %s\n   Error: %s\n",
				i+1, err.BlockHeight, err.TxHash, err.Error)
		}
	}

	if len(result.BlockStats) > 0 {
		fmt.Println()
		fmt.Println("Block Statistics:")
		fmt.Println("----------------")
		for _, stat := range result.BlockStats {
			fmt.Printf("Block %d:\n", stat.BlockHeight)
			fmt.Printf("  Transactions Sent: %d\n", stat.TransactionsSent)
			fmt.Printf("  Successful Transactions: %d\n", stat.SuccessfulTxs)
			fmt.Printf("  Failed Transactions: %d\n", stat.FailedTxs)
			fmt.Printf("  Gas Limit: %d\n", stat.GasLimit)
			fmt.Printf("  Total Gas Used: %d\n", stat.TotalGasUsed)
			fmt.Printf("  Block Gas Utilization: %.2f%%\n", stat.BlockGasUtilization*100)
		}
	}

	if len(result.NodeDistribution) > 0 {
		fmt.Println()
		fmt.Println("Node Distribution:")
		fmt.Println("-----------------")
		for addr, stats := range result.NodeDistribution {
			fmt.Printf("Node %s:\n", addr)
			fmt.Printf("  GRPC Address: %s\n", stats.NodeAddresses.GRPC)
			fmt.Printf("  RPC Address: %s\n", stats.NodeAddresses.RPC)
			fmt.Printf("  Transactions Sent: %d\n", stats.TransactionsSent)
			fmt.Printf("  Successful Transactions: %d\n", stats.SuccessfulTxs)
			fmt.Printf("  Failed Transactions: %d\n", stats.FailedTxs)
			fmt.Printf("  Block Participation: %d\n", stats.BlockParticipation)
		}
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
