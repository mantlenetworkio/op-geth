package core

import (
	"math/big"
	"testing"
)

var (
	MantleMainnetChainId = big.NewInt(5000)
	MantleSepoliaChainId = big.NewInt(5003)
	OtherChainID         = big.NewInt(1_000_000)
)

func TestGetUpgradeConfigForMantle(t *testing.T) {
	mainnetUpgradeConfig := GetUpgradeConfigForMantle(MantleMainnetChainId)
	if *mainnetUpgradeConfig.BaseFeeTime != *MantleMainnetUpgradeConfig.BaseFeeTime {
		t.Errorf("wrong baseFeeTime: got %v, want %v", *mainnetUpgradeConfig.BaseFeeTime, *MantleMainnetUpgradeConfig.BaseFeeTime)
	}

	sepoliaUpgradeConfig := GetUpgradeConfigForMantle(MantleSepoliaChainId)
	if *sepoliaUpgradeConfig.BaseFeeTime != *MantleSepoliaUpgradeConfig.BaseFeeTime {
		t.Errorf("wrong baseFeeTime: got %v, want %v", *sepoliaUpgradeConfig.BaseFeeTime, *MantleSepoliaUpgradeConfig.BaseFeeTime)
	}

	upgradeConfig := GetUpgradeConfigForMantle(nil)
	if upgradeConfig != nil {
		t.Errorf("upgradeConfig should be nil, upgradeConfig: got %v, want %v", upgradeConfig, nil)
	}

	defaultUpgradeConfig := GetUpgradeConfigForMantle(OtherChainID)
	if *defaultUpgradeConfig.BaseFeeTime != *MantleDefaultUpgradeConfig.BaseFeeTime {
		t.Errorf("upgradeConfig should be nil, upgradeConfig: got %v, want %v", upgradeConfig, nil)
	}
}
