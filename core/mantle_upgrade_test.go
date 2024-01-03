package core

import (
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/params"
)

var (
	NonExistChainID = big.NewInt(-1)
)

func TestGetUpgradeConfigForMantle(t *testing.T) {
	mainnetUpgradeConfig := GetUpgradeConfigForMantle(params.MantleMainnetChainId)
	if *mainnetUpgradeConfig.BaseFeeTime != *MantleMainnetUpgradeConfig.BaseFeeTime {
		t.Errorf("wrong baseFeeTime: got %v, want %v", *mainnetUpgradeConfig.BaseFeeTime, *MantleMainnetUpgradeConfig.BaseFeeTime)
	}

	sepoliaUpgradeConfig := GetUpgradeConfigForMantle(params.MantleSepoliaChainId)
	if *sepoliaUpgradeConfig.BaseFeeTime != *MantleSepoliaUpgradeConfig.BaseFeeTime {
		t.Errorf("wrong baseFeeTime: got %v, want %v", *sepoliaUpgradeConfig.BaseFeeTime, *MantleSepoliaUpgradeConfig.BaseFeeTime)
	}

	upgradeConfig := GetUpgradeConfigForMantle(NonExistChainID)
	if upgradeConfig != nil {
		t.Errorf("upgradeConfig should be nil, upgradeConfig: got %v, want %v", upgradeConfig, nil)
	}
}
