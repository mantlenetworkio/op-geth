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
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/params"
)

var (
	// EcotoneL1AttributesSelector is the selector indicating Ecotone style L1 gas attributes.
	EcotoneL1AttributesSelector = []byte{0x44, 0x0a, 0x5e, 0x20}

	L1CostIntercept  = big.NewInt(-42_585_600)
	L1CostFastlzCoef = big.NewInt(836_500)

	MinTransactionSize       = big.NewInt(100)
	MinTransactionSizeScaled = new(big.Int).Mul(MinTransactionSize, big.NewInt(1e6))
)

type RollupCostData struct {
	Zeroes, Ones uint64
	FastLzSize   uint64
}

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

// estimatedDASizeScaled estimates the number of bytes the transaction will occupy in the DA batch using the Fjord
// linear regression model, and returns this value scaled up by 1e6.
func (cd RollupCostData) estimatedDASizeScaled() *big.Int {
	fastLzSize := new(big.Int).SetUint64(cd.FastLzSize)
	estimatedSize := new(big.Int).Add(L1CostIntercept, new(big.Int).Mul(L1CostFastlzCoef, fastLzSize))

	if estimatedSize.Cmp(MinTransactionSizeScaled) < 0 {
		estimatedSize.Set(MinTransactionSizeScaled)
	}
	return estimatedSize
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
type L1CostFunc func(blockNum uint64, blockTime uint64, dataGas RollupCostData, isDepositTx bool, to *common.Address) *big.Int

var (
	L1BaseFeeSlot  = common.BigToHash(big.NewInt(1))
	OverheadSlot   = common.BigToHash(big.NewInt(5))
	ScalarSlot     = common.BigToHash(big.NewInt(6))
	TokenRatioSlot = common.BigToHash(big.NewInt(0))

	L1BlockAddr   = common.HexToAddress("0x4200000000000000000000000000000000000015")
	GasOracleAddr = common.HexToAddress("0x420000000000000000000000000000000000000F")
	Decimals      = big.NewInt(1_000_000)
)

// NewL1CostFunc returns a function used for calculating L1 fee cost.
// This depends on the oracles because gas costs can change over time.
// It returns nil if there is no applicable cost function.
func NewL1CostFunc(config *params.ChainConfig, statedb StateGetter) L1CostFunc {
	cacheBlockNum := ^uint64(0)
	var l1BaseFee, overhead, scalar, tokenRatio *big.Int
	return func(blockNum uint64, blockTime uint64, rollupCostData RollupCostData, isDepositTx bool, to *common.Address) *big.Int {
		rollupDataGas := rollupCostData.DataGas(blockTime, config) // Only fake txs for RPC view-calls are 0.
		if config.Optimism == nil || isDepositTx || rollupDataGas == 0 {
			return common.Big0
		}
		if blockNum != cacheBlockNum {
			l1BaseFee = statedb.GetState(L1BlockAddr, L1BaseFeeSlot).Big()
			overhead = statedb.GetState(L1BlockAddr, OverheadSlot).Big()
			scalar = statedb.GetState(L1BlockAddr, ScalarSlot).Big()
			tokenRatio = statedb.GetState(GasOracleAddr, TokenRatioSlot).Big()
			cacheBlockNum = blockNum
		}

		// update the tokenRatio, so set the cacheBlockNum as default value and query the latest tokenRatio next time
		if to != nil && *to == GasOracleAddr {
			cacheBlockNum = ^uint64(0)
		}

		return L1Cost(rollupDataGas, l1BaseFee, overhead, scalar, tokenRatio)
	}
}

func L1Cost(rollupDataGas uint64, l1BaseFee, overhead, scalar, tokenRatio *big.Int) *big.Int {
	l1GasUsed := new(big.Int).SetUint64(rollupDataGas)
	l1GasUsed = l1GasUsed.Add(l1GasUsed, overhead)
	l1Cost := l1GasUsed.Mul(l1GasUsed, l1BaseFee)
	l1Cost = l1Cost.Mul(l1Cost, scalar)
	l1Cost = l1Cost.Mul(l1Cost, tokenRatio)
	return l1Cost.Div(l1Cost, Decimals)
}

// DeriveL1GasInfo reads L1 gas related information to be included
// on the receipt
func DeriveL1GasInfo(state StateGetter) (*big.Int, *big.Int, *big.Int, *big.Float, *big.Int) {
	l1BaseFee, overhead, scalar, scaled := readL1BlockStorageSlots(L1BlockAddr, state)
	tokenRatio := readGPOStorageSlots(GasOracleAddr, state)
	return l1BaseFee, overhead, scalar, scaled, tokenRatio
}

func readL1BlockStorageSlots(addr common.Address, state StateGetter) (*big.Int, *big.Int, *big.Int, *big.Float) {
	l1BaseFee := state.GetState(addr, L1BaseFeeSlot)
	overhead := state.GetState(addr, OverheadSlot)
	scalar := state.GetState(addr, ScalarSlot)
	scaled := scaleDecimals(scalar.Big(), Decimals)
	return l1BaseFee.Big(), overhead.Big(), scalar.Big(), scaled
}

func readGPOStorageSlots(addr common.Address, state StateGetter) *big.Int {
	tokenRatio := state.GetState(addr, TokenRatioSlot)
	return tokenRatio.Big()
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
