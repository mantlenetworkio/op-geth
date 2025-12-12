package core

import "errors"

// OP-Stack errors.
var (
	// ErrTxFilteredOut indicates an ingress filter has rejected the transaction from
	// being included in the pool.
	ErrTxFilteredOut = errors.New("transaction filtered out")
)
