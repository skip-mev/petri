package node

import (
	"bytes"
	"context"
	"fmt"

	"github.com/cosmos/cosmos-sdk/crypto/keyring"

	"github.com/pelletier/go-toml/v2"

	"reflect"
	"time"

	petritypes "github.com/skip-mev/petri/core/v3/types"
)

type Toml map[string]any

// recursiveModifyToml will apply toml modifications at the current depth,
// then recurse for new depths
func recursiveModifyToml(c map[string]any, modifications Toml) error {
	for key, value := range modifications {
		if reflect.ValueOf(value).Kind() == reflect.Map {
			cV, ok := c[key]
			if !ok {
				// Did not find section in existing config, populating fresh.
				cV = make(Toml)
			}
			// Retrieve existing config to apply overrides to.
			cVM, ok := cV.(map[string]any)

			if !ok {
				cVM = make(Toml)
			}
			if err := recursiveModifyToml(cVM, value.(Toml)); err != nil {
				return err
			}
			c[key] = cVM
		} else {
			// Not a map, so we can set override value directly.
			c[key] = value
		}
	}
	return nil
}
func GenerateDefaultClientConfig(chainID string) Toml {
	clientConfig := make(Toml)

	clientConfig["chain-id"] = chainID
	clientConfig["keyring-backend"] = keyring.BackendTest
	clientConfig["output"] = "text"
	clientConfig["node"] = "http://localhost:26657"
	clientConfig["broadcast-mode"] = "sync"

	return clientConfig
}

// GenerateDefaultConsensusConfig returns a default / sensible config for CometBFT
func GenerateDefaultConsensusConfig() Toml {
	cometBftConfig := make(Toml)

	// Set Log Level to info
	cometBftConfig["log_level"] = "info"

	p2p := make(Toml)

	// Allow p2p strangeness
	p2p["allow_duplicate_ip"] = true
	p2p["addr_book_strict"] = false

	cometBftConfig["p2p"] = p2p

	consensusConfig := make(Toml)

	blockTime := (time.Duration(2) * time.Second).String() // todo(zygimantass): make configurable
	consensusConfig["timeout_commit"] = blockTime
	consensusConfig["timeout_propose"] = blockTime

	cometBftConfig["consensus"] = consensusConfig

	instrumentationConfig := make(Toml)
	instrumentationConfig["prometheus"] = true

	cometBftConfig["instrumentation"] = instrumentationConfig

	rpc := make(Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"
	rpc["allowed_origins"] = []string{"*"}

	cometBftConfig["rpc"] = rpc

	return cometBftConfig
}

// GenerateDefaultAppConfig returns a default / sensible config for the Cosmos SDK
func GenerateDefaultAppConfig(c petritypes.ChainConfig) Toml {
	sdkConfig := make(Toml)
	sdkConfig["minimum-gas-prices"] = c.GasPrices

	grpc := make(Toml)

	// Enable public GRPC
	grpc["address"] = "0.0.0.0:9090"

	sdkConfig["grpc"] = grpc

	api := make(Toml)

	// Enable public REST API
	api["enable"] = true
	api["swagger"] = true
	api["address"] = "tcp://0.0.0.0:1317"

	sdkConfig["api"] = api

	telemetry := make(Toml)
	telemetry["enabled"] = true
	telemetry["prometheus-retention-time"] = 3600

	sdkConfig["telemetry"] = telemetry

	if c.IsEVMChain {
		evm := make(Toml)
		evm["tracer"] = ""
		evm["max-tx-gas-wanted"] = 0
		evm["cache-preimage"] = false
		evm["evm-chain-id"] = c.EVMConfig.ChainId

		sdkConfig["evm"] = evm

		jsonRPC := make(Toml)
		jsonRPC["enable"] = true
		jsonRPC["address"] = "127.0.0.1:8545"
		jsonRPC["ws-address"] = "127.0.0.1:8546"
		jsonRPC["api"] = "eth,net,web3"
		jsonRPC["gas-cap"] = 25000000
		jsonRPC["allow-insecure-unlock"] = true
		jsonRPC["evm-timeout"] = "5s"
		jsonRPC["txfee-cap"] = 1
		jsonRPC["filter-cap"] = 200
		jsonRPC["feehistory-cap"] = 100
		jsonRPC["logs-cap"] = 10000
		jsonRPC["block-range-cap"] = 10000
		jsonRPC["http-timeout"] = "30s"
		jsonRPC["http-idle-timeout"] = "2m0s"
		jsonRPC["allow-unprotected-txs"] = false
		jsonRPC["max-open-connections"] = 0
		jsonRPC["enable-indexer"] = false
		jsonRPC["metrics-address"] = "127.0.0.1:6065"
		jsonRPC["fix-revert-gas-refund-height"] = 0

		sdkConfig["json-rpc"] = jsonRPC
	}

	return sdkConfig
}

// ModifyTomlConfigFile will modify a TOML config file at filePath with the provided modifications.
// If a certain path defined in modifications is not found in the existing config, it will return an error
func (n *Node) ModifyTomlConfigFile(
	ctx context.Context,
	filePath string,
	modifications Toml,
) error {
	config, err := n.ReadFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", filePath, err)
	}

	var c Toml
	if err := toml.Unmarshal(config, &c); err != nil {
		return fmt.Errorf("failed to unmarshal %s: %w", filePath, err)
	}

	if err := recursiveModifyToml(c, modifications); err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	if err := toml.NewEncoder(buf).Encode(c); err != nil {
		return err
	}

	if err := n.WriteFile(ctx, filePath, buf.Bytes()); err != nil {
		return fmt.Errorf("overwriting %s: %w", filePath, err)
	}

	return nil
}

// SetDefaultConfigs will generate the default configs for CometBFT and the app, and write them to disk
func (n *Node) SetDefaultConfigs(ctx context.Context, chainID string) error {
	appConfig := GenerateDefaultAppConfig(n.GetChainConfig())
	consensusConfig := GenerateDefaultConsensusConfig()
	clientConfig := GenerateDefaultClientConfig(chainID)

	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/app.toml",
		appConfig,
	); err != nil {
		return err
	}

	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/config.toml",
		consensusConfig,
	); err != nil {
		return err
	}

	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/client.toml",
		clientConfig,
	); err != nil {
		return err
	}

	return nil
}

// SetPersistentPeers will set the node's persistent peers in the CometBFT config
func (n *Node) SetPersistentPeers(ctx context.Context, peers string) error {
	cometBftConfig := make(Toml)

	p2pConfig := make(Toml)
	p2pConfig["persistent_peers"] = peers

	cometBftConfig["p2p"] = p2pConfig

	return n.ModifyTomlConfigFile(
		ctx,
		"config/config.toml",
		cometBftConfig,
	)
}
