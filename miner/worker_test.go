package miner

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus"
	"github.com/ethereum/go-ethereum/consensus/clique"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto/kzg4844"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/triedb"
)

func (bc *testBlockChain) Engine() consensus.Engine {
	return ethash.NewFaker()
}

func (bc *testBlockChain) GetHeader(hash common.Hash, number uint64) *types.Header {
	return &types.Header{
		Number:     big.NewInt(int64(number)),
		ParentHash: hash,
		Time:       1000,
		Difficulty: big.NewInt(int64(number)),
	}
}

func TestEnvironmentCopy(t *testing.T) {
	// Create original environment
	db := rawdb.NewMemoryDatabase()
	stateDB, err := state.New(common.Hash{}, state.NewDatabase(triedb.NewDatabase(db, nil), nil))
	if err != nil {
		t.Fatalf("failed to create state: %v", err)
	}

	original := &environment{
		signer:   types.NewEIP155Signer(big.NewInt(1)),
		state:    stateDB,
		tcount:   10,
		coinbase: common.HexToAddress("0x1234567890123456789012345678901234567890"),
		header: &types.Header{
			Number:     big.NewInt(1),
			ParentHash: common.HexToHash("0x123"),
			Time:       1000,
			Difficulty: big.NewInt(1),
		},
		gasPool: new(core.GasPool).AddGas(1000000),
		blobs:   5,
	}

	// Add some transactions
	original.txs = []*types.Transaction{
		types.NewTransaction(1, common.HexToAddress("0x1"), big.NewInt(100), 21000, big.NewInt(1), nil),
		types.NewTransaction(2, common.HexToAddress("0x2"), big.NewInt(200), 21000, big.NewInt(2), nil),
	}

	// Add some receipts
	original.receipts = []*types.Receipt{
		{
			Status:            types.ReceiptStatusSuccessful,
			CumulativeGasUsed: 21000,
			Logs:              []*types.Log{},
		},
	}

	// Add some sidecars
	original.sidecars = []*types.BlobTxSidecar{
		{
			Blobs:       []kzg4844.Blob{{1, 2, 3}},
			Commitments: []kzg4844.Commitment{{4, 5, 6}},
			Proofs:      []kzg4844.Proof{{7, 8, 9}},
		},
	}

	// Create chainConfig
	chainDB := rawdb.NewMemoryDatabase()
	triedb := triedb.NewDatabase(chainDB, nil)
	genesis := minerTestGenesisBlock(15, 11_500_000, common.HexToAddress("12345"))
	chainConfig, _, _, err := core.SetupGenesisBlock(chainDB, triedb, genesis)
	if err != nil {
		t.Fatalf("can't create new chain config: %v", err)
	}
	// Create consensus engine
	engine := clique.New(chainConfig.Clique, chainDB)
	// Create Ethereum backend
	bc, err := core.NewBlockChain(chainDB, nil, genesis, nil, engine, vm.Config{}, nil)
	if err != nil {
		t.Fatalf("can't create new chain %v", err)
	}
	statedb, _ := state.New(bc.Genesis().Root(), bc.StateCache())
	blockchain := &testBlockChain{bc.Genesis().Root(), chainConfig, statedb, 10000000, new(event.Feed)}
	// Create EVM
	blockCtx := core.NewEVMBlockContext(original.header, blockchain, nil, chainConfig, original.state)
	original.evm = vm.NewEVM(blockCtx, original.state, chainConfig, vm.Config{})

	// Execute copy
	copied := original.copy(blockchain)

	// Test basic fields
	if copied.signer.ChainID().Cmp(original.signer.ChainID()) != 0 {
		t.Error("signer chain ID mismatch")
	}
	if copied.tcount != original.tcount {
		t.Error("tcount mismatch")
	}
	if copied.coinbase != original.coinbase {
		t.Error("coinbase mismatch")
	}
	if copied.blobs != original.blobs {
		t.Error("blobs count mismatch")
	}

	// Test header
	if copied.header.Number.Cmp(original.header.Number) != 0 {
		t.Error("header number mismatch")
	}
	if copied.header.ParentHash != original.header.ParentHash {
		t.Error("header parent hash mismatch")
	}
	if copied.header.Time != original.header.Time {
		t.Error("header time mismatch")
	}

	// Test gas pool
	if copied.gasPool.Gas() != original.gasPool.Gas() {
		t.Error("gas pool mismatch")
	}

	// Test transactions
	if len(copied.txs) != len(original.txs) {
		t.Error("txs length mismatch")
	}
	for i, tx := range copied.txs {
		if tx.Hash() != original.txs[i].Hash() {
			t.Error("tx hash mismatch at index", i)
		}
	}

	// Test receipts
	if len(copied.receipts) != len(original.receipts) {
		t.Error("receipts length mismatch")
	}
	for i, receipt := range copied.receipts {
		if receipt.Status != original.receipts[i].Status {
			t.Error("receipt status mismatch at index", i)
		}
	}

	// Test sidecars
	if len(copied.sidecars) != len(original.sidecars) {
		t.Error("sidecars length mismatch")
	}
	for i, sidecar := range copied.sidecars {
		if len(sidecar.Blobs) != len(original.sidecars[i].Blobs) {
			t.Error("sidecar blobs length mismatch at index", i)
		}
	}

	// Test EVM
	if copied.evm == nil {
		t.Error("evm is nil in copy")
	}
	if copied.evm.ChainConfig().ChainID.Cmp(original.evm.ChainConfig().ChainID) != 0 {
		t.Error("evm chain config mismatch")
	}

	// Test that modifying copy doesn't affect original
	copied.tcount = 20
	if original.tcount == copied.tcount {
		t.Error("modifying copy affected original")
	}

	// Modify gas pool
	copied.gasPool = new(core.GasPool).AddGas(2000000)
	if original.gasPool.Gas() == copied.gasPool.Gas() {
		t.Error("modifying copy's gas pool affected original")
	}

	copied.txs[0] = types.NewTransaction(3, common.HexToAddress("0x3"), big.NewInt(300), 21000, big.NewInt(3), nil)
	if original.txs[0].Hash() == copied.txs[0].Hash() {
		t.Error("modifying copy's txs affected original")
	}
}
