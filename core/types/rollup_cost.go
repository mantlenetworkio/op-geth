// Copyright 2022 The go-ethereum Authors
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

package types

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/holiman/uint256"
)

var (
	MantleArsiaL1AttributesSelector = []byte{0x49, 0xe7, 0x23, 0x83}

	L1BaseFeeSlot = common.BigToHash(big.NewInt(1))
	OverheadSlot  = common.BigToHash(big.NewInt(5))
	ScalarSlot    = common.BigToHash(big.NewInt(6))
	// L1BlobBaseFeeSlot was added with the Ecotone upgrade and stores the blobBaseFee L1 gas
	// attribute.
	L1BlobBaseFeeSlot = common.BigToHash(big.NewInt(7))
	// L1FeeScalarsSlot as of the Ecotone upgrade stores the 32-bit basefeeScalar and
	// blobBaseFeeScalar L1 gas attributes at offsets `BaseFeeScalarSlotOffset` and
	// `BlobBaseFeeScalarSlotOffset` respectively.
	L1FeeScalarsSlot = common.BigToHash(big.NewInt(3))

	TokenRatioSlot        = common.BigToHash(big.NewInt(0))
	OperatorFeeParamsSlot = common.BigToHash(big.NewInt(8))
	sixteen               = big.NewInt(16)

	L1BlockAddr   = common.HexToAddress("0x4200000000000000000000000000000000000015")
	GasOracleAddr = common.HexToAddress("0x420000000000000000000000000000000000000F")
	Decimals      = big.NewInt(1_000_000)
	fjordDivisor  = big.NewInt(1_000_000_000_000)

	EcotoneL1AttributesSelector = []byte{0x44, 0x0a, 0x5e, 0x20}

	L1CostIntercept  = big.NewInt(-42_585_600)
	L1CostFastlzCoef = big.NewInt(836_500)

	MinTransactionSize       = big.NewInt(100)
	MinTransactionSizeScaled = new(big.Int).Mul(MinTransactionSize, big.NewInt(1e6))

	emptyScalars = make([]byte, 8)
	oneHundred   = big.NewInt(100)
)

type RollupCostData struct {
	Zeroes, Ones uint64
	FastLzSize   uint64
}

const (
	// The two 4-byte Ecotone fee scalar values are packed into the same storage slot as the 8-byte
	// sequence number and have the following Solidity offsets within the slot. Note that Solidity
	// offsets correspond to the last byte of the value in the slot, counting backwards from the
	// end of the slot. For example, The 8-byte sequence number has offset 0, and is therefore
	// stored as big-endian format in bytes [24:32) of the slot.
	BaseFeeScalarSlotOffset     = 12 // bytes [16:20) of the slot
	BlobBaseFeeScalarSlotOffset = 8  // bytes [20:24) of the slot

	// scalarSectionStart is the beginning of the scalar values segment in the slot
	// array. baseFeeScalar is in the first four bytes of the segment, blobBaseFeeScalar the next
	// four.
	scalarSectionStart = 32 - BaseFeeScalarSlotOffset - 4

	BedrockL1AttributesLen = 260
	// IsthmusL1AttributesLen = 176
	JovianL1AttributesLen = 178
)

func init() {
	if BlobBaseFeeScalarSlotOffset != BaseFeeScalarSlotOffset-4 {
		panic("this code assumes the scalars are at adjacent positions in the scalars slot")
	}
}

// OperatorCostFunc is used in the state transition to determine the operator fee charged to the
// sender of non-Deposit transactions. It returns 0 if no operator fee is charged.
type OperatorCostFunc func(gasUsed uint64, blockTime uint64) *uint256.Int

// A RollupTransaction provides all the input data needed to compute the total rollup cost.
type RollupTransaction interface {
	RollupCostData() RollupCostData
	Gas() uint64
}

// TotalRollupCostFunc is used in the transaction pool to determine the total rollup cost,
// including both the data availability fee and the operator fee. It returns nil if both costs are nil.
type TotalRollupCostFunc func(tx RollupTransaction, blockTime uint64) *uint256.Int

type operatorCostFunc func(gasUsed uint64) *uint256.Int

func NewRollupCostData(data []byte) (out RollupCostData) {
	for _, b := range data {
		if b == 0 {
			out.Zeroes++
		} else {
			out.Ones++
		}
	}
	out.FastLzSize = uint64(FlzCompressLen(data))
	return out
}

