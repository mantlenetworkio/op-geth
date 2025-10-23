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
	ArsiaFee   = big.NewInt(3203000)
	bedrockFee = uint256.NewInt(11326000000000000000)

	arisaOperatorFee = uint256.NewInt(1256417826611659930)
	// the emptyTx is out of bounds for the linear regression so it uses the minimum size

	bedrockGas      = big.NewInt(1618)
	arisaGas        = big.NewInt(480)
	minimumArsiaGas = big.NewInt(1600) // fastlz size of minimum txn, 100_000_000 * 16 / 1e6
)

func TestFjordL1CostFuncMinimumBounds(t *testing.T) {
	costFunc := NewL1CostFuncFjord(
		baseFee,
		blobBaseFee,
		baseFeeScalar,
		blobBaseFeeScalar,
		tokenRatio,
	)

	// Minimum size transactions:
	// -42.5856 + 0.8365*110 = 49.4294
	// -42.5856 + 0.8365*150 = 82.8894
	// -42.5856 + 0.8365*170 = 99.6194
	for _, fastLzsize := range []uint64{100, 150, 170} {
		c, g := costFunc(RollupCostData{
			FastLzSize: fastLzsize,
		})

		require.Equal(t, minimumArsiaGas, g)
		require.Equal(t, ArsiaFee, c)
	}

	// Larger size transactions:
	// -42.5856 + 0.8365*171 = 100.4559
	// -42.5856 + 0.8365*175 = 108.8019
	// -42.5856 + 0.8365*200 = 124.7144
	for _, fastLzsize := range []uint64{171, 175, 200} {
		c, g := costFunc(RollupCostData{
			FastLzSize: fastLzsize,
		})

		require.Greater(t, g.Uint64(), minimumArsiaGas.Uint64())
		require.Greater(t, c.Uint64(), ArsiaFee.Uint64())
	}
}

// TestFjordL1CostSolidityParity tests that the cost function for the fjord upgrade matches a Solidity
// test to ensure the outputs are the same.
func TestFjordL1CostSolidityParity(t *testing.T) {
	costFunc := NewL1CostFuncFjord(
		big.NewInt(2*1e6),
		big.NewInt(3*1e6),
		big.NewInt(20),
		big.NewInt(15),
		big.NewInt(1000000),
	)

	c0, g0 := costFunc(RollupCostData{
		FastLzSize: 235,
	})

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

	costFunc := NewL1CostFuncArsia(config, statedb)
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
	costFunc = NewL1CostFuncArsia(config, statedb)
	fee = costFunc(emptyTx.RollupCostData(), time)
	require.NotNil(t, fee)
	require.Equal(t, bedrockFee.Uint64(), fee.Uint64())

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
