// Copyright 2025 The op-geth Authors
// This file is part of the op-geth library.
//
// The op-geth library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The op-geth library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package legacypool

import (
	"encoding/binary"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

// TestRollupCostFn_TxReplacement tests cost calculation during transaction replacement.
func TestRollupCostFn_TxReplacement(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Set rollup cost function that returns cost based on gas
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		// Simple mock: rollup cost = gas * 1000
		return uint256.NewInt(tx.Gas() * 1000)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0) // 0.01 ETH

	// Add first transaction
	tx1 := pricedTransaction(0, 21000, big.NewInt(1000000000), key) // 1 gwei
	err := pool.addRemoteSync(tx1)
	require.NoError(t, err)

	// Replace with higher gas price
	tx2 := pricedTransaction(0, 21000, big.NewInt(2000000000), key) // 2 gwei, 100% bump
	err = pool.addRemoteSync(tx2)
	require.NoError(t, err)

	// Verify tx2 replaced tx1
	require.Nil(t, pool.all.Get(tx1.Hash()))
	require.NotNil(t, pool.all.Get(tx2.Hash()))
}

// TestRollupCostFn_Overflow tests rollup cost causing overflow.
func TestRollupCostFn_Overflow(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Set a rollup cost that causes overflow
	maxUint256 := new(uint256.Int).SetAllOne()
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return maxUint256
	}

	// Create transaction
	tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	from, _ := types.Sender(types.HomesteadSigner{}, tx)

	// Set maximum balance
	pool.currentState.AddBalance(from, maxUint256, 0)

	// Should fail due to overflow
	err := pool.addRemote(tx)
	require.Error(t, err)
	require.Contains(t, err.Error(), "insufficient funds")
}

// TestRollupCostFn_ZeroCost tests the boundary case where rollup cost is 0.
func TestRollupCostFn_ZeroCost(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Return zero rollup cost
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return uint256.NewInt(0)
	}

	// Create transaction
	tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	from, _ := types.Sender(types.HomesteadSigner{}, tx)

	// Set balance to cover only base cost
	pool.currentState.AddBalance(from, uint256.MustFromBig(new(big.Int).Mul(tx.Cost(), big.NewInt(2))), 0)

	// Should succeed
	err := pool.addRemote(tx)
	require.NoError(t, err)
}

// TestRollupCostFn_WithDataTx tests rollup cost for transactions containing data.
func TestRollupCostFn_WithDataTx(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Compute rollup cost based on data size
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		costData := tx.RollupCostData()
		// Simple cost model: 4 wei per zero byte, 16 wei per non-zero byte
		cost := costData.Zeroes*4 + costData.Ones*16
		return uint256.NewInt(cost)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 100000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Create a transaction with data
	data := make([]byte, 1000)
	for i := range data {
		if i%2 == 0 {
			data[i] = 0 // 500 zero bytes
		} else {
			data[i] = 1 // 500 non-zero bytes
		}
	}

	tx := pricedDataTransaction(0, 100000, big.NewInt(1000000000), key, 1000)
	err := pool.addRemote(tx)
	require.NoError(t, err)
}

// TestRollupCostFn_ReplacementWithDifferentRollupCost tests replacement with different rollup costs.
func TestRollupCostFn_ReplacementWithDifferentRollupCost(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Mock rollup cost based on data size
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		costData := tx.RollupCostData()
		totalBytes := costData.Zeroes + costData.Ones
		return uint256.NewInt(totalBytes * 1000) // 1000 wei per byte
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 100000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Add a small-data transaction
	tx1 := pricedDataTransaction(0, 100000, big.NewInt(1000000000), key, 100) // 100 bytes
	err := pool.addRemoteSync(tx1)
	require.NoError(t, err)

	// Replace with higher price and larger data
	tx2 := pricedDataTransaction(0, 100000, big.NewInt(2000000000), key, 1000) // 1000 bytes
	err = pool.addRemoteSync(tx2)
	require.NoError(t, err)

	// Verify tx2 replaced tx1
	require.Nil(t, pool.all.Get(tx1.Hash()))
	require.NotNil(t, pool.all.Get(tx2.Hash()))
}

