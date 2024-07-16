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
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   nil, //TODO set upgrade timestamp
		MantleEd25519Time:     nil,
	}
	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleSepoliaChainId,
		BaseFeeTime:           u64Ptr(1_704_891_600),
		BVMETHMintUpgradeTime: u64Ptr(1_720_594_800),
		MetaTxV2UpgradeTime:   u64Ptr(1_720_594_800),
		MetaTxV3UpgradeTime:   u64Ptr(1_720_594_800),
		MantleEd25519Time:     nil,
	}
	MantleSepoliaQA3UpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleSepoliaQA3ChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(1_717_689_600),
		MantleEd25519Time:     nil,
	}
	MantleSepoliaQA9UpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleSepoliaQA9ChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(1_716_962_400),
		MantleEd25519Time:     nil,
	}
	MantleLocalUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               params.MantleLocalChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(0),
		MantleEd25519Time:     u64Ptr(0),
	}
	MantleDefaultUpgradeConfig = MantleUpgradeChainConfig{
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(0),
		MantleEd25519Time:     u64Ptr(0),
	}
)

type MantleUpgradeChainConfig struct {
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection

	BaseFeeTime           *uint64 `json:"baseFeeTime"`           // Mantle BaseFee switch time (nil = no fork, 0 = already on mantle baseFee)
	BVMETHMintUpgradeTime *uint64 `json:"bvmETHMintUpgradeTime"` // BVM_ETH mint upgrade switch time (nil = no fork, 0 = already on)
	MetaTxV2UpgradeTime   *uint64 `json:"metaTxV2UpgradeTime"`   // MetaTxV1UpgradeBlock identifies the current block height is using metaTx with MetaTxSignDataV2
	MetaTxV3UpgradeTime   *uint64 `json:"metaTxV3UpgradeTime"`   // MetaTxV3UpgradeBlock identifies the current block height is ensuring sponsor and sender are not the same
	MantleEd25519Time     *uint64 `json:"mantleEd25519Time"`     // MantleEd25519Time
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
	case params.MantleSepoliaQA3ChainId.Int64():
		return &MantleSepoliaQA3UpgradeConfig
	case params.MantleSepoliaQA9ChainId.Int64():
		return &MantleSepoliaQA9UpgradeConfig
	case params.MantleLocalChainId.Int64():
		return &MantleLocalUpgradeConfig
	default:
		return &MantleDefaultUpgradeConfig
	}
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
