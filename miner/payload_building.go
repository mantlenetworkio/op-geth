// Copyright 2022 The go-ethereum Authors
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

package miner

import (
	"crypto/sha256"
	"encoding/binary"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/stateless"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/rlp"
)

// BuildPayloadArgs contains the provided parameters for building payload.
// Check engine-api specification for more details.
// https://github.com/ethereum/execution-apis/blob/main/src/engine/cancun.md#payloadattributesv3
type BuildPayloadArgs struct {
	Parent       common.Hash           // The parent block to build payload on top
	Timestamp    uint64                // The provided timestamp of generated payload
	FeeRecipient common.Address        // The provided recipient address for collecting transaction fee
	Random       common.Hash           // The provided randomness value
	Withdrawals  types.Withdrawals     // The provided withdrawals
	BeaconRoot   *common.Hash          // The provided beaconRoot (Cancun)
	Version      engine.PayloadVersion // Versioning byte for payload id calculation.

	NoTxPool     bool                 // Optimism addition: option to disable tx pool contents from being included
	Transactions []*types.Transaction // Optimism addition: txs forced into the block via engine API
	GasLimit     *uint64              // Optimism addition: override gas limit of the block to build
	BaseFee      *big.Int             // Optimism addition: override base fee of the block to build
}

// Id computes an 8-byte identifier by hashing the components of the payload arguments.
func (args *BuildPayloadArgs) Id() engine.PayloadID {
	hasher := sha256.New()
	hasher.Write(args.Parent[:])
	binary.Write(hasher, binary.BigEndian, args.Timestamp)
	hasher.Write(args.Random[:])
	hasher.Write(args.FeeRecipient[:])
	rlp.Encode(hasher, args.Withdrawals)
	if args.BeaconRoot != nil {
		hasher.Write(args.BeaconRoot[:])
	}
	if args.NoTxPool || len(args.Transactions) > 0 { // extend if extra payload attributes are used
		binary.Write(hasher, binary.BigEndian, args.NoTxPool)
		binary.Write(hasher, binary.BigEndian, uint64(len(args.Transactions)))
		for _, tx := range args.Transactions {
			h := tx.Hash()
			hasher.Write(h[:])
		}
	}
	if args.GasLimit != nil {
		binary.Write(hasher, binary.BigEndian, *args.GasLimit)
	}
	var out engine.PayloadID
	copy(out[:], hasher.Sum(nil)[:8])
	out[0] = byte(args.Version)
	return out
}

// Payload wraps the built payload(block waiting for sealing). According to the
// engine-api specification, EL should build the initial version of the payload
// which has an empty transaction set and then keep update it in order to maximize
// the revenue. Therefore, the empty-block here is always available and full-block
// will be set/updated afterwards.
type Payload struct {
	id            engine.PayloadID
	empty         *types.Block
	emptyWitness  *stateless.Witness
	full          *types.Block
	fullWitness   *stateless.Witness
	sidecars      []*types.BlobTxSidecar
	emptyRequests [][]byte
	requests      [][]byte
	fullFees      *big.Int
	stop          chan struct{}
	lock          sync.Mutex
	cond          *sync.Cond
}

// newPayload initializes the payload object.
func newPayload(empty *types.Block, emptyRequests [][]byte, witness *stateless.Witness, id engine.PayloadID) *Payload {
	payload := &Payload{
		id:            id,
		empty:         empty,
		emptyRequests: emptyRequests,
		emptyWitness:  witness,
		stop:          make(chan struct{}),
	}
	log.Info("Starting work on payload", "id", payload.id)
	payload.cond = sync.NewCond(&payload.lock)
	return payload
}

// update updates the full-block with latest built version.
func (payload *Payload) update(r *newPayloadResult, elapsed time.Duration) {
	payload.lock.Lock()
	defer payload.lock.Unlock()

	select {
	case <-payload.stop:
		return // reject stale update
	default:
	}
	// Ensure the newly provided full block has a higher transaction fee.
	// In post-merge stage, there is no uncle reward anymore and transaction
	// fee(apart from the mev revenue) is the only indicator for comparison.
	if payload.full == nil || r.fees.Cmp(payload.fullFees) > 0 {
		payload.full = r.block
		payload.fullFees = r.fees
		payload.sidecars = r.sidecars
		payload.requests = r.requests
		payload.fullWitness = r.witness

		feesInEther := new(big.Float).Quo(new(big.Float).SetInt(r.fees), big.NewFloat(params.Ether))
		log.Info("Updated payload",
			"id", payload.id,
			"number", r.block.NumberU64(),
			"hash", r.block.Hash(),
			"txs", len(r.block.Transactions()),
			"withdrawals", len(r.block.Withdrawals()),
			"gas", r.block.GasUsed(),
			"fees", feesInEther,
			"root", r.block.Root(),
			"elapsed", common.PrettyDuration(elapsed),
		)
	}
	payload.cond.Broadcast() // fire signal for notifying full block
}

