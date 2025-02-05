package metrics

import (
	"context"
	"fmt"
	"go.uber.org/zap"
	"sort"
	"time"

	"github.com/skip-mev/catalyst/internal/cosmos/wallet"

	"github.com/skip-mev/catalyst/internal/cosmos/client"

	"github.com/skip-mev/catalyst/internal/types"
)

// MetricsCollector collects and processes metrics for load tests
type MetricsCollector struct {
	startTime         time.Time
	endTime           time.Time
	blocksProcessed   int
	txsByBlock        map[int64][]types.SentTx
	txsByNode         map[string][]types.SentTx
	txsByMsgType      map[types.MsgType][]types.SentTx
	gasUsageByMsgType map[types.MsgType][]int64
	logger            *zap.Logger
}

// NewMetricsCollector creates a new metrics collector
func NewMetricsCollector() MetricsCollector {
	return MetricsCollector{
		startTime:         time.Now(),
		txsByBlock:        make(map[int64][]types.SentTx),
		txsByNode:         make(map[string][]types.SentTx),
		txsByMsgType:      make(map[types.MsgType][]types.SentTx),
		gasUsageByMsgType: make(map[types.MsgType][]int64),
		logger:            zap.L().Named("metrics_collector"),
	}
}

// GroupSentTxs groups sent txs by block, node, and message type
func (m *MetricsCollector) GroupSentTxs(ctx context.Context, sentTxs []types.SentTx, gasLimit int, client *client.Chain) {
	m.endTime = time.Now()

	for i := range sentTxs {
		tx := &sentTxs[i]

		if tx.Err == nil {
			txResponse, err := wallet.GetTxResponse(ctx, client, tx.TxHash)
			if err != nil {
				m.logger.Error("Error getting tx response. Metrics calculation might be inaccurate as a result", zap.Error(err))
			}
			tx.TxResponse = txResponse

			m.txsByBlock[tx.TxResponse.Height] = append(m.txsByBlock[tx.TxResponse.Height], *tx)
			if tx.TxResponse.GasUsed > 0 {
				m.gasUsageByMsgType[tx.MsgType] = append(m.gasUsageByMsgType[tx.MsgType], int64(tx.TxResponse.GasUsed))
			}

		}
		m.txsByNode[tx.NodeAddress] = append(m.txsByNode[tx.NodeAddress], *tx)
		m.txsByMsgType[tx.MsgType] = append(m.txsByMsgType[tx.MsgType], *tx)
	}

	m.blocksProcessed = len(m.txsByBlock)
}

// calculateGasStats calculates gas statistics for a slice of gas values
func (m *MetricsCollector) calculateGasStats(gasUsage []int64) types.GasStats {
	if len(gasUsage) == 0 {
		return types.GasStats{}
	}

	// Sort gas usage for min and max calculations
	sort.Slice(gasUsage, func(i, j int) bool {
		return gasUsage[i] < gasUsage[j]
	})

	var total int64
	for _, gas := range gasUsage {
		total += gas
	}

	return types.GasStats{
		Average: total / int64(len(gasUsage)),
		Min:     gasUsage[0],
		Max:     gasUsage[len(gasUsage)-1],
		Total:   total,
	}
}

