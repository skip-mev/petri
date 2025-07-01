package node

import (
	"bytes"
	"context"
	"fmt"

	"github.com/pelletier/go-toml/v2"

	"reflect"

	petritypes "github.com/skip-mev/petri/core/v3/types"
)

// recursiveModifyToml will apply toml modifications at the current depth,
// then recurse for new depths
func recursiveModifyToml(c map[string]any, modifications petritypes.Toml) error {
	for key, value := range modifications {
		if reflect.ValueOf(value).Kind() == reflect.Map {
			cV, ok := c[key]
			if !ok {
				// Did not find section in existing config, populating fresh.
				cV = make(petritypes.Toml)
			}
			// Retrieve existing config to apply overrides to.
			cVM, ok := cV.(map[string]any)

			if !ok {
				cVM = make(petritypes.Toml)
			}
			if err := recursiveModifyToml(cVM, value.(petritypes.Toml)); err != nil {
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

// ModifyTomlConfigFile will modify a TOML config file at filePath with the provided modifications.
// If a certain path defined in modifications is not found in the existing config, it will return an error
func (n *Node) ModifyTomlConfigFile(
	ctx context.Context,
	filePath string,
	modifications petritypes.Toml,
) error {
	config, err := n.ReadFile(ctx, filePath)
	if err != nil {
		return fmt.Errorf("failed to retrieve %s: %w", filePath, err)
	}

	var c petritypes.Toml
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

// SetChainConfigs will apply the provided configs to the node configuration files
func (n *Node) SetChainConfigs(ctx context.Context, chainConfig petritypes.ChainConfig) error {
	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/app.toml",
		chainConfig.AppConfig,
	); err != nil {
		return fmt.Errorf("failed to apply app config: %w", err)
	}

	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/config.toml",
		chainConfig.ConsensusConfig,
	); err != nil {
		return fmt.Errorf("failed to apply consensus config: %w", err)
	}

	if err := n.ModifyTomlConfigFile(
		ctx,
		"config/client.toml",
		chainConfig.ClientConfig,
	); err != nil {
		return fmt.Errorf("failed to apply client config: %w", err)
	}

	return nil
}

// SetPersistentPeers will set the node's persistent peers in the CometBFT config
func (n *Node) SetPersistentPeers(ctx context.Context, peers string) error {
	cometBftConfig := make(petritypes.Toml)

	p2pConfig := make(petritypes.Toml)
	p2pConfig["persistent_peers"] = peers

	cometBftConfig["p2p"] = p2pConfig

	return n.ModifyTomlConfigFile(
		ctx,
		"config/config.toml",
		cometBftConfig,
	)
}
