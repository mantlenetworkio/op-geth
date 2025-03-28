package preconf

import (
	"testing"
	"time"
)

func TestMinerConfig_String(t *testing.T) {
	config := MinerConfig{
		OptimismNodeHTTP: "http://test-optimism:8545",
		L1RPCHTTP:        "http://test-l1:8545",
		L1DepositAddress: "0x1234567890abcdef1234567890abcdef12345678",
		ToleranceBlock:   5,
	}

	expected := "EnablePreconfChecker: false, OptimismNodeHTTP: http://test-optimism:8545, L1RPCHTTP: http://test-l1:8545, L1DepositAddress: 0x1234567890abcdef1234567890abcdef12345678, ToleranceBlock: 5, MantleToleranceDuration: 10s, EthToleranceDuration: 2m0s, EthToleranceBlock: 8"
	if got := config.String(); got != expected {
		t.Errorf("MinerConfig.String() = %v, want %v", got, expected)
	}
}

func TestMinerConfig_MantleToleranceDuration(t *testing.T) {
	tests := []struct {
		name           string
		toleranceBlock int64
		want           time.Duration
	}{
		{"Zero tolerance", 0, 0 * time.Second},
		{"Default tolerance", 3, 6 * time.Second},
		{"Custom tolerance", 10, 20 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MinerConfig{
				ToleranceBlock: tt.toleranceBlock,
			}
			if got := config.MantleToleranceDuration(); got != tt.want {
				t.Errorf("MinerConfig.MantleToleranceDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMinerConfig_EthToleranceDuration(t *testing.T) {
	tests := []struct {
		name           string
		toleranceBlock int64
		want           time.Duration
	}{
		{"Zero tolerance", 0, 60 * time.Second},     // (0+3+2)*12
		{"Default tolerance", 3, 96 * time.Second},  // (3+3+2)*12
		{"Custom tolerance", 10, 180 * time.Second}, // (10+3+2)*12
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MinerConfig{
				ToleranceBlock: tt.toleranceBlock,
			}
			if got := config.EthToleranceDuration(); got != tt.want {
				t.Errorf("MinerConfig.EthToleranceDuration() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestDefaultMinerConfig(t *testing.T) {
	// Verify default configuration values
	if DefaultMinerConfig.OptimismNodeHTTP != "http://localhost:7545" {
		t.Errorf("DefaultMinerConfig.OptimismNodeHTTP = %v, want %v",
			DefaultMinerConfig.OptimismNodeHTTP, "http://localhost:7545")
	}

	if DefaultMinerConfig.L1RPCHTTP != "http://localhost:8545" {
		t.Errorf("DefaultMinerConfig.L1RPCHTTP = %v, want %v",
			DefaultMinerConfig.L1RPCHTTP, "http://localhost:8545")
	}

	if DefaultMinerConfig.L1DepositAddress != "0xa513E6E4b8f2a923D98304ec87F64353C4D5C853" {
		t.Errorf("DefaultMinerConfig.L1DepositAddress = %v, want %v",
			DefaultMinerConfig.L1DepositAddress, "0xa513E6E4b8f2a923D98304ec87F64353C4D5C853")
	}

	if DefaultMinerConfig.ToleranceBlock != 3 {
		t.Errorf("DefaultMinerConfig.ToleranceBlock = %v, want %v",
			DefaultMinerConfig.ToleranceBlock, 3)
	}
}

func TestMinerConfig_EthToleranceBlock(t *testing.T) {
	tests := []struct {
		name           string
		toleranceBlock int64
		want           uint64
	}{
		{"Zero tolerance", 0, 3},      // 0+3
		{"Default tolerance", 3, 6},   // 3+3
		{"Custom tolerance", 10, 13},  // 10+3
		{"Negative tolerance", -1, 2}, // -1+3
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := MinerConfig{
				ToleranceBlock: tt.toleranceBlock,
			}
			if got := config.EthToleranceBlock(); got != tt.want {
				t.Errorf("MinerConfig.EthToleranceBlock() = %v, want %v", got, tt.want)
			}
		})
	}
}
