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
		ToleranceBlock:       3,
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

func (c *MinerConfig) MantleToleranceDuration() time.Duration {
	return time.Duration(c.ToleranceBlock*2) * time.Second
}

// 3 is the fixed delay of 3 blocks for the op-node to start deriving
// 2 is the possible delay of 2 blocks for l1rpc to obtain the latest block height
func (c *MinerConfig) EthToleranceDuration() time.Duration {
	return time.Duration(c.ToleranceBlock+3+2) * 12 * time.Second
}

func (c *MinerConfig) EthToleranceBlock() uint64 {
	return uint64(c.ToleranceBlock + 3)
}
