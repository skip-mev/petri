package node

import (
	"context"
	"fmt"

	"go.uber.org/zap"
)

// InitHome initializes the node's home directory
func (n *Node) InitHome(ctx context.Context) error {
	n.logger.Info("initializing home", zap.String("name", n.Definition.Name))
	chainConfig := n.chain.GetConfig()

	stdout, stderr, exitCode, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"init", n.Definition.Name, "--chain-id", chainConfig.ChainId}...))
	n.logger.Debug("init home", zap.String("stdout", stdout), zap.String("stderr", stderr), zap.Int("exitCode", exitCode))

	if err != nil {
		return fmt.Errorf("failed to init home: %w", err)
	}

	if exitCode != 0 {
		return fmt.Errorf("failed to init home (exit code %d): %s", exitCode, stderr)
	}

	return nil
}
