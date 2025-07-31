package preconf

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
)

type Params struct {
	// ParentHash Detect block fork
	ParentHash common.Hash `json:"parentHash" gencodec:"required"`
	// Block number must match the one currently being built by the EVM,
	//otherwise the pre-confirmation transaction will fail
	BlockNumber uint64 `json:"blockNumber" gencodec:"required"`
	// Timeout
	Timeout string `json:"timeout" gencodec:"optional"`
	// Transactions if there are multiple transactions, they must support atomic operations.
	Transactions []hexutil.Bytes `json:"transactions" gencodec:"optional"`
	// ForkBlock If an L2 block reorg occurs, the previous cache needs to be forcibly cleared. Default is false.
	ForkBlock bool `json:"forkBlock" gencodec:"optional" default:"false"`
}

type Response struct {
	Status   string          `json:"status"`
	Receipts []hexutil.Bytes `json:"receipts"`
}
