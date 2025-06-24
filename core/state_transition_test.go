package core

import (
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
	if refund := st.calcRefund(4000, false); refund != 1000000 {
		t.Errorf("before skadi calc refund is: %d, expectd: %v", refund, 1000000)
	}

	// gasUsed = st.initialGas/tokeRatio - st.gasRemaining = 10000000000/4000 - 500000 = 2000000
	// maxRefund = gasUsed/5 = 2000000/5 = 400000
	// st.state.GetRefund() = 1000000 > maxRefund, so return 400000
	if refund := st.calcRefund(4000, true); refund != 400000 {
		t.Errorf("after skadi calc refund is: %d, expectd: %v", refund, 400000)
	}
}