func (r RollupCostData) DataGas(time uint64, cfg *params.ChainConfig) (gas uint64) {
	gas = r.Zeroes * params.TxDataZeroGas
	if cfg.IsRegolith(time) {
		gas += r.Ones * params.TxDataNonZeroGasEIP2028
	} else {
		gas += (r.Ones + 68) * params.TxDataNonZeroGasEIP2028
	}
	return gas
}

// EstimatedDASize estimates the number of bytes the transaction will occupy in its DA batch using the Fjord linear
// regression model.
func (cd RollupCostData) EstimatedDASize() *big.Int {
	b := cd.estimatedDASizeScaled()
	return b.Div(b, big.NewInt(1e6))
}

type StateGetter interface {
	GetState(common.Address, common.Hash) common.Hash
}

// L1CostFunc is used in the state transition to determine the cost of a rollup message.
// Returns nil if there is no cost.
type L1CostFunc func(rcd RollupCostData, blockTime uint64) *big.Int

type L1CostFuncArsiaGasUsed func(rcd RollupCostData, blockTime uint64) *big.Int

// l1CostFunc is an internal version of L1CostFunc that also returns the gasUsed for use in
// receipts.
type l1CostFunc func(rcd RollupCostData) (fee, gasUsed *big.Int)

type l1CostFuncArsiaGasUsed func(rcd RollupCostData) (calldataGasUsed *big.Int)

// NewL1CostFunc returns a function used for calculating L1 fee cost.
// This depends on the oracles because gas costs can change over time.
// It returns nil if there is no applicable cost function.
func NewL1CostFunc(config *params.ChainConfig, statedb StateGetter) L1CostFunc {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc l1CostFunc
	selectFunc := func(blockTime uint64) l1CostFunc {
		if !config.IsMantleArsia(blockTime) {
			return NewL1CostFuncBeforeArsia(config, statedb, blockTime)
		}
		// Note: the various state variables below are not initialized from the DB until this
		// point to allow deposit transactions from the block to be processed first by state
		// transition.  This behavior is consensus critical!
		l1FeeScalars := statedb.GetState(L1BlockAddr, L1FeeScalarsSlot).Bytes()
		l1BlobBaseFee := statedb.GetState(L1BlockAddr, L1BlobBaseFeeSlot).Big()
		l1BaseFee := statedb.GetState(L1BlockAddr, L1BaseFeeSlot).Big()

		// Edge case: the very first Ecotone block requires we use the Bedrock cost
		// function. We detect this scenario by checking if the Ecotone parameters are
		// unset. Note here we rely on assumption that the scalar parameters are adjacent
		// in the buffer and l1BaseFeeScalar comes first. We need to check this prior to
		// other forks, as the first block of Fjord and Ecotone could be the same block.
		firstEcotoneBlock := l1BlobBaseFee.BitLen() == 0 &&
			bytes.Equal(emptyScalars, l1FeeScalars[scalarSectionStart:scalarSectionStart+8])
		if firstEcotoneBlock {
			log.Info("using bedrock l1 cost func for first Arisa block", "time", blockTime)
			return NewL1CostFuncBeforeArsia(config, statedb, blockTime)
		}

		l1BaseFeeScalar, l1BlobBaseFeeScalar := ExtractEcotoneFeeParams(l1FeeScalars)

		return NewL1CostFuncArsia(
			l1BaseFee,
			l1BlobBaseFee,
			l1BaseFeeScalar,
			l1BlobBaseFeeScalar,
			statedb,
			blockTime,
		)
	}

	return func(rollupCostData RollupCostData, blockTime uint64) *big.Int {
		if rollupCostData == (RollupCostData{}) {
			return nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit).
		}
		if forBlock != blockTime {
			if forBlock != ^uint64(0) {
				// best practice is not to re-use l1 cost funcs across different blocks, but we
				// make it work just in case.
				log.Info("l1 cost func re-used for different L1 block", "oldTime", forBlock, "newTime", blockTime)
			}
			forBlock = blockTime
			cachedFunc = selectFunc(blockTime)
		}
		fee, _ := cachedFunc(rollupCostData)
		return fee
	}
}