// ProcessResults returns the final load test results
func (m *MetricsCollector) ProcessResults(gasLimit int) types.LoadTestResult {
	result := types.LoadTestResult{
		Overall: types.OverallStats{
			StartTime:       m.startTime,
			EndTime:         m.endTime,
			Runtime:         m.endTime.Sub(m.startTime),
			BlocksProcessed: m.blocksProcessed,
		},
		ByMessage: make(map[types.MsgType]types.MessageStats),
		ByNode:    make(map[string]types.NodeStats),
		ByBlock:   make([]types.BlockStat, 0),
	}

	var totalTxs, successfulTxs, failedTxs int
	var totalGasUsed int64

	// Process message type stats
	for msgType, txs := range m.txsByMsgType {
		stats := types.MessageStats{
			Transactions: types.TransactionStats{
				Total: len(txs),
			},
			Gas: m.calculateGasStats(m.gasUsageByMsgType[msgType]),
			Errors: types.ErrorStats{
				ErrorCounts: make(map[string]int),
			},
		}

		for _, tx := range txs {
			if tx.Err != nil {
				stats.Transactions.Failed++
				errMsg := tx.Err.Error()
				stats.Errors.ErrorCounts[errMsg]++
				stats.Errors.BroadcastErrors = append(stats.Errors.BroadcastErrors, types.BroadcastError{
					TxHash:      tx.TxHash,
					Error:       errMsg,
					MsgType:     msgType,
					NodeAddress: tx.NodeAddress,
				})
			} else {
				stats.Transactions.Successful++
			}
		}

		result.ByMessage[msgType] = stats
		totalTxs += stats.Transactions.Total
		successfulTxs += stats.Transactions.Successful
		failedTxs += stats.Transactions.Failed
		totalGasUsed += stats.Gas.Total
	}

	// Update overall stats
	result.Overall.TotalTransactions = totalTxs
	result.Overall.SuccessfulTransactions = successfulTxs
	result.Overall.FailedTransactions = failedTxs
	if successfulTxs > 0 {
		result.Overall.AvgGasPerTransaction = totalGasUsed / int64(successfulTxs)
	}

	// Process node stats
	for nodeAddr, txs := range m.txsByNode {
		msgCounts := make(map[types.MsgType]int)
		gasUsage := make([]int64, 0)
		stats := types.NodeStats{
			Address: nodeAddr,
			TransactionStats: types.TransactionStats{
				Total: len(txs),
			},
			MessageCounts: msgCounts,
		}

		for _, tx := range txs {
			msgCounts[tx.MsgType]++

			if tx.Err != nil {
				stats.TransactionStats.Failed++
			} else {
				stats.TransactionStats.Successful++
				if tx.TxResponse != nil && tx.TxResponse.GasUsed > 0 {
					gasUsage = append(gasUsage, tx.TxResponse.GasUsed)
				}
			}
		}

		stats.GasStats = m.calculateGasStats(gasUsage)
		result.ByNode[nodeAddr] = stats
	}

	// Process block stats
	var totalGasUtilization float64
	for height, txs := range m.txsByBlock {
		msgStats := make(map[types.MsgType]types.MessageBlockStats)
		var blockGasUsed int64

		for _, tx := range txs {
			stats := msgStats[tx.MsgType]
			stats.TransactionsSent++

			if tx.Err != nil {
				stats.FailedTxs++
			} else if tx.TxResponse != nil {
				stats.SuccessfulTxs++
				stats.GasUsed += tx.TxResponse.GasUsed
				blockGasUsed += tx.TxResponse.GasUsed
			}

			msgStats[tx.MsgType] = stats
		}

		blockStats := types.BlockStat{
			BlockHeight:    height,
			MessageStats:   msgStats,
			TotalGasUsed:   blockGasUsed,
			GasLimit:       gasLimit,
			GasUtilization: float64(blockGasUsed) / float64(gasLimit),
		}

		// Get block timestamp from any transaction in the block
		if len(txs) > 0 {
			timestamp, err := time.Parse(time.RFC3339, txs[0].TxResponse.Timestamp)
			if err != nil {
				m.logger.Error("Error parsing tx timestamp", zap.String("tx_hash", txs[0].TxHash),
					zap.String("timestamp", txs[0].TxResponse.Timestamp), zap.Error(err))
			}
			blockStats.Timestamp = timestamp
		}

		result.ByBlock = append(result.ByBlock, blockStats)
		totalGasUtilization += blockStats.GasUtilization
	}

	if len(result.ByBlock) > 0 {
		result.Overall.AvgBlockGasUtilization = totalGasUtilization / float64(len(result.ByBlock))
	}

	// Sort blocks by height
	sort.Slice(result.ByBlock, func(i, j int) bool {
		return result.ByBlock[i].BlockHeight < result.ByBlock[j].BlockHeight
	})

	return result
}

