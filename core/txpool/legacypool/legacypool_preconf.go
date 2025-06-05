package legacypool

import (
	"fmt"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/preconf"
)

// SubscribeNewPreconfTxEvent subscribes to new preconf transaction events.
func (pool *LegacyPool) SubscribeNewPreconfTxEvent(ch chan<- core.NewPreconfTxEvent) event.Subscription {
	return pool.preconfTxFeed.Subscribe(ch)
}

// SubscribeNewPreconfTxRequestEvent subscribes to new preconf transaction request events.
func (pool *LegacyPool) SubscribeNewPreconfTxRequestEvent(ch chan<- *core.NewPreconfTxRequest) event.Subscription {
	return pool.preconfTxRequestFeed.Subscribe(ch)
}

// PendingPreconfTxs retrieves the preconf transactions and the pending transactions.
func (pool *LegacyPool) PendingPreconfTxs(filter txpool.PendingFilter) ([]*types.Transaction, map[common.Address][]*txpool.LazyTransaction) {
	defer preconf.MetricsPreconfTxPoolFilterCost(time.Now())
	// If only blob transactions are requested, this pool is unsuitable as it
	// contains none, don't even bother.
	if filter.OnlyBlobTxs {
		return nil, nil
	}
	pool.mu.Lock()
	defer pool.mu.Unlock()

	pending := pool.pendingWithFilter(filter)
	return pool.extractPreconfTxsFromPending(pending), pending
}

// extractPreconfTxsFromPending extracts pre-confirmation transactions from the pending pool and ensures consistency
// between pending and preconfTxs. It checks that all preconf transactions in pending are in pool.preconfTxs,
// and all transactions in pool.preconfTxs are in pending. Any inconsistencies are logged as errors.
// Finally, it removes preconf transactions from pending and returns them while preserving the order of remaining transactions.
//
// Parameters:
// - pending: A map of addresses to their pending transactions.
//
// Returns:
// - A slice of pre-confirmation transactions extracted from pending.
func (pool *LegacyPool) extractPreconfTxsFromPending(pending map[common.Address][]*txpool.LazyTransaction) []*types.Transaction {
	// check preconf tx in pending map and also in preconfTxs
	for from, txs := range pending {
		if pool.config.Preconf.IsPreconfTxFrom(from) {
			for _, tx := range txs {
				if pool.config.Preconf.IsPreconfTx(&from, tx.Tx.To()) && !pool.preconfTxs.Contains(tx.Tx.Hash()) {
					// This tx will be sealed like a normal tx, not a preconf tx
					log.Error("Missing preconf tx in preconfTxs, please report the issue", "tx", tx.Tx.Hash(), "from", from.Hex(), "nonce", tx.Tx.Nonce())
					continue
				}
			}
		}
	}

	// removes the preconf transaction from the pending map, maintaining the order.
	preconfTxs := make([]*types.Transaction, 0)
	for _, preconfTx := range pool.preconfTxs.TxEntries() {
		preconfTxHash := preconfTx.Tx.Hash()

		// Get the slice of transactions for the target address
		txs, exists := pending[preconfTx.From]

		// If the transaction isn't in pending map but it's expected to be there,
		// show the error log.
		if !exists || len(txs) == 0 {
			log.Error("Missing transaction in pending map, please report the issue", "hash", preconfTxHash)
			pool.preconfTxs.Remove(preconfTxHash) // remove it prevent log always print
			continue
		}

		// Create a new slice to hold the transactions that are not deleted
		var newTxs []*txpool.LazyTransaction
		for _, tx := range txs {
			if tx.Tx.Hash() != preconfTxHash {
				newTxs = append(newTxs, tx) // Only keep the transactions that are not to be deleted
			}
		}

		// Update the slice in the map
		if len(newTxs) == 0 {
			delete(pending, preconfTx.From) // If the slice is empty, delete the entry for that address
		} else {
			pending[preconfTx.From] = newTxs // Replace with the new slice
		}

		// preconf error tx will still be sealed to the block
		if preconfTx.Status == core.PreconfStatusSuccess || preconfTx.Status == core.PreconfStatusFailed {
			preconfTxs = append(preconfTxs, preconfTx.Tx)
		}
	}

	_ = pool.cleanTimeoutPreconfTxs()
	return preconfTxs
}

func (pool *LegacyPool) cleanTimeoutPreconfTxs() int {
	removed := pool.preconfTxs.CleanTimeout()
	for _, tx := range removed {
		pool.removeTx(tx.Tx.Hash(), true, true)
	}
	return len(removed)
}

// PreconfReady closes the preconfReadyCh channel to notify the miner that preconf is ready
// This is called every time a worker is ready with an env, but it only closes once, so we need to use sync.Once to ensure it only closes once
func (pool *LegacyPool) PreconfReady() {
	pool.preconfReadyOnce.Do(func() {
		close(pool.preconfReadyCh)
		log.Info("preconf ready")
	})
}