// TestRollupCostFn_DemoteUnexecutablesWithRollupCost tests demote behavior with rollup cost.
func TestRollupCostFn_DemoteUnexecutablesWithRollupCost(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCost := uint256.NewInt(500000)
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return rollupCost
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Calculate total cost per transaction
	baseCost := new(big.Int).Mul(big.NewInt(21000), big.NewInt(1000000000))
	baseCost.Add(baseCost, big.NewInt(100))
	totalCostPerTx := new(big.Int).Add(baseCost, rollupCost.ToBig())

	// Set balance sufficient for only 2 transactions
	balance := new(big.Int).Mul(totalCostPerTx, big.NewInt(2))
	pool.currentState.AddBalance(from, uint256.MustFromBig(balance), 0)

	// Try to add 3 transactions
	for i := 0; i < 3; i++ {
		tx := pricedTransaction(uint64(i), 21000, big.NewInt(1000000000), key)
		pool.addRemoteSync(tx)
	}

	// Trigger demote
	pool.mu.Lock()
	pool.demoteUnexecutables()
	pool.mu.Unlock()

	// Verify at most 2 transactions remain in pending
	pending, queued := pool.Stats()
	require.LessOrEqual(t, pending, 2, "should have at most 2 pending transactions due to balance limit")
	t.Logf("pending: %d, queued: %d", pending, queued)
}

// TestRollupCostFn_EmptyRollupCostData tests the case where RollupCostData is empty.
func TestRollupCostFn_EmptyRollupCostData(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Set a function that checks RollupCostData
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		costData := tx.RollupCostData()
		if costData == (types.RollupCostData{}) {
			return nil // Return nil for empty data
		}
		return uint256.NewInt(100000)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Create a transaction without data
	tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	err := pool.addRemote(tx)
	require.NoError(t, err)
}

// TestRollupCostFn_GapNonceWithRollupCost tests nonce gap handling with rollup cost.
func TestRollupCostFn_GapNonceWithRollupCost(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCost := uint256.NewInt(300000)
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return rollupCost
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Add nonce 0
	tx0 := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	err := pool.addRemoteSync(tx0)
	require.NoError(t, err)

	// Add nonce 2 (skip nonce 1)
	tx2 := pricedTransaction(2, 21000, big.NewInt(1000000000), key)
	err = pool.addRemoteSync(tx2)
	require.NoError(t, err, "should accept transaction with gap nonce")

	// tx2 should be in queue, not pending
	pending, queued := pool.Stats()
	require.Equal(t, 1, pending, "only tx0 should be pending")
	require.Equal(t, 1, queued, "tx2 should be queued")
}

// TestRollupCostFn_AccountSlotsLimit tests account slot limits combined with rollup cost.
func TestRollupCostFn_AccountSlotsLimit(t *testing.T) {
	t.Parallel()

	config := testTxPoolConfig
	config.AccountSlots = 2 // Limit each account to 2 transaction slots
	config.GlobalSlots = 10

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	blockchain := newTestBlockChain(params.TestChainConfig, 10000000, statedb, new(event.Feed))

	pool := New(config, blockchain)
	key, _ := crypto.GenerateKey()

	rollupCost := uint256.NewInt(100000)
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return rollupCost
	}

	reserver := newReserver()
	pool.Init(1, blockchain.CurrentBlock(), reserver)
	defer pool.Close()

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Try to add multiple transactions
	for i := 0; i < 5; i++ {
		tx := pricedTransaction(uint64(i), 21000, big.NewInt(1000000000), key)
		pool.addRemoteSync(tx)
	}

	// Due to slot limits, not all transactions will be in pending
	pending, queued := pool.Stats()
	total := pending + queued
	require.LessOrEqual(t, total, 5, "some transactions may be rejected or queued")
	t.Logf("pending: %d, queued: %d, total: %d", pending, queued, total)
}

