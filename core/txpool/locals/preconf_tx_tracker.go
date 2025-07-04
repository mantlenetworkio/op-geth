package locals

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/preconf"
)

// A PreconfTxTracker periodically journales a transaction pool to disk.
//
// Mantle addition.
type PreconfTxTracker struct {
	all map[common.Hash]*types.Transaction // All tracked transactions

	journal   *journal       // Journal of local transaction to back up to disk
	rejournal time.Duration  // How often to rotate journal
	pool      *txpool.TxPool // The tx pool to interact with

	shutdownCh chan struct{}
	mu         sync.Mutex
	wg         sync.WaitGroup
}

func NewPreconfTxTracker(journalPath string, journalTime time.Duration, pool *txpool.TxPool) *PreconfTxTracker {
	return &PreconfTxTracker{
		all:        make(map[common.Hash]*types.Transaction),
		journal:    newTxJournal(journalPath),
		rejournal:  journalTime,
		pool:       pool,
		shutdownCh: make(chan struct{}),
	}
}

// Track adds a preconf transaction to the tracked set.
// Note: blob-type transactions are ignored.
// No need to lock, because rotate needs to lock the pool, and Track also locks the pool before, so they won't conflict
func (tracker *PreconfTxTracker) Track(tx *types.Transaction) {
	tracker.TrackAll([]*types.Transaction{tx}, false)
}

func (tracker *PreconfTxTracker) TrackAll(txs []*types.Transaction, clean bool) {
	tracker.mu.Lock()
	defer tracker.mu.Unlock()

	// rotate will clean the journal, only keeping pre-confirmed transactions that are still in the pool and not yet sealed
	if clean {
		tracker.all = make(map[common.Hash]*types.Transaction)
	}

	for _, tx := range txs {
		if tx.Type() == types.BlobTxType {
			return
		}
		// If we're already tracking it, it's a no-op
		if _, ok := tracker.all[tx.Hash()]; ok {
			return
		}
		tracker.all[tx.Hash()] = tx

		if tracker.journal != nil {
			err := tracker.journal.insert(tx)
			if err != nil {
				log.Error("PreconfTxTracker: Failed to insert transaction into journal", "tx", tx.Hash(), "err", err)
			} else {
				log.Trace("PreconfTxTracker: Inserted transaction into journal", "tx", tx.Hash())
			}
		}
	}
	preconf.PreconfTxJournalGauge.Update(int64(len(tracker.all)))
}

// Start implements node.Lifecycle interface
// Start is called after all services have been constructed and the networking
// layer was also initialized to spawn any goroutines required by the service.
func (tracker *PreconfTxTracker) Start() error {
	tracker.wg.Add(1)
	go tracker.loop()
	return nil
}

// Stop implements node.Lifecycle interface
// Stop terminates all goroutines belonging to the service, blocking until they
// are all terminated.
func (tracker *PreconfTxTracker) Stop() error {
	close(tracker.shutdownCh)
	tracker.wg.Wait()
	return nil
}

func (tracker *PreconfTxTracker) loop() {
	defer log.Info("PreconfTxTracker: Stopped")
	defer tracker.wg.Done()

	start, journalPreconfTxs := time.Now(), make([]*types.Transaction, 0)
	log.Info("PreconfTxTracker: Start loading transactions from journal...")
	if err := tracker.journal.load(func(transactions []*types.Transaction) []error {
		log.Info("PreconfTxTracker: Start adding transactions to pool", "count", len(transactions), "loading_duration", time.Since(start))
		errs := tracker.pool.Add(transactions, true)
		log.Info("PreconfTxTracker: Done adding transactions to pool", "total_duration", time.Since(start))
		journalPreconfTxs = transactions
		return errs
	}); err != nil {
		log.Error("PreconfTxTracker: Transaction journal loading failed. Exiting.", "err", err)
		return
	}
	// rotate to initialize journal writer, so preconf txs can be tracked
	tojournal := map[common.Address]types.Transactions{common.BytesToAddress([]byte("preconf")): journalPreconfTxs}
	if err := tracker.journal.rotate(tojournal); err != nil {
		log.Error("PreconfTxTracker: Transaction journal rotation failed", "err", err)
	}
	defer tracker.journal.close()

	ticker := time.NewTicker(tracker.rejournal)
	defer ticker.Stop()

	journal := func() {
		// No need to recheck, as journal is only used during restart and only accepts pre-confirmed transactions that won't be evicted by the system
		start := time.Now()
		preconfTxs, _ := tracker.pool.PendingPreconfTxs(txpool.PendingFilter{})
		tracker.TrackAll(preconfTxs, true)
		// Use same address as key, so preconf transactions will be processed in order during rotation
		// without needing to modify the rotate function signature
		tojournal := map[common.Address]types.Transactions{common.BytesToAddress([]byte("preconf")): preconfTxs}
		if err := tracker.journal.rotate(tojournal); err != nil {
			log.Error("PreconfTxTracker: Transaction journal rotation failed", "err", err)
		} else {
			log.Debug("PreconfTxTracker: Transaction journal rotated", "count", len(preconfTxs), "duration", time.Since(start))
		}
	}

	for {
		select {
		case <-tracker.shutdownCh:
			journal()
			return
		case <-ticker.C:
			journal()
		}
	}
}