func (pool *LegacyPool) addPreconfTx(tx *types.Transaction) {
	log.Trace("addPreconfTx", "tx", tx.Hash())
	txHash := tx.Hash()

	// check tx is preconf tx
	from, _ := types.Sender(pool.signer, tx)
	if !pool.config.Preconf.IsPreconfTx(&from, tx.To()) {
		log.Debug("preconf from and to is not match", "tx", txHash)
		return
	}

	// handle preconf tx
	pool.handlePreconfTx(from, tx)
}

func (pool *LegacyPool) handlePreconfTx(from common.Address, tx *types.Transaction) {
	txHash := tx.Hash()

	// add tx to preconfTxs and send preconf request event should keep same order
	pool.preconfTxs.Add(from, tx)

	// If preconfReadyCh is not closed, it means this is a preconf tx restored from journal after system restart.
	// In this case, we don't need to execute preconfirmation again to avoid resource contention with worker.
	select {
	case <-pool.preconfReadyCh:
	default:
		// only success preconf tx can be restored from journal
		pool.preconfTxs.SetStatus(txHash, core.PreconfStatusSuccess)
		log.Debug("handle preconf tx from journal", "tx", txHash)
		return
	}

	// send preconf request event
	result := make(chan *core.PreconfResponse, 1) // buffer 1 to avoid worker blocking
	preconfTxRequest := &core.NewPreconfTxRequest{
		Tx:            tx,
		PreconfResult: result,
		Status:        core.PreconfStatusWaiting,
		ClosePreconfResultFn: func() {
			close(result)
		},
	}
	pool.preconfTxRequestFeed.Send(preconfTxRequest)
	log.Debug("txpool sent preconf tx request", "tx", txHash)

	// goroutine to avoid blocking
	go func() {
		log.Trace("handlePreconfTxs", "tx", tx.Hash())
		defer preconf.MetricsPreconfTxPoolHandleCost(time.Now())
		tx := preconfTxRequest.Tx

		// default preconf event
		event := core.NewPreconfTxEvent{
			TxHash:                 txHash,
			PredictedL2BlockNumber: hexutil.Uint64(0),
			Status:                 core.PreconfStatusWaiting,
		}

		// timeout
		timeout := time.NewTimer(pool.config.Preconf.PreconfTimeout)
		defer timeout.Stop()
		now := time.Now()
		// wait for miner.worker preconf response
		select {
		case response := <-result:
			log.Trace("txpool received preconf tx response", "tx", txHash, "duration", time.Since(now))
			if response.Err == nil {
				event.Status = core.PreconfStatusSuccess
			} else {
				event.Status = core.PreconfStatusFailed
				event.Reason = response.Err.Error()
			}
			if response.Receipt != nil {
				if response.Receipt.Status == types.ReceiptStatusSuccessful {
					event.Status = core.PreconfStatusSuccess
					event.Receipt = core.PreconfTxReceipt{Logs: core.NewLogs(response.Receipt.Logs)}
				} else {
					event.Status = core.PreconfStatusFailed
					event.Reason = vm.ErrExecutionReverted.Error()
				}
				event.PredictedL2BlockNumber = hexutil.Uint64(response.Receipt.BlockNumber.Uint64())
			}
		case <-timeout.C:
			status := preconfTxRequest.SetStatus(core.PreconfStatusWaiting, core.PreconfStatusTimeout)
			if status == core.PreconfStatusTimeout {
				event.Reason = fmt.Sprintf("preconf timeout, over %s timeout", time.Since(now))
				pool.preconfTxs.SetStatus(txHash, core.PreconfStatusTimeout)
				event.Status = core.PreconfStatusTimeout
			} else {
				event.Status = status
			}
		}

		// add preconf success tx to journal
		if event.Status == core.PreconfStatusSuccess {
			preconf.PreconfTxSuccessMeter.Mark(1)
			log.Trace("preconf success", "tx", txHash)
		} else {
			preconf.PreconfTxFailureMeter.Mark(1)
			log.Warn("preconf failure", "tx", txHash, "nonce", tx.Nonce(), "reason", event.Reason)
		}

		// send preconf event
		pool.preconfTxFeed.Send(event)
	}()
}

func (pool *LegacyPool) SetPreconfTxStatus(txHash common.Hash, status core.PreconfStatus) {
	// preconfTxs.SetStatus is thread safe
	pool.preconfTxs.SetStatus(txHash, status)
}

func (pool *LegacyPool) recoverTimeoutPreconfTx(tx *types.Transaction) {
	log.Trace("recoverTimeoutPreconfTx", "tx", tx.Hash())
	pool.preconfTxs.Remove(tx.Hash())
	pool.addPreconfTx(tx)
}
