package preconf

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
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

// Helper function to create a transaction with given nonce and sender
func newTestTx(key *ecdsa.PrivateKey, nonce uint64) *types.Transaction {
	tx := types.NewTransaction(nonce, common.HexToAddress("0x123"), common.Big0, 21000, common.Big1, nil)
	signer := types.LatestSignerForChainID(common.Big1)
	tx, _ = types.SignTx(tx, signer, key)
	return tx
}

func TestTimedTxSet_Forward(t *testing.T) {
	addr1, err := crypto.HexToECDSA("ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80")
	if err != nil {
		t.Fatalf("failed to convert hex to ecdsa: %v", err)
	}
	addr2, err := crypto.HexToECDSA("654c6b97f400c2facec28bcb2ae04f2bf99e007bd6e41b2ce221481e30840e49")
	if err != nil {
		t.Fatalf("failed to convert hex to ecdsa: %v", err)
	}
	addr := common.HexToAddress("0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266")
	tests := []struct {
		name    string
		setup   func(*TimedTxSet)
		addr    common.Address
		nonce   uint64
		wantTxs int // Expected number of transactions left
	}{
		{
			name: "Remove older nonces",
			setup: func(s *TimedTxSet) {
				s.Add(newTestTx(addr1, 1))
				s.Add(newTestTx(addr1, 2))
				s.Add(newTestTx(addr1, 3))
			},
			addr:    addr,
			nonce:   3,
			wantTxs: 1, // Only nonce 3 should remain
		},
		{
			name: "No matching address",
			setup: func(s *TimedTxSet) {
				s.Add(newTestTx(addr1, 1))
				s.Add(newTestTx(addr2, 2))
			},
			addr:    common.HexToAddress("0x3"),
			nonce:   5,
			wantTxs: 2, // No removals
		},
		{
			name: "Empty set",
			setup: func(s *TimedTxSet) {
				// No transactions added
			},
			addr:    addr,
			nonce:   5,
			wantTxs: 0,
		},
		{
			name: "Remove all transactions",
			setup: func(s *TimedTxSet) {
				s.Add(newTestTx(addr1, 1))
				s.Add(newTestTx(addr1, 2))
			},
			addr:    addr,
			nonce:   10,
			wantTxs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Setup TimedTxSet
			s := NewTimedTxSet()
			tt.setup(s)

			// Execute Forward
			s.Forward(tt.addr, tt.nonce)

			// Verify results
			assert.Equal(t, tt.wantTxs, s.Len(), "unexpected number of transactions")
		})
	}
}
