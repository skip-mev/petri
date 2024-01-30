package node

import (
	"bytes"
	"context"
	"fmt"
	toml "github.com/pelletier/go-toml/v2"
	petritypes "github.com/skip-mev/petri/general/v2/types"
	"reflect"
	"time"
)

type Toml map[string]any

// recursiveModifyToml will apply toml modifications at the current depth,
// then recurse for new depths.
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
				return fmt.Errorf("failed to convert section to (map[string]any), found (%T)", cV)
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

func GenerateDefaultAppConfig(c petritypes.ChainI) Toml {
	sdkConfig := make(Toml)
	sdkConfig["minimum-gas-prices"] = c.GetConfig().GasPrices

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

	return sdkConfig
}

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

func (n *Node) SetDefaultConfigs(ctx context.Context) error {
	appConfig := GenerateDefaultAppConfig(n.chain)
	consensusConfig := GenerateDefaultConsensusConfig()

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

	return nil
}

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
