// Copyright 2023 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package eip4844

import (
	"errors"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/core/types"
)

var (
	BlobTxBytesPerFieldElement         uint64 = 32      // Size in bytes of a field element
	BlobTxFieldElementsPerBlob         uint64 = 4096    // Number of field elements stored in a single data blob
	BlobTxHashVersion                  uint64 = 0x01    // Version byte of the commitment hash
	BlobTxBlobGasPerBlob               uint64 = 1 << 17 // Gas consumption of a single data blob (== blob byte size)
	BlobTxMinBlobGasprice              uint64 = 1       // Minimum gas price for data blobs
	BlobTxBlobGaspriceUpdateFraction   uint64 = 3338477 // Controls the maximum rate of change for blob gas price
	BlobTxPointEvaluationPrecompileGas uint64 = 50000   // Gas price for the point evaluation precompile.

	BlobTxTargetBlobGasPerBlock = 3 * BlobTxBlobGasPerBlob // Target consumable blob gas for data blobs per block (for 1559-like pricing)
	MaxBlobGasPerBlock          = 6 * BlobTxBlobGasPerBlob // Maximum consumable blob gas for data blobs per block

	minBlobGasPrice            = big.NewInt(int64(BlobTxMinBlobGasprice))
	blobGaspriceUpdateFraction = big.NewInt(int64(BlobTxBlobGaspriceUpdateFraction))
)

// VerifyEIP4844Header verifies the presence of the excessBlobGas field and that
// if the current block contains no transactions, the excessBlobGas is updated
// accordingly.
func VerifyEIP4844Header(parent, header *types.Header) error {
	// Verify the header is not malformed
	if header.ExcessBlobGas == nil {
		return errors.New("header is missing excessBlobGas")
	}
	if header.BlobGasUsed == nil {
		return errors.New("header is missing blobGasUsed")
	}
	// Verify that the blob gas used remains within reasonable limits.
	if *header.BlobGasUsed > MaxBlobGasPerBlock {
		return fmt.Errorf("blob gas used %d exceeds maximum allowance %d", *header.BlobGasUsed, MaxBlobGasPerBlock)
	}
	if *header.BlobGasUsed%BlobTxBlobGasPerBlob != 0 {
		return fmt.Errorf("blob gas used %d not a multiple of blob gas per blob %d", header.BlobGasUsed, BlobTxBlobGasPerBlob)
	}
	// Verify the excessBlobGas is correct based on the parent header
	var (
		parentExcessBlobGas uint64
		parentBlobGasUsed   uint64
	)
	if parent.ExcessBlobGas != nil {
		parentExcessBlobGas = *parent.ExcessBlobGas
		parentBlobGasUsed = *parent.BlobGasUsed
	}
	expectedExcessBlobGas := CalcExcessBlobGas(parentExcessBlobGas, parentBlobGasUsed)
	if *header.ExcessBlobGas != expectedExcessBlobGas {
		return fmt.Errorf("invalid excessBlobGas: have %d, want %d, parent excessBlobGas %d, parent blobDataUsed %d",
			*header.ExcessBlobGas, expectedExcessBlobGas, parentExcessBlobGas, parentBlobGasUsed)
	}
	return nil
}

// CalcExcessBlobGas calculates the excess blob gas after applying the set of
// blobs on top of the excess blob gas.
func CalcExcessBlobGas(parentExcessBlobGas uint64, parentBlobGasUsed uint64) uint64 {
	excessBlobGas := parentExcessBlobGas + parentBlobGasUsed
	if excessBlobGas < BlobTxTargetBlobGasPerBlock {
		return 0
	}
	return excessBlobGas - BlobTxTargetBlobGasPerBlock
}

// CalcBlobFee calculates the blobfee from the header's excess blob gas field.
func CalcBlobFee(excessBlobGas uint64) *big.Int {
	return fakeExponential(minBlobGasPrice, new(big.Int).SetUint64(excessBlobGas), blobGaspriceUpdateFraction)
}

// fakeExponential approximates factor * e ** (numerator / denominator) using
// Taylor expansion.
func fakeExponential(factor, numerator, denominator *big.Int) *big.Int {
	var (
		output = new(big.Int)
		accum  = new(big.Int).Mul(factor, denominator)
	)
	for i := 1; accum.Sign() > 0; i++ {
		output.Add(output, accum)

		accum.Mul(accum, numerator)
		accum.Div(accum, denominator)
		accum.Div(accum, big.NewInt(int64(i)))
	}
	return output.Div(output, denominator)
}
