package ethclient

import "github.com/ethereum/go-ethereum/common"

var BlockHashCache map[common.Hash]common.Hash

func init() {
	BlockHashCache = make(map[common.Hash]common.Hash)
}
