package node

import (
	"context"
	"go.uber.org/zap"
)

func (n *Node) InitHome(ctx context.Context) error {
	n.logger.Info("initializing home", zap.String("name", n.Definition.Name))
	chainConfig := n.chain.GetConfig()

	_, _, _, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"init", n.Definition.Name, "--chain-id", chainConfig.ChainId}...))

	return err
}
