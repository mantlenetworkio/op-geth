package preconf

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var DefaultTxPoolConfig = TxPoolConfig{
	FromPreconfs:   make([]common.Address, 0),
	ToPreconfs:     make([]common.Address, 0),
	AllPreconfs:    false,
	PreconfTimeout: 1 * time.Second,
}

type TxPoolConfig struct {
	FromPreconfs   []common.Address // Addresses that should be treated by default as preconfs
	ToPreconfs     []common.Address // Addresses that should be treated by default as preconfs
	AllPreconfs    bool             // Whether pre transaction handling should be always enabled
	PreconfTimeout time.Duration    // Timeout for preconf requests
}

func (c *TxPoolConfig) IsPreconfTx(from, to *common.Address) bool {
	if from == nil || to == nil {
		return false
	}

	// If AllPreconfs is true, all transactions are considered preconf
	if c.AllPreconfs {
		return true
	}

	// Check if from is in FromPreconfs
	for _, preconfFrom := range c.FromPreconfs {
		if preconfFrom == *from {
			// If from matches, check if to is in ToPreconfs
			for _, preconfTo := range c.ToPreconfs {
				if preconfTo == *to {
					return true
				}
			}
			return false // from matches but to does not match
		}
	}
	return false // from does not match
}
