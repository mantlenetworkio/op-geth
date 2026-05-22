package main

import (
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/basefee"
	basefee_stress "github.com/ethereum/go-ethereum/tests/mantletest/preconf/basefee_stress"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/blockfull"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/check"
	frontrunning "github.com/ethereum/go-ethereum/tests/mantletest/preconf/front_running"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/sort"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/stress"
)

// There are three tests that require manual modification of op-geth configuration to cover:
// 1. Set txpool.preconftimeout to a very small value (e.g. 1ms) to test timeout handling.
// 2. Manually restart op-geth while processing a large number of preconfirmation transactions to test journal handling.
// 3. Manually modify op-geth's gaslimit upper bound (e.g. 200000000000) to test handling when preconfirmation transactions fill up the block.
func main() {
	check.Check()
	stress.StressTest()
	frontrunning.TransferTest()
	sort.SortTest()
	frontrunning.ERC20Test()

	// baseFee consistency & block-full tests (added for preconf baseFee recalc PR)
	basefee.BaseFeeConsistencyTest()
	blockfull.BlockFullTest()
	basefee_stress.StressTest()

	// reorg.L1ReorgDetection(common.HexToHash("0xe3f60268eb85440e5b2212cb748b3ea3df4cac7973a846ea16f7fa85c68a5eda"))
}