func NewL1CostFuncBeforeArsia(config *params.ChainConfig, statedb StateGetter, blockTime uint64) l1CostFunc {

	return func(rollupCostData RollupCostData) (fee, gasUsed *big.Int) {
		if rollupCostData == (RollupCostData{}) {
			return nil, nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit)
		}
		rollupDataGas := rollupCostData.DataGas(blockTime, config) // Only fake txs for RPC view-calls are 0.
		if config.Optimism == nil || rollupDataGas == 0 {
			return common.Big0, common.Big0
		}

		l1BaseFee := statedb.GetState(L1BlockAddr, L1BaseFeeSlot).Big()
		overhead := statedb.GetState(L1BlockAddr, OverheadSlot).Big()
		scalar := statedb.GetState(L1BlockAddr, ScalarSlot).Big()
		tokenRatio := statedb.GetState(GasOracleAddr, TokenRatioSlot).Big()

		gasWithOverhead := new(big.Int).SetUint64(rollupDataGas)
		gasWithOverhead.Add(gasWithOverhead, overhead)

		l1Cost := new(big.Int).Mul(gasWithOverhead, l1BaseFee)
		l1Cost.Mul(l1Cost, scalar)
		l1Cost.Mul(l1Cost, tokenRatio)
		l1CostFee := new(big.Int).Div(l1Cost, Decimals)

		return l1CostFee, gasWithOverhead
	}
}

func NewL1CostFuncArsiaGasUsed(config *params.ChainConfig, statedb StateGetter) L1CostFuncArsiaGasUsed {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc l1CostFuncArsiaGasUsed
	selectFunc := func(blockTime uint64) l1CostFuncArsiaGasUsed {

		// Note: the various state variables below are not initialized from the DB until this
		// point to allow deposit transactions from the block to be processed first by state
		// transition.  This behavior is consensus critical!
		l1FeeScalars := statedb.GetState(L1BlockAddr, L1FeeScalarsSlot).Bytes()
		l1BlobBaseFee := statedb.GetState(L1BlockAddr, L1BlobBaseFeeSlot).Big()

		// Edge case: the very first Ecotone block requires we use the Bedrock cost
		// function. We detect this scenario by checking if the Ecotone parameters are
		// unset. Note here we rely on assumption that the scalar parameters are adjacent
		// in the buffer and l1BaseFeeScalar comes first. We need to check this prior to
		// other forks, as the first block of Fjord and Ecotone could be the same block.
		firstEcotoneBlock := l1BlobBaseFee.BitLen() == 0 &&
			bytes.Equal(emptyScalars, l1FeeScalars[scalarSectionStart:scalarSectionStart+8])
		if firstEcotoneBlock {
			log.Info("using bedrock l1 cost func for first Arisa block", "time", blockTime)
			return func(costData RollupCostData) (calldataGasUsed *big.Int) {
				rollupDataGas := costData.DataGas(blockTime, config) // Only fake txs for RPC view-calls are 0.
				if config.Optimism == nil || rollupDataGas == 0 {
					return common.Big0
				}
				overhead := statedb.GetState(L1BlockAddr, OverheadSlot).Big()

				gasWithOverhead := new(big.Int).SetUint64(rollupDataGas)
				gasWithOverhead.Add(gasWithOverhead, overhead)
				return gasWithOverhead
			}
		}

		return (func(costData RollupCostData) (calldataGasUsed *big.Int) {
			estimatedSize := costData.estimatedDASizeScaled()
			calldataGasUsed = new(big.Int).Mul(estimatedSize, new(big.Int).SetUint64(params.TxDataNonZeroGasEIP2028))
			calldataGasUsed.Div(calldataGasUsed, big.NewInt(1e6))
			return calldataGasUsed
		})
	}

	return func(rollupCostData RollupCostData, blockTime uint64) *big.Int {
		if rollupCostData == (RollupCostData{}) {
			return nil // Do not charge if there is no rollup cost-data (e.g. RPC call or deposit).
		}
		if forBlock != blockTime {
			if forBlock != ^uint64(0) {
				// best practice is not to re-use l1 cost funcs across different blocks, but we
				// make it work just in case.
				log.Info("l1 cost func re-used for different L1 block", "oldTime", forBlock, "newTime", blockTime)
			}
			forBlock = blockTime
			cachedFunc = selectFunc(blockTime)
		}
		gasUsed := cachedFunc(rollupCostData)
		return gasUsed
	}
}

func NewL1CostFuncArsia(l1BaseFee, l1BlobBaseFee, baseFeeScalar, blobFeeScalar *big.Int, statedb StateGetter, blockTime uint64) l1CostFunc {
	fjordCostFunc := NewL1CostFuncFjord(l1BaseFee, l1BlobBaseFee, baseFeeScalar, blobFeeScalar)

	return func(costData RollupCostData) (fee, gasUsed *big.Int) {
		currentTokenRatio := statedb.GetState(GasOracleAddr, TokenRatioSlot).Big()

		fee, gasUsed = fjordCostFunc(costData)
		fee = new(big.Int).Mul(fee, currentTokenRatio)
		return fee, gasUsed
	}
}

