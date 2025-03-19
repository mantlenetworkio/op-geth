package preconf

import (
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

// TimedTxSet is a time-based transaction set
type TimedTxSet struct {
	mu      sync.Mutex               // Mutex to ensure thread safety
	txMap   map[common.Hash]*txEntry // Mapping from hash to transaction entry
	txQueue []*txEntry               // Time-ordered transaction queue (FIFO)
}

// txEntry contains the transaction and its added time
type txEntry struct {
	tx        *types.Transaction // Transaction object
	addedTime time.Time          // Added time
}

// NewTimedTxSet creates a new TimedTxSet
func NewTimedTxSet() *TimedTxSet {
	return &TimedTxSet{
		txMap:   make(map[common.Hash]*txEntry),
		txQueue: make([]*txEntry, 0),
	}
}

// Add adds a transaction to the set
// If the transaction already exists, update its time
func (s *TimedTxSet) Add(tx *types.Transaction) {
	s.mu.Lock()
	defer s.mu.Unlock()

	hash := tx.Hash()
	entry := &txEntry{
		tx:        tx,
		addedTime: time.Now(),
	}

	// If the transaction already exists, replace the old entry
	if oldEntry, exists := s.txMap[hash]; exists {
		// Remove the old entry from the queue
		for i, e := range s.txQueue {
			if e == oldEntry {
				s.txQueue = append(s.txQueue[:i], s.txQueue[i+1:]...)
				break
			}
		}
	}

	// Add the new entry
	s.txMap[hash] = entry
	s.txQueue = append(s.txQueue, entry)

	// Metrics
	MetricsPendingPreconfInc(1)
	log.Trace("preconf added", "tx", tx.Hash().Hex())
}

// Contains checks if the transaction is in the set
func (s *TimedTxSet) Contains(hash common.Hash) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.txMap[hash]
	return exists
}

// Get returns the transaction for the specified hash, or nil if it doesn't exist
func (s *TimedTxSet) Get(hash common.Hash) *types.Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.txMap[hash]; exists {
		return entry.tx
	}
	return nil
}

// Remove removes the transaction for the specified hash
func (s *TimedTxSet) Remove(hash common.Hash) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if entry, exists := s.txMap[hash]; exists {
		// Remove from the queue
		for i, e := range s.txQueue {
			if e == entry {
				s.txQueue = append(s.txQueue[:i], s.txQueue[i+1:]...)
				break
			}
		}
		// Remove from the map
		delete(s.txMap, hash)

		// Metrics
		MetricsPendingPreconfDec(1)
		log.Trace("preconf removed", "tx", hash)
	}
}

// Transactions returns an array of transactions in time order (FIFO)
func (s *TimedTxSet) Transactions() []*types.Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()

	result := make([]*types.Transaction, len(s.txQueue))
	for i, entry := range s.txQueue {
		result[i] = entry.tx
	}
	return result
}

// Len returns the number of transactions in the set
func (s *TimedTxSet) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	return len(s.txMap)
}

// Clear clears the set
func (s *TimedTxSet) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.txMap = make(map[common.Hash]*txEntry)
	s.txQueue = make([]*txEntry, 0)
}

func (s *TimedTxSet) Forward(addr common.Address, nonce uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Iterate over the list to be deleted
	var toHashRemove []common.Hash

	for _, entry := range s.txQueue {
		from, _ := types.Sender(types.LatestSignerForChainID(entry.tx.ChainId()), entry.tx)
		if from == addr && entry.tx.Nonce() < nonce {
			toHashRemove = append(toHashRemove, entry.tx.Hash())
			log.Trace("preconf removed by forward", "tx", entry.tx.Hash(), "nonce", nonce, "tx.nonce", entry.tx.Nonce())
		}
	}

	// Process the transactions to be deleted
	for _, hash := range toHashRemove {
		// Remove from the queue
		for i, e := range s.txQueue {
			if e.tx.Hash() == hash {
				s.txQueue = append(s.txQueue[:i], s.txQueue[i+1:]...)
				break
			}
		}

		// Remove from the map
		if _, exists := s.txMap[hash]; exists {
			delete(s.txMap, hash)

			// Metrics update
			MetricsPendingPreconfDec(1)
		}
	}
}
