package types

import (
	"encoding/binary"
	"math/big"
	"math/rand"
	"testing"

	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	baseFee  = big.NewInt(1000 * 1e6)
	overhead = big.NewInt(50)
	scalar   = big.NewInt(7 * 1e6)

	blobBaseFee         = big.NewInt(10 * 1e6)
	baseFeeScalar       = big.NewInt(2)
	blobBaseFeeScalar   = big.NewInt(3)
	operatorFeeScalar   = big.NewInt(1439103868)
	operatorFeeConstant = big.NewInt(1256417826609331460)
	tokenRatio          = big.NewInt(1000000)

	// below are the expected cost func outcomes for the above parameter settings on the emptyTx
	// which is defined in transaction_test.go
	ArsiaFee   = big.NewInt(3203000000000)
	bedrockFee = uint256.NewInt(11326000000000000000)

	arisaOperatorFee = uint256.NewInt(1256650673615173860)
	// the emptyTx is out of bounds for the linear regression so it uses the minimum size

	bedrockGas      = big.NewInt(1618)
	minimumArsiaGas = big.NewInt(1600) // fastlz size of minimum txn, 100_000_000 * 16 / 1e6
)

func TestArsiaL1CostFuncMinimumBounds(t *testing.T) {
	// Use Fjord cost function and multiply by tokenRatio (same as Arsia logic)
	fjordCostFunc := NewL1CostFuncFjord(baseFee, blobBaseFee, baseFeeScalar, blobBaseFeeScalar)

	// Minimum size transactions:
	// -42.5856 + 0.8365*110 = 49.4294
	// -42.5856 + 0.8365*150 = 82.8894
	// -42.5856 + 0.8365*170 = 99.6194
	for _, fastLzsize := range []uint64{100, 150, 170} {
		fee, g := fjordCostFunc(RollupCostData{
			FastLzSize: fastLzsize,
		})
		c := new(big.Int).Mul(fee, tokenRatio)

		require.Equal(t, minimumArsiaGas, g)
		require.Equal(t, ArsiaFee, c)
	}

	// Larger size transactions:
	// -42.5856 + 0.8365*171 = 100.4559
	// -42.5856 + 0.8365*175 = 108.8019
	// -42.5856 + 0.8365*200 = 124.7144
	for _, fastLzsize := range []uint64{171, 175, 200} {
		fee, g := fjordCostFunc(RollupCostData{
			FastLzSize: fastLzsize,
		})
		c := new(big.Int).Mul(fee, tokenRatio)

		require.Greater(t, g.Uint64(), minimumArsiaGas.Uint64())
		require.Greater(t, c.Uint64(), ArsiaFee.Uint64())
	}
}

// TestArsiaL1CostSolidityParity tests that the cost function for the Arsia upgrade matches a Solidity
// test to ensure the outputs are the same.
func TestArsiaL1CostSolidityParity(t *testing.T) {
	// Use Fjord cost function and multiply by tokenRatio (same as Arsia logic)
	fjordCostFunc := NewL1CostFuncFjord(
		big.NewInt(2*1e6),
		big.NewInt(3*1e6),
		big.NewInt(20),
		big.NewInt(15),
	)
	testTokenRatio := big.NewInt(1)

	fee, g0 := fjordCostFunc(RollupCostData{
		FastLzSize: 235,
	})
	c0 := new(big.Int).Mul(fee, testTokenRatio)

	require.Equal(t, big.NewInt(2463), g0)
	require.Equal(t, big.NewInt(105484), c0)
}

type testStateGetter struct {
	baseFee, blobBaseFee, overhead, scalar, tokenRatio *big.Int
	baseFeeScalar, blobBaseFeeScalar                   uint32
	operatorFeeScalar                                  uint32
	operatorFeeConstant                                uint64
}

