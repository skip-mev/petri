package loadtest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	logging "github.com/skip-mev/catalyst/internal/shared"
	"go.uber.org/zap"

	"cosmossdk.io/math"
	sdk "github.com/cosmos/cosmos-sdk/types"
	banktypes "github.com/cosmos/cosmos-sdk/x/bank/types"
	"github.com/skip-mev/catalyst/internal/cosmos/client"
	"github.com/skip-mev/catalyst/internal/cosmos/wallet"
	"github.com/skip-mev/catalyst/internal/metrics"
	"github.com/skip-mev/catalyst/internal/types"
)

// Runner represents a load test runner that executes a single LoadTestSpec
type Runner struct {
	spec      types.LoadTestSpec
	clients   []*client.Chain
	wallets   []*wallet.InteractingWallet
	gasUsed   int64
	gasLimit  int
	numTxs    int
	mu        sync.Mutex
	processed int
	collector metrics.MetricsCollector
	logger    *zap.Logger
	nonces    map[string]uint64
	noncesMu  sync.RWMutex
	sentTxs   []struct {
		txHash string
		wallet *wallet.InteractingWallet
		sentAt time.Time
	}
	sentTxsMu sync.RWMutex
}

// NewRunner creates a new load test runner for a given spec
func NewRunner(ctx context.Context, spec types.LoadTestSpec) (*Runner, error) {
	var clients []*client.Chain
	for _, node := range spec.NodesAddresses {
		client, err := client.NewClient(ctx, node.RPC, node.GRPC, spec.ChainID, spec.GasDenom)
		if err != nil {
			return nil, fmt.Errorf("failed to create client for node %s: %v", node.RPC, err)
		}
		clients = append(clients, client)
	}

	if len(clients) == 0 {
		return nil, fmt.Errorf("no valid clients created")
	}

	var chainClients []types.ChainI
	for _, c := range clients {
		chainClients = append(chainClients, c)
	}
	clientPool := types.NewClientPool(chainClients)
	var wallets []*wallet.InteractingWallet
	for _, privKey := range spec.PrivateKeys {
		wallet := wallet.NewInteractingWallet(privKey, spec.Bech32Prefix, clientPool)
		wallets = append(wallets, wallet)
	}

	runner := &Runner{
		spec:      spec,
		clients:   clients,
		wallets:   wallets,
		collector: metrics.NewMetricsCollector(),
		logger:    logging.FromContext(ctx),
		nonces:    make(map[string]uint64),
		sentTxs: make([]struct {
			txHash string
			wallet *wallet.InteractingWallet
			sentAt time.Time
		}, 0),
	}

	for _, w := range wallets {
		acc, err := clients[0].GetAccount(ctx, w.FormattedAddress())
		if err != nil {
			return nil, fmt.Errorf("failed to get account for wallet %s: %w", w.FormattedAddress(), err)
		}
		runner.nonces[w.FormattedAddress()] = acc.GetSequence()
	}

	if err := runner.initGasEstimation(ctx); err != nil {
		return nil, err
	}

	return runner, nil
}

// initGasEstimation performs initial gas estimation for transactions
func (r *Runner) initGasEstimation(ctx context.Context) error {
	if len(r.wallets) < 2 {
		return fmt.Errorf("need at least 2 wallets for gas estimation")
	}

	client := r.clients[0]

	blockGasLimit, err := client.GetGasLimit()
	if err != nil {
		return fmt.Errorf("failed to get block gas limit: %w", err)
	}

	r.gasLimit = blockGasLimit
	if r.spec.BlockGasLimitTarget <= 0 || r.spec.BlockGasLimitTarget > 1 {
		return fmt.Errorf("block gas limit target must be between 0 and 1, got %f", r.spec.BlockGasLimitTarget)
	}

	wallet := r.wallets[0]
	amount := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, math.NewInt(1000000)))
	toAddr := r.wallets[1].Address()
	msg := banktypes.NewMsgSend(wallet.Address(), toAddr, amount)

	acc, err := client.GetAccount(ctx, wallet.FormattedAddress())
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	tx, err := wallet.CreateSignedTx(ctx, client, 0, sdk.Coins{}, acc.GetSequence(), acc.GetAccountNumber(), msg)
	if err != nil {
		return fmt.Errorf("failed to create transaction for simulation: %w", err)
	}

	txBytes, err := client.EncodingConfig.TxConfig.TxEncoder()(tx)
	if err != nil {
		return fmt.Errorf("failed to encode transaction: %w", err)
	}

	gasUsed, err := client.EstimateGasUsed(ctx, txBytes)
	if err != nil {
		return fmt.Errorf("failed to estimate gas: %w", err)
	}

	r.gasUsed = int64(gasUsed)
	targetGasLimit := float64(blockGasLimit) * r.spec.BlockGasLimitTarget
	r.numTxs = int(targetGasLimit / float64(gasUsed))

	fmt.Printf("Gas estimation results: blockGasLimit=%d, gasUsed=%d, targetGasLimit=%f, numTxs=%d\n",
		blockGasLimit, gasUsed, targetGasLimit, r.numTxs)

	if r.numTxs <= 0 {
		return fmt.Errorf("calculated number of transactions per block is zero or negative: %d", r.numTxs)
	}

	return nil
}

