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
	"sync"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
)

type PreconfStatus string

const (
	PreconfStatusSuccess PreconfStatus = "success"
	PreconfStatusFailed  PreconfStatus = "failed"
	PreconfStatusTimeout PreconfStatus = "timeout"
	PreconfStatusWaiting PreconfStatus = "waiting"
)

type PreconfTxReceipt struct {
	Logs []*types.Log `json:"logs"`
}

// NewPreconfTxsEvent is posted when a preconf transaction enters the transaction pool.
type NewPreconfTxEvent struct {
	TxHash                 common.Hash      `json:"txHash"`
	Status                 PreconfStatus    `json:"status"`
	Reason                 string           `json:"reason"`      // "optional failure message"
	PredictedL2BlockNumber hexutil.Uint64   `json:"blockHeight"` // "predicted L2 block number"
	Receipt                PreconfTxReceipt `json:"receipt"`
}

// NewPreconfTxRequestEvent is posted when a preconf transaction request enters the transaction pool.
type NewPreconfTxRequest struct {
	Tx                   *types.Transaction
	mu                   sync.Mutex
	Status               PreconfStatus
	PreconfResult        chan<- *PreconfResponse
	ClosePreconfResultFn func()
}

func (e *NewPreconfTxRequest) GetStatus() PreconfStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.Status
}

func (e *NewPreconfTxRequest) SetStatus(statusBefore PreconfStatus, status PreconfStatus) PreconfStatus {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.Status == statusBefore {
		e.Status = status
	}
	return e.Status
}

type PreconfResponse struct {
	Receipt *types.Receipt
	Err     error
}

// NewTxsEvent is posted when a batch of transactions enter the transaction pool.
type NewTxsEvent struct{ Txs []*types.Transaction }

// NewMinedBlockEvent is posted when a block has been imported.
type NewMinedBlockEvent struct{ Block *types.Block }

// RemovedLogsEvent is posted when a reorg happens
type RemovedLogsEvent struct{ Logs []*types.Log }

type ChainEvent struct {
	Block *types.Block
	Hash  common.Hash
	Logs  []*types.Log
}

type ChainSideEvent struct {
	Block *types.Block
}

type ChainHeadEvent struct{ Block *types.Block }
