package preconf

import (
	"time"

	"github.com/ethereum/go-ethereum/common"
)

var DefaultConfig = Config{
	Preconfs:       []common.Address{common.HexToAddress("0xa4e97dFd56E0E30A2542d666Ef04ACC102310083")},
	NoPreconfs:     false,
	PreconfTimeout: 1 * time.Second,
}

// core/txpool/txpool.go
type Config struct {
	Preconfs       []common.Address // Addresses that should be treated by default as preconfs
	NoPreconfs     bool             // Whether pre transaction handling should be disabled
	PreconfTimeout time.Duration    // Timeout for preconf requests
}

func (c *Config) IsPreconf(addr *common.Address) bool {
	if addr == nil {
		return false
	}

	for _, preconf := range c.Preconfs {
		if preconf == *addr {
			return true
		}
	}
	return false
}