func NewL1CostFuncFjord(l1BaseFee, l1BlobBaseFee, baseFeeScalar, blobFeeScalar *big.Int) l1CostFunc {
	return func(costData RollupCostData) (fee, calldataGasUsed *big.Int) {
		// Fjord L1 cost function:
		// l1FeeScaled = baseFeeScalar*l1BaseFee*16 + blobFeeScalar*l1BlobBaseFee
		// estimatedSize = max(minTransactionSize, intercept + fastlzCoef*fastlzSize)
		// l1Cost = estimatedSize * l1FeeScaled / 1e12

		scaledL1BaseFee := new(big.Int).Mul(baseFeeScalar, l1BaseFee)
		calldataCostPerByte := new(big.Int).Mul(scaledL1BaseFee, sixteen)
		blobCostPerByte := new(big.Int).Mul(blobFeeScalar, l1BlobBaseFee)
		l1FeeScaled := new(big.Int).Add(calldataCostPerByte, blobCostPerByte)
		estimatedSize := costData.estimatedDASizeScaled()
		l1CostScaled := new(big.Int).Mul(estimatedSize, l1FeeScaled)
		l1Cost := new(big.Int).Div(l1CostScaled, fjordDivisor)

		calldataGasUsed = new(big.Int).Mul(estimatedSize, new(big.Int).SetUint64(params.TxDataNonZeroGasEIP2028))
		calldataGasUsed.Div(calldataGasUsed, big.NewInt(1e6))

		return l1Cost, calldataGasUsed
	}
}

func (cd RollupCostData) estimatedDASizeScaled() *big.Int {
	fastLzSize := new(big.Int).SetUint64(cd.FastLzSize)
	estimatedSize := new(big.Int).Add(L1CostIntercept, new(big.Int).Mul(L1CostFastlzCoef, fastLzSize))

	if estimatedSize.Cmp(MinTransactionSizeScaled) < 0 {
		estimatedSize.Set(MinTransactionSizeScaled)
	}
	return estimatedSize
}

func L1Cost(rollupDataGas uint64, l1BaseFee, overhead, scalar, tokenRatio *big.Int) *big.Int {
	l1GasUsed := new(big.Int).SetUint64(rollupDataGas)
	l1GasUsed = l1GasUsed.Add(l1GasUsed, overhead)
	l1Cost := l1GasUsed.Mul(l1GasUsed, l1BaseFee)
	l1Cost = l1Cost.Mul(l1Cost, scalar)
	l1Cost = l1Cost.Mul(l1Cost, tokenRatio)
	return l1Cost.Div(l1Cost, Decimals)
}

func NewTotalRollupCostFunc(config *params.ChainConfig, statedb StateGetter) TotalRollupCostFunc {
	if !config.IsOptimism() {
		return nil
	}
	l1CostFunc := NewL1CostFunc(config, statedb)
	operatorCostFunc := NewOperatorCostFunc(config, statedb)

	return func(tx RollupTransaction, blockTime uint64) *uint256.Int {
		// proper caching is happening inside the individual cost functions
		l1Cost := l1CostFunc(tx.RollupCostData(), blockTime)
		operatorCost := operatorCostFunc(tx.Gas(), blockTime)
		if l1Cost == nil && operatorCost == nil {
			return nil
		}

		var totalCost *uint256.Int
		var overflow bool
		if l1Cost != nil {
			totalCost, overflow = uint256.FromBig(l1Cost)
			if overflow {
				panic("overflow in total rollup cost: l1Cost")
			}
		} else {
			totalCost = new(uint256.Int)
		}

		// Note, the operator cost currently always returns a non-nil value, so we wouldn't
		// need the nil-check here. But we keep it for future-proofing.
		if operatorCost != nil {
			_, overflow = totalCost.AddOverflow(totalCost, operatorCost)
			if overflow {
				panic("overflow in total rollup cost: operatorCost")
			}
		}
		return totalCost
	}
}