// TestRollupCostFn_BeforeArsiaLegacyCost tests the legacy (Bedrock) cost model before Arsia.
func TestRollupCostFn_BeforeArsiaLegacyCost(t *testing.T) {
	t.Parallel()

	// Create Optimism config without Arsia upgrade time (simulates pre-Arsia)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(420),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime: new(uint64), // Regolith enabled at time 0
		// MantleArsiaTime: nil (not set, indicates pre-Arsia)
	}
	*config.RegolithTime = 0

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

	// Set Bedrock-style L1 attributes
	l1BaseFee := big.NewInt(1000000000) // 1 gwei
	overhead := big.NewInt(2100)        // fixed overhead
	scalar := big.NewInt(1000000)       // scalar (divided by 1e6)
	tokenRatio := big.NewInt(1)         // tokenRatio = 1 (avoid gasMultiplier amplification)
	statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(l1BaseFee))
	statedb.SetState(types.L1BlockAddr, types.OverheadSlot, common.BigToHash(overhead))
	statedb.SetState(types.L1BlockAddr, types.ScalarSlot, common.BigToHash(scalar))
	statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(tokenRatio))

	blockchain := newTestBlockChain(config, 30000000, statedb, new(event.Feed))

	pool := New(testTxPoolConfig, blockchain)
	key, _ := crypto.GenerateKey()

	reserver := newReserver()
	pool.Init(1000000, blockchain.CurrentBlock(), reserver)
	defer pool.Close()

	// Verify rollupCostFn is properly initialized
	require.NotNil(t, pool.rollupCostFn, "rollupCostFn should be initialized for Optimism chain")

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance (must cover Bedrock L1 cost)
	pool.currentState.AddBalance(from, uint256.NewInt(1000000000000000000), 0) // 1 ETH

	// Create transaction (no data)
	tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)

	// Manually calculate expected L1 cost (Bedrock model)
	// L1 cost = (rollupDataGas + overhead) * l1BaseFee * scalar * tokenRatio / Decimals
	rollupData := tx.RollupCostData()
	rollupDataGas := rollupData.DataGas(0, config)
	gasWithOverhead := new(big.Int).SetUint64(rollupDataGas)
	gasWithOverhead.Add(gasWithOverhead, overhead)
	expectedL1Cost := new(big.Int).Mul(gasWithOverhead, l1BaseFee)
	expectedL1Cost.Mul(expectedL1Cost, scalar)
	expectedL1Cost.Mul(expectedL1Cost, tokenRatio)
	expectedL1Cost.Div(expectedL1Cost, types.Decimals)

	t.Logf("Bedrock L1 Cost: %s", expectedL1Cost.String())
	t.Logf("Base Cost: %s", tx.Cost().String())
	t.Logf("Rollup Data Gas: %d", rollupDataGas)

	err := pool.addRemote(tx)
	require.NoError(t, err, "transaction should be accepted before Arsia with legacy cost model")
}

// TestRollupCostFn_AfterArsiaWithOperatorFee tests post-Arsia cost calculation including operator fee.
func TestRollupCostFn_AfterArsiaWithOperatorFee(t *testing.T) {
	t.Parallel()

	// Create Optimism config with Arsia upgrade time
	arsiaTime := uint64(1000)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(420),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime:    &arsiaTime,
		MantleArsiaTime: &arsiaTime,
	}

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

	// Set post-Arsia L1 attributes (Fjord model)
	l1BaseFee := big.NewInt(1000000000)    // 1 gwei
	l1BlobBaseFee := big.NewInt(1000000)   // 1 blob gwei
	baseFeeScalar := uint32(5000)          // 0.005
	blobBaseFeeScalar := uint32(100000)    // 0.1
	operatorFeeScalar := uint32(500)       // 0.0005
	operatorFeeConstant := uint64(1000000) // 1000000 wei

	statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(l1BaseFee))
	statedb.SetState(types.L1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(l1BlobBaseFee))

	// Set L1FeeScalars (Ecotone format)
	l1FeeScalars := make([]byte, 32)
	binary.BigEndian.PutUint32(l1FeeScalars[16:20], baseFeeScalar)
	binary.BigEndian.PutUint32(l1FeeScalars[20:24], blobBaseFeeScalar)
	statedb.SetState(types.L1BlockAddr, types.L1FeeScalarsSlot, common.BytesToHash(l1FeeScalars))

	// Set OperatorFeeParams
	operatorFeeParams := make([]byte, 32)
	binary.BigEndian.PutUint32(operatorFeeParams[20:24], operatorFeeScalar)
	binary.BigEndian.PutUint64(operatorFeeParams[24:32], operatorFeeConstant)
	statedb.SetState(types.L1BlockAddr, types.OperatorFeeParamsSlot, common.BytesToHash(operatorFeeParams))

	// Set tokenRatio (still used after Arsia)
	tokenRatio := big.NewInt(1000000) // 1.0
	statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(tokenRatio))

	blockchain := newTestBlockChain(config, 30000000, statedb, new(event.Feed))

	pool := New(testTxPoolConfig, blockchain)
	key, _ := crypto.GenerateKey()

	reserver := newReserver()
	head := blockchain.CurrentBlock()
	head.Time = arsiaTime + 100 // Set header time to post-Arsia
	pool.Init(1000000, head, reserver)
	defer pool.Close()

	// Verify rollupCostFn is properly initialized
	require.NotNil(t, pool.rollupCostFn, "rollupCostFn should be initialized for Optimism chain")

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Create transaction with more gas to test operator fee
	tx := pricedTransaction(0, 100000, big.NewInt(1000000000), key)

	t.Logf("Base Cost: %s", tx.Cost().String())
	t.Logf("Transaction Gas: %d", tx.Gas())

	err := pool.addRemote(tx)
	require.NoError(t, err, "transaction should be accepted after Arsia with operator fee")
}

