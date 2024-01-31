package types

import (
	"context"
	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/skip-mev/petri/core/v2/provider"
	"go.uber.org/zap"
	"google.golang.org/grpc"
)

type NodeConfig struct {
	Name  string
	Index int

	IsValidator bool

	Chain    ChainI
	Provider provider.Provider
}

type NodeDefinitionModifier func(provider.TaskDefinition, NodeConfig) provider.TaskDefinition

type NodeCreator func(context.Context, *zap.Logger, NodeConfig) (NodeI, error)

type NodeI interface {
	GetConfig() NodeConfig

	GetTMClient(context.Context) (*rpchttp.HTTP, error)
	GetGRPCClient(context.Context) (*grpc.ClientConn, error)
	Height(context.Context) (uint64, error)

	InitHome(context.Context) error

	AddGenesisAccount(context.Context, string, []sdk.Coin) error
	GenerateGenTx(context.Context, sdk.Coin) error
	CopyGenTx(context.Context, NodeI) error
	CollectGenTxs(context.Context) error

	GenesisFileContent(context.Context) ([]byte, error)
	OverwriteGenesisFile(context.Context, []byte) error

	CreateWallet(context.Context, string, WalletConfig) (WalletI, error)
	RecoverKey(context.Context, string, string) error
	KeyBech32(context.Context, string, string) (string, error)

	SetDefaultConfigs(context.Context) error
	SetPersistentPeers(context.Context, string) error

	NodeId(context.Context) (string, error)

	GetTask() *provider.Task
	GetIP(context.Context) (string, error)
}