func (sg *testStateGetter) GetState(addr common.Address, slot common.Hash) common.Hash {
	buf := common.Hash{}
	switch slot {
	case L1BaseFeeSlot:
		sg.baseFee.FillBytes(buf[:])
	case OverheadSlot:
		sg.overhead.FillBytes(buf[:])
	case ScalarSlot:
		sg.scalar.FillBytes(buf[:])
	case L1BlobBaseFeeSlot:
		sg.blobBaseFee.FillBytes(buf[:])
	case L1FeeScalarsSlot:
		// fetch Ecotone fee scalars
		offset := scalarSectionStart
		binary.BigEndian.PutUint32(buf[offset:offset+4], sg.baseFeeScalar)
		binary.BigEndian.PutUint32(buf[offset+4:offset+8], sg.blobBaseFeeScalar)
	case OperatorFeeParamsSlot:
		// fetch operator fee scalars
		binary.BigEndian.PutUint32(buf[20:24], sg.operatorFeeScalar)
		binary.BigEndian.PutUint64(buf[24:32], sg.operatorFeeConstant)
	case TokenRatioSlot:
		sg.tokenRatio.FillBytes(buf[:])
	default:
		panic("unknown slot")
	}
	return buf
}

func TestNewL1CostFuncArsia(t *testing.T) {
	time := uint64(10)

	config := &params.ChainConfig{
		Optimism: params.OptimismTestConfig.Optimism,
	}
	statedb := &testStateGetter{
		baseFee:           baseFee,
		overhead:          overhead,
		scalar:            scalar,
		blobBaseFee:       blobBaseFee,
		baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
		blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
		tokenRatio:        tokenRatio,
	}
	config.MantleArsiaTime = &time

	costFunc := NewL1CostFunc(config, statedb)
	require.NotNil(t, costFunc)

	// empty cost data should result in nil fee
	fee := costFunc(RollupCostData{}, time)
	require.Nil(t, fee)

	fee = costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee)
	require.Equal(t, ArsiaFee, fee)

	// emptyTx fee w/ ecotone config, but simulate first ecotone block by blowing away the ecotone
	// params. Should result in bedrock fee.
	statedb.baseFeeScalar = 0
	statedb.blobBaseFeeScalar = 0
	statedb.blobBaseFee = new(big.Int)
	costFunc = NewL1CostFunc(config, statedb)
	fee = costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee)
	require.Equal(t, bedrockFee.Uint64(), fee.Uint64())

}

// TestTokenRatioChangesDuringTx tests that when a transaction modifies tokenRatio,
// the Arsia L1 cost function uses the new value immediately (no caching).
func TestTokenRatioChangesDuringTx(t *testing.T) {
	time := uint64(10)

	config := &params.ChainConfig{
		Optimism: params.OptimismTestConfig.Optimism,
	}
	statedb := &testStateGetter{
		baseFee:           baseFee,
		overhead:          overhead,
		scalar:            scalar,
		blobBaseFee:       blobBaseFee,
		baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
		blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
		tokenRatio:        big.NewInt(1000000),
	}
	config.MantleArsiaTime = &time

	costFunc := NewL1CostFunc(config, statedb)
	require.NotNil(t, costFunc)

	fee1 := costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee1)

	// Simulate a transaction modifying tokenRatio to 2000000
	statedb.tokenRatio = big.NewInt(2000000)

	// After state change, should use NEW tokenRatio immediately (no caching in Arsia)
	fee2 := costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee2)
	require.Equal(t, fee1.Uint64()*2, fee2.Uint64(), "Should use new tokenRatio immediately")

	// Subsequent call - still uses 2000000
	fee3 := costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee3)
	require.Equal(t, fee2.Uint64(), fee3.Uint64(), "Should continue using current tokenRatio")

	// No change, should still use 2000000
	fee4 := costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee4)
	require.Equal(t, fee3.Uint64(), fee4.Uint64(), "Should continue using current tokenRatio")
}

