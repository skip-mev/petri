package node

import "context"

func (n *Node) InitHome(ctx context.Context) error {
	chainConfig := n.chain.GetConfig()

	_, _, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"init", n.Definition.Name, "--chain-id", chainConfig.ChainId}...))

	return err
}
