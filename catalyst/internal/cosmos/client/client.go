package client

import (
	"context"
	"fmt"
	"time"

	logging "github.com/skip-mev/catalyst/internal/shared"

	sdkClient "github.com/cosmos/cosmos-sdk/client"
	"github.com/cosmos/cosmos-sdk/codec"
	codectypes "github.com/cosmos/cosmos-sdk/codec/types"
	"github.com/cosmos/cosmos-sdk/std"
	"github.com/cosmos/cosmos-sdk/types/tx/signing"
	authtx "github.com/cosmos/cosmos-sdk/x/auth/tx"

	rpchttp "github.com/cometbft/cometbft/rpc/client/http"
	"github.com/cometbft/cometbft/rpc/jsonrpc/client"
	tmtypes "github.com/cometbft/cometbft/types"
	sdk "github.com/cosmos/cosmos-sdk/types"
	txtypes "github.com/cosmos/cosmos-sdk/types/tx"
	authtypes "github.com/cosmos/cosmos-sdk/x/auth/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/skip-mev/catalyst/internal/types"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var _ types.ChainI = (*Chain)(nil)

type Chain struct {
	cometClient    *rpchttp.HTTP
	txClient       txtypes.ServiceClient
	encodingConfig types.EncodingConfig
	gRPCConn       *grpc.ClientConn
	logger         *zap.Logger
	chainID        string
	nodeAddress    types.NodeAddress
}

func NewClient(ctx context.Context, rpcAddress, grpcAddress, chainID string) (*Chain, error) {
	httpClient, err := client.DefaultHTTPClient(rpcAddress)
	if err != nil {
		return nil, fmt.Errorf("failed to create http client: %w", err)
	}

	rpcClient, err := rpchttp.NewWithClient(rpcAddress, "/websocket", httpClient)
	if err != nil {
		return nil, fmt.Errorf("failed to create rpc client: %w", err)
	}

	if err := rpcClient.Start(); err != nil {
		return nil, fmt.Errorf("failed to start rpc client: %w", err)
	}

	grpcConn, err := grpc.Dial(
		grpcAddress,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create grpc connection: %w", err)
	}

	c := &Chain{
		cometClient: rpcClient,
		txClient:    txtypes.NewServiceClient(grpcConn),
		gRPCConn:    grpcConn,
		chainID:     chainID,
		nodeAddress: types.NodeAddress{
			RPC:  rpcAddress,
			GRPC: grpcAddress,
		},
		encodingConfig: types.EncodingConfig{
			InterfaceRegistry: getInterfaceRegistry(),
			Codec:             getCodec(),
			TxConfig:          getTxConfig(),
		},
		logger: logging.FromContext(ctx),
	}

	status, err := c.cometClient.Status(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get node status: %w", err)
	}

	nodeChainID := status.NodeInfo.Network
	if nodeChainID != chainID {
		return nil, fmt.Errorf("chain ID mismatch: node reports %s but load test expects %s", nodeChainID, chainID)
	}

	return c, nil
}

func (c *Chain) SubscribeToBlocks(ctx context.Context, handler types.BlockHandler) error {
	query := fmt.Sprintf("%s = '%s'", tmtypes.EventTypeKey, tmtypes.EventNewBlock)

	eventCh, err := c.cometClient.Subscribe(ctx, "loadtest", query, 100)
	if err != nil {
		return fmt.Errorf("failed to subscribe to blocks: %w", err)
	}

	defer func() {
		err := c.cometClient.Unsubscribe(ctx, "loadtest", query)
		if err != nil {
			fmt.Printf("failed to unsubscribe: %v\n", err)
		}
	}()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case event, ok := <-eventCh:
			if !ok {
				return fmt.Errorf("event channel closed unexpectedly")
			}

			newBlockEvent, ok := event.Data.(tmtypes.EventDataNewBlock)
			if !ok {
				c.logger.Error("Unexpected event type",
					zap.Any("Event data received", event.Data))
				continue
			}
			c.logger.Debug("received new block event", zap.Int64("height", newBlockEvent.Block.Height))

			params, err := c.cometClient.ConsensusParams(ctx, nil)
			if err != nil {
				c.logger.Error("Failed to get consensus params from the block", zap.Error(err))
				continue
			}

			block := types.Block{
				Height:    newBlockEvent.Block.Height,
				GasLimit:  params.ConsensusParams.Block.MaxGas,
				Timestamp: newBlockEvent.Block.Time,
			}
			handler(block)
		}
	}
}