func (r *Runner) Run(ctx context.Context) (types.LoadTestResult, error) {
	startTime := time.Now()
	done := make(chan struct{})

	subCtx, cancelSub := context.WithCancel(ctx)
	defer cancelSub()

	subscriptionErr := make(chan error, 1)
	blockCh := make(chan types.Block, 1)

	go func() {
		err := r.clients[0].SubscribeToBlocks(subCtx, func(block types.Block) {
			select {
			case blockCh <- block:
			case <-subCtx.Done():
				return
			}
		})
		subscriptionErr <- err
	}()

	go func() {
		for {
			select {
			case <-subCtx.Done():
				return
			case block := <-blockCh:
				r.mu.Lock()
				r.logger.Info("processing block", zap.Int64("height", block.Height),
					zap.Time("timestamp", block.Timestamp), zap.Int64("gas_limit", block.GasLimit))

				r.processed++

				txsSent, err := r.sendBlockTransactions(ctx)
				if err != nil {
					r.logger.Error("error sending block transactions", zap.Error(err))
				}
				r.logger.Info("sent block transactions successfully", zap.Int("txsSent", txsSent))

				if r.processed >= r.spec.NumOfBlocks {
					fmt.Printf("Completed %d blocks\n", r.processed)
					r.mu.Unlock()
					cancelSub()
					select {
					case done <- struct{}{}:
					default:
					}
					return
				}

				if time.Since(startTime) >= r.spec.Runtime {
					r.logger.Info("Load test completed - runtime elapsed")
					r.mu.Unlock()
					cancelSub()
					select {
					case done <- struct{}{}:
					default:
					}
					return
				}

				r.mu.Unlock()
				r.logger.Info("processed block", zap.Int64("height", block.Height))
			}
		}
	}()

	select {
	case <-ctx.Done():
		r.logger.Info("Load test interrupted")
		return types.LoadTestResult{}, ctx.Err()
	case <-done:
		// Test completed normally (either by blocks or runtime)
		// Wait for subscription to fully close and check if it was a clean shutdown
		if err := <-subscriptionErr; err != nil && err != context.Canceled {
			return types.LoadTestResult{}, fmt.Errorf("subscription error: %w", err)
		}

		r.logger.Info("querying transaction results and recording metrics")

		type blockStats struct {
			gasUsed       int64
			txsSent       int
			successfulTxs int
			failedTxs     int
			timestamp     time.Time
		}
		blockStatsMap := make(map[int64]*blockStats)

		client := r.clients[0]
		for _, tx := range r.sentTxs {
			txResp, err := tx.wallet.GetTxResponse(ctx, client, tx.txHash)
			if err != nil {
				r.logger.Error("failed to get transaction response",
					zap.String("txHash", tx.txHash),
					zap.Error(err))
				r.collector.RecordTransactionFailure(
					tx.txHash,
					err,
				)
				continue
			}

			if _, exists := blockStatsMap[txResp.Height]; !exists {
				blockStatsMap[txResp.Height] = &blockStats{}
			}
			stats := blockStatsMap[txResp.Height]
			stats.txsSent++

			if txResp.Code != 0 {
				stats.failedTxs++
				r.collector.RecordTransactionFailure(
					tx.txHash,
					fmt.Errorf("transaction failed: %s", txResp.RawLog),
				)
				continue
			}

			stats.successfulTxs++
			stats.gasUsed += txResp.GasUsed

			txTime, err := time.Parse(time.RFC3339, txResp.Timestamp)
			if err != nil {
				r.logger.Error("failed to parse tx timestamp",
					zap.String("txHash", tx.txHash),
					zap.Error(err))
				txTime = time.Now()
			}

			r.collector.RecordTransactionSuccess(
				tx.txHash,
				float64(txTime.Sub(tx.sentAt).Milliseconds()),
				txResp.GasUsed,
				client.GetNodeAddress().RPC,
			)
		}

		var heights []int64
		for height := range blockStatsMap {
			heights = append(heights, height)
		}
		sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })

		for _, height := range heights {
			stats := blockStatsMap[height]

			r.collector.RecordBlockStats(
				height,
				r.gasLimit,
				stats.gasUsed,
				stats.txsSent,
				stats.successfulTxs,
				stats.failedTxs,
			)
		}
	case err := <-subscriptionErr:
		// Subscription ended with error before completion
		if err != context.Canceled {
			return types.LoadTestResult{}, fmt.Errorf("failed to subscribe to blocks: %w", err)
		}
	}

	return r.collector.GetResults(), nil
}

