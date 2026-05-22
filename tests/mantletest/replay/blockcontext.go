// Copyright 2026 The go-ethereum Authors
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

//go:build mantle_replay
// +build mantle_replay

package replay

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

// BuildBlockContext returns a vm.BlockContext suitable for replaying a single
// historical Mantle transaction. It wires the canonical Mantle L1CostFunc
// (which reads pre-Arsia L1Block/GasOracle slots when applicable, or
// post-Arsia per-tx params otherwise).
//
// OperatorCostFunc is wired unconditionally — types.NewOperatorCostFunc is
// itself fork-aware and returns a zero-cost stub when the chain is not yet on
// Arsia, so passing it in is safe across all replays.
func BuildBlockContext(cfg *params.ChainConfig, db *state.StateDB, header *types.Header) vm.BlockContext {
	return vm.BlockContext{
		CanTransfer:      core.CanTransfer,
		Transfer:         core.Transfer,
		GetHash:          func(uint64) common.Hash { return common.Hash{} },
		Coinbase:         header.Coinbase,
		BlockNumber:      new(big.Int).Set(header.Number),
		Time:             header.Time,
		Difficulty:       big.NewInt(0),
		BaseFee:          new(big.Int).Set(header.BaseFee),
		GasLimit:         header.GasLimit,
		L1CostFunc:       types.NewL1CostFunc(cfg, db),
		OperatorCostFunc: types.NewOperatorCostFunc(cfg, db),
	}
}
