package chain

import "context"

func (n *Node) InitHome(ctx context.Context) error {
	_, _, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"init", n.Definition.Name, "--chain-id", n.chain.Config.ChainId}...))

	return err
}