// TestScenarioA_FeeParamsChange tests Scenario A: pure fee params change (no ratio change).
// Block: Tx1(L1BlockInfo deposit updates fee params) → Tx2(regular tx)
// Expected: Tx2 charge uses new fee params.
func TestScenarioA_FeeParamsChange(t *testing.T) {
	blockTime := uint64(10)
	arsiaTime := uint64(10)
	oldBaseFee := big.NewInt(1000 * 1e6)
	newBaseFee := big.NewInt(2000 * 1e6)

	t.Run("Arsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism:        params.OptimismTestConfig.Optimism,
			MantleArsiaTime: &arsiaTime,
		}
		statedb := &testStateGetter{
			baseFee:           oldBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        tokenRatio,
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (deposit): empty cost data → nil return, selectFunc not triggered
		fee := costFunc(RollupCostData{}, blockTime)
		require.Nil(t, fee)

		// Simulate Tx1 execution: L1BlockInfo updates fee params
		statedb.baseFee = newBaseFee

		// Tx2 (regular): selectFunc triggered NOW, reads new fee params
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx2)

		// Verify Tx2 uses new fee params by comparing with a reference
		refStatedb := &testStateGetter{
			baseFee:           newBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        tokenRatio,
		}
		refFunc := NewL1CostFunc(config, refStatedb)
		feeRef := refFunc(emptyTx.RollupCostData(), blockTime)

		require.Equal(t, feeRef, feeTx2, "Tx2 charge should use new fee params")
	})

	t.Run("BeforeArsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism: params.OptimismTestConfig.Optimism,
		}
		statedb := &testStateGetter{
			baseFee:           oldBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        tokenRatio,
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (deposit): empty cost data → nil
		fee := costFunc(RollupCostData{}, blockTime)
		require.Nil(t, fee)

		// Simulate Tx1 execution: update fee params
		statedb.baseFee = newBaseFee

		// Tx2 (regular): closure created with new fee params (cached at creation)
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx2)

		// Verify uses new fee params
		refStatedb := &testStateGetter{
			baseFee:           newBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        tokenRatio,
		}
		refFunc := NewL1CostFunc(config, refStatedb)
		feeRef := refFunc(emptyTx.RollupCostData(), blockTime)

		require.Equal(t, feeRef, feeTx2, "Tx2 charge should use new fee params")
	})
}

// TestScenarioB_TokenRatioChange tests Scenario B: pure token ratio change (no fee params change).
// Block: Tx1(regular) → Tx2(SetTokenRatio) → Tx3(regular)
// Expected: Tx2 charge uses old ratio, Tx3 charge uses new ratio.
func TestScenarioB_TokenRatioChange(t *testing.T) {
	blockTime := uint64(10)
	arsiaTime := uint64(10)

	t.Run("Arsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism:        params.OptimismTestConfig.Optimism,
			MantleArsiaTime: &arsiaTime,
		}
		statedb := &testStateGetter{
			baseFee:           baseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        big.NewInt(1000000),
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (regular): uses initial tokenRatio
		feeTx1 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx1)

		// Tx2 preCheck (before SetTokenRatio execution): ratio not yet changed
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx1, feeTx2, "Tx2 charge should use old ratio (state not yet changed)")

		// Simulate Tx2 execution: SetTokenRatio changes tokenRatio
		statedb.tokenRatio = big.NewInt(2000000)

		// Tx3 (regular): should use NEW tokenRatio
		feeTx3 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx1.Uint64()*2, feeTx3.Uint64(), "Tx3 charge should use new ratio (2x)")
	})

	t.Run("BeforeArsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism: params.OptimismTestConfig.Optimism,
		}
		statedb := &testStateGetter{
			baseFee:           baseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        big.NewInt(1000000),
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (regular): uses initial tokenRatio
		feeTx1 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx1)

		// Tx2 preCheck (before SetTokenRatio execution): ratio not yet changed
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx1, feeTx2, "Tx2 charge should use old ratio")

		// Simulate Tx2 execution: SetTokenRatio changes tokenRatio
		statedb.tokenRatio = big.NewInt(2000000)

		// Tx3 (regular): should use NEW tokenRatio
		feeTx3 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx1.Uint64()*2, feeTx3.Uint64(), "Tx3 charge should use new ratio (2x)")
	})
}