// TestRollupCostFn_ArsiaTransitionBlock tests the first block at Arsia upgrade time.
func TestRollupCostFn_ArsiaTransitionBlock(t *testing.T) {
	t.Parallel()

	// Arsia upgrade time
	arsiaTime := uint64(1000)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(420),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime:    &arsiaTime,
		MantleArsiaTime: &arsiaTime,
	}

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

	// Set first Arsia block state (Ecotone params not yet set)
	l1BaseFee := big.NewInt(1000000000)
	l1BlobBaseFee := big.NewInt(0) // Zero at first block
	statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(l1BaseFee))
	statedb.SetState(types.L1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(l1BlobBaseFee))

	// Empty L1FeeScalars (first Arsia block)
	emptyScalars := make([]byte, 32)
	statedb.SetState(types.L1BlockAddr, types.L1FeeScalarsSlot, common.BytesToHash(emptyScalars))

	// Set Bedrock params (fallback)
	overhead := big.NewInt(2100)
	scalar := big.NewInt(1000000)
	tokenRatio := big.NewInt(1) // tokenRatio = 1 (post-Arsia gasMultiplier=1, but Bedrock fallback still uses tokenRatio)
	statedb.SetState(types.L1BlockAddr, types.OverheadSlot, common.BigToHash(overhead))
	statedb.SetState(types.L1BlockAddr, types.ScalarSlot, common.BigToHash(scalar))
	statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(tokenRatio))

	blockchain := newTestBlockChain(config, 30000000, statedb, new(event.Feed))

	pool := New(testTxPoolConfig, blockchain)
	key, _ := crypto.GenerateKey()

	reserver := newReserver()
	head := blockchain.CurrentBlock()
	head.Time = arsiaTime // Set header time to exactly the Arsia upgrade time
	pool.Init(1000000, head, reserver)
	defer pool.Close()

	// Verify rollupCostFn is properly initialized
	require.NotNil(t, pool.rollupCostFn, "rollupCostFn should be initialized")

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Create transaction
	tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)

	err := pool.addRemote(tx)
	require.NoError(t, err, "transaction should be accepted in first Arsia block using Bedrock fallback")
}

// TestRollupCostFn_TokenRatioChange tests handling when tokenRatio changes at runtime.
func TestRollupCostFn_TokenRatioChange(t *testing.T) {
	t.Parallel()

	arsiaTime := uint64(1000)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(420),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime:    &arsiaTime,
		MantleArsiaTime: &arsiaTime,
	}

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

	// Set post-Arsia L1 attributes
	l1BaseFee := big.NewInt(1000000000)
	l1BlobBaseFee := big.NewInt(1000000)
	baseFeeScalar := uint32(5000)
	blobBaseFeeScalar := uint32(100000)

	statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(l1BaseFee))
	statedb.SetState(types.L1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(l1BlobBaseFee))

	l1FeeScalars := make([]byte, 32)
	binary.BigEndian.PutUint32(l1FeeScalars[16:20], baseFeeScalar)
	binary.BigEndian.PutUint32(l1FeeScalars[20:24], blobBaseFeeScalar)
	statedb.SetState(types.L1BlockAddr, types.L1FeeScalarsSlot, common.BytesToHash(l1FeeScalars))

	// Initial tokenRatio = 1.0
	initialTokenRatio := big.NewInt(1000000)
	statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(initialTokenRatio))

	blockchain := newTestBlockChain(config, 30000000, statedb, new(event.Feed))

	pool := New(testTxPoolConfig, blockchain)
	key, _ := crypto.GenerateKey()

	reserver := newReserver()
	head := blockchain.CurrentBlock()
	head.Time = arsiaTime + 100 // Set header time to post-Arsia
	pool.Init(1000000, head, reserver)
	defer pool.Close()

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	// Add first transaction (tokenRatio = 1.0)
	tx1 := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	err := pool.addRemoteSync(tx1)
	require.NoError(t, err, "first transaction should be accepted")

	// Change tokenRatio (simulate GasOracle update)
	newTokenRatio := big.NewInt(1500000) // 1.5
	pool.currentState.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(newTokenRatio))

	// Add second transaction (should use the new tokenRatio)
	tx2 := pricedTransaction(1, 21000, big.NewInt(1000000000), key)
	err = pool.addRemoteSync(tx2)
	require.NoError(t, err, "second transaction should be accepted with new tokenRatio")

	// Verify both transactions are in the pool
	pending, _ := pool.Stats()
	require.Equal(t, 2, pending, "both transactions should be in pending")
}

