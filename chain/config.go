package chain

import (
	"fmt"
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

	cometBftConfig["consensusConfig"] = consensusConfig

	rpc := make(Toml)

	// Enable public RPC
	rpc["laddr"] = "tcp://0.0.0.0:26657"
	rpc["allowed_origins"] = []string{"*"}

	cometBftConfig["rpc"] = rpc

	return cometBftConfig
}

func GenerateDefaultAppConfig(c *Chain) Toml {
	sdkConfig := make(Toml)
	sdkConfig["minimum-gas-prices"] = c.Config.GasPrices

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
