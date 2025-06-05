// contains preconf tx pool related functions
package txpool

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
)

type preconfTxPool interface {
	// SubscribeNewPreconfTxEvent subscribes to new preconf transaction events.
	SubscribeNewPreconfTxEvent(ch chan<- core.NewPreconfTxEvent) event.Subscription

	// SubscribeNewPreconfTxRequestEvent subscribes to new preconf transaction request events.
	SubscribeNewPreconfTxRequestEvent(ch chan<- *core.NewPreconfTxRequest) event.Subscription

	// PendingPreconfTxs returns all currently processable preconf and pending transactions, grouped by origin
	// account and sorted by nonce.
	PendingPreconfTxs(filter PendingFilter) ([]*types.Transaction, map[common.Address][]*LazyTransaction)

	// PreconfReady closes the preconfReadyCh channel to notify the miner that preconf is ready
	// This is called every time a worker is ready with an env, but it only closes once, so we need to use sync.Once to ensure it only closes once
	PreconfReady()

	// SetPreconfTxStatus sets the status of a preconf transaction
	SetPreconfTxStatus(txHash common.Hash, status core.PreconfStatus)
}

// SubscribeNewPreconfTxEvent registers a subscription of NewPreconfTxEvent and
// starts sending event to the given channel.
func (p *TxPool) SubscribeNewPreconfTxEvent(ch chan<- core.NewPreconfTxEvent) event.Subscription {
	subs := make([]event.Subscription, len(p.subpools))
	for i, subpool := range p.subpools {
		subs[i] = subpool.SubscribeNewPreconfTxEvent(ch)
	}
	return p.subs.Track(event.JoinSubscriptions(subs...))
}

// SubscribeNewPreconfTxRequest registers a subscription of NewPreconfTxRequest and
// starts sending event to the given channel.
func (p *TxPool) SubscribeNewPreconfTxRequestEvent(ch chan<- *core.NewPreconfTxRequest) event.Subscription {
	subs := make([]event.Subscription, len(p.subpools))
	for i, subpool := range p.subpools {
		subs[i] = subpool.SubscribeNewPreconfTxRequestEvent(ch)
	}
	return p.subs.Track(event.JoinSubscriptions(subs...))
}

func (p *TxPool) PendingPreconfTxs(filter PendingFilter) ([]*types.Transaction, map[common.Address][]*LazyTransaction) {
	preconfTxs := make([]*types.Transaction, 0)
	pendingTxs := make(map[common.Address][]*LazyTransaction)
	for _, subpool := range p.subpools {
		preconfTxsSub, pendingTxsSub := subpool.PendingPreconfTxs(filter)
		preconfTxs = append(preconfTxs, preconfTxsSub...)
		for addr, txs := range pendingTxsSub {
			pendingTxs[addr] = append(pendingTxs[addr], txs...)
		}
	}
	return preconfTxs, pendingTxs
}

// PreconfReady closes the preconfReadyCh channel to notify the miner that preconf is ready
// This is called every time a worker is ready with an env, but it only closes once, so we need to use sync.Once to ensure it only closes once
func (p *TxPool) PreconfReady() {
	for _, subpool := range p.subpools {
		subpool.PreconfReady()
	}
}

func (p *TxPool) SetPreconfTxStatus(txHash common.Hash, status core.PreconfStatus) {
	for _, subpool := range p.subpools {
		subpool.SetPreconfTxStatus(txHash, status)
	}
}
