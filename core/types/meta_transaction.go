package types

import (
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

type MetaTransactionData struct {
	ExpireHeight uint64
	Payload      []byte
	// Signature values
	V *big.Int
	R *big.Int
	S *big.Int
}

type MetaTransactionSignData struct {
	ChainID      *big.Int
	Nonce        uint64
	GasTipCap    *big.Int
	GasFeeCap    *big.Int
	Gas          uint64
	To           *common.Address `rlp:"nil"`
	Value        *big.Int
	Data         []byte
	AccessList   AccessList
	ExpireHeight uint64
}

func (metaTxSignData *MetaTransactionSignData) Hash() common.Hash {
	return rlpHash(metaTxSignData)
}
