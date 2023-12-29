package core

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

var (
	MantleSepoliaUpgradeConfig = MantleUpgradeChainConfig{
		ChainID:                 params.MantleSepoliaChainId,
		MantleBaseFeeForSepolia: big.NewInt(473_611),
	}
)

type MantleUpgradeChainConfig struct {
	ChainID                 *big.Int `json:"chainId"`            // chainId identifies the current chain and is used for replay protection
	MantleBaseFeeForSepolia *big.Int `json:"mantleBaseFeeBlock"` // Mantle BaseFee switch block (nil = no fork, 0 = already on mantle baseFee)
}

func getUpgradeConfigForMantle(chainID *big.Int) *MantleUpgradeChainConfig {
	switch chainID {
	case params.MantleSepoliaChainId:
		return &MantleSepoliaUpgradeConfig
	default:
		return nil
	}
}
