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
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/holiman/uint256"
)

// Well-known Mantle pre-Arsia system addresses.
var (
	// L1BlockAddr is the L1Block predeploy (0x4200..0015). Holds the L1 fee
	// parameters that L1CostFunc reads when pricing rollup data.
	L1BlockAddr = common.HexToAddress("0x4200000000000000000000000000000000000015")
	// GasOracleAddr is the Mantle GasPriceOracle predeploy (0x420F). Slot 0
	// holds the Mantle-specific tokenRatio multiplier used pre-Arsia.
	GasOracleAddr = common.HexToAddress("0x420000000000000000000000000000000000000F")
)

// nonEmptyMarkerCode is a single INVALID-opcode byte injected as account code
// so that StateDB.Finalise() does not delete an account that only has storage
// slots set. EIP-161 removes balance==0 / nonce==0 / code-empty accounts at
// finalisation, which would silently zero out the injected L1Block /
// GasOracle slots before ApplyTransaction reads them.
var nonEmptyMarkerCode = []byte{0xFE}

// L1BlockState mirrors the three slots NewL1CostFuncBeforeArsia reads from
// the L1Block predeploy (see core/types/rollup_cost.go:231-234). Pre-Arsia
// only.
type L1BlockState struct {
	L1BaseFee *big.Int // slot 1
	Overhead  *big.Int // slot 5
	Scalar    *big.Int // slot 6
}

// L1BlockArsiaState mirrors the slots NewL1CostFuncArsia + NewOperatorCostFunc
// read from the L1Block predeploy post-Arsia. Slots 3 and 8 are packed
// (baseFeeScalar/blobBaseFeeScalar in slot 3; operatorFeeScalar/
// operatorFeeConstant in slot 8), so they are exposed as raw 32-byte hashes
// and the test author copies the on-chain value verbatim from eth_getStorageAt
// rather than trying to re-pack them locally.
type L1BlockArsiaState struct {
	L1BaseFee         *big.Int    // slot 1
	L1FeeScalars      common.Hash // slot 3 (raw packed: baseFeeScalar + blobBaseFeeScalar + ...)
	L1BlobBaseFee     *big.Int    // slot 7
	OperatorFeeParams common.Hash // slot 8 (raw packed: operatorFeeScalar + operatorFeeConstant + ...)
}

// PreState describes the on-chain state snapshot the replay needs at block-1.
// All maps are optional; nil maps are treated as empty.
type PreState struct {
	// Balances seeds account balances (sender/sponsor/recipient/etc.).
	Balances map[common.Address]*uint256.Int
	// Nonces seeds account nonces. Most replays leave nonces zero.
	Nonces map[common.Address]uint64
	// Codes injects deployed bytecode. Injecting empty code is fine but
	// callers usually want this to keep an account non-empty.
	Codes map[common.Address][]byte
	// Storage injects arbitrary storage slots for arbitrary accounts.
	Storage map[common.Address]map[common.Hash]common.Hash
	// L1Block, when set, populates the L1Block predeploy slots needed by
	// pre-Arsia L1CostFunc. The account is auto-marked non-empty.
	L1Block *L1BlockState
	// L1BlockArsia, when set, populates the L1Block predeploy slots needed by
	// post-Arsia L1CostFunc + NewOperatorCostFunc. Mutually exclusive with
	// L1Block — pick the one that matches the case's fork boundary.
	L1BlockArsia *L1BlockArsiaState
	// TokenRatio, when set, populates GasOracle slot 0 (Mantle-specific
	// multiplier). Pre-Arsia L1CostFunc reads it; post-Arsia replays may set
	// it to keep state shape close to mainnet (cheap insurance).
	TokenRatio *big.Int
}

// Apply writes the snapshot into db. Accounts that only receive storage slots
// are auto-marked with a single INVALID-opcode byte of code so that
// state.Finalise() does not delete them under EIP-161.
func (p PreState) Apply(db *state.StateDB) {
	for addr, bal := range p.Balances {
		db.SetBalance(addr, bal, tracing.BalanceChangeUnspecified)
	}
	for addr, nonce := range p.Nonces {
		db.SetNonce(addr, nonce, tracing.NonceChangeUnspecified)
	}
	for addr, code := range p.Codes {
		db.SetCode(addr, code, tracing.CodeChangeUnspecified)
	}
	for addr, slots := range p.Storage {
		ensureNonEmpty(db, addr)
		for slot, val := range slots {
			db.SetState(addr, slot, val)
		}
	}
	if p.L1Block != nil {
		injectL1Block(db, *p.L1Block)
	}
	if p.L1BlockArsia != nil {
		injectL1BlockArsia(db, *p.L1BlockArsia)
	}
	if p.TokenRatio != nil {
		injectGasOracle(db, p.TokenRatio)
	}
}

// injectL1Block writes the three slots NewL1CostFuncBeforeArsia reads and
// guarantees the account survives Finalise().
func injectL1Block(db *state.StateDB, s L1BlockState) {
	ensureNonEmpty(db, L1BlockAddr)
	if s.L1BaseFee != nil {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(1)), common.BigToHash(s.L1BaseFee))
	}
	if s.Overhead != nil {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(5)), common.BigToHash(s.Overhead))
	}
	if s.Scalar != nil {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(6)), common.BigToHash(s.Scalar))
	}
}

// injectGasOracle writes the Mantle tokenRatio multiplier into GasOracle slot 0
// and guarantees the account survives Finalise().
func injectGasOracle(db *state.StateDB, tokenRatio *big.Int) {
	ensureNonEmpty(db, GasOracleAddr)
	db.SetState(GasOracleAddr, common.Hash{}, common.BigToHash(tokenRatio))
}

// injectL1BlockArsia writes the post-Arsia L1Block slots. NewL1CostFuncArsia
// reads slot 1 (l1BaseFee), slot 3 (packed l1FeeScalars), and slot 7
// (l1BlobBaseFee); NewOperatorCostFunc reads slot 8 (packed operator params).
func injectL1BlockArsia(db *state.StateDB, s L1BlockArsiaState) {
	ensureNonEmpty(db, L1BlockAddr)
	if s.L1BaseFee != nil {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(1)), common.BigToHash(s.L1BaseFee))
	}
	if (s.L1FeeScalars != common.Hash{}) {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(3)), s.L1FeeScalars)
	}
	if s.L1BlobBaseFee != nil {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(7)), common.BigToHash(s.L1BlobBaseFee))
	}
	if (s.OperatorFeeParams != common.Hash{}) {
		db.SetState(L1BlockAddr, common.BigToHash(big.NewInt(8)), s.OperatorFeeParams)
	}
}

// ensureNonEmpty marks an account non-empty if no code has been set yet, so
// EIP-161 empty-delete does not wipe its storage during Finalise().
func ensureNonEmpty(db *state.StateDB, addr common.Address) {
	if len(db.GetCode(addr)) == 0 {
		db.SetCode(addr, nonEmptyMarkerCode, tracing.CodeChangeUnspecified)
	}
}