// TestRollupCostFn_BeforeAfterArsiaComparison compares cost differences before and after Arsia.
func TestRollupCostFn_BeforeAfterArsiaComparison(t *testing.T) {
	t.Parallel()

	arsiaTime := uint64(500)

	// Create two configs: one pre-Arsia, one post-Arsia
	configBefore := &params.ChainConfig{
		ChainID:        big.NewInt(420),
		HomesteadBlock: big.NewInt(0),
		BerlinBlock:    big.NewInt(0),
		LondonBlock:    big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime: new(uint64),
		// MantleArsiaTime not set
	}
	*configBefore.RegolithTime = 0

	configAfter := &params.ChainConfig{
		ChainID:        big.NewInt(420),
		HomesteadBlock: big.NewInt(0),
		BerlinBlock:    big.NewInt(0),
		LondonBlock:    big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime:    &arsiaTime,
		MantleArsiaTime: &arsiaTime,
	}

	// Test pre-Arsia - use tokenRatio=1 to avoid gasMultiplier issue
	t.Run("Before Arsia", func(t *testing.T) {
		statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

		// Bedrock-style params
		statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(big.NewInt(1000000000)))
		statedb.SetState(types.L1BlockAddr, types.OverheadSlot, common.BigToHash(big.NewInt(2100)))
		statedb.SetState(types.L1BlockAddr, types.ScalarSlot, common.BigToHash(big.NewInt(1000000)))
		statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(big.NewInt(1))) // tokenRatio=1

		blockchain := newTestBlockChain(configBefore, 30000000, statedb, new(event.Feed))
		pool := New(testTxPoolConfig, blockchain)
		key, _ := crypto.GenerateKey()

		reserver := newReserver()
		pool.Init(1000000, blockchain.CurrentBlock(), reserver)
		defer pool.Close()

		from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))
		pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

		tx := pricedTransaction(0, 100000, big.NewInt(1000000000), key)
		err := pool.addRemote(tx)
		require.NoError(t, err)
		t.Log("Before Arsia: transaction accepted")
	})

	// Test post-Arsia
	t.Run("After Arsia", func(t *testing.T) {
		statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

		// Arsia (Fjord) style params
		statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(big.NewInt(1000000000)))
		statedb.SetState(types.L1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(big.NewInt(1000000)))

		l1FeeScalars := make([]byte, 32)
		binary.BigEndian.PutUint32(l1FeeScalars[16:20], 5000)
		binary.BigEndian.PutUint32(l1FeeScalars[20:24], 100000)
		statedb.SetState(types.L1BlockAddr, types.L1FeeScalarsSlot, common.BytesToHash(l1FeeScalars))

		operatorFeeParams := make([]byte, 32)
		binary.BigEndian.PutUint32(operatorFeeParams[20:24], 500)
		binary.BigEndian.PutUint64(operatorFeeParams[24:32], 1000000)
		statedb.SetState(types.L1BlockAddr, types.OperatorFeeParamsSlot, common.BytesToHash(operatorFeeParams))

		statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(big.NewInt(1000000)))

		blockchain := newTestBlockChain(configAfter, 30000000, statedb, new(event.Feed))

		pool := New(testTxPoolConfig, blockchain)
		key, _ := crypto.GenerateKey()

		reserver := newReserver()
		head := blockchain.CurrentBlock()
		head.Time = arsiaTime + 100
		pool.Init(1000000, head, reserver)
		defer pool.Close()

		from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))
		pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

		tx := pricedTransaction(0, 100000, big.NewInt(1000000000), key)
		err := pool.addRemote(tx)
		require.NoError(t, err)
		t.Log("After Arsia: transaction accepted with operator fee")
	})
}

