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

package core

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

const (
	PreconfStatusSuccess = "success"
	PreconfStatusFailed  = "failed"
)

// NewPreconfTxsEvent is posted when a preconf transaction enters the transaction pool.
type NewPreconfTxEvent struct {
	TxHash                 common.Hash    `json:"txHash"`
	Status                 string         `json:"status"`      // "success" | "failed"
	Reason                 string         `json:"reason"`      // "optional failure message"
	PredictedL2BlockNumber hexutil.Uint64 `json:"blockHeight"` // "predicted L2 block number"
}

// NewPreconfTxRequestEvent is posted when a preconf transaction request enters the transaction pool.
type NewPreconfTxRequest struct {
	Tx                   *types.Transaction
	PreconfResult        chan<- *PreconfResponse
	ClosePreconfResultFn func()
}

type PreconfResponse struct {
	Receipt *types.Receipt
	Err     error
}

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*types.Transaction }

// RemovedLogsEvent is posted when a reorg happens
type RemovedLogsEvent struct{ Logs []*types.Log }

type ChainEvent struct {
	Header *types.Header
}

type ChainHeadEvent struct {
	Header *types.Header
}
