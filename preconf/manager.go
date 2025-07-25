package preconf

import (
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/event"
	"github.com/ethereum/go-ethereum/log"
	"sync"
)

type PreConfManager struct {
	config        *TxPoolConfig
	readyCh       chan struct{} // Channel to signal preconf is ready
	readyOnce     sync.Once     // Ensures readyCh is closed only once
	txRequestFeed event.Feed    // Feed for preconf transaction requests
	txFeed        event.Feed    // Feed for preconf transaction events
	txs           *FIFOTxSet    // Set of preconf transactions
	mohoTxs       *MohoTxSet
}

func NewPreConfManager(config *TxPoolConfig) *PreConfManager {
	return &PreConfManager{
		config:  config,
		readyCh: make(chan struct{}),
		txs:     NewFIFOTxSet(),
		mohoTxs: NewMohoTxSet(),
	}
}

func (pm *PreConfManager) Ready() {
	pm.readyOnce.Do(func() {
		close(pm.readyCh)
		log.Info("preconf manager ready")
	})
}

func (pm *PreConfManager) IsReady() bool {
	select {
	case <-pm.readyCh:
		return true
	default:
		return false
	}
}

func (pm *PreConfManager) AddMohoTxs(address common.Address, hashes []common.Hash) {
	pm.mohoTxs.AddBatch(address, hashes)
}

func (pm *PreConfManager) RemoveMohoTxs(address common.Address, hashes []common.Hash) {
	for _, hash := range hashes {
		pm.mohoTxs.Remove(address, hash)
	}
}

func (pm *PreConfManager) IsPreConfTransaction(hash *common.Hash, from, to *common.Address, fromOnly bool) bool {
	if fromOnly {
		if pm.config.isPreconfTxFrom(from) {
			return true
		}
		return pm.mohoTxs.ContainsFrom(*from)
	} else {
		if pm.config.isPreconfTx(from, to) {
			return true
		} else {
			if hash != nil {
				return pm.mohoTxs.Contains(*from, *hash)
			} else {
				return false
			}
		}
	}
}

func (pm *PreConfManager) RemoveTxsAndMohoTxs(address common.Address, hash common.Hash) {
	pm.mohoTxs.Remove(address, hash)
	log.Trace("preconf manager remove moho tx", "hash", hash)
	pm.txs.Remove(hash)
	log.Trace("preconf manager remove tx", "hash", hash)
}

func (pm *PreConfManager) RemoveNonceTooLow(address common.Address, nonce uint64) {
	remove := pm.txs.Forward(address, nonce)
	for _, tx := range remove {
		if pm.mohoTxs.Contains(address, tx) {
			pm.mohoTxs.Remove(address, tx)
		}
	}
}

func (pm *PreConfManager) Clear() {
	pm.txs.Clear()
	pm.mohoTxs.Clear()
}

func (pm *PreConfManager) GetStatus(hash common.Hash) *core.PreconfStatus {
	return pm.txs.GetStatus(hash)
}

func (pm *PreConfManager) SetStatus(hash common.Hash, status core.PreconfStatus) int {
	return pm.txs.SetStatus(hash, status)
}

// CleanTimeOut only clear txs set
func (pm *PreConfManager) CleanTimeOut() []*TxEntry {
	return pm.txs.CleanTimeout()
}

func (pm *PreConfManager) SubscribePreConfTxEvent(ch chan<- core.NewPreconfTxEvent) event.Subscription {
	return pm.txFeed.Subscribe(ch)
}

func (pm *PreConfManager) SubscribePreConfRequestEvent(ch chan<- *core.NewPreconfTxRequest) event.Subscription {
	return pm.txRequestFeed.Subscribe(ch)
}

func (pm *PreConfManager) SendPreConfTxEvent(event *core.NewPreconfTxEvent) int {
	return pm.txFeed.Send(event)
}

func (pm *PreConfManager) SendPreConfRequestEvent(event *core.NewPreconfTxRequest) int {
	return pm.txRequestFeed.Send(event)
}

func (pm *PreConfManager) GetTxsEntries() []*TxEntry {
	return pm.txs.TxEntries()
}

func (pm *PreConfManager) AddTx(address common.Address, tx *types.Transaction) {
	pm.txs.Add(address, tx)
}

func (pm *PreConfManager) RemoveTx(hash common.Hash) {
	pm.txs.Remove(hash)
}

func (pm *PreConfManager) AddMohoTx(address common.Address, hash common.Hash) {
	pm.mohoTxs.Add(address, hash)
}

func (pm *PreConfManager) ContainsTx(hash common.Hash) bool {
	return pm.txs.Contains(hash)
}