// TestScenarioC_FeeParamsAndTokenRatioChange tests Scenario C: fee params + token ratio change.
// Block: Tx1(L1BlockInfo deposit) → Tx2(regular) → Tx3(SetTokenRatio) → Tx4(regular)
// Expected: Tx2 charge = new fee + old ratio, Tx3 charge = new fee + old ratio,
//
//	Tx4 charge = new fee + new ratio.
func TestScenarioC_FeeParamsAndTokenRatioChange(t *testing.T) {
	blockTime := uint64(10)
	arsiaTime := uint64(10)
	oldBaseFee := big.NewInt(1000 * 1e6)
	newBaseFee := big.NewInt(2000 * 1e6)

	t.Run("Arsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism:        params.OptimismTestConfig.Optimism,
			MantleArsiaTime: &arsiaTime,
		}
		statedb := &testStateGetter{
			baseFee:           oldBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        big.NewInt(1000000),
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (deposit): empty cost data → nil, selectFunc not triggered
		fee := costFunc(RollupCostData{}, blockTime)
		require.Nil(t, fee)

		// Simulate Tx1 execution: L1BlockInfo updates fee params
		statedb.baseFee = newBaseFee

		// Tx2 (regular): uses new fee params + old ratio
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx2)

		// Tx3 preCheck (before SetTokenRatio): uses new fee params + old ratio
		feeTx3 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx2, feeTx3, "Tx3 charge should equal Tx2 (same fee params + same ratio)")

		// Simulate Tx3 execution: SetTokenRatio changes tokenRatio
		statedb.tokenRatio = big.NewInt(2000000)

		// Tx4 (regular): uses new fee params + new ratio
		feeTx4 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx2.Uint64()*2, feeTx4.Uint64(),
			"Tx4 charge should use new fee params + new ratio (2x Tx2)")
	})

	t.Run("BeforeArsia", func(t *testing.T) {
		config := &params.ChainConfig{
			Optimism: params.OptimismTestConfig.Optimism,
		}
		statedb := &testStateGetter{
			baseFee:           oldBaseFee,
			overhead:          overhead,
			scalar:            scalar,
			blobBaseFee:       blobBaseFee,
			baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
			blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
			tokenRatio:        big.NewInt(1000000),
		}

		costFunc := NewL1CostFunc(config, statedb)

		// Tx1 (deposit): empty cost data → nil
		fee := costFunc(RollupCostData{}, blockTime)
		require.Nil(t, fee)

		// Simulate Tx1 execution: update fee params
		statedb.baseFee = newBaseFee

		// Tx2 (regular): closure created with new fee params + old ratio
		feeTx2 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.NotNil(t, feeTx2)

		// Tx3 preCheck (before SetTokenRatio): same fee params + old ratio
		feeTx3 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx2, feeTx3, "Tx3 charge should equal Tx2")

		// Simulate Tx3 execution: SetTokenRatio
		statedb.tokenRatio = big.NewInt(2000000)

		// Tx4 (regular): detects ratio change, refreshes fee params, uses new ratio
		feeTx4 := costFunc(emptyTx.RollupCostData(), blockTime)
		require.Equal(t, feeTx2.Uint64()*2, feeTx4.Uint64(),
			"Tx4 charge should use new fee params + new ratio (2x Tx2)")
	})
}

// TestBeforeArsia_FeeParamsCachedUntilRatioChange tests that in BeforeArsia mode,
// fee params are cached at closure creation and only refreshed when tokenRatio changes.
func TestBeforeArsia_FeeParamsCachedUntilRatioChange(t *testing.T) {
	blockTime := uint64(10)
	config := &params.ChainConfig{
		Optimism: params.OptimismTestConfig.Optimism,
	}
	statedb := &testStateGetter{
		baseFee:           big.NewInt(1000 * 1e6),
		overhead:          overhead,
		scalar:            scalar,
		blobBaseFee:       blobBaseFee,
		baseFeeScalar:     uint32(baseFeeScalar.Uint64()),
		blobBaseFeeScalar: uint32(blobBaseFeeScalar.Uint64()),
		tokenRatio:        big.NewInt(1000000),
	}

	costFunc := NewL1CostFuncBeforeArsia(config, statedb, blockTime)

	fee1, _ := costFunc(emptyTx.RollupCostData())
	require.NotNil(t, fee1)

	// Change fee params but NOT tokenRatio
	statedb.baseFee = big.NewInt(2000 * 1e6)

	// Fee params are cached at creation → fee unchanged
	fee2, _ := costFunc(emptyTx.RollupCostData())
	require.Equal(t, fee1, fee2, "should use cached fee params when tokenRatio unchanged")

	// Now change tokenRatio → triggers refresh of fee params
	statedb.tokenRatio = big.NewInt(2000000)

	fee3, _ := costFunc(emptyTx.RollupCostData())
	// fee3 uses refreshed baseFee (2x) and new tokenRatio (2x) → 4x fee1
	require.Equal(t, fee1.Uint64()*4, fee3.Uint64(),
		"should use refreshed fee params (2x) and new tokenRatio (2x) = 4x original")
}