// sendBlockTransactions sends transactions for a single block
func (r *Runner) sendBlockTransactions(ctx context.Context) (txsSent int, err error) {
	startTime := time.Now()

	type txResult struct {
		txHash  string
		wallet  *wallet.InteractingWallet
		err     error
		nodeRPC string
	}
	results := make(chan txResult, r.numTxs)

	var wg sync.WaitGroup
	for i := 0; i < r.numTxs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			wallet := r.wallets[rand.Intn(len(r.wallets))]
			amount := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, math.NewInt(1000000)))
			toAddr := r.wallets[rand.Intn(len(r.wallets))].Address()

			gasSettings := types.GasSettings{
				Gas:         r.gasUsed,
				PricePerGas: 1,
				GasDenom:    r.spec.GasDenom,
			}
			client := wallet.GetClient()

			fromAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(wallet.Address()))
			if err != nil {
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}

			toAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(toAddr))
			if err != nil {
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}

			msg := banktypes.NewMsgSend(fromAccAddress, toAccAddress, amount)
			gasWithBuffer := uint64(float64(gasSettings.Gas) * 1.3)
			fees := sdk.NewCoins(sdk.NewCoin(gasSettings.GasDenom, math.NewInt(int64(gasWithBuffer)*gasSettings.PricePerGas)))

			r.noncesMu.Lock()
			nonce := r.nonces[wallet.FormattedAddress()]
			r.nonces[wallet.FormattedAddress()]++

			acc, err := client.GetAccount(ctx, wallet.FormattedAddress())
			if err != nil {
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}

			tx, err := wallet.CreateSignedTx(ctx, client, gasWithBuffer, fees, nonce, acc.GetAccountNumber(), msg)
			if err != nil {
				if err.Error() == "account sequence mismatch" {
					r.logger.Error("sequence mismatch",
						zap.String("wallet", wallet.FormattedAddress()),
						zap.Uint64("nonce_tracked", nonce),
						zap.Uint64("actual_nonce", acc.GetSequence()),
						zap.Error(err))
				}
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}

			txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
			if err != nil {
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}

			res, err := client.BroadcastTx(ctx, txBytes)
			if err != nil {
				results <- txResult{err: err, nodeRPC: client.GetNodeAddress().RPC}
				return
			}
			r.noncesMu.Unlock()

			results <- txResult{
				txHash:  res.TxHash,
				wallet:  wallet,
				nodeRPC: client.GetNodeAddress().RPC,
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		txsSent++
		if result.err != nil {
			r.logger.Error("failed to send transaction",
				zap.Error(result.err),
				zap.String("node", result.nodeRPC))
			continue
		}

		r.sentTxsMu.Lock()
		r.sentTxs = append(r.sentTxs, struct {
			txHash string
			wallet *wallet.InteractingWallet
			sentAt time.Time
		}{
			txHash: result.txHash,
			wallet: result.wallet,
			sentAt: startTime,
		})
		r.sentTxsMu.Unlock()
	}

	return txsSent, nil
}
