package types

import (
	"context"
	"time"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"

	"github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/crypto/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

type EncodingConfig struct {
	InterfaceRegistry codectypes.InterfaceRegistry
	Codec             codec.Codec
	TxConfig          client.TxConfig
}

// BlockHandler is a callback function for new blocks
type BlockHandler func(block Block)

type ChainI interface {
	BroadcastTx(ctx context.Context, txBytes []byte) (*sdk.TxResponse, error)
	EstimateGasUsed(ctx context.Context, txBytes []byte) (uint64, error)
	GetAccount(ctx context.Context, address string) (sdk.AccountI, error)
	GetEncodingConfig() EncodingConfig
	GetChainID() string
	GetNodeAddress() NodeAddress
	SubscribeToBlocks(ctx context.Context, handler BlockHandler) error
	GetTxClient(ctx context.Context) txtypes.ServiceClient
	GetCometClient(ctx context.Context) *rpchttp.HTTP
}

type Block struct {
	Height    int64
	GasLimit  int64
	Timestamp time.Time
}

type LoadTestSpec struct {
	ChainID             string
	BlockGasLimitTarget float64 // Target percentage of block gas limit to use (0.0-1.0)
	Runtime             time.Duration
	NumOfBlocks         int
	NodesAddresses      []NodeAddress
	PrivateKeys         []types.PrivKey
	GasDenom            string
	Bech32Prefix        string
}

type NodeAddress struct {
	GRPC string `json:"grpc"`
	RPC  string `json:"rpc"`
}

type LoadTestResult struct {
	TotalTransactions      int
	SuccessfulTransactions int
	FailedTransactions     int
	BroadcastErrors        []BroadcastError
	AvgGasPerTransaction   int
	AvgBlockGasUtilization float64
	BlocksProcessed        int
	StartTime              time.Time
	EndTime                time.Time
	Runtime                time.Duration
	BlockStats             []BlockStat
	NodeStats              map[string]NodeStats
}

// BroadcastError represents errors during broadcasting transactions
type BroadcastError struct {
	BlockHeight int64  // Block height where the error occurred (0 indicates tx did not make it to a block)
	TxHash      string // Hash of the transaction that failed
	Error       string // Error message
}

// BlockStat represents stats for each individual block
type BlockStat struct {
	BlockHeight         int64   // Height of the block
	TransactionsSent    int     // Number of transactions sent for the block
	SuccessfulTxs       int     // Number of successful transactions included in the block
	FailedTxs           int     // Number of transactions that failed
	GasLimit            int     // Gas limit of the block
	TotalGasUsed        int64   // Total gas used in the block
	BlockGasUtilization float64 // Percentage of block gas limit utilized
}

// NodeStats represents stats for transactions handled by a specific node
type NodeStats struct {
	Address          string // Addresses of the node
	TransactionsSent int    // Number of transactions sent to this node
	SuccessfulTxs    int    // Number of successful transactions broadcasted to this node
	FailedTxs        int    // Number of transactions that failed on this node
}

type SentTx struct {
	TxHash      string
	NodeAddress string
	Err         error
}
