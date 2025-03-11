package main

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/tests/preconf/reorg"
	"github.com/ethereum/go-ethereum/tests/preconf/sort"
	"github.com/ethereum/go-ethereum/tests/preconf/stress"
)

func main() {
	sort.SortTest()
	stress.StressTest()
	reorg.L1ReorgDetection(common.HexToHash("0xe3f60268eb85440e5b2212cb748b3ea3df4cac7973a846ea16f7fa85c68a5eda"))
}
