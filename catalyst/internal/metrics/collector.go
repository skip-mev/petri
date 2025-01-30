package metrics

import (
	"context"
	"fmt"
	"github.com/skip-mev/catalyst/internal/cosmos/wallet"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"

	"github.com/skip-mev/catalyst/internal/types"
)

// MetricsCollector is responsible for collecting and aggregating metrics during a load test
type MetricsCollector interface {
	RecordTransactionSuccess(txHash string, gasUsed int64, nodeAddress string)
	RecordTransactionFailure(txHash string, err error)
	RecordBlockStats(blockHeight int64, gasLimit int, gasUsed int64, txsSent int, successfulTxs int, failedTxs int)
	GetResults() types.LoadTestResult
	ProcessSentTxs(ctx context.Context, sentTxs []types.SentTx, gasLimit int, client types.ChainI)
}

// DefaultMetricsCollector implements the MetricsCollector interface
type DefaultMetricsCollector struct {
	mu sync.RWMutex

	startTime time.Time
	endTime   time.Time

	totalTxs      int
	successfulTxs int
	failedTxs     int
	logger        *zap.Logger

	totalGasUsed int64

	broadcastErrors []types.BroadcastError
	blockStats      []types.BlockStat

	nodeStats map[string]*types.NodeStats
}

var _ MetricsCollector = (*DefaultMetricsCollector)(nil)

func NewMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		startTime:  time.Now(),
		nodeStats:  make(map[string]*types.NodeStats),
		blockStats: make([]types.BlockStat, 0),
		logger:     zap.NewNop(),
	}
}

func (c *DefaultMetricsCollector) ProcessSentTxs(ctx context.Context, sentTxs []types.SentTx, gasLimit int, client types.ChainI) {
	c.logger.Info("querying transaction results and recording metrics")

	type blockStats struct {
		gasUsed       int64
		txsSent       int
		successfulTxs int
		failedTxs     int
		timestamp     time.Time
	}
	blockStatsMap := make(map[int64]*blockStats)

	for _, tx := range sentTxs {
		txResp, err := wallet.GetTxResponse(ctx, client, tx.TxHash)
		if err != nil {
			c.logger.Error("failed to get transaction response",
				zap.String("txHash", tx.TxHash),
				zap.Error(err))
			c.RecordTransactionFailure(
				tx.TxHash,
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
			c.RecordTransactionFailure(
				tx.TxHash,
				fmt.Errorf("transaction failed: %s", txResp.RawLog),
			)
			continue
		}

		stats.successfulTxs++
		stats.gasUsed += txResp.GasUsed

		c.RecordTransactionSuccess(
			tx.TxHash,
			txResp.GasUsed,
			tx.NodeAddress,
		)
	}

	var heights []int64
	for height := range blockStatsMap {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool { return heights[i] < heights[j] })

	for _, height := range heights {
		stats := blockStatsMap[height]

		c.RecordBlockStats(
			height,
			gasLimit,
			stats.gasUsed,
			stats.txsSent,
			stats.successfulTxs,
			stats.failedTxs,
		)
	}
}

func (c *DefaultMetricsCollector) RecordTransactionSuccess(txHash string, gasUsed int64, nodeAddress string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logger, _ := zap.NewDevelopment()
	logger.Info("Recording TransactionSuccess", zap.Any("hash", txHash))

	c.totalTxs++
	c.successfulTxs++
	c.totalGasUsed += gasUsed

	stats, exists := c.nodeStats[nodeAddress]
	if !exists {
		stats = &types.NodeStats{
			Address:          nodeAddress,
			TransactionsSent: 0,
			SuccessfulTxs:    0,
			FailedTxs:        0,
		}
		c.nodeStats[nodeAddress] = stats
	}

	stats.TransactionsSent++
	stats.SuccessfulTxs++
}

func (c *DefaultMetricsCollector) RecordTransactionFailure(txHash string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	logger, _ := zap.NewDevelopment()
	logger.Info("Recording TransactionFailure", zap.Any("hash", txHash))

	c.totalTxs++
	c.failedTxs++

	c.broadcastErrors = append(c.broadcastErrors, types.BroadcastError{
		TxHash: txHash,
		Error:  err.Error(),
	})
}

func (c *DefaultMetricsCollector) RecordBlockStats(blockHeight int64, gasLimit int, gasUsed int64, txsSent int, successfulTxs int, failedTxs int) {
	c.mu.Lock()
	defer c.mu.Unlock()

	blockStat := types.BlockStat{
		BlockHeight:         blockHeight,
		TransactionsSent:    txsSent,
		SuccessfulTxs:       successfulTxs,
		FailedTxs:           failedTxs,
		GasLimit:            gasLimit,
		TotalGasUsed:        gasUsed,
		BlockGasUtilization: float64(gasUsed) / float64(gasLimit),
	}

	c.blockStats = append(c.blockStats, blockStat)

	for _, stats := range c.nodeStats {
		if stats.TransactionsSent > 0 {
			stats.BlockParticipation++
		}
	}
}

func (c *DefaultMetricsCollector) GetResults() types.LoadTestResult {
	c.mu.RLock()
	defer c.mu.RUnlock()

	c.endTime = time.Now()

	avgGasPerTx := 0
	if c.totalTxs > 0 {
		avgGasPerTx = int(c.totalGasUsed) / c.totalTxs
	}

	avgBlockGasUtilization := 0.0
	totalBlocks := len(c.blockStats)
	if totalBlocks > 0 {
		for _, stat := range c.blockStats {
			avgBlockGasUtilization += stat.BlockGasUtilization
		}
		avgBlockGasUtilization /= float64(totalBlocks)
	}

	nodeDistribution := make(map[string]types.NodeStats)
	for addr, stats := range c.nodeStats {
		nodeDistribution[addr] = *stats
	}

	return types.LoadTestResult{
		TotalTransactions:      c.totalTxs,
		SuccessfulTransactions: c.successfulTxs,
		FailedTransactions:     c.failedTxs,
		BroadcastErrors:        c.broadcastErrors,
		AvgGasPerTransaction:   avgGasPerTx,
		AvgBlockGasUtilization: avgBlockGasUtilization,
		BlocksProcessed:        totalBlocks,
		StartTime:              c.startTime,
		EndTime:                c.endTime,
		Runtime:                c.endTime.Sub(c.startTime),
		BlockStats:             c.blockStats,
		NodeDistribution:       nodeDistribution,
	}
}