// TestRollupCostFn_ArsiaOperatorFeeZero tests the case where operator fee is zero.
func TestRollupCostFn_ArsiaOperatorFeeZero(t *testing.T) {
	t.Parallel()

	arsiaTime := uint64(1000)
	config := &params.ChainConfig{
		ChainID:        big.NewInt(420),
		HomesteadBlock: big.NewInt(0),
		BerlinBlock:    big.NewInt(0),
		LondonBlock:    big.NewInt(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  6,
			EIP1559Denominator: 50,
		},
		RegolithTime:    &arsiaTime,
		MantleArsiaTime: &arsiaTime,
	}

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())

	// Set L1 attributes
	statedb.SetState(types.L1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(big.NewInt(1000000000)))
	statedb.SetState(types.L1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(big.NewInt(1000000)))

	l1FeeScalars := make([]byte, 32)
	binary.BigEndian.PutUint32(l1FeeScalars[16:20], 5000)
	binary.BigEndian.PutUint32(l1FeeScalars[20:24], 100000)
	statedb.SetState(types.L1BlockAddr, types.L1FeeScalarsSlot, common.BytesToHash(l1FeeScalars))

	// All operator fee params set to zero
	operatorFeeParams := make([]byte, 32)
	statedb.SetState(types.L1BlockAddr, types.OperatorFeeParamsSlot, common.BytesToHash(operatorFeeParams))

	statedb.SetState(types.GasOracleAddr, types.TokenRatioSlot, common.BigToHash(big.NewInt(1000000)))

	blockchain := newTestBlockChain(config, 30000000, statedb, new(event.Feed))

	pool := New(testTxPoolConfig, blockchain)
	key, _ := crypto.GenerateKey()

	reserver := newReserver()
	head := blockchain.CurrentBlock()
	head.Time = arsiaTime + 100
	pool.Init(1000000, head, reserver)
	defer pool.Close()

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0)

	tx := pricedTransaction(0, 100000, big.NewInt(1000000000), key)
	err := pool.addRemote(tx)
	require.NoError(t, err, "transaction should be accepted with zero operator fee")
}

// TestRollupCostFn_NilReplacementTx tests that replacing a pending tx works correctly
// when rollupCostFn is nil (non-Optimism chains). This exercises the ExistingCost callback
// path where pool.rollupCostFn could previously panic with nil dereference.
func TestRollupCostFn_NilReplacementTx(t *testing.T) {
	t.Parallel()

	// Use non-Optimism config so rollupCostFn remains nil
	config := params.TestChainConfig
	pool, key := setupPoolWithConfig(config)
	defer pool.Close()

	// Verify rollupCostFn is nil
	require.Nil(t, pool.rollupCostFn, "rollupCostFn should be nil for non-Optimism chain")

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set enough balance
	pool.currentState.AddBalance(from, uint256.NewInt(10000000000000000), 0) // 0.01 ETH

	// Add first transaction at nonce 0
	tx1 := pricedTransaction(0, 21000, big.NewInt(1000000000), key) // 1 gwei
	err := pool.addRemoteSync(tx1)
	require.NoError(t, err, "first tx should be accepted")

	// Replace with higher gas price - this triggers ExistingCost callback
	// which previously could panic if rollupCostFn is nil
	tx2 := pricedTransaction(0, 21000, big.NewInt(2000000000), key) // 2 gwei
	err = pool.addRemoteSync(tx2)
	require.NoError(t, err, "replacement tx should be accepted without panic")

	// Verify replacement happened
	require.Nil(t, pool.all.Get(tx1.Hash()), "old tx should be removed")
	require.NotNil(t, pool.all.Get(tx2.Hash()), "new tx should exist")
}

