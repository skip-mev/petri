package metrics

import (
	"sync"
	"time"

	"github.com/skip-mev/catalyst/internal/types"
)

// MetricsCollector is responsible for collecting and aggregating metrics during a load test
type MetricsCollector interface {
	RecordTransactionSuccess(txHash string, latencyMs float64, gasUsed int64, nodeAddress string)
	RecordTransactionFailure(txHash string, blockHeight int, err error, nodeAddress string)
	RecordBlockStats(blockHeight, gasLimit, gasUsed int64, txsSent int, successfulTxs int, failedTxs int, productionTime time.Duration)
	GetResults() types.LoadTestResult
}

// DefaultMetricsCollector implements the MetricsCollector interface
type DefaultMetricsCollector struct {
	mu sync.RWMutex

	startTime time.Time
	endTime   time.Time

	totalTxs      int
	successfulTxs int
	failedTxs     int

	totalGasUsed int64
	totalLatency float64

	broadcastErrors []types.BroadcastError
	blockStats      []types.BlockStat

	nodeStats map[string]*types.NodeStats
}

func NewMetricsCollector() *DefaultMetricsCollector {
	return &DefaultMetricsCollector{
		startTime:  time.Now(),
		nodeStats:  make(map[string]*types.NodeStats),
		blockStats: make([]types.BlockStat, 0),
	}
}

func (c *DefaultMetricsCollector) RecordTransactionSuccess(txHash string, latencyMs float64, gasUsed int64, nodeAddress string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalTxs++
	c.successfulTxs++
	c.totalGasUsed += gasUsed
	c.totalLatency += latencyMs

	// Update node-specific stats
	stats, exists := c.nodeStats[nodeAddress]
	if !exists {
		stats = &types.NodeStats{
			NodeAddresses:    types.NodeAddress{RPC: nodeAddress},
			TransactionsSent: 0,
			SuccessfulTxs:    0,
			FailedTxs:        0,
			AvgLatencyMs:     0,
		}
		c.nodeStats[nodeAddress] = stats
	}

	stats.TransactionsSent++
	stats.SuccessfulTxs++
	stats.AvgLatencyMs = ((stats.AvgLatencyMs * float64(stats.TransactionsSent-1)) + latencyMs) / float64(stats.TransactionsSent)
}

func (c *DefaultMetricsCollector) RecordTransactionFailure(txHash string, blockHeight int, err error, nodeAddress string) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.totalTxs++
	c.failedTxs++

	c.broadcastErrors = append(c.broadcastErrors, types.BroadcastError{
		BlockHeight: blockHeight,
		TxHash:      txHash,
		Error:       err.Error(),
	})

	// Update node-specific stats
	stats, exists := c.nodeStats[nodeAddress]
	if !exists {
		stats = &types.NodeStats{
			NodeAddresses:    types.NodeAddress{RPC: nodeAddress},
			TransactionsSent: 0,
			SuccessfulTxs:    0,
			FailedTxs:        0,
			AvgLatencyMs:     0,
		}
		c.nodeStats[nodeAddress] = stats
	}

	stats.TransactionsSent++
	stats.FailedTxs++
}

func (c *DefaultMetricsCollector) RecordBlockStats(blockHeight, gasLimit, gasUsed int64, txsSent int, successfulTxs int, failedTxs int, productionTime time.Duration) {
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
		BlockProductionTime: productionTime,
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

	avgLatency := 0.0
	if c.successfulTxs > 0 {
		avgLatency = c.totalLatency / float64(c.successfulTxs)
	}

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
		AvgBroadcastLatency:    avgLatency,
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
