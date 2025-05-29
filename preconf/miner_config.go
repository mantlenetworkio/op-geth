package preconf

import (
	"fmt"
	"time"
)

var (
	DefaultMinerConfig = MinerConfig{
		EnablePreconfChecker: false, // let rpc disable preconf checker
		OptimismNodeHTTP:     "http://localhost:7545",
		L1RPCHTTP:            "http://localhost:8545",
		L1DepositAddress:     "0xa513E6E4b8f2a923D98304ec87F64353C4D5C853",
		ToleranceBlock:       6,
		PreconfBufferBlock:   6,
	}
)

type MinerConfig struct {
	EnablePreconfChecker bool
	OptimismNodeHTTP     string
	L1RPCHTTP            string
	L1DepositAddress     string
	ToleranceBlock       int64
	PreconfBufferBlock   uint64
}

func (c *MinerConfig) String() string {
	return fmt.Sprintf("EnablePreconfChecker: %t, OptimismNodeHTTP: %s, L1RPCHTTP: %s, L1DepositAddress: %s, ToleranceBlock: %d, MantleToleranceDuration: %s, EthToleranceDuration: %s, EthToleranceBlock: %d, PreconfBufferBlock: %d", c.EnablePreconfChecker, c.OptimismNodeHTTP, c.L1RPCHTTP, c.L1DepositAddress, c.ToleranceBlock, c.MantleToleranceDuration(), c.EthToleranceDuration(), c.EthToleranceBlock(), c.PreconfBufferBlock)
}

// When the current configuration is 6s, there are still occasional false positives.
// If the default is 6, the result is 6*2s=12s
func (c *MinerConfig) MantleToleranceDuration() time.Duration {
	return time.Duration(c.ToleranceBlock*2) * time.Second
}

// 3 is the fixed delay of 3 blocks for the op-node to start deriving
// When the current configuration is 1m36s, there are still occasional false positives.
// If the default is 6, the result is 6+3=9*12s=108s=1m48s
func (c *MinerConfig) EthToleranceDuration() time.Duration {
	return time.Duration(c.ToleranceBlock+3) * 12 * time.Second
}

func (c *MinerConfig) EthToleranceBlock() uint64 {
	return uint64(c.ToleranceBlock + 3)
}