// PrintResults prints the load test results in a clean, formatted way
func (m *MetricsCollector) PrintResults(result types.LoadTestResult) {
	fmt.Println("\n=== Load Test Results ===")

	fmt.Println("\n🎯 Overall Statistics:")
	fmt.Printf("Total Transactions: %d\n", result.Overall.TotalTransactions)
	fmt.Printf("Successful Transactions: %d\n", result.Overall.SuccessfulTransactions)
	fmt.Printf("Failed Transactions: %d\n", result.Overall.FailedTransactions)
	fmt.Printf("Average Gas Per Transaction: %d\n", result.Overall.AvgGasPerTransaction)
	fmt.Printf("Average Block Gas Utilization: %.2f%%\n", result.Overall.AvgBlockGasUtilization*100)
	fmt.Printf("Runtime: %s\n", result.Overall.Runtime)
	fmt.Printf("Blocks Processed: %d\n", result.Overall.BlocksProcessed)

	fmt.Println("\n📊 Message Type Statistics:")
	for msgType, stats := range result.ByMessage {
		fmt.Printf("\n%s:\n", msgType)
		fmt.Printf("  Transactions:\n")
		fmt.Printf("    Total: %d\n", stats.Transactions.Total)
		fmt.Printf("    Successful: %d\n", stats.Transactions.Successful)
		fmt.Printf("    Failed: %d\n", stats.Transactions.Failed)
		fmt.Printf("  Gas Usage:\n")
		fmt.Printf("    Average: %d\n", stats.Gas.Average)
		fmt.Printf("    Min: %d\n", stats.Gas.Min)
		fmt.Printf("    Max: %d\n", stats.Gas.Max)
		fmt.Printf("    Total: %d\n", stats.Gas.Total)
		if len(stats.Errors.BroadcastErrors) > 0 {
			fmt.Printf("  Errors:\n")
			for errType, count := range stats.Errors.ErrorCounts {
				fmt.Printf("    %s: %d occurrences\n", errType, count)
			}
		}
	}

	fmt.Println("\n🖥️  Node Statistics:")
	for nodeAddr, stats := range result.ByNode {
		fmt.Printf("\n%s:\n", nodeAddr)
		fmt.Printf("  Transactions:\n")
		fmt.Printf("    Total: %d\n", stats.TransactionStats.Total)
		fmt.Printf("    Successful: %d\n", stats.TransactionStats.Successful)
		fmt.Printf("    Failed: %d\n", stats.TransactionStats.Failed)
		fmt.Printf("  Message Distribution:\n")
		for msgType, count := range stats.MessageCounts {
			fmt.Printf("    %s: %d\n", msgType, count)
		}
		fmt.Printf("  Gas Usage:\n")
		fmt.Printf("    Average: %d\n", stats.GasStats.Average)
		fmt.Printf("    Min: %d\n", stats.GasStats.Min)
		fmt.Printf("    Max: %d\n", stats.GasStats.Max)
	}

	fmt.Println("\n📦 Block Statistics Summary:")
	fmt.Printf("Total Blocks: %d\n", len(result.ByBlock))
	var totalGasUtilization float64
	var maxGasUtilization float64
	minGasUtilization := result.ByBlock[0].GasUtilization // set first block as min initially
	var maxGasBlock int64
	var minGasBlock int64
	for _, block := range result.ByBlock {
		totalGasUtilization += block.GasUtilization
		if block.GasUtilization > maxGasUtilization {
			maxGasUtilization = block.GasUtilization
			maxGasBlock = block.BlockHeight
		}
		if block.GasUtilization < minGasUtilization {
			minGasUtilization = block.GasUtilization
			minGasBlock = block.BlockHeight
		}
	}
	avgGasUtilization := totalGasUtilization / float64(len(result.ByBlock))
	fmt.Printf("Average Gas Utilization: %.2f%%\n", avgGasUtilization*100)
	fmt.Printf("Min Gas Utilization: %.2f%% (Block %d)\n", minGasUtilization*100, minGasBlock)
	fmt.Printf("Max Gas Utilization: %.2f%% (Block %d)\n", maxGasUtilization*100, maxGasBlock)
}
