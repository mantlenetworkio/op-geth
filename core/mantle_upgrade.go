package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

var (
	MantleMainnetUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:     params.MantleMainnetChainId,
		BaseFeeTime: u64Ptr(0),
	}

	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:     params.MantleSepoliaChainId,
		BaseFeeTime: u64Ptr(1_703_759_533),
	}
)

type MantleUpgradeChainConfig struct {
	ChainID     *big.Int `json:"chainId"`     // chainId identifies the current chain and is used for replay protection
	BaseFeeTime *uint64  `json:"BaseFeeTime"` // Mantle BaseFee switch time (nil = no fork, 0 = already on mantle baseFee)
}

func GetUpgradeConfigForMantle(chainID *big.Int) *MantleUpgradeChainConfig {
	switch chainID {
	case params.MantleMainnetChainId:
		return &MantleMainnetUpgradeConfig
	case params.MantleSepoliaChainId:
		return &MantleSepoliaUpgradeConfig
	default:
		return nil
	}
}

func u64Ptr(v uint64) *uint64 {
	return &v
}
