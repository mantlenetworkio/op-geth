package preconf

import (
	"sync"

	"github.com/ethereum/go-ethereum/common"
)

type MohoTxSet struct {
	// Group transactions by from address
	fromTxs map[common.Address]map[common.Hash]struct{}
	mu      sync.RWMutex
}

func NewMohoTxSet() *MohoTxSet {
	return &MohoTxSet{
		fromTxs: make(map[common.Address]map[common.Hash]struct{}),
	}
}

// Add adds a single transaction hash to the specified from address
func (set *MohoTxSet) Add(from common.Address, hash common.Hash) {
	set.mu.Lock()
	defer set.mu.Unlock()

	if set.fromTxs[from] == nil {
		set.fromTxs[from] = make(map[common.Hash]struct{})
	}
	set.fromTxs[from][hash] = struct{}{}
}

// AddBatch adds multiple transaction hashes to the specified from address
func (set *MohoTxSet) AddBatch(from common.Address, hashes []common.Hash) {
	set.mu.Lock()
	defer set.mu.Unlock()

	if set.fromTxs[from] == nil {
		set.fromTxs[from] = make(map[common.Hash]struct{})
	}

	for _, hash := range hashes {
		set.fromTxs[from][hash] = struct{}{}
	}
}

// Remove removes a specific transaction hash from the specified from address
func (set *MohoTxSet) Remove(from common.Address, hash common.Hash) {
	set.mu.Lock()
	defer set.mu.Unlock()

	if txs, ok := set.fromTxs[from]; ok {
		delete(txs, hash)
		// If the from address has no more transactions, remove the entire from entry
		if len(txs) == 0 {
			delete(set.fromTxs, from)
		}
	}
}

// RemoveFrom removes all transaction hashes for the specified from address
func (set *MohoTxSet) RemoveFrom(from common.Address) {
	set.mu.Lock()
	defer set.mu.Unlock()
	delete(set.fromTxs, from)
}

// Contains checks if the specified from address contains a specific transaction hash
func (set *MohoTxSet) Contains(from common.Address, hash common.Hash) bool {
	set.mu.RLock()
	defer set.mu.RUnlock()

	if txs, ok := set.fromTxs[from]; ok {
		_, exists := txs[hash]
		return exists
	}
	return false
}

// ContainsFrom checks if the specified from address exists
func (set *MohoTxSet) ContainsFrom(from common.Address) bool {
	set.mu.RLock()
	defer set.mu.RUnlock()

	_, ok := set.fromTxs[from]
	return ok
}

// GetFromTxs gets all transaction hashes for the specified from address
func (set *MohoTxSet) GetFromTxs(from common.Address) []common.Hash {
	set.mu.RLock()
	defer set.mu.RUnlock()

	if txs, ok := set.fromTxs[from]; ok {
		result := make([]common.Hash, 0, len(txs))
		for hash := range txs {
			result = append(result, hash)
		}
		return result
	}
	return nil
}

// Len returns the total number of transactions
func (set *MohoTxSet) Len() int {
	set.mu.RLock()
	defer set.mu.RUnlock()

	total := 0
	for _, txs := range set.fromTxs {
		total += len(txs)
	}
	return total
}

// FromCount returns the number of from addresses
func (set *MohoTxSet) FromCount() int {
	set.mu.RLock()
	defer set.mu.RUnlock()
	return len(set.fromTxs)
}

// Clear clears all data
func (set *MohoTxSet) Clear() {
	set.mu.Lock()
	defer set.mu.Unlock()
	set.fromTxs = make(map[common.Address]map[common.Hash]struct{})
}

// GetAllFroms gets all from addresses
func (set *MohoTxSet) GetAllFroms() []common.Address {
	set.mu.RLock()
	defer set.mu.RUnlock()

	result := make([]common.Address, 0, len(set.fromTxs))
	for from := range set.fromTxs {
		result = append(result, from)
	}
	return result
}
