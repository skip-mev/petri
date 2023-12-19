package chain

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	"strings"
	"time"
)

func (n *Node) GenesisFileContent(ctx context.Context) ([]byte, error) {
	bz, err := n.Task.ReadFile(ctx, "config/genesis.json")

	if err != nil {
		return nil, err
	}

	return bz, err
}

func (n *Node) CopyGenTx(ctx context.Context, dstNode *Node) error {
	nid, err := n.NodeId(ctx)

	if err != nil {
		return err
	}

	path := fmt.Sprintf("config/gentx/gentx-%s.json", nid)

	gentx, err := n.Task.ReadFile(context.Background(), path)

	if err != nil {
		return err
	}

	return dstNode.Task.WriteFile(context.Background(), path, gentx)
}

func (n *Node) AddGenesisAccount(ctx context.Context, address string, genesisAmounts []types.Coin) error {
	amount := ""

	for i, coin := range genesisAmounts {
		if i != 0 {
			amount += ","
		}

		amount += fmt.Sprintf("%d%s", coin.Amount.Int64(), coin.Denom)
	}

	ctx, cancel := context.WithTimeout(ctx, time.Minute)
	defer cancel()

	var command []string

	command = append(command, "genesis", "add-genesis-account", strings.Replace(address, ".", "", -1), amount)
	command = n.BinCommand(command...)

	_, _, err := n.Task.RunCommand(ctx, command)

	if err != nil {
		return err
	}

	return nil
}

func (n *Node) GenerateGentx(ctx context.Context, genesisSelfDelegation types.Coin) error {
	var command []string

	command = append(command, "genesis", "gentx", validatorKey, fmt.Sprintf("%d%s", genesisSelfDelegation.Amount.Int64(), genesisSelfDelegation.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--chain-id", n.chain.Config.ChainId)

	command = n.BinCommand(command...)

	_, _, err := n.Task.RunCommand(ctx, command)

	return err
}

func (n *Node) CollectGentxs(ctx context.Context) error {
	_, _, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"genesis", "collect-gentxs", "--home", n.chain.Config.HomeDir}...))

	return err
}

func (n *Node) OverwriteGenesisFile(ctx context.Context, bz []byte) error {
	return n.Task.WriteFile(ctx, "config/genesis.json", bz)
}
