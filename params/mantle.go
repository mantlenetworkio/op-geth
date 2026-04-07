package params

import (
	"math/big"
)

// Mantle chain config
var (
	MantleMainnetChainId = big.NewInt(5000)
	MantleSepoliaChainId = big.NewInt(5003)
	MantleLocalChainId   = big.NewInt(1337)
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
		MantleSkadiTime:       u64Ptr(1_756_278_000),
		MantleLimbTime:        u64Ptr(1_768_374_000),
		MantleArsiaTime:       u64Ptr(1_776_841_200),
	}
	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:               MantleSepoliaChainId,
		BaseFeeTime:           u64Ptr(1_704_891_600),
		BVMETHMintUpgradeTime: u64Ptr(1_720_594_800),
		MetaTxV2UpgradeTime:   u64Ptr(1_720_594_800),
		MetaTxV3UpgradeTime:   u64Ptr(1_720_594_800),
		ProxyOwnerUpgradeTime: nil,
		MantleEverestTime:     u64Ptr(1_737_010_800),
		MantleSkadiTime:       u64Ptr(1_752_649_200),
		MantleLimbTime:        u64Ptr(1_764_745_200),
		MantleArsiaTime:       u64Ptr(1_774_422_000),
	}

	// Starting from arsia, hardcoding upgrade timestamps of qa/dev/local networks are deprecated.
	// You should specify them either in deploy config json (will beactivated in genesis) or
	// through CLI override flags (will be activated in future).
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
		MantleArsiaTime:       u64Ptr(0),
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
		MantleArsiaTime:       u64Ptr(0),
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
	MantleLimbTime        *uint64 `json:"mantleLimbTime"`        // MantleLimbTime identifies the current block time is ensuring osaka upgrade
	MantleArsiaTime       *uint64 `json:"mantleArsiaTime"`       // MantleArsiaTime identifies the current block time is ensuring arsia upgrade
}

// GetUpgradeConfigForMantle returns a mantle upgrade config for Mainnet and Sepolia.
func GetUpgradeConfigForMantle(chainID *big.Int) *MantleUpgradeChainConfig {
	if chainID == nil {
		return nil
	}
	switch chainID.Int64() {
	case MantleMainnetChainId.Int64():
		return &MantleMainnetUpgradeConfig
	case MantleSepoliaChainId.Int64():
		return &MantleSepoliaUpgradeConfig
	default:
		return nil
	}
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