// TestRollupCostFn_ExactBalanceBoundary tests the precise balance boundary where
// balance == baseCost + rollupCost should succeed, and balance - 1 should fail.
func TestRollupCostFn_ExactBalanceBoundary(t *testing.T) {
	t.Parallel()

	rollupCost := uint256.NewInt(1234567890)

	// Test 1: Exact balance should succeed
	t.Run("ExactBalance", func(t *testing.T) {
		pool, key := setupPool()
		defer pool.Close()

		pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
			return new(uint256.Int).Set(rollupCost)
		}

		tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
		from, _ := types.Sender(types.HomesteadSigner{}, tx)

		// Calculate exact total cost = baseCost + rollupCost
		baseCost := tx.Cost()
		exactBalance := new(big.Int).Add(baseCost, rollupCost.ToBig())

		pool.currentState.SetBalance(from, uint256.MustFromBig(exactBalance), 0)
		err := pool.addRemote(tx)
		require.NoError(t, err, "exact balance should be accepted")
	})

	// Test 2: 1 wei short should fail
	t.Run("OneWeiShort", func(t *testing.T) {
		pool, key := setupPool()
		defer pool.Close()

		pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
			return new(uint256.Int).Set(rollupCost)
		}

		tx := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
		from, _ := types.Sender(types.HomesteadSigner{}, tx)

		baseCost := tx.Cost()
		exactBalance := new(big.Int).Add(baseCost, rollupCost.ToBig())
		shortBalance := new(big.Int).Sub(exactBalance, big.NewInt(1))

		pool.currentState.SetBalance(from, uint256.MustFromBig(shortBalance), 0)
		err := pool.addRemote(tx)
		require.Error(t, err, "1 wei short should be rejected")
		require.Contains(t, err.Error(), "insufficient funds")
	})
}

// TestRollupCostFn_DynamicFeeTx tests rollup cost with EIP-1559 DynamicFee transactions.
func TestRollupCostFn_DynamicFeeTx(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCost := uint256.NewInt(5000000)
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(rollupCost)
	}

	// Create EIP-1559 tx: cost = gas * gasFeeCap + value
	tx := dynamicFeeTx(0, 21000, big.NewInt(2000000000), big.NewInt(1000000000), key)
	from, _ := types.Sender(types.LatestSignerForChainID(params.TestChainConfig.ChainID), tx)

	// Calculate total cost
	baseCost := tx.Cost() // gas * gasFeeCap + value
	totalCost := new(big.Int).Add(baseCost, rollupCost.ToBig())

	// Set sufficient balance
	pool.currentState.AddBalance(from, uint256.MustFromBig(new(big.Int).Mul(totalCost, big.NewInt(2))), 0)

	err := pool.addRemote(tx)
	require.NoError(t, err, "EIP-1559 tx with rollup cost should be accepted")
}

// TestRollupCostFn_DynamicFeeTxInsufficientBalance tests EIP-1559 tx rejected when
// balance only covers base cost but not rollup cost.
func TestRollupCostFn_DynamicFeeTxInsufficientBalance(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCost := uint256.NewInt(1000000000000) // 1e12
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(rollupCost)
	}

	tx := dynamicFeeTx(0, 21000, big.NewInt(2000000000), big.NewInt(1000000000), key)
	from, _ := types.Sender(types.LatestSignerForChainID(params.TestChainConfig.ChainID), tx)

	// Only set enough for base cost, not rollup cost
	pool.currentState.SetBalance(from, uint256.MustFromBig(tx.Cost()), 0)

	err := pool.addRemote(tx)
	require.Error(t, err, "EIP-1559 tx should be rejected when balance doesn't cover rollup cost")
	require.Contains(t, err.Error(), "insufficient funds")
}

// TestRollupCostFn_MultiTxCumulativeBalance tests that with multiple pending
// transactions and rollup costs, the pool correctly rejects transactions when
// the cumulative base cost + new tx's rollup cost exceeds balance.
// Note: ExistingExpenditure uses list.totalcost (base costs only) for performance,
// consistent with upstream geth design. The new tx's rollup cost IS included via TotalTxCost.
func TestRollupCostFn_MultiTxCumulativeBalance(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCostPerTx := uint256.NewInt(1000000000000) // 1e12 per tx
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(rollupCostPerTx)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Each tx base cost = 21000 * 1e9 + 100 = ~21e12
	// Each tx total cost (base + rollup) = ~22e12
	// Balance = 3 * totalCost = ~66e12
	//
	// Adding 4th tx check: need = spent(baseCosts of 3 txs = ~63e12) + cost(~22e12) = ~85e12 > ~66e12 -> rejected
	baseCostPerTx := new(big.Int).Add(
		new(big.Int).Mul(big.NewInt(21000), big.NewInt(1000000000)),
		big.NewInt(100),
	)
	totalCostPerTx := new(big.Int).Add(baseCostPerTx, rollupCostPerTx.ToBig())

	balance := new(big.Int).Mul(totalCostPerTx, big.NewInt(3))
	pool.currentState.AddBalance(from, uint256.MustFromBig(balance), 0)

	// Add 3 transactions - all should succeed
	for i := 0; i < 3; i++ {
		tx := pricedTransaction(uint64(i), 21000, big.NewInt(1000000000), key)
		err := pool.addRemoteSync(tx)
		require.NoError(t, err, "tx %d should succeed - balance covers 3 txs total cost", i)
	}

	// 4th transaction should fail - cumulative base costs + new tx total cost exceeds balance
	tx4 := pricedTransaction(3, 21000, big.NewInt(1000000000), key)
	err := pool.addRemoteSync(tx4)
	require.Error(t, err, "4th tx should fail - balance only covers 3 txs")
	require.Contains(t, err.Error(), "insufficient funds")

	// Verify exactly 3 pending
	pending, _ := pool.Stats()
	require.Equal(t, 3, pending)
}

