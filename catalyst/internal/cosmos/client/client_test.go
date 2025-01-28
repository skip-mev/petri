package client

//import (
//	"context"
//	"github.com/skip-mev/catalyst/internal/node"
//	"testing"
//	"time"
//
//	"github.com/stretchr/testify/assert"
//	"github.com/stretchr/testify/require"
//)
//
//const (
//	rpcAddress  = "https://cosmos-rpc.publicnode.com:443"
//	grpcAddress = "cosmos-grpc.publicnode.com:443"
//)
//
//func TestNewClient(t *testing.T) {
//	ctx := context.Background()
//	client, err := node.NewClient(ctx, rpcAddress, grpcAddress)
//	require.NoError(t, err)
//	require.NotNil(t, client)
//	defer client.Close()
//}
//
//func TestGetLatestBlockHeight(t *testing.T) {
//	ctx := context.Background()
//	client, err := node.NewClient(ctx, rpcAddress, grpcAddress)
//	require.NoError(t, err)
//	defer client.Close()
//
//	height, err := client.GetLatestBlockHeight(ctx)
//	require.NoError(t, err)
//	assert.Greater(t, height, int64(0))
//}
//
//func TestGetGasLimit(t *testing.T) {
//	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
//	defer cancel()
//
//	client, err := node.NewClient(ctx, rpcAddress, grpcAddress)
//	require.NoError(t, err)
//	defer client.Close()
//
//	gasLimit, err := client.GetGasLimit()
//	require.NoError(t, err)
//	assert.Greater(t, gasLimit, 0)
//}
//
//func TestSubscribeToBlocks(t *testing.T) {
//	ctx := context.Background()
//
//	client, err := node.NewClient(ctx, rpcAddress, grpcAddress)
//	require.NoError(t, err)
//	defer client.Close()
//
//	blockCh := make(chan node.Block)
//	go func() {
//		err := client.SubscribeToBlocks(ctx, func(block node.Block) {
//			blockCh <- block
//		})
//		if err != nil && ctx.Err() == nil {
//			t.Errorf("SubscribeToBlocks error: %v", err)
//		}
//	}()
//
//	// Wait for at least one block
//	select {
//	case block := <-blockCh:
//		assert.Greater(t, block.Height, int64(0))
//		assert.Greater(t, block.GasLimit, int64(0))
//		assert.False(t, block.Timestamp.IsZero())
//	case <-ctx.Done():
//		t.Fatal("timeout waiting for block")
//	}
//}
