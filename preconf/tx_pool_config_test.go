package preconf

import (
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

func Test_IsPreconfTx(t *testing.T) {
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")

	tests := []struct {
		name   string
		config TxPoolConfig
		from   *common.Address
		to     *common.Address
		want   bool
	}{
		{
			name: "Nil from or to",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1},
				ToPreconfs:   []common.Address{addr2},
			},
			from: nil,
			to:   &addr2,
			want: false,
		},
		{
			name: "AllPreconfs true",
			config: TxPoolConfig{
				AllPreconfs: true,
			},
			from: &addr1,
			to:   &addr2,
			want: true,
		},
		{
			name: "Matching from and to",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1},
				ToPreconfs:   []common.Address{addr2},
			},
			from: &addr1,
			to:   &addr2,
			want: true,
		},
		{
			name: "Matching from but not to",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1},
				ToPreconfs:   []common.Address{addr2},
			},
			from: &addr1,
			to:   &addr3,
			want: false,
		},
		{
			name: "Non-matching from",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1},
				ToPreconfs:   []common.Address{addr2},
			},
			from: &addr3,
			to:   &addr2,
			want: false,
		},
		{
			name: "Empty preconf lists",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{},
				ToPreconfs:   []common.Address{},
			},
			from: &addr1,
			to:   &addr2,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &tt.config
			got := c.IsPreconfTx(tt.from, tt.to)
			if got != tt.want {
				t.Errorf("IsPreconfTx() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_DefaultConfig(t *testing.T) {
	config := TxPoolConfig{
		FromPreconfs:   make([]common.Address, 0),
		ToPreconfs:     make([]common.Address, 0),
		AllPreconfs:    false,
		PreconfTimeout: 1 * time.Second,
	}

	addr := common.HexToAddress("0x1111111111111111111111111111111111111111")
	if config.IsPreconfTx(&addr, &addr) {
		t.Errorf("Default config should not mark any tx as preconf")
	}
}

func Test_IsPreconfTxFrom(t *testing.T) {
	addr1 := common.HexToAddress("0x1111111111111111111111111111111111111111")
	addr2 := common.HexToAddress("0x2222222222222222222222222222222222222222")
	addr3 := common.HexToAddress("0x3333333333333333333333333333333333333333")

	tests := []struct {
		name   string
		config TxPoolConfig
		from   common.Address
		want   bool
	}{
		{
			name: "AllPreconfs is true",
			config: TxPoolConfig{
				AllPreconfs: true,
			},
			from: addr1,
			want: true,
		},
		{
			name: "Address is in FromPreconfs list",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1, addr2},
				AllPreconfs:  false,
			},
			from: addr1,
			want: true,
		},
		{
			name: "Address is not in FromPreconfs list",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1, addr2},
				AllPreconfs:  false,
			},
			from: addr3,
			want: false,
		},
		{
			name: "FromPreconfs list is empty",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{},
				AllPreconfs:  false,
			},
			from: addr1,
			want: false,
		},
		{
			name: "AllPreconfs is false but address is in the ToPreconfs list",
			config: TxPoolConfig{
				FromPreconfs: []common.Address{addr1, addr2},
				ToPreconfs:   []common.Address{addr3},
				AllPreconfs:  false,
			},
			from: addr3,
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &tt.config
			got := c.IsPreconfTxFrom(tt.from)
			if got != tt.want {
				t.Errorf("IsPreconfTxFrom() = %v, want %v", got, tt.want)
			}
		})
	}
}
