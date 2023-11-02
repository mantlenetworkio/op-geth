package types

import (
	"bytes"
	"errors"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/rlp"
)

const (
	MetaTxPrefixLength = 32
)

var (
	// ASCII code of "MantleMetaTxPrefix"
	MetaTxPrefix, _ = hexutil.Decode("0x00000000000000000000000000004D616E746C654D6574615478507265666978")

	ErrExpiredMetaTx           = errors.New("expired meta transaction")
	ErrInvalidGasFeeSponsorSig = errors.New("invalid gas fee sponsor signature")
	ErrGasFeeSponsorMismatch   = errors.New("gas fee sponsor address is mismatch with signature")
)

type MetaTxParams struct {
	ExpireHeight uint64
	Payload      []byte

	// In tx simulation, Signature will be empty, user can specify GasFeeSponsor to sponsor gas fee
	GasFeeSponsor common.Address
	// Signature values
	V *big.Int
	R *big.Int
	S *big.Int
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

func DecodeMetaTxParams(txData []byte) (*MetaTxParams, error) {
	if len(txData) <= len(MetaTxPrefix) {
		return nil, nil
	}
	if !bytes.Equal(txData[:MetaTxPrefixLength], MetaTxPrefix) {
		return nil, nil
	}

	var metaTxParams MetaTxParams
	err := rlp.DecodeBytes(txData[MetaTxPrefixLength:], &metaTxParams)
	if err != nil {
		return nil, err
	}

	return &metaTxParams, nil
}

func DecodeAndVerifyMetaTxParams(tx *Transaction) (*MetaTxParams, error) {
	if tx.Type() != DynamicFeeTxType {
		return nil, nil
	}

	if mtp := tx.metaTxParams.Load(); mtp != nil {
		mtpCache, ok := mtp.(*MetaTxParams)
		if ok {
			return mtpCache, nil
		}
	}

	metaTxParams, err := DecodeMetaTxParams(tx.Data())
	if err != nil {
		return nil, err
	}
	if metaTxParams == nil {
		return nil, nil
	}

	metaTxSignData := &MetaTxSignData{
		ChainID:      tx.ChainId(),
		Nonce:        tx.Nonce(),
		GasTipCap:    tx.GasTipCap(),
		GasFeeCap:    tx.GasFeeCap(),
		Gas:          tx.Gas(),
		To:           tx.To(),
		Value:        tx.Value(),
		Data:         metaTxParams.Payload,
		AccessList:   tx.AccessList(),
		ExpireHeight: metaTxParams.ExpireHeight,
	}

	gasFeeSponsorSigner, err := recoverPlain(metaTxSignData.Hash(), metaTxParams.R, metaTxParams.S, metaTxParams.V, true)
	if err != nil {
		return nil, ErrInvalidGasFeeSponsorSig
	}

	if gasFeeSponsorSigner != metaTxParams.GasFeeSponsor {
		return nil, ErrGasFeeSponsorMismatch
	}

	tx.metaTxParams.Store(metaTxParams)

	return metaTxParams, nil
}

func (metaTxSignData *MetaTxSignData) Hash() common.Hash {
	return rlpHash(metaTxSignData)
}
