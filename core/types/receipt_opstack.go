package types

import (
	"fmt"

	"github.com/ethereum/go-ethereum/params"
)

// deriveOPStackFields derives the OP Stack specific fields for each receipt.
// It must only be called for blocks with at least one transaction (the L1 attributes deposit).
func (rs Receipts) deriveOPStackFields(config *params.ChainConfig, blockTime uint64, txs []*Transaction) error {
	// Exit early if there are only deposit transactions, for which no fields are derived.
	if txs[len(txs)-1].IsDepositTx() {
		return nil
	}

	l1AttributesData := txs[0].Data()

	var daFootprintGasScalar uint64
	isArsia := config.IsMantleArsia(blockTime)
	if isArsia {
		scalar, err := ExtractDAFootprintGasScalar(l1AttributesData)
		if err != nil {
			return fmt.Errorf("failed to extract DA footprint gas scalar: %w", err)
		}
		daFootprintGasScalar = uint64(scalar)
	}

	for i := range rs {
		if txs[i].IsDepositTx() {
			continue
		}
		rcd := txs[i].RollupCostData()
		if isArsia {
			rs[i].DAFootprintGasScalar = &daFootprintGasScalar
			rs[i].BlobGasUsed = daFootprintGasScalar * rcd.EstimatedDASize().Uint64()
		}
	}
	return nil
}
