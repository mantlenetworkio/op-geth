// Copyright 2025 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package txpool

import (
	"crypto/ecdsa"
	"encoding/binary"
	"errors"
	"math"
	"math/big"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
)

func TestValidateTransactionEIP2681(t *testing.T) {
	key, err := crypto.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}

	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   5000000,
		Time:       1,
		Difficulty: big.NewInt(1),
	}

	signer := types.LatestSigner(params.TestChainConfig)

	// Create validation options
	opts := &ValidationOptions{
		Config:       params.TestChainConfig,
		Accept:       0xFF, // Accept all transaction types
		MaxSize:      32 * 1024,
		MaxBlobCount: 6,
		MinTip:       big.NewInt(0),
	}

	tests := []struct {
		name    string
		nonce   uint64
		wantErr error
	}{
		{
			name:    "normal nonce",
			nonce:   42,
			wantErr: nil,
		},
		{
			name:    "max allowed nonce (2^64-2)",
			nonce:   math.MaxUint64 - 1,
			wantErr: nil,
		},
		{
			name:    "EIP-2681 nonce overflow (2^64-1)",
			nonce:   math.MaxUint64,
			wantErr: core.ErrNonceMax,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tx := createTestTransaction(key, tt.nonce)
			err := ValidateTransaction(tx, head, signer, opts)

			if tt.wantErr == nil {
				if err != nil {
					t.Errorf("ValidateTransaction() error = %v, wantErr nil", err)
				}
			} else {
				if err == nil {
					t.Errorf("ValidateTransaction() error = nil, wantErr %v", tt.wantErr)
				} else if !errors.Is(err, tt.wantErr) {
					t.Errorf("ValidateTransaction() error = %v, wantErr %v", err, tt.wantErr)
				}
			}
		})
	}
}

// createTestTransaction creates a basic transaction for testing
func createTestTransaction(key *ecdsa.PrivateKey, nonce uint64) *types.Transaction {
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")

	txdata := &types.LegacyTx{
		Nonce:    nonce,
		To:       &to,
		Value:    big.NewInt(1000),
		Gas:      21000,
		GasPrice: big.NewInt(1),
		Data:     nil,
	}

	tx := types.NewTx(txdata)
	signedTx, _ := types.SignTx(tx, types.HomesteadSigner{}, key)
	return signedTx
}

// Test parameters for operator fee calculation
var (
	testBaseFee             = big.NewInt(1000 * 1e6) // 1000 gwei
	testBlobBaseFee         = big.NewInt(10 * 1e6)   // 10 gwei
	testBaseFeeScalar       = uint32(2)
	testBlobBaseFeeScalar   = uint32(3)
	testOperatorFeeScalar   = uint32(1439103868)          // ~1.44e9
	testOperatorFeeConstant = uint64(1256417826609331460) // ~1.26e18
	testTokenRatio          = big.NewInt(1000000)         // 1e6

	// Pre-calculated expected operator fee for gas = 21000:
	// operatorFee = gas * operatorFeeScalar * 100 + operatorFeeConstant
	// = 21000 * 1439103868 * 100 + 1256417826609331460
	// = 3022118122800000 + 1256417826609331460
	// = 1259439944732131460
	expectedOperatorFeeForGas21000 = new(big.Int).SetUint64(1259439944732131460)
)

// testStateGetterForValidation implements types.StateGetter for testing rollup cost functions
type testStateGetterForValidation struct {
	baseFee, blobBaseFee, overhead, scalar, tokenRatio *big.Int
	baseFeeScalar, blobBaseFeeScalar                   uint32
	operatorFeeScalar                                  uint32
	operatorFeeConstant                                uint64
}

func (sg *testStateGetterForValidation) GetState(addr common.Address, slot common.Hash) common.Hash {
	buf := common.Hash{}
	switch slot {
	case types.L1BaseFeeSlot:
		sg.baseFee.FillBytes(buf[:])
	case types.OverheadSlot:
		sg.overhead.FillBytes(buf[:])
	case types.ScalarSlot:
		sg.scalar.FillBytes(buf[:])
	case types.L1BlobBaseFeeSlot:
		sg.blobBaseFee.FillBytes(buf[:])
	case types.L1FeeScalarsSlot:
		// Ecotone fee scalars at offset 24
		binary.BigEndian.PutUint32(buf[24:28], sg.baseFeeScalar)
		binary.BigEndian.PutUint32(buf[28:32], sg.blobBaseFeeScalar)
	case types.OperatorFeeParamsSlot:
		// Operator fee params: scalar at [20:24], constant at [24:32]
		binary.BigEndian.PutUint32(buf[20:24], sg.operatorFeeScalar)
		binary.BigEndian.PutUint64(buf[24:32], sg.operatorFeeConstant)
	case types.TokenRatioSlot:
		sg.tokenRatio.FillBytes(buf[:])
	default:
		// Return empty hash for unknown slots
	}
	return buf
}

