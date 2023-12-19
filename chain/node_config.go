package chain

import (
	"bytes"
	"context"
	"fmt"
	"github.com/pelletier/go-toml/v2"
)

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
