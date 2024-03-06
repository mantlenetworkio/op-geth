package types

import (
	"math/rand"
	"testing"

	"github.com/ethereum/go-ethereum/params"
	"github.com/stretchr/testify/require"
)

func TestRollupGasData(t *testing.T) {
	for i := 0; i < 100; i++ {
		zeroes := rand.Uint64()
		ones := rand.Uint64()

		r := RollupGasData{
			Zeroes:  zeroes,
			NonZero: ones,
		}
		time := uint64(1)
		cfg := &params.ChainConfig{
			RegolithTime: &time,
		}
		gasPreRegolith := r.DataGas(0, cfg)
		gasPostRegolith := r.DataGas(1, cfg)

		require.Equal(t, r.Zeroes*params.TxDataZeroGas+(r.NonZero+BeforeRegolithUpdateNonZeroSize)*params.TxDataNonZeroGasEIP2028, gasPreRegolith)
		require.Equal(t, r.Zeroes*params.TxDataZeroGas+r.NonZero*params.TxDataNonZeroGasEIP2028, gasPostRegolith)
	}
}