// TestNewL1CostFunc tests that the appropriate cost function is selected based on the
// configuration and statedb values.
func TestNewOperatorCostFunc(t *testing.T) {
	time := uint64(10)
	config := &params.ChainConfig{
		Optimism: params.OptimismTestConfig.Optimism,
	}
	statedb := &testStateGetter{
		baseFee:             baseFee,
		overhead:            overhead,
		scalar:              scalar,
		blobBaseFee:         blobBaseFee,
		baseFeeScalar:       uint32(baseFeeScalar.Uint64()),
		blobBaseFeeScalar:   uint32(blobBaseFeeScalar.Uint64()),
		operatorFeeScalar:   uint32(operatorFeeScalar.Uint64()),
		operatorFeeConstant: operatorFeeConstant.Uint64(),
	}

	config.MantleArsiaTime = &time
	costFunc := NewOperatorCostFunc(config, statedb)
	fee := costFunc(bedrockGas.Uint64(), time)
	require.NotNil(t, fee)
	require.Equal(t, arisaOperatorFee, fee)
}

// copy of emptyTx with non-zero gas
var emptyTxWithGas = NewTransaction(
	0,
	common.HexToAddress("095e7baea6a6c7c4c2dfeb977efac326af552d87"),
	big.NewInt(0), bedrockGas.Uint64(), big.NewInt(0),
	nil,
)

// TestTotalRollupCostFunc tests that the total rollup cost function correctly
// combines the L1 cost and operator cost.
func TestTotalRollupCostFunc(t *testing.T) {
	later := uint64(10)
	config := &params.ChainConfig{
		Optimism:        params.OptimismTestConfig.Optimism,
		MantleArsiaTime: &later,
	}
	statedb := &testStateGetter{
		baseFee:             baseFee,
		overhead:            overhead,
		scalar:              scalar,
		blobBaseFee:         blobBaseFee,
		baseFeeScalar:       uint32(baseFeeScalar.Uint64()),
		blobBaseFeeScalar:   uint32(blobBaseFeeScalar.Uint64()),
		operatorFeeScalar:   uint32(operatorFeeScalar.Uint64()),
		operatorFeeConstant: operatorFeeConstant.Uint64(),
		tokenRatio:          tokenRatio,
	}

	costFunc := NewTotalRollupCostFunc(config, statedb)
	expCost := uint256.MustFromBig(ArsiaFee)
	cost := costFunc(emptyTxWithGas, later+1)
	require.NotNil(t, cost)
	expCost.Add(expCost, arisaOperatorFee)
	require.Equal(t, expCost, cost, "Isthmus total rollup cost should contain L1 cost and operator cost")
}

func TestRollupCostData(t *testing.T) {
	for i := 0; i < 100; i++ {
		zeroes := rand.Uint64()
		ones := rand.Uint64()

		r := RollupCostData{
			Zeroes: zeroes,
			Ones:   ones,
		}
		time := uint64(1)
		cfg := &params.ChainConfig{
			RegolithTime: &time,
		}
		gasPreRegolith := r.DataGas(0, cfg)
		gasPostRegolith := r.DataGas(1, cfg)

		require.Equal(t, r.Zeroes*params.TxDataZeroGas+(r.Ones+68)*params.TxDataNonZeroGasEIP2028, gasPreRegolith)
		require.Equal(t, r.Zeroes*params.TxDataZeroGas+r.Ones*params.TxDataNonZeroGasEIP2028, gasPostRegolith)
	}
}
