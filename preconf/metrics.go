package preconf

import (
	"time"

	"github.com/ethereum/go-ethereum/metrics"
)

// metrics
var (
	// OpNode status metrics
	OpNodeL1CurrentGauge        = metrics.NewRegisteredGauge("preconf/opnode/l1/current", nil)
	OpNodeL1HeadGauge           = metrics.NewRegisteredGauge("preconf/opnode/l1/head", nil)
	OpNodeL2UnsafeGauge         = metrics.NewRegisteredGauge("preconf/opnode/l2/unsafe", nil)
	OpNodeEngineSyncTargetGauge = metrics.NewRegisteredGauge("preconf/opnode/engine/sync_target", nil)
	OpNodeSyncStatusGauge       = metrics.NewRegisteredGauge("preconf/opnode/sync/status", nil) // 1:OK, 0:Not OK

	// L1 Deposit status metrics
	L1ClientStatusGauge   = metrics.NewRegisteredGauge("preconf/l1/client/status", nil) // 1:OK, 0:Not OK
	L1DepositTxCountGauge = metrics.NewRegisteredGauge("preconf/l1/deposit/count", nil)

	// OpGeth environment status metrics
	OpGethEnvBlockNumberGauge = metrics.NewRegisteredGauge("preconf/opgeth/env/block_number", nil)

	// Pre-confirm transaction counters
	PreconfTxPendingGauge = metrics.NewRegisteredGauge("preconf/txpool/pending", nil)
	PreconfTxSuccessMeter = metrics.NewRegisteredMeter("preconf/tx/success", nil)
	PreconfTxFailureMeter = metrics.NewRegisteredMeter("preconf/tx/failure", nil)

	// Pre-confirm processing time
	PreconfTxPoolHandleTimer  = metrics.NewRegisteredTimer("preconf/txpool/handle", nil)
	PreconfTxPoolForwardTimer = metrics.NewRegisteredTimer("preconf/txpool/forward", nil)
	PreconfTxPoolFilterTimer  = metrics.NewRegisteredTimer("preconf/txpool/filter", nil)
	PreconfMinerExecuteTimer  = metrics.NewRegisteredTimer("preconf/execute", nil)
	PreconfAPIHandleTimer     = metrics.NewRegisteredTimer("preconf/api/handle", nil)
)

// OpNode status update
func MetricsOpNodeSyncStatus(status *OptimismSyncStatus, optimismSyncStatusOK bool) {
	if status != nil {
		OpNodeL1CurrentGauge.Update(int64(status.CurrentL1.Number))
		OpNodeL1HeadGauge.Update(int64(status.HeadL1.Number))
		OpNodeL2UnsafeGauge.Update(int64(status.UnsafeL2.Number))
		OpNodeEngineSyncTargetGauge.Update(int64(status.EngineSyncTarget.Number))
	}
	if optimismSyncStatusOK {
		OpNodeSyncStatusGauge.Update(1)
	} else {
		OpNodeSyncStatusGauge.Update(0)
	}
}

// L1 Deposit status update
func MetricsL1Deposit(ok bool, count int) {
	if ok {
		L1ClientStatusGauge.Update(1)
	} else {
		L1ClientStatusGauge.Update(0)
	}
	L1DepositTxCountGauge.Update(int64(count))
}

// OpGeth status update
func MetricsOpGethEnvBlockNumber(number int64) {
	OpGethEnvBlockNumberGauge.Update(number)
}

// Pre-confirm transaction processing timing
func MetricsPreconfTxPoolHandleCost(start time.Time) {
	PreconfTxPoolHandleTimer.Update(time.Since(start))
}

func MetricsPreconfTxPoolForwardCost(start time.Time) {
	PreconfTxPoolForwardTimer.Update(time.Since(start))
}

func MetricsPreconfTxPoolFilterCost(start time.Time) {
	PreconfTxPoolFilterTimer.Update(time.Since(start))
}

func MetricsPreconfExecuteCost(start time.Time) {
	PreconfMinerExecuteTimer.Update(time.Since(start))
}

func MetricsPreconfAPIHandleCost(start time.Time) {
	PreconfAPIHandleTimer.Update(time.Since(start))
}

// Pending pre-confirm transaction counter
func MetricsPendingPreconfInc(count int) {
	PreconfTxPendingGauge.Inc(int64(count))
}

func MetricsPendingPreconfDec(count int) {
	PreconfTxPendingGauge.Dec(int64(count))
}
