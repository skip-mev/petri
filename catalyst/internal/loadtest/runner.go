package loadtest

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/rand"
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
	spec               types.LoadTestSpec
	clients            []*client.Chain
	wallets            []*wallet.InteractingWallet
	txGasEstimation    int64
	gasLimit           int
	numTxs             int
	mu                 sync.Mutex
	numBlocksProcessed int
	collector          metrics.MetricsCollector
	logger             *zap.Logger
	nonces             map[string]uint64
	noncesMu           sync.RWMutex
	sentTxs            []types.SentTx
	sentTxsMu          sync.RWMutex
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

	var wallets []*wallet.InteractingWallet
	for i, privKey := range spec.PrivateKeys {
		client := clients[i%len(clients)]
		wallet := wallet.NewInteractingWallet(privKey, spec.Bech32Prefix, client)
		wallets = append(wallets, wallet)
	}

	runner := &Runner{
		spec:      spec,
		clients:   clients,
		wallets:   wallets,
		collector: metrics.NewMetricsCollector(),
		logger:    logging.FromContext(ctx),
		nonces:    make(map[string]uint64),
		sentTxs:   make([]types.SentTx, 0),
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
	client := r.clients[0]

	blockGasLimit, err := client.GetGasLimit()
	if err != nil {
		return fmt.Errorf("failed to get block gas limit: %w", err)
	}

	r.gasLimit = blockGasLimit
	if r.spec.BlockGasLimitTarget <= 0 || r.spec.BlockGasLimitTarget > 1 {
		return fmt.Errorf("block gas limit target must be between 0 and 1, got %f", r.spec.BlockGasLimitTarget)
	}

	fromWallet := r.wallets[0]
	amount := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, math.NewInt(1000000)))
	var toAddr []byte
	if len(r.wallets) == 1 {
		toAddr = fromWallet.Address()
	} else {
		toAddr = r.wallets[1].Address()
	}

	msg := banktypes.NewMsgSend(fromWallet.Address(), toAddr, amount)

	acc, err := client.GetAccount(ctx, fromWallet.FormattedAddress())
	if err != nil {
		return fmt.Errorf("failed to get account: %w", err)
	}

	tx, err := fromWallet.CreateSignedTx(ctx, client, 0, sdk.Coins{}, acc.GetSequence(), acc.GetAccountNumber(), msg)
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

	r.txGasEstimation = int64(gasUsed)
	targetGasLimit := float64(blockGasLimit) * r.spec.BlockGasLimitTarget
	r.numTxs = int(targetGasLimit / float64(gasUsed))

	r.logger.Info("Gas estimation results",
		zap.Int("blockGasLimit", blockGasLimit),
		zap.Uint64("txGasEstimation", gasUsed),
		zap.Float64("targetGasLimit", targetGasLimit),
		zap.Int("numTxs", r.numTxs))

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

				_, err := r.sendBlockTransactions(ctx)
				if err != nil {
					r.logger.Error("error sending block transactions", zap.Error(err))
				}

				if r.numBlocksProcessed >= r.spec.NumOfBlocks {
					r.logger.Info("Load test completed- number of blocks desired reached",
						zap.Int("blocks", r.numBlocksProcessed))
					r.mu.Unlock()
					cancelSub()
					done <- struct{}{}
					return
				}

				if time.Since(startTime) >= r.spec.Runtime {
					r.logger.Info("Load test completed - runtime elapsed")
					r.mu.Unlock()
					cancelSub()
					done <- struct{}{}
					return
				}

				r.mu.Unlock()
				r.numBlocksProcessed++
				r.logger.Info("processed block", zap.Int64("height", block.Height))
			}
		}
	}()

	select {
	case <-ctx.Done():
		r.logger.Info("Load test interrupted")
		return types.LoadTestResult{}, ctx.Err()
	case <-done:
		if err := <-subscriptionErr; err != nil && err != context.Canceled {
			return types.LoadTestResult{}, fmt.Errorf("subscription error: %w", err)
		}
		client := r.clients[0]
		r.collector.ProcessSentTxs(ctx, r.sentTxs, r.gasLimit, client)
	case err := <-subscriptionErr:
		// Subscription ended with error before completion
		if err != context.Canceled {
			return types.LoadTestResult{}, fmt.Errorf("failed to subscribe to blocks: %w", err)
		}
	}

	return r.collector.GetResults(), nil
}

// sendBlockTransactions sends transactions for a single block
func (r *Runner) sendBlockTransactions(ctx context.Context) (int, error) {
	results := make(chan types.SentTx, r.numTxs)
	txsSent := 0
	var wg sync.WaitGroup
	for i := 0; i < r.numTxs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			wallet := r.wallets[rand.Intn(len(r.wallets))]
			amount := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, math.NewInt(1000000)))
			toAddr := r.wallets[rand.Intn(len(r.wallets))].Address()

			client := wallet.GetClient()

			fromAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(wallet.Address()))
			if err != nil {
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
				return
			}

			toAccAddress, err := sdk.AccAddressFromHexUnsafe(hex.EncodeToString(toAddr))
			if err != nil {
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
				return
			}

			msg := banktypes.NewMsgSend(fromAccAddress, toAccAddress, amount)
			gasWithBuffer := uint64(float64(r.txGasEstimation) * 1.4)
			fees := sdk.NewCoins(sdk.NewCoin(r.spec.GasDenom, math.NewInt(int64(gasWithBuffer)*1)))

			r.noncesMu.Lock()
			nonce := r.nonces[wallet.FormattedAddress()]

			acc, err := client.GetAccount(ctx, wallet.FormattedAddress())
			if err != nil {
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
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
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
				return
			}

			txBytes, err := client.GetEncodingConfig().TxConfig.TxEncoder()(tx)
			if err != nil {
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
				return
			}

			res, err := client.BroadcastTx(ctx, txBytes)
			if err != nil {
				results <- types.SentTx{Err: err, NodeAddress: client.GetNodeAddress().RPC}
				return
			}
			r.nonces[wallet.FormattedAddress()]++
			r.noncesMu.Unlock()

			results <- types.SentTx{
				TxHash:      res.TxHash,
				NodeAddress: client.GetNodeAddress().RPC,
				Err:         nil,
			}
		}()
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	for result := range results {
		txsSent++
		if result.Err != nil {
			r.logger.Error("failed to send transaction",
				zap.Error(result.Err),
				zap.String("node", result.NodeAddress))
			continue
		}

		r.sentTxsMu.Lock()
		r.sentTxs = append(r.sentTxs, result)
		r.sentTxsMu.Unlock()
	}

	return txsSent, nil
}