// createDynamicFeeTx creates a EIP-1559 transaction for testing
func createDynamicFeeTx(key *ecdsa.PrivateKey, nonce uint64, gas uint64, value *big.Int, gasTipCap, gasFeeCap *big.Int) *types.Transaction {
	to := common.HexToAddress("0x0000000000000000000000000000000000000001")
	txdata := &types.DynamicFeeTx{
		Nonce:     nonce,
		To:        &to,
		Value:     value,
		Gas:       gas,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Data:      nil,
	}
	tx := types.NewTx(txdata)
	signer := types.LatestSignerForChainID(big.NewInt(1))
	signedTx, _ := types.SignTx(tx, signer, key)
	return signedTx
}

// TestValidateTransactionWithState_OperatorFee tests that ValidateTransactionWithState
// correctly includes operator fee in total cost calculation when IsMantleArsia is true
func TestValidateTransactionWithState_OperatorFee(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(key.PublicKey)
	arsiaTime := uint64(10)

	// Create chain config with MantleArsia enabled
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1),
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
		Optimism:            params.OptimismTestConfig.Optimism,
		MantleArsiaTime:     &arsiaTime,
	}

	// Create state getter for rollup cost function
	stateGetter := &testStateGetterForValidation{
		baseFee:             testBaseFee,
		blobBaseFee:         testBlobBaseFee,
		overhead:            big.NewInt(50),
		scalar:              big.NewInt(7 * 1e6),
		baseFeeScalar:       testBaseFeeScalar,
		blobBaseFeeScalar:   testBlobBaseFeeScalar,
		operatorFeeScalar:   testOperatorFeeScalar,
		operatorFeeConstant: testOperatorFeeConstant,
		tokenRatio:          testTokenRatio,
	}

	// Create the rollup cost function
	rollupCostFn := types.NewTotalRollupCostFunc(config, stateGetter)
	require.NotNil(t, rollupCostFn)

	// Transaction parameters
	gas := uint64(21000)
	gasFeeCap := big.NewInt(2000000000) // 2 gwei
	gasTipCap := big.NewInt(1000000000) // 1 gwei
	txValue := big.NewInt(0)

	// Create test transaction
	tx := createDynamicFeeTx(key, 0, gas, txValue, gasTipCap, gasFeeCap)

	// Calculate expected total cost:
	// 1. Base tx cost = gas * gasFeeCap + value = 21000 * 2e9 + 0 = 42e12
	baseTxCost := new(big.Int).Mul(big.NewInt(int64(gas)), gasFeeCap)
	baseTxCost.Add(baseTxCost, txValue)

	// 2. Rollup cost = L1 cost + operator fee
	blockTime := arsiaTime + 1
	rollupCost := rollupCostFn(tx, blockTime)
	require.NotNil(t, rollupCost)

	// Total cost = base tx cost + rollup cost
	expectedTotalCost := new(big.Int).Add(baseTxCost, rollupCost.ToBig())

	t.Logf("Base tx cost: %s", baseTxCost.String())
	t.Logf("Rollup cost (including operator fee): %s", rollupCost.ToBig().String())
	t.Logf("Expected total cost: %s", expectedTotalCost.String())

	// Create header
	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		Time:       blockTime,
		Difficulty: big.NewInt(0), // Post-merge
		BaseFee:    gasFeeCap,
	}

	signer := types.LatestSigner(config)

	tests := []struct {
		name          string
		balance       *big.Int
		rollupCostFn  RollupCostFunc
		wantErr       bool
		wantErrString string
	}{
		{
			name:    "sufficient balance with operator fee",
			balance: expectedTotalCost, // Exactly enough
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return rollupCostFn(tx, blockTime)
			},
			wantErr: false,
		},
		{
			name:    "insufficient balance when operator fee not accounted",
			balance: new(big.Int).Sub(expectedTotalCost, big.NewInt(1)), // 1 wei short
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return rollupCostFn(tx, blockTime)
			},
			wantErr:       true,
			wantErrString: "insufficient funds",
		},
		{
			name:    "insufficient balance without rollup cost function (nil)",
			balance: baseTxCost, // Only base cost, no rollup cost
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return rollupCostFn(tx, blockTime)
			},
			wantErr:       true,
			wantErrString: "insufficient funds",
		},
		{
			name:         "no rollup cost function - should use base cost only",
			balance:      baseTxCost,
			rollupCostFn: nil, // No rollup cost function
			wantErr:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh state for each test
			statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
			statedb.SetBalance(from, uint256.MustFromBig(tt.balance), tracing.BalanceChangeUnspecified)
			statedb.SetNonce(from, 0, tracing.NonceChangeUnspecified)

			// Set up the state with L1Block and GasOracle values
			// L1Block address
			l1BlockAddr := common.HexToAddress("0x4200000000000000000000000000000000000015")
			gasOracleAddr := common.HexToAddress("0x420000000000000000000000000000000000000F")

			// Set L1 base fee
			statedb.SetState(l1BlockAddr, types.L1BaseFeeSlot, common.BigToHash(testBaseFee))
			// Set blob base fee
			statedb.SetState(l1BlockAddr, types.L1BlobBaseFeeSlot, common.BigToHash(testBlobBaseFee))

			// Set fee scalars (Ecotone format)
			var feeScalars common.Hash
			binary.BigEndian.PutUint32(feeScalars[24:28], testBaseFeeScalar)
			binary.BigEndian.PutUint32(feeScalars[28:32], testBlobBaseFeeScalar)
			statedb.SetState(l1BlockAddr, types.L1FeeScalarsSlot, feeScalars)

			// Set operator fee params
			var operatorParams common.Hash
			binary.BigEndian.PutUint32(operatorParams[20:24], testOperatorFeeScalar)
			binary.BigEndian.PutUint64(operatorParams[24:32], testOperatorFeeConstant)
			statedb.SetState(l1BlockAddr, types.OperatorFeeParamsSlot, operatorParams)

			// Set token ratio
			statedb.SetState(gasOracleAddr, types.TokenRatioSlot, common.BigToHash(testTokenRatio))

			opts := &ValidationOptionsWithState{
				State:  statedb,
				Config: config,
				ExistingExpenditure: func(addr common.Address) *big.Int {
					return big.NewInt(0)
				},
				ExistingCost: func(addr common.Address, nonce uint64) *big.Int {
					return nil
				},
				RollupCostFn: tt.rollupCostFn,
			}

			err := ValidateTransactionWithState(tx, head, signer, opts)

			if tt.wantErr {
				require.Error(t, err)
				if tt.wantErrString != "" {
					require.Contains(t, err.Error(), tt.wantErrString)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateTransactionWithState_OperatorFeeCalculation verifies the exact operator fee calculation
func TestValidateTransactionWithState_OperatorFeeCalculation(t *testing.T) {
	// Test that operator fee is calculated correctly:
	// operatorFee = gas * operatorFeeScalar * 100 + operatorFeeConstant

	gas := uint64(21000)
	operatorFeeScalar := big.NewInt(int64(testOperatorFeeScalar))
	operatorFeeConstant := new(big.Int).SetUint64(testOperatorFeeConstant)

	// Calculate expected fee
	expectedFee := new(big.Int).SetUint64(gas)
	expectedFee.Mul(expectedFee, operatorFeeScalar)
	expectedFee.Mul(expectedFee, big.NewInt(100))
	expectedFee.Add(expectedFee, operatorFeeConstant)

	t.Logf("Gas: %d", gas)
	t.Logf("Operator fee scalar: %s", operatorFeeScalar.String())
	t.Logf("Operator fee constant: %s", operatorFeeConstant.String())
	t.Logf("Calculated operator fee: %s", expectedFee.String())
	t.Logf("Expected operator fee (pre-calculated): %s", expectedOperatorFeeForGas21000.String())

	require.Equal(t, expectedOperatorFeeForGas21000, expectedFee,
		"Operator fee calculation mismatch")
}

// TestValidateTransactionWithState_NoOperatorFeeWhenNotArsia tests that operator fee
// is not included when MantleArsia is not activated
func TestValidateTransactionWithState_NoOperatorFeeWhenNotArsia(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(key.PublicKey)

	// Create chain config WITHOUT MantleArsia (nil MantleArsiaTime)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1),
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
		Optimism:            params.OptimismTestConfig.Optimism,
		MantleArsiaTime:     nil, // Not activated
	}

	// In pre-Arsia mode, gasMultiplier = tokenRatio, so we need to use tokenRatio = 1
	// to avoid intrinsic gas being multiplied by a large factor
	preArsiaTokenRatio := big.NewInt(1) // tokenRatio = 1 for testing

	// Transaction parameters
	gas := uint64(21000)
	gasFeeCap := big.NewInt(2000000000) // 2 gwei
	gasTipCap := big.NewInt(1000000000) // 1 gwei
	txValue := big.NewInt(0)

	// Create test transaction
	tx := createDynamicFeeTx(key, 0, gas, txValue, gasTipCap, gasFeeCap)

	// Base tx cost only (no operator fee since not Arsia)
	baseTxCost := new(big.Int).Mul(big.NewInt(int64(gas)), gasFeeCap)
	baseTxCost.Add(baseTxCost, txValue)

	// Create header
	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		Time:       100,
		Difficulty: big.NewInt(0),
		BaseFee:    gasFeeCap,
	}

	signer := types.LatestSigner(config)

	// Create state with only base tx cost - should succeed without operator fee
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.SetBalance(from, uint256.MustFromBig(baseTxCost), tracing.BalanceChangeUnspecified)
	statedb.SetNonce(from, 0, tracing.NonceChangeUnspecified)

	// Set token ratio = 1 (needed for pre-Arsia gas multiplier)
	gasOracleAddr := common.HexToAddress("0x420000000000000000000000000000000000000F")
	statedb.SetState(gasOracleAddr, types.TokenRatioSlot, common.BigToHash(preArsiaTokenRatio))

	opts := &ValidationOptionsWithState{
		State:  statedb,
		Config: config,
		ExistingExpenditure: func(addr common.Address) *big.Int {
			return big.NewInt(0)
		},
		ExistingCost: func(addr common.Address, nonce uint64) *big.Int {
			return nil
		},
		RollupCostFn: nil, // No rollup cost function
	}

	err = ValidateTransactionWithState(tx, head, signer, opts)
	require.NoError(t, err, "Should succeed when not in Arsia mode with only base tx cost")
}

// TestValidateTransactionWithState_PreArsiaRollupCostFn tests the RollupCostFn behavior
// in pre-Arsia mode. With unified rollupCostFn, rollup cost is always checked against balance.
func TestValidateTransactionWithState_PreArsiaRollupCostFn(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(key.PublicKey)

	// Create chain config WITHOUT MantleArsia (pre-Arsia mode)
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1),
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
		Optimism:            params.OptimismTestConfig.Optimism,
		MantleArsiaTime:     nil, // Not activated - pre-Arsia
	}

	// Use tokenRatio = 1 to avoid gas multiplier issues
	preArsiaTokenRatio := big.NewInt(1)

	// Transaction parameters
	gas := uint64(100000)
	gasFeeCap := big.NewInt(2000000000)
	gasTipCap := big.NewInt(1000000000)
	txValue := big.NewInt(0)

	// Create test transaction
	tx := createDynamicFeeTx(key, 0, gas, txValue, gasTipCap, gasFeeCap)

	// Base tx cost
	baseTxCost := new(big.Int).Mul(big.NewInt(int64(gas)), gasFeeCap)
	baseTxCost.Add(baseTxCost, txValue)

	// Simulated rollup cost
	simulatedRollupCost := uint256.NewInt(50000000000000) // 50e12 wei

	// Total cost = base + rollup
	totalCost := new(big.Int).Add(baseTxCost, simulatedRollupCost.ToBig())

	// Create header
	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		Time:       100,
		Difficulty: big.NewInt(0),
		BaseFee:    gasFeeCap,
	}

	signer := types.LatestSigner(config)

	tests := []struct {
		name         string
		balance      *big.Int
		rollupCostFn RollupCostFunc
		wantErr      bool
		errString    string
	}{
		{
			name:    "sufficient balance covers base + rollup cost",
			balance: totalCost,
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return new(uint256.Int).Set(simulatedRollupCost)
			},
			wantErr: false,
		},
		{
			name:    "insufficient balance - only covers base cost not rollup cost",
			balance: baseTxCost,
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return new(uint256.Int).Set(simulatedRollupCost)
			},
			wantErr:   true,
			errString: "insufficient funds",
		},
		{
			name:         "nil RollupCostFn - should pass with only base cost",
			balance:      baseTxCost,
			rollupCostFn: nil,
			wantErr:      false,
		},
		{
			name:    "RollupCostFn returns nil - should pass with only base cost",
			balance: baseTxCost,
			rollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
				return nil
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create fresh state
			statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
			statedb.SetBalance(from, uint256.MustFromBig(tt.balance), tracing.BalanceChangeUnspecified)
			statedb.SetNonce(from, 0, tracing.NonceChangeUnspecified)

			// Set token ratio = 1
			gasOracleAddr := common.HexToAddress("0x420000000000000000000000000000000000000F")
			statedb.SetState(gasOracleAddr, types.TokenRatioSlot, common.BigToHash(preArsiaTokenRatio))

			opts := &ValidationOptionsWithState{
				State:  statedb,
				Config: config,
				ExistingExpenditure: func(addr common.Address) *big.Int {
					return big.NewInt(0)
				},
				ExistingCost: func(addr common.Address, nonce uint64) *big.Int {
					return nil
				},
				RollupCostFn: tt.rollupCostFn,
			}

			err := ValidateTransactionWithState(tx, head, signer, opts)

			if tt.wantErr {
				require.Error(t, err)
				if tt.errString != "" {
					require.Contains(t, err.Error(), tt.errString)
				}
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateTransactionWithState_RollupCostIncludedInBalance verifies that with
// unified rollupCostFn, the balance check always includes rollup cost (unlike old
// l1CostFn which was checked against gas).
func TestValidateTransactionWithState_RollupCostIncludedInBalance(t *testing.T) {
	key, err := crypto.GenerateKey()
	require.NoError(t, err)

	from := crypto.PubkeyToAddress(key.PublicKey)

	// Pre-Arsia config
	config := &params.ChainConfig{
		ChainID:             big.NewInt(1),
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
		Optimism:            params.OptimismTestConfig.Optimism,
		MantleArsiaTime:     nil,
	}

	preArsiaTokenRatio := big.NewInt(1)

	gas := uint64(100000)
	gasFeeCap := big.NewInt(2000000000)
	gasTipCap := big.NewInt(1000000000)
	txValue := big.NewInt(0)

	tx := createDynamicFeeTx(key, 0, gas, txValue, gasTipCap, gasFeeCap)

	// Only base cost, NOT including rollup cost
	baseTxCost := new(big.Int).Mul(big.NewInt(int64(gas)), gasFeeCap)

	// Rollup cost
	rollupCost := uint256.NewInt(50000000000000) // 50e12

	head := &types.Header{
		Number:     big.NewInt(1),
		GasLimit:   30000000,
		Time:       100,
		Difficulty: big.NewInt(0),
		BaseFee:    gasFeeCap,
	}

	signer := types.LatestSigner(config)

	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	// Set balance to ONLY cover base tx cost (not rollup cost)
	statedb.SetBalance(from, uint256.MustFromBig(baseTxCost), tracing.BalanceChangeUnspecified)
	statedb.SetNonce(from, 0, tracing.NonceChangeUnspecified)
	gasOracleAddr := common.HexToAddress("0x420000000000000000000000000000000000000F")
	statedb.SetState(gasOracleAddr, types.TokenRatioSlot, common.BigToHash(preArsiaTokenRatio))

	opts := &ValidationOptionsWithState{
		State:  statedb,
		Config: config,
		ExistingExpenditure: func(addr common.Address) *big.Int {
			return big.NewInt(0)
		},
		ExistingCost: func(addr common.Address, nonce uint64) *big.Int {
			return nil
		},
		RollupCostFn: func(tx types.RollupTransaction) *uint256.Int {
			return new(uint256.Int).Set(rollupCost)
		},
	}

	// With unified rollupCostFn, this FAILS because balance must cover
	// base cost + rollup cost. This is different from old l1CostFn behavior
	// where L1 cost was deducted from gas, not balance.
	err = ValidateTransactionWithState(tx, head, signer, opts)
	require.Error(t, err, "Balance should include rollup cost with unified rollupCostFn")
	require.Contains(t, err.Error(), "insufficient funds")

	t.Logf("Base tx cost: %s", baseTxCost.String())
	t.Logf("Rollup cost: %s (now included in balance check)", rollupCost.ToBig().String())
}