// NewOperatorCostFunc returns a function used for calculating operator fees, or nil if this is
// not an op-stack chain.
func NewOperatorCostFunc(config *params.ChainConfig, statedb StateGetter) OperatorCostFunc {
	if config.Optimism == nil {
		return nil
	}
	forBlock := ^uint64(0)
	var cachedFunc operatorCostFunc

	selectFunc := func(blockTime uint64) operatorCostFunc {
		if !config.IsMantleArsia(blockTime) {
			return func(gas uint64) *uint256.Int {
				return uint256.NewInt(0)
			}
		}
		operatorFeeParams := statedb.GetState(L1BlockAddr, OperatorFeeParamsSlot)
		if operatorFeeParams == (common.Hash{}) {
			return func(gas uint64) *uint256.Int {
				return uint256.NewInt(0)
			}
		}
		operatorFeeScalar, operatorFeeConstant := ExtractOperatorFeeParams(operatorFeeParams)

		return newOperatorCostFunc(operatorFeeScalar, operatorFeeConstant)
	}

	return func(gas uint64, blockTime uint64) *uint256.Int {
		if forBlock != blockTime {
			forBlock = blockTime
			cachedFunc = selectFunc(blockTime)
		}

		return cachedFunc(gas)
	}
}

func newOperatorCostFunc(operatorFeeScalar *big.Int, operatorFeeConstant *big.Int) operatorCostFunc {
	return func(gas uint64) *uint256.Int {
		fee := new(big.Int).SetUint64(gas)
		fee = fee.Mul(fee, operatorFeeScalar)
		fee = fee.Mul(fee, oneHundred)
		fee = fee.Add(fee, operatorFeeConstant)
		feeU256, overflow := uint256.FromBig(fee)
		if overflow {
			// This should never happen, as (u64.max * u32.max / 1e6) + u64.max is an int of bit length 77
			panic("overflow in operator cost calculation")
		}

		return feeU256
	}
}

// DeriveL1GasInfoMantle reads L1 gas related information to be included before Arsia
// on the receipt
func DeriveL1GasInfoMantle(state StateGetter) (*big.Int, *big.Int, *big.Int, *big.Float) {
	l1BaseFee, overhead, scalar, scaled := readL1BlockStorageSlots(L1BlockAddr, state)
	return l1BaseFee, overhead, scalar, scaled
}

func DeriveL1GasInfo(state StateGetter) (*big.Int, *big.Int, *big.Int, *big.Int, *big.Int) {
	l1FeeScalars := state.GetState(L1BlockAddr, L1FeeScalarsSlot).Bytes()
	l1BlobBaseFee := state.GetState(L1BlockAddr, L1BlobBaseFeeSlot).Big()
	l1BaseFeeScalar, l1BlobBaseFeeScalar := ExtractEcotoneFeeParams(l1FeeScalars)
	operatorFeeParams := state.GetState(L1BlockAddr, OperatorFeeParamsSlot)
	if operatorFeeParams == (common.Hash{}) {
		return l1BaseFeeScalar, l1BlobBaseFeeScalar, l1BlobBaseFee, nil, nil
	}
	operatorFeeScalar, operatorFeeConstant := ExtractOperatorFeeParams(operatorFeeParams)

	return l1BaseFeeScalar, l1BlobBaseFeeScalar, l1BlobBaseFee, operatorFeeScalar, operatorFeeConstant
}

// CalcDAFootprint calculates the total DA footprint of a block for an OP Stack chain.
// Jovian introduces a DA footprint block limit which is stored in the BlobGasUsed header field and that is taken
// into account during base fee updates.
// CalcDAFootprint must not be called for pre-Jovian blocks.
func CalcDAFootprint(txs []*Transaction) (uint64, error) {
	if len(txs) == 0 || !txs[0].IsDepositTx() {
		return 0, errors.New("missing deposit transaction")
	}

	// First Jovian block doesn't set the DA footprint gas scalar yet and
	// it must not have user transactions.
	data := txs[0].Data()
	log.Info("tx0 data length", "length", len(data))
	if len(data) == BedrockL1AttributesLen {
		if !txs[len(txs)-1].IsDepositTx() {
			// sufficient to check last transaction because deposits precede non-deposit txs
			return 0, errors.New("unexpected non-deposit transactions in Jovian activation block")
		}
		return 0, nil
	} // ExtractDAFootprintGasScalar catches all invalid lengths

	daFootprintGasScalar, err := ExtractDAFootprintGasScalar(data)
	if err != nil {
		return 0, err
	}
	var daFootprint uint64
	for _, tx := range txs {
		if tx.IsDepositTx() {
			continue
		}
		daFootprint += tx.RollupCostData().EstimatedDASize().Uint64() * uint64(daFootprintGasScalar)
	}
	return daFootprint, nil
}

