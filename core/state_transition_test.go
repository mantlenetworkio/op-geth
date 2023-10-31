package core

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"

	"github.com/stretchr/testify/require"
)

var (
	userKey, _          = crypto.HexToECDSA("eef77acb6c6a6eebc5b363a475ac583ec7eccdb42b6481424c60f59aa326547f")
	gasFeeSponsorKey, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
)

func TestFigureOutMetaTxParams(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	data, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)
	dynamicTx := &types.DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4700000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       data,
		AccessList: nil,
	}

	metaTxSignData := &types.MetaTxSignData{
		ChainID:      dynamicTx.ChainID,
		Nonce:        dynamicTx.Nonce,
		GasTipCap:    dynamicTx.GasTipCap,
		GasFeeCap:    dynamicTx.GasFeeCap,
		Gas:          dynamicTx.Gas,
		To:           dynamicTx.To,
		Value:        dynamicTx.Value,
		Data:         dynamicTx.Data,
		AccessList:   dynamicTx.AccessList,
		ExpireHeight: expireHeight,
	}

	sig, err := crypto.Sign(metaTxSignData.Hash().Bytes(), gasFeeSponsorKey)
	require.NoError(t, err)

	metaTxData := &types.MetaTxData{
		ExpireHeight: expireHeight,
		Payload:      metaTxSignData.Data,
		Signature:    sig,
	}

	metaTxDataBz, err := rlp.EncodeToBytes(metaTxData)
	require.NoError(t, err)

	dynamicTx.Data = append(metaTxPrefix, metaTxDataBz...)
	tx := types.NewTx(dynamicTx)
	signer := types.LatestSignerForChainID(chainId)
	signature, err := crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	tx, err = tx.WithSignature(signer, signature)
	require.NoError(t, err)

	// test expected metaTx
	cfg := params.MainnetChainConfig
	currentHeight := big.NewInt(20_000_000)
	baseFee := big.NewInt(1e9)

	msg, err := TransactionToMessage(tx, types.MakeSigner(cfg, currentHeight), baseFee)
	require.NoError(t, err)

	metaTxParams, err := figureOutMetaTxParams(msg, currentHeight.Uint64(), chainId)
	require.NoError(t, err)

	require.Equal(t, gasFeeSponsorAddr.String(), metaTxParams.GasFeeSponsor.String())
	require.Equal(t, hexutil.Encode(data), hexutil.Encode(metaTxParams.Payload))

	// Test ErrExpiredMetaTx
	currentHeight = big.NewInt(20_000_011)

	msg, err = TransactionToMessage(tx, types.MakeSigner(cfg, currentHeight), baseFee)
	require.NoError(t, err)

	metaTxParams, err = figureOutMetaTxParams(msg, currentHeight.Uint64(), chainId)
	require.Equal(t, err, types.ErrExpiredMetaTx)

	// Test ErrInvalidGasFeeSponsorSig
	sig[len(sig)-1] = sig[len(sig)-1] + 1 // modify signature
	metaTxData = &types.MetaTxData{
		ExpireHeight: expireHeight,
		Payload:      metaTxSignData.Data,
		Signature:    sig,
	}

	metaTxDataBz, err = rlp.EncodeToBytes(metaTxData)
	require.NoError(t, err)

	dynamicTx.Data = append(metaTxPrefix, metaTxDataBz...)
	tx = types.NewTx(dynamicTx)
	signature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	tx, err = tx.WithSignature(signer, signature)
	require.NoError(t, err)

	currentHeight = big.NewInt(20_000_009)
	msg, err = TransactionToMessage(tx, types.MakeSigner(cfg, currentHeight), baseFee)
	require.NoError(t, err)

	_, err = figureOutMetaTxParams(msg, currentHeight.Uint64(), chainId)
	require.Equal(t, err, types.ErrInvalidGasFeeSponsorSig)
}
