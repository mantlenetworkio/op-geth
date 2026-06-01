package core

import (
	"errors"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/core/state"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/params"
)

func TestCalcRefund(t *testing.T) {
	statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
	statedb.AddRefund(1000000)
	evm := vm.NewEVM(vm.BlockContext{BlockNumber: big.NewInt(100)}, statedb, params.OptimismTestConfig, vm.Config{})
	gp := GasPool(200000000000)
	st := newStateTransition(evm, &Message{}, &gp)
	st.initialGas = 10000000000
	st.gasRemaining = 500000

	// gasUsed = st.initialGas - st.gasRemaining = 10000000000 - 500000 = 9999500000
	// maxRefund = gasUsed/5 = 2000000/5 = 1999900000
	// st.state.GetRefund() = 1000000 < maxRefund, so return 1000000
	if refund := st.calcRefundMantle(4000, false); refund != 1000000 {
		t.Errorf("before skadi calc refund is: %d, expectd: %v", refund, 1000000)
	}

	// gasUsed = st.initialGas/tokeRatio - st.gasRemaining = 10000000000/4000 - 500000 = 2000000
	// maxRefund = gasUsed/5 = 2000000/5 = 400000
	// st.state.GetRefund() = 1000000 > maxRefund, so return 400000
	if refund := st.calcRefundMantle(4000, true); refund != 400000 {
		t.Errorf("after skadi calc refund is: %d, expectd: %v", refund, 400000)
	}
}

func TestDepositGasPoolReservation(t *testing.T) {
	mkSt := func(gp *GasPool, msg *Message) *stateTransition {
		statedb, _ := state.New(types.EmptyRootHash, state.NewDatabaseForTesting())
		// Regolith activates at time=0 in OptimismTestConfig, so the system-tx
		// branch returns ErrSystemTxNotSupported as in production.
		evm := vm.NewEVM(
			vm.BlockContext{BlockNumber: big.NewInt(100), Time: 1_000_000},
			statedb,
			params.OptimismTestConfig,
			vm.Config{},
		)
		return newStateTransition(evm, msg, gp)
	}

	t.Run("deposit reserves GasLimit when pool is sufficient", func(t *testing.T) {
		gp := GasPool(30_000_000)
		msg := &Message{IsDepositTx: true, GasLimit: 2_000_000}
		st := mkSt(&gp, msg)

		if _, err := st.preCheck(true); err != nil {
			t.Fatalf("preCheck err: %v", err)
		}
		if got := gp.Gas(); got != 28_000_000 {
			t.Fatalf("gp.Gas() after deposit: got %d, want 28_000_000", got)
		}
	})

	t.Run("deposit fails when pool is insufficient", func(t *testing.T) {
		gp := GasPool(30_000_000)
		msg := &Message{IsDepositTx: true, GasLimit: 30_000_001}
		st := mkSt(&gp, msg)

		_, err := st.preCheck(true)
		if !errors.Is(err, ErrGasLimitReached) {
			t.Fatalf("preCheck err: got %v, want ErrGasLimitReached", err)
		}
		if got := gp.Gas(); got != 30_000_000 {
			t.Fatalf("gp mutated after failed SubGas: got %d, want 30_000_000", got)
		}
	})

	t.Run("system tx must not touch the gas pool", func(t *testing.T) {
		gp := GasPool(30_000_000)
		msg := &Message{IsDepositTx: true, IsSystemTx: true, GasLimit: 1_000_000}
		st := mkSt(&gp, msg)

		// Post-Regolith system txs return ErrSystemTxNotSupported before
		// touching gp. We only assert gp is untouched.
		_, _ = st.preCheck(true)
		if got := gp.Gas(); got != 30_000_000 {
			t.Fatalf("system tx mutated gp: got %d, want 30_000_000", got)
		}
	})

	t.Run("multiple deposits drain pool cumulatively", func(t *testing.T) {
		gp := GasPool(30_000_000)
		for i, gl := range []uint64{500_000, 1_500_000, 3_000_000} {
			msg := &Message{IsDepositTx: true, GasLimit: gl}
			st := mkSt(&gp, msg)
			if _, err := st.preCheck(true); err != nil {
				t.Fatalf("deposit %d preCheck err: %v", i, err)
			}
		}
		// 30M - 0.5M - 1.5M - 3M = 25M
		if got := gp.Gas(); got != 25_000_000 {
			t.Fatalf("cumulative gp: got %d, want 25_000_000", got)
		}
	})
}
