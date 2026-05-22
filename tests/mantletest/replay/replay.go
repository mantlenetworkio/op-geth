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

// Package replay holds byte-equal Mantle mainnet transaction replays used
// to guard execution-layer merges against historical regressions (pre-Arsia
// fee model, MetaTx, deposit/L1Block storage layout, etc.).
//
// Tests in this package require the `mantle_replay` build tag so they do not
// run as part of the default `go test ./...` suite. Run them explicitly:
//
//	go test -tags mantle_replay ./tests/mantletest/replay/...
//
// Each ReplayCase declares a real mainnet transaction plus its golden receipt;
// Run() applies the tx against an in-memory StateDB seeded with the on-chain
// pre-state and asserts byte-equal parity with the mainnet outcome.
package replay

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

// GoldenReceipt captures the mainnet receipt fields that a replay must match
// byte-for-byte. Pointer fields are optional — nil means "skip this assertion".
type GoldenReceipt struct {
	Status    uint64   // required (use 0 or 1 as the receipt status)
	GasUsed   uint64   // required
	L1Fee     *big.Int // optional (non-deposit, post-Bedrock receipts only)
	L1GasUsed *big.Int // optional
}

// ReplayCase fully describes one historical Mantle mainnet transaction and
// the byte-equal invariants the local execution layer must reproduce.
type ReplayCase struct {
	// Name is a short identifier used in t.Log output (e.g. "metatx-5d1d3888").
	Name string

	// Tx is the locally rebuilt transaction. It MUST match ExpectHash to prove
	// codec parity before the replay even runs.
	Tx *types.Transaction
	// ExpectHash is the mainnet tx hash; codec-parity gate.
	ExpectHash common.Hash

	// Header is the block header used to seed BlockContext. The replay does
	// not need a real parent chain — only fields read by ApplyTransaction
	// (Number/Time/BaseFee/GasLimit/Coinbase).
	Header *types.Header

	// Sender is the recovered tx sender, used for post-state invariants.
	Sender common.Address

	// PreState seeds the in-memory StateDB before ApplyTransaction.
	PreState PreState

	// Golden is the mainnet receipt to match byte-for-byte.
	Golden GoldenReceipt

	// PostNonce, when non-nil, asserts the exact nonce of each address after
	// the tx (typically {sender: prevNonce+1}).
	PostNonce map[common.Address]uint64
	// PostBalanceZero asserts each listed address has balance==0 after the tx
	// (useful for MetaTx where the user is fully sponsored).
	PostBalanceZero []common.Address
}

// Run executes the replay end-to-end and asserts byte-equal parity with
// mainnet. It fails fast on the first divergence so the diagnostic stays
// focused on the actual regression source.
func (c *ReplayCase) Run(t *testing.T, cfg *params.ChainConfig) {
	t.Helper()
	require.NotNil(t, c.Tx, "ReplayCase.Tx must be set")
	require.NotNil(t, c.Header, "ReplayCase.Header must be set")
	require.NotNil(t, cfg, "chain config must be supplied")

	// ── codec parity gate ──────────────────────────────────────────────
	require.Equal(t, c.ExpectHash, c.Tx.Hash(),
		"[%s] reconstructed tx hash must match mainnet (codec parity)", c.Name)

	// ── fresh in-memory StateDB + pre-state injection ──────────────────
	db, err := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	require.NoError(t, err, "[%s] state.New", c.Name)
	c.PreState.Apply(db)

	// ── EVM wiring with real L1CostFunc ────────────────────────────────
	blockCtx := BuildBlockContext(cfg, db, c.Header)
	require.NotNil(t, blockCtx.L1CostFunc,
		"[%s] L1CostFunc must be wired (mantle Optimism config missing?)", c.Name)
	evm := vm.NewEVM(blockCtx, db, cfg, vm.Config{})
	gp := core.NewGasPool(c.Header.GasLimit)

	// ── apply ──────────────────────────────────────────────────────────
	receipt, err := core.ApplyTransaction(evm, gp, db, c.Header, c.Tx)
	require.NoError(t, err, "[%s] ApplyTransaction", c.Name)
	require.NotNil(t, receipt, "[%s] receipt must be non-nil", c.Name)

	// ── byte-equal receipt assertions ──────────────────────────────────
	require.Equal(t, c.Golden.Status, receipt.Status,
		"[%s] receipt.Status local=%d, mainnet=%d", c.Name, receipt.Status, c.Golden.Status)

	require.Equal(t, c.Golden.GasUsed, receipt.GasUsed,
		"[%s] receipt.GasUsed local=%d, mainnet=%d", c.Name, receipt.GasUsed, c.Golden.GasUsed)

	if c.Golden.L1Fee != nil {
		require.NotNil(t, receipt.L1Fee, "[%s] receipt.L1Fee must be populated", c.Name)
		require.Zero(t, c.Golden.L1Fee.Cmp(receipt.L1Fee),
			"[%s] receipt.L1Fee local=%s, mainnet=%s",
			c.Name, receipt.L1Fee.String(), c.Golden.L1Fee.String())
	}
	if c.Golden.L1GasUsed != nil {
		require.NotNil(t, receipt.L1GasUsed, "[%s] receipt.L1GasUsed must be populated", c.Name)
		require.Zero(t, c.Golden.L1GasUsed.Cmp(receipt.L1GasUsed),
			"[%s] receipt.L1GasUsed local=%s, mainnet=%s",
			c.Name, receipt.L1GasUsed.String(), c.Golden.L1GasUsed.String())
	}

	// ── post-state invariants ──────────────────────────────────────────
	for addr, want := range c.PostNonce {
		require.Equal(t, want, db.GetNonce(addr),
			"[%s] nonce(%s) local=%d, want=%d",
			c.Name, addr.Hex(), db.GetNonce(addr), want)
	}
	for _, addr := range c.PostBalanceZero {
		require.True(t, db.GetBalance(addr).Sign() == 0,
			"[%s] balance(%s) must be zero, got %s",
			c.Name, addr.Hex(), db.GetBalance(addr).String())
	}

	t.Logf("[%s] REPLAY OK: hash=%s status=%d gasUsed=%d l1Fee=%v l1GasUsed=%v",
		c.Name, receipt.TxHash.Hex(), receipt.Status, receipt.GasUsed, receipt.L1Fee, receipt.L1GasUsed)
}
