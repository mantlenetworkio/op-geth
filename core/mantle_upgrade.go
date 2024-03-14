package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

var (
	MantleMainnetUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleMainnetChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxUpgradeTime:     u64Ptr(0),
	}

	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleSepoliaChainId,
		BaseFeeTime:           u64Ptr(1_704_891_600),
		BVMETHMintUpgradeTime: nil, //TODO set upgrade timestamp
		MetaTxUpgradeTime:     nil, //TODO set upgrade timestamp
	}
	MantleLocalUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleLocalChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxUpgradeTime:     u64Ptr(0),
	}
	MantleDefaultUpgradeConfig = MantleUpgradeChainConfig{
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxUpgradeTime:     u64Ptr(0),
	}
)

type MantleUpgradeChainConfig struct {
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection

	BaseFeeTime           *uint64 `json:"BaseFeeTime"`           // Mantle BaseFee switch time (nil = no fork, 0 = already on mantle baseFee)
	BVMETHMintUpgradeTime *uint64 `json:"BVMETHMintUpgradeTime"` // BVM_ETH mint upgrade switch time (nil = no fork, 0 = already on)
	MetaTxUpgradeTime     *uint64 `json:"metaTxUpgradeTime"`     // MetaTxUpgradeBlock identifies the current block height is using metaTx with MetaTxSignDataV2
}

func GetUpgradeConfigForMantle(chainID *big.Int) *MantleUpgradeChainConfig {
	if chainID == nil {
		return nil
	}
	switch chainID.Int64() {
	case params.MantleMainnetChainId.Int64():
		return &MantleMainnetUpgradeConfig
	case params.MantleSepoliaChainId.Int64():
		return &MantleSepoliaUpgradeConfig
	case params.MantleLocalChainId.Int64():
		return &MantleLocalUpgradeConfig
	default:
		return &MantleDefaultUpgradeConfig
	}
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
