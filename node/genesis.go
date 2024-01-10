package node

import (
	"context"
	"fmt"
	"github.com/cosmos/cosmos-sdk/crypto/keyring"
	"github.com/cosmos/cosmos-sdk/types"
	petritypes "github.com/skip-mev/petri/types"
	"go.uber.org/zap"
	"strings"
	"time"
)

func (n *Node) GenesisFileContent(ctx context.Context) ([]byte, error) {
	n.logger.Info("reading genesis file", zap.String("node", n.Definition.Name))

	bz, err := n.Task.ReadFile(ctx, "config/genesis.json")

	if err != nil {
		return nil, err
	}

	return bz, err
}

func (n *Node) CopyGenTx(ctx context.Context, dstNode petritypes.NodeI) error {
	n.logger.Info("copying gen tx", zap.String("from", n.GetConfig().Name), zap.String("to", dstNode.GetConfig().Name))

	nid, err := n.NodeId(ctx)

	if err != nil {
		return err
	}

	path := fmt.Sprintf("config/gentx/gentx-%s.json", nid)

	n.logger.Debug("reading gen tx", zap.String("node", n.GetConfig().Name))
	gentx, err := n.Task.ReadFile(context.Background(), path)

	if err != nil {
		return err
	}

	n.logger.Debug("writing gen tx", zap.String("node", dstNode.GetConfig().Name))
	return dstNode.GetTask().WriteFile(context.Background(), path, gentx)
}

func (n *Node) AddGenesisAccount(ctx context.Context, address string, genesisAmounts []types.Coin) error {
	n.logger.Debug("adding genesis account", zap.String("node", n.Definition.Name), zap.String("address", address))

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

func (n *Node) GenerateGenTx(ctx context.Context, genesisSelfDelegation types.Coin) error {
	n.logger.Info("generating genesis transaction", zap.String("node", n.Definition.Name))

	chainConfig := n.chain.GetConfig()

	var command []string

	command = append(command, "genesis", "gentx", petritypes.ValidatorKeyName, fmt.Sprintf("%d%s", genesisSelfDelegation.Amount.Int64(), genesisSelfDelegation.Denom),
		"--keyring-backend", keyring.BackendTest,
		"--chain-id", chainConfig.ChainId)

	command = n.BinCommand(command...)

	_, _, err := n.Task.RunCommand(ctx, command)

	return err
}

func (n *Node) CollectGenTxs(ctx context.Context) error {
	n.logger.Info("collecting genesis transactions", zap.String("node", n.Definition.Name))

	chainConfig := n.chain.GetConfig()

	_, _, err := n.Task.RunCommand(ctx, n.BinCommand([]string{"genesis", "collect-gentxs", "--home", chainConfig.HomeDir}...))

	return err
}

func (n *Node) OverwriteGenesisFile(ctx context.Context, bz []byte) error {
	n.logger.Info("overwriting genesis file", zap.String("node", n.Definition.Name))

	return n.Task.WriteFile(ctx, "config/genesis.json", bz)
}
