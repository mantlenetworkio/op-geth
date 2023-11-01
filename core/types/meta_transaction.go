package types

import (
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
)

var (
	ErrExpiredMetaTx           = errors.New("expired meta transaction")
	ErrInvalidGasFeeSponsorSig = errors.New("invalid gas fee sponsor signature")
	ErrGasFeeSponsorMismatch   = errors.New("gas fee sponsor address is mismatch with signature")
)

type MetaTxData struct {
	ExpireHeight uint64
	Payload      []byte

	// In tx simulation, Signature will be empty, user can specify GasFeeSponsor to sponsor gas fee
	GasFeeSponsor common.Address
	Signature     []byte
}

type MetaTxParams struct {
	Payload       []byte
	GasFeeSponsor common.Address
}

type MetaTxSignData struct {
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

func (metaTxSignData *MetaTxSignData) Hash() common.Hash {
	return rlpHash(metaTxSignData)
}
