// Copyright 2014 The go-ethereum Authors
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

package txpool

import (
	"errors"
)

var (
	// ErrAlreadyKnown is returned if the transactions is already contained
	// within the pool.
	ErrAlreadyKnown = errors.New("already known")

	// ErrInvalidSender is returned if the transaction contains an invalid signature.
	ErrInvalidSender = errors.New("invalid sender")

	// ErrUnderpriced is returned if a transaction's gas price is too low to be
	// included in the pool. If the gas price is lower than the minimum configured
	// one for the transaction pool, use ErrTxGasPriceTooLow instead.
	ErrUnderpriced = errors.New("transaction underpriced")

	// ErrReplaceUnderpriced is returned if a transaction is attempted to be replaced
	// with a different one without the required price bump.
	ErrReplaceUnderpriced = errors.New("replacement transaction underpriced")

	// ErrTxGasPriceTooLow is returned if a transaction's gas price is below the
	// minimum configured for the transaction pool.
	ErrTxGasPriceTooLow = errors.New("transaction gas price below minimum")

	// ErrAccountLimitExceeded is returned if a transaction would exceed the number
	// allowed by a pool for a single account.
	ErrAccountLimitExceeded = errors.New("account limit exceeded")

	// ErrGasLimit is returned if a transaction's requested gas limit exceeds the
	// maximum allowance of the current block.
	ErrGasLimit = errors.New("exceeds block gas limit")

	// ErrNegativeValue is a sanity error to ensure no one is able to specify a
	// transaction with a negative value.
	ErrNegativeValue = errors.New("negative value")

	// ErrOversizedData is returned if the input data of a transaction is greater
	// than some meaningful limit a user might use. This is not a consensus error
	// making the transaction invalid, rather a DOS protection.
	ErrOversizedData = errors.New("oversized data")

	// ErrAlreadyReserved is returned if the sender address has a pending transaction
	// in a different subpool. For example, this error is returned in response to any
	// input transaction of non-blob type when a blob transaction from this sender
	// remains pending (and vice-versa).
	ErrAlreadyReserved = errors.New("address already reserved")

	// ErrInflightTxLimitReached is returned when the maximum number of in-flight
	// transactions is reached for specific accounts.
	ErrInflightTxLimitReached = errors.New("in-flight transaction limit reached for delegated accounts")

	// ErrPreconfInProcess is returned if a transaction is in process as an preconf transaction
	ErrPreconfInProcess = errors.New("exist preconf transaction in process")
)
