package params

import (
	"math/big"
)

// Mantle chain config
var (
	MantleMainnetChainId  = big.NewInt(5000)
	MantleSepoliaChainId  = big.NewInt(5003)
	MantleHoodiQA3ChainId = big.NewInt(561113)
	MantleLocalChainId    = big.NewInt(1337)
)

var (
	MantleMainnetUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               MantleMainnetChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(1_742_367_600),
		ProxyOwnerUpgradeTime: u64Ptr(1_742_367_600),
		MantleEverestTime:     u64Ptr(1_742_367_600),
		MantleSkadiTime:       nil,
		MantleLimbTime:        nil,
	}
	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               MantleSepoliaChainId,
		BaseFeeTime:           u64Ptr(1_704_891_600),
		BVMETHMintUpgradeTime: u64Ptr(1_720_594_800),
		MetaTxV2UpgradeTime:   u64Ptr(1_720_594_800),
		MetaTxV3UpgradeTime:   u64Ptr(1_720_594_800),
		ProxyOwnerUpgradeTime: nil,
		MantleEverestTime:     u64Ptr(1_737_010_800),
		MantleSkadiTime:       nil,
		MantleLimbTime:        nil,
	}
	MantleHoodiQA3UpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               MantleHoodiQA3ChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(0),
		ProxyOwnerUpgradeTime: nil,
		MantleEverestTime:     u64Ptr(0),
		MantleSkadiTime:       u64Ptr(1_749_625_200),
		MantleLimbTime:        nil,
	}
	MantleLocalUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               MantleLocalChainId,
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(0),
		ProxyOwnerUpgradeTime: nil,
		MantleEverestTime:     u64Ptr(0),
		MantleSkadiTime:       u64Ptr(0),
		MantleLimbTime:        u64Ptr(0),
	}
	MantleDefaultUpgradeConfig = MantleUpgradeChainConfig{
		BaseFeeTime:           u64Ptr(0),
		BVMETHMintUpgradeTime: u64Ptr(0),
		MetaTxV2UpgradeTime:   u64Ptr(0),
		MetaTxV3UpgradeTime:   u64Ptr(0),
		ProxyOwnerUpgradeTime: nil,
		MantleEverestTime:     u64Ptr(0),
		MantleSkadiTime:       u64Ptr(0),
		MantleLimbTime:        u64Ptr(0),
	}
)

type MantleUpgradeChainConfig struct {
	ChainID *big.Int `json:"chainId"` // chainId identifies the current chain and is used for replay protection

	BaseFeeTime           *uint64 `json:"baseFeeTime"`           // Mantle BaseFee switch time (nil = no fork, 0 = already on mantle baseFee)
	BVMETHMintUpgradeTime *uint64 `json:"bvmETHMintUpgradeTime"` // BVM_ETH mint upgrade switch time (nil = no fork, 0 = already on)
	MetaTxV2UpgradeTime   *uint64 `json:"metaTxV2UpgradeTime"`   // MetaTxV1UpgradeBlock identifies the current block height is using metaTx with MetaTxSignDataV2
	MetaTxV3UpgradeTime   *uint64 `json:"metaTxV3UpgradeTime"`   // MetaTxV3UpgradeBlock identifies the current block height is ensuring sponsor and sender are not the same
	ProxyOwnerUpgradeTime *uint64 `json:"proxyOwnerUpgradeTime"` // ProxyOwnerUpgradeBlock identifies the current block time is ensuring the L2ProxyAdmin contract owner is set to NewProxyAdminOwnerAddress
	MantleEverestTime     *uint64 `json:"mantleEverestTime"`     // MantleEverestTime identifies the current block time is ensuring eip-7212 & disable MetaTx
	MantleSkadiTime       *uint64 `json:"mantleSkadiTime"`       // MantleSkadiTime identifies the current block time is ensuring prague upgrade
	MantleLimbTime        *uint64 `json:"mantleOperatorFeeTime"` // MantleOperatorFeeTime identifies the current block time is using operator fee
}

func GetUpgradeConfigForMantle(chainID *big.Int) *MantleUpgradeChainConfig {
	if chainID == nil {
		return nil
	}
	switch chainID.Int64() {
	case MantleMainnetChainId.Int64():
		return &MantleMainnetUpgradeConfig
	case MantleSepoliaChainId.Int64():
		return &MantleSepoliaUpgradeConfig
	case MantleHoodiQA3ChainId.Int64():
		return &MantleHoodiQA3UpgradeConfig
	case MantleLocalChainId.Int64():
		return &MantleLocalUpgradeConfig
	default:
		return &MantleDefaultUpgradeConfig
	}
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