// TestRollupCostFn_ReplacementWithExistingExpenditure tests that tx replacement
// correctly accounts for rollup costs in both ExistingExpenditure and ExistingCost.
func TestRollupCostFn_ReplacementWithExistingExpenditure(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	rollupCost := uint256.NewInt(500000000000) // 5e11
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(rollupCost)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	// Set balance that can cover 2 txs with rollup cost + 1 replacement
	baseCostPerTx := new(big.Int).Add(
		new(big.Int).Mul(big.NewInt(21000), big.NewInt(2000000000)), // using 2gwei for replacement headroom
		big.NewInt(100),
	)
	totalPerTx := new(big.Int).Add(baseCostPerTx, rollupCost.ToBig())
	balance := new(big.Int).Mul(totalPerTx, big.NewInt(2))
	pool.currentState.AddBalance(from, uint256.MustFromBig(balance), 0)

	// Add tx at nonce 0 and nonce 1
	tx0 := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	err := pool.addRemoteSync(tx0)
	require.NoError(t, err)

	tx1 := pricedTransaction(1, 21000, big.NewInt(1000000000), key)
	err = pool.addRemoteSync(tx1)
	require.NoError(t, err)

	// Replace tx0 with higher gas price
	tx0r := pricedTransaction(0, 21000, big.NewInt(2000000000), key)
	err = pool.addRemoteSync(tx0r)
	require.NoError(t, err, "replacement should succeed")

	// Verify replacement
	require.Nil(t, pool.all.Get(tx0.Hash()))
	require.NotNil(t, pool.all.Get(tx0r.Hash()))
	require.NotNil(t, pool.all.Get(tx1.Hash()))
}

// TestRollupCostFn_RollupCostChangesAfterReset tests that when rollupCostFn changes
// after a pool reset, new transactions are validated with the updated cost function.
func TestRollupCostFn_RollupCostChangesAfterReset(t *testing.T) {
	t.Parallel()

	pool, key := setupPool()
	defer pool.Close()

	// Start with low rollup cost
	lowCost := uint256.NewInt(100000)
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(lowCost)
	}

	from, _ := types.Sender(types.HomesteadSigner{}, pricedTransaction(0, 21000, big.NewInt(1), key))

	baseCost := new(big.Int).Add(
		new(big.Int).Mul(big.NewInt(21000), big.NewInt(1000000000)),
		big.NewInt(100),
	)
	// Set balance = baseCost + lowCost (tight)
	totalLow := new(big.Int).Add(baseCost, lowCost.ToBig())
	pool.currentState.SetBalance(from, uint256.MustFromBig(totalLow), 0)

	// This should succeed with low cost
	tx1 := pricedTransaction(0, 21000, big.NewInt(1000000000), key)
	err := pool.addRemote(tx1)
	require.NoError(t, err, "should succeed with low rollup cost")

	// Simulate cost function update (as would happen during resetRollupCostFn)
	highCost := uint256.NewInt(1000000000000) // 1e12 - much higher
	pool.rollupCostFn = func(tx types.RollupTransaction) *uint256.Int {
		return new(uint256.Int).Set(highCost)
	}

	// New tx should fail because balance can't cover the higher rollup cost
	tx2 := pricedTransaction(1, 21000, big.NewInt(1000000000), key)
	err = pool.addRemote(tx2)
	require.Error(t, err, "should fail with higher rollup cost")
	require.Contains(t, err.Error(), "insufficient funds")
}
