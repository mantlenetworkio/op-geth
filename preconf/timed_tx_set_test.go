package preconf

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
)

func TestTimedTxSet(t *testing.T) {
	set := NewTimedTxSet()

	// Test transactions
	tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
	tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)

	// Test Add and Contains
	set.Add(tx1)
	if !set.Contains(tx1.Hash()) {
		t.Errorf("Expected tx1 to be in set")
	}
	if set.Len() != 1 {
		t.Errorf("Expected length 1, got %d", set.Len())
	}

	// Test time order
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	set.Add(tx2)
	txs := set.Transactions()
	if len(txs) != 2 || txs[0].Hash() != tx1.Hash() || txs[1].Hash() != tx2.Hash() {
		t.Errorf("Transactions not in FIFO order: %v", txs)
	}

	// Test Get
	if got := set.Get(tx1.Hash()); got != tx1 {
		t.Errorf("Get returned wrong transaction")
	}

	// Test Remove
	set.Remove(tx1.Hash())
	if set.Contains(tx1.Hash()) {
		t.Errorf("tx1 should have been removed")
	}
	if set.Len() != 1 {
		t.Errorf("Expected length 1 after remove, got %d", set.Len())
	}

	// Test Clear
	set.Clear()
	if set.Len() != 0 {
		t.Errorf("Expected length 0 after clear, got %d", set.Len())
	}
}
