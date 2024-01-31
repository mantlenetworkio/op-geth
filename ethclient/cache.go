package ethclient

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/log"
)

var blockHashCache map[common.Hash]common.Hash

func init() {
	blockHashCache = make(map[common.Hash]common.Hash)
}

func SetBlockHashCache(hash, cacheHash common.Hash) {
	blockHashCache[hash] = cacheHash
}

func GetBlockHashCache(hash common.Hash) common.Hash {
	cacheBlockHash, ok := blockHashCache[hash]
	log.Info("GetBlockHashCache", "hash", hash.String(), "cacheHash", cacheBlockHash.String())
	if !ok {
		return hash
	}
	return cacheBlockHash
}
