package preconf

import (
	"crypto/ecdsa"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/stretchr/testify/assert"
)

func TestFIFOTxSet(t *testing.T) {
	set := NewFIFOTxSet()

	// Test transactions
	tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
	tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)
	from1, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx1)
	from2, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx2)

	// Test Add and Contains
	set.Add(from1, tx1)
	if !set.Contains(tx1.Hash()) {
		t.Errorf("Expected tx1 to be in set")
	}
	if set.Len() != 1 {
		t.Errorf("Expected length 1, got %d", set.Len())
	}

	// Test time order
	time.Sleep(1 * time.Millisecond) // Ensure time difference
	set.Add(from2, tx2)
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

func TestFIFOTxSet_Forward(t *testing.T) {
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
		setup   func(*FIFOTxSet)
		addr    common.Address
		nonce   uint64
		wantTxs int // Expected number of transactions left
	}{
		{
			name: "Remove older nonces",
			setup: func(s *FIFOTxSet) {
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 1))
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 2))
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 3))
			},
			addr:    addr,
			nonce:   3,
			wantTxs: 1, // Only nonce 3 should remain
		},
		{
			name: "No matching address",
			setup: func(s *FIFOTxSet) {
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 1))
				s.Add(crypto.PubkeyToAddress(addr2.PublicKey), newTestTx(addr2, 2))
			},
			addr:    common.HexToAddress("0x3"),
			nonce:   5,
			wantTxs: 2, // No removals
		},
		{
			name: "Empty set",
			setup: func(s *FIFOTxSet) {
				// No transactions added
			},
			addr:    addr,
			nonce:   5,
			wantTxs: 0,
		},
		{
			name: "Remove all transactions",
			setup: func(s *FIFOTxSet) {
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 1))
				s.Add(crypto.PubkeyToAddress(addr1.PublicKey), newTestTx(addr1, 2))
			},
			addr:    addr,
			nonce:   10,
			wantTxs: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			// Setup FIFOTxSet
			s := NewFIFOTxSet()
			tt.setup(s)

			// Execute Forward
			s.Forward(tt.addr, tt.nonce)

			// Verify results
			assert.Equal(t, tt.wantTxs, s.Len(), "unexpected number of transactions")
		})
	}
}

func TestFIFOTxSet_SetStatus(t *testing.T) {
	// Create a new FIFOTxSet
	set := NewFIFOTxSet()

	// Create test transactions
	tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
	tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)
	from1, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx1)
	from2, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx2)

	// Add transactions to the set
	set.Add(from1, tx1)
	set.Add(from2, tx2)

	tests := []struct {
		name           string
		hash           common.Hash
		status         core.PreconfStatus
		expectedResult int
	}{
		{
			name:           "Set status for existing transaction",
			hash:           tx1.Hash(),
			status:         core.PreconfStatusSuccess,
			expectedResult: 1, // Should return 1 for successful status update
		},
		{
			name:           "Set status for non-existing transaction",
			hash:           common.HexToHash("0xdead"),
			status:         core.PreconfStatusSuccess,
			expectedResult: 0, // Should return 0 if transaction doesn't exist
		},
		{
			name:           "Set timeout status for existing transaction",
			hash:           tx2.Hash(),
			status:         core.PreconfStatusTimeout,
			expectedResult: 1, // Should return 1 for successful status update
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := set.SetStatus(tt.hash, tt.status)
			assert.Equal(t, tt.expectedResult, result, "unexpected result from SetStatus")

			// Verify the status was set correctly if the transaction exists
			if tt.expectedResult == 1 {
				entry, exists := set.txMap[tt.hash]
				assert.True(t, exists, "transaction should exist in the map")
				assert.Equal(t, tt.status, entry.Status, "status should be updated correctly")
			}
		})
	}
}

func TestFIFOTxSet_CleanTimeout(t *testing.T) {
	tests := []struct {
		name              string
		setup             func(*FIFOTxSet)
		expectedRemoved   int
		expectedRemaining int
	}{
		{
			name: "No timeout transactions",
			setup: func(s *FIFOTxSet) {
				// Add transactions without timeout status
				tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
				tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)
				from, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx1)

				s.Add(from, tx1)
				s.Add(from, tx2)

				// Set some non-timeout statuses
				s.SetStatus(tx1.Hash(), core.PreconfStatusSuccess)
				s.SetStatus(tx2.Hash(), core.PreconfStatusFailed)
			},
			expectedRemoved:   0,
			expectedRemaining: 2,
		},
		{
			name: "All timeout transactions",
			setup: func(s *FIFOTxSet) {
				// Add transactions with timeout status
				tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
				tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)
				from, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx1)

				s.Add(from, tx1)
				s.Add(from, tx2)

				// Set timeout status for all
				s.SetStatus(tx1.Hash(), core.PreconfStatusTimeout)
				s.SetStatus(tx2.Hash(), core.PreconfStatusTimeout)
			},
			expectedRemoved:   2,
			expectedRemaining: 0,
		},
		{
			name: "Mixed status transactions",
			setup: func(s *FIFOTxSet) {
				// Add a mix of transactions with different statuses
				tx1 := types.NewTransaction(1, common.HexToAddress("0x1"), nil, 0, nil, nil)
				tx2 := types.NewTransaction(2, common.HexToAddress("0x2"), nil, 0, nil, nil)
				tx3 := types.NewTransaction(3, common.HexToAddress("0x3"), nil, 0, nil, nil)
				tx4 := types.NewTransaction(4, common.HexToAddress("0x4"), nil, 0, nil, nil)
				from, _ := types.Sender(types.LatestSignerForChainID(common.Big1), tx1)

				s.Add(from, tx1)
				s.Add(from, tx2)
				s.Add(from, tx3)
				s.Add(from, tx4)

				// Set different statuses
				s.SetStatus(tx1.Hash(), core.PreconfStatusSuccess)
				s.SetStatus(tx2.Hash(), core.PreconfStatusTimeout)
				s.SetStatus(tx3.Hash(), core.PreconfStatusFailed)
				s.SetStatus(tx4.Hash(), core.PreconfStatusTimeout)
			},
			expectedRemoved:   2,
			expectedRemaining: 2,
		},
		{
			name: "Empty set",
			setup: func(s *FIFOTxSet) {
				// No transactions added
			},
			expectedRemoved:   0,
			expectedRemaining: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup FIFOTxSet
			s := NewFIFOTxSet()
			tt.setup(s)

			// Remember initial length
			initialLen := s.Len()

			// Execute CleanTimeout
			removed := s.CleanTimeout()

			// Verify results
			assert.Equal(t, tt.expectedRemoved, removed, "unexpected number of removed transactions")
			assert.Equal(t, tt.expectedRemaining, s.Len(), "unexpected number of remaining transactions")
			assert.Equal(t, initialLen-len(removed), s.Len(), "inconsistent removal count")

			// Verify all remaining transactions don't have timeout status
			for _, entry := range s.TxEntries() {
				assert.NotEqual(t, core.PreconfStatusTimeout, entry.Status,
					"transaction with timeout status should have been removed")
			}
		})
	}
}