// Resolve returns the latest built payload and also terminates the background
// thread for updating payload. It's safe to be called multiple times.
func (payload *Payload) Resolve() *engine.ExecutionPayloadEnvelope {
	payload.lock.Lock()
	defer payload.lock.Unlock()

	select {
	case <-payload.stop:
	default:
		close(payload.stop)
	}
	if payload.full != nil {
		envelope := engine.BlockToExecutableData(payload.full, payload.fullFees, payload.sidecars, payload.requests)
		if payload.fullWitness != nil {
			envelope.Witness = new(hexutil.Bytes)
			*envelope.Witness, _ = rlp.EncodeToBytes(payload.fullWitness) // cannot fail
		}
		return envelope
	}
	envelope := engine.BlockToExecutableData(payload.empty, big.NewInt(0), nil, payload.emptyRequests)
	if payload.emptyWitness != nil {
		envelope.Witness = new(hexutil.Bytes)
		*envelope.Witness, _ = rlp.EncodeToBytes(payload.emptyWitness) // cannot fail
	}
	return envelope
}

// ResolveEmpty is basically identical to Resolve, but it expects empty block only.
// It's only used in tests.
func (payload *Payload) ResolveEmpty() *engine.ExecutionPayloadEnvelope {
	payload.lock.Lock()
	defer payload.lock.Unlock()

	envelope := engine.BlockToExecutableData(payload.empty, big.NewInt(0), nil, payload.emptyRequests)
	if payload.emptyWitness != nil {
		envelope.Witness = new(hexutil.Bytes)
		*envelope.Witness, _ = rlp.EncodeToBytes(payload.emptyWitness) // cannot fail
	}
	return envelope
}

// ResolveFull is basically identical to Resolve, but it expects full block only.
// Don't call Resolve until ResolveFull returns, otherwise it might block forever.
func (payload *Payload) ResolveFull() *engine.ExecutionPayloadEnvelope {
	payload.lock.Lock()
	defer payload.lock.Unlock()

	if payload.full == nil {
		select {
		case <-payload.stop:
			return nil
		default:
		}
		// Wait the full payload construction. Note it might block
		// forever if Resolve is called in the meantime which
		// terminates the background construction process.
		payload.cond.Wait()
	}
	// Terminate the background payload construction
	select {
	case <-payload.stop:
	default:
		close(payload.stop)
	}
	envelope := engine.BlockToExecutableData(payload.full, payload.fullFees, payload.sidecars, payload.requests)
	if payload.fullWitness != nil {
		envelope.Witness = new(hexutil.Bytes)
		*envelope.Witness, _ = rlp.EncodeToBytes(payload.fullWitness) // cannot fail
	}
	return envelope
}

// buildPayload builds the payload according to the provided parameters.
func (miner *Miner) buildPayload(args *BuildPayloadArgs, witness bool) (*Payload, error) {
	// Build the initial version with no transaction included. It should be fast
	// enough to run. The empty payload can at least make sure there is something
	// to deliver for not missing slot.
	emptyParams := &generateParams{
		timestamp:   args.Timestamp,
		forceTime:   true,
		parentHash:  args.Parent,
		coinbase:    args.FeeRecipient,
		random:      args.Random,
		withdrawals: args.Withdrawals,
		beaconRoot:  args.BeaconRoot,
		noTxs:       true,
		txs:         args.Transactions,
		gasLimit:    args.GasLimit,
		baseFee:     args.BaseFee,
	}
	empty := miner.generateWork(emptyParams, witness)
	if empty.err != nil {
		return nil, empty.err
	}
	// Construct a payload object for return.
	payload := newPayload(empty.block, empty.requests, empty.witness, args.Id())

	if args.NoTxPool { // don't start the background payload updating job if there is no tx pool to pull from
		// make sure to make it appear as full, otherwise it will wait indefinitely for payload building to complete.
		payload.full = empty.block
		payload.fullFees = empty.fees
		payload.fullWitness = empty.witness
		payload.requests = empty.requests
		return payload, nil
	}

	fullParams := &generateParams{
		timestamp:   args.Timestamp,
		forceTime:   true,
		parentHash:  args.Parent,
		coinbase:    args.FeeRecipient,
		random:      args.Random,
		withdrawals: args.Withdrawals,
		beaconRoot:  args.BeaconRoot,
		noTxs:       false,
		txs:         args.Transactions,
		gasLimit:    args.GasLimit,
		baseFee:     args.BaseFee,
	}

	// Since we skip building the empty block when using the tx pool, we need to explicitly
	// validate the BuildPayloadArgs here.
	blockTime, err := miner.validateParams(fullParams)
	if err != nil {
		return nil, err
	}

	// Spin up a routine for updating the payload in background. This strategy
	// can maximum the revenue for including transactions with highest fee.
	go func() {
		// Setup the timer for re-building the payload. The initial clock is kept
		// for triggering process immediately.
		timer := time.NewTimer(0)
		defer timer.Stop()

		// Setup the timer for terminating the payload building process as determined
		// by validateParams.
		endTimer := time.NewTimer(blockTime)
		defer endTimer.Stop()

		for {
			select {
			case <-timer.C:
				start := time.Now()
				r := miner.generateWork(fullParams, witness)
				if r.err == nil {
					payload.update(r, time.Since(start))
				} else {
					log.Info("Error while generating work", "id", payload.id, "err", r.err)
				}
				timer.Reset(miner.config.Recommit)
			case <-payload.stop:
				log.Info("Stopping work on payload", "id", payload.id, "reason", "delivery")
				return
			case <-endTimer.C:
				log.Info("Stopping work on payload", "id", payload.id, "reason", "timeout")
				return
			}
		}
	}()
	return payload, nil
}