// ExtractDAFootprintGasScalar extracts the DA footprint gas scalar from the L1 attributes transaction data
// of a Jovian-enabled block.
func ExtractDAFootprintGasScalar(data []byte) (uint16, error) {
	if len(data) < JovianL1AttributesLen {
		return 0, fmt.Errorf("L1 attributes transaction data too short for DA footprint gas scalar: %d", len(data))
	}
	// Future forks need to be added here
	if !bytes.Equal(data[0:4], MantleArsiaL1AttributesSelector) {
		return 0, fmt.Errorf("L1 attributes transaction data does not have Arsia selector")
	}
	daFootprintGasScalar := binary.BigEndian.Uint16(data[JovianL1AttributesLen-2 : JovianL1AttributesLen])
	return daFootprintGasScalar, nil
}

func ExtractEcotoneFeeParams(l1FeeParams []byte) (l1BaseFeeScalar, l1BlobBaseFeeScalar *big.Int) {
	offset := scalarSectionStart
	l1BaseFeeScalar = new(big.Int).SetBytes(l1FeeParams[offset : offset+4])
	l1BlobBaseFeeScalar = new(big.Int).SetBytes(l1FeeParams[offset+4 : offset+8])
	return
}

func readL1BlockStorageSlots(addr common.Address, state StateGetter) (*big.Int, *big.Int, *big.Int, *big.Float) {
	l1BaseFee := state.GetState(addr, L1BaseFeeSlot)
	overhead := state.GetState(addr, OverheadSlot)
	scalar := state.GetState(addr, ScalarSlot)
	scaled := scaleDecimals(scalar.Big(), Decimals)

	return l1BaseFee.Big(), overhead.Big(), scalar.Big(), scaled
}

func ExtractOperatorFeeParams(operatorFeeParams common.Hash) (operatorFeeScalar, operatorFeeConstant *big.Int) {
	operatorFeeScalar = new(big.Int).SetBytes(operatorFeeParams[20:24])
	operatorFeeConstant = new(big.Int).SetBytes(operatorFeeParams[24:32])
	return
}

// scaleDecimals will scale a value by decimals
func scaleDecimals(scalar, divisor *big.Int) *big.Float {
	fscalar := new(big.Float).SetInt(scalar)
	fdivisor := new(big.Float).SetInt(divisor)
	// fscalar / fdivisor
	return new(big.Float).Quo(fscalar, fdivisor)
}

// FlzCompressLen returns the length of the data after compression through FastLZ, based on
// https://github.com/Vectorized/solady/blob/5315d937d79b335c668896d7533ac603adac5315/js/solady.js
func FlzCompressLen(ib []byte) uint32 {
	n := uint32(0)
	ht := make([]uint32, 8192)
	u24 := func(i uint32) uint32 {
		return uint32(ib[i]) | (uint32(ib[i+1]) << 8) | (uint32(ib[i+2]) << 16)
	}
	cmp := func(p uint32, q uint32, e uint32) uint32 {
		l := uint32(0)
		for e -= q; l < e; l++ {
			if ib[p+l] != ib[q+l] {
				e = 0
			}
		}
		return l
	}
	literals := func(r uint32) {
		n += 0x21 * (r / 0x20)
		r %= 0x20
		if r != 0 {
			n += r + 1
		}
	}
	match := func(l uint32) {
		l--
		n += 3 * (l / 262)
		if l%262 >= 6 {
			n += 3
		} else {
			n += 2
		}
	}
	hash := func(v uint32) uint32 {
		return ((2654435769 * v) >> 19) & 0x1fff
	}
	setNextHash := func(ip uint32) uint32 {
		ht[hash(u24(ip))] = ip
		return ip + 1
	}
	a := uint32(0)
	ipLimit := uint32(len(ib)) - 13
	if len(ib) < 13 {
		ipLimit = 0
	}
	for ip := a + 2; ip < ipLimit; {
		r := uint32(0)
		d := uint32(0)
		for {
			s := u24(ip)
			h := hash(s)
			r = ht[h]
			ht[h] = ip
			d = ip - r
			if ip >= ipLimit {
				break
			}
			ip++
			if d <= 0x1fff && s == u24(r) {
				break
			}
		}
		if ip >= ipLimit {
			break
		}
		ip--
		if ip > a {
			literals(ip - a)
		}
		l := cmp(r+3, ip+3, ipLimit+9)
		match(l)
		ip = setNextHash(setNextHash(ip + l))
		a = ip
	}
	literals(uint32(len(ib)) - a)
	return n
}
