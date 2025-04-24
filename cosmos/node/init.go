package node

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// InitHome initializes the node's home directory
func (n *Node) InitHome(ctx context.Context) error {
	n.logger.Info("initializing home", zap.String("name", n.GetDefinition().Name))

	//if err := n.WriteFile(ctx, "config/client.toml", []byte("chain-id = \"\"")); err != nil {
	//	return fmt.Errorf("failed to write client.toml: %w", err)
	//}
	//
	//if err := n.ModifyTomlConfigFile(ctx, "config/client.toml", GenerateDefaultClientConfig()); err != nil {
	//	return fmt.Errorf("failed to modify client.toml: %w", err)
	//}
	//
	//stdout, stderr, exitCode, err := n.RunCommand(ctx, []string{"ls", "-lah", "/gaia/config"})
	//n.logger.Debug("ls home", zap.String("stdout", stdout), zap.String("stderr", stderr), zap.Int("exitCode", exitCode))
	stdout, stderr, exitCode, err := n.RunCommand(ctx, n.BinCommand([]string{"init", n.GetDefinition().Name, "--chain-id", n.GetChainConfig().ChainId}...))

	n.logger.Debug("init home", zap.String("stdout", stdout), zap.String("stderr", stderr), zap.Int("exitCode", exitCode))

	if err != nil {
		return fmt.Errorf("failed to init home: %w", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to init home (exit code %d): %s, stdout: %s", exitCode, stderr, stdout)
	}

	return nil
}