func (c *Chain) GetGasLimit() (int, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	block, err := c.cometClient.Block(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get latest block: %w", err)
	}

	if block.Block == nil {
		return 0, fmt.Errorf("block is nil")
	}

	height := block.Block.Height
	params, err := c.cometClient.ConsensusParams(ctx, &height)
	if err != nil {
		return 0, fmt.Errorf("failed to get consensus params: %w", err)
	}

	if params == nil {
		return 0, fmt.Errorf("consensus params response is nil")
	}

	maxGas := params.ConsensusParams.Block.MaxGas
	if maxGas <= 0 {
		return 0, fmt.Errorf("invalid max gas value: %d", maxGas)
	}

	return int(maxGas), nil
}

func (c *Chain) EstimateGasUsed(ctx context.Context, txBz []byte) (uint64, error) {
	r, err := c.txClient.Simulate(ctx, &txtypes.SimulateRequest{TxBytes: txBz})
	if err != nil {
		return 0, fmt.Errorf("failed to simulate transaction: %w", err)
	}

	return r.GasInfo.GasUsed, nil
}

func (c *Chain) BroadcastTx(ctx context.Context, txBytes []byte) (*sdk.TxResponse, error) {
	resp, err := c.txClient.BroadcastTx(ctx, &txtypes.BroadcastTxRequest{
		TxBytes: txBytes,
		Mode:    txtypes.BroadcastMode_BROADCAST_MODE_SYNC,
	})
	if err != nil {
		return resp.TxResponse, err
	}

	if resp.TxResponse.Code != 0 {
		c.logger.Error("Failed to broadcast transaction", zap.String("tx_hash", resp.TxResponse.TxHash),
			zap.Uint32("code", resp.TxResponse.Code), zap.String("raw_log", resp.TxResponse.RawLog))
		return resp.TxResponse, fmt.Errorf("transaction %s failed with error code: %d. Raw log: %s",
			resp.TxResponse.TxHash, resp.TxResponse.Code, resp.TxResponse.RawLog)
	}

	return resp.TxResponse, nil
}

func (c *Chain) GetAccount(ctx context.Context, address string) (sdk.AccountI, error) {
	authClient, err := c.getAuthClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get auth client: %w", err)
	}

	res, err := authClient.Account(ctx, &authtypes.QueryAccountRequest{
		Address: address,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to query account: %w", err)
	}

	var acc sdk.AccountI
	err = c.encodingConfig.InterfaceRegistry.UnpackAny(res.Account, &acc)
	if err != nil {
		return nil, fmt.Errorf("failed to unpack account: %w", err)
	}

	return acc, nil
}

func (c *Chain) GetNodeAddress() types.NodeAddress {
	return c.nodeAddress
}

func (c *Chain) GetEncodingConfig() types.EncodingConfig {
	return c.encodingConfig
}

func (c *Chain) GetChainID() string {
	return c.chainID
}

func (c *Chain) getAuthClient(ctx context.Context) (authtypes.QueryClient, error) {
	return authtypes.NewQueryClient(c.gRPCConn), nil
}

func (c *Chain) getBankClient(ctx context.Context) (banktypes.QueryClient, error) {
	return banktypes.NewQueryClient(c.gRPCConn), nil
}

func (c *Chain) GetTxClient(ctx context.Context) txtypes.ServiceClient {
	return c.txClient
}

func (c *Chain) GetCometClient(ctx context.Context) *rpchttp.HTTP {
	return c.cometClient
}

func getInterfaceRegistry() codectypes.InterfaceRegistry {
	registry := codectypes.NewInterfaceRegistry()
	std.RegisterInterfaces(registry)
	authtypes.RegisterInterfaces(registry)
	banktypes.RegisterInterfaces(registry)
	return registry
}

func getCodec() *codec.ProtoCodec {
	registry := getInterfaceRegistry()
	return codec.NewProtoCodec(registry)
}

func getTxConfig() sdkClient.TxConfig {
	cdc := getCodec()
	signingModes := []signing.SignMode{signing.SignMode_SIGN_MODE_DIRECT}
	return authtx.NewTxConfig(cdc, signingModes)
}
