package miner

import (
	"errors"
	"time"

	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/log"
)

func (miner *Miner) preconfLoop() {
	defer miner.preconfTxRequestSub.Unsubscribe()
	for {
		select {
		case ev := <-miner.preconfTxRequestCh:
			now := time.Now()
			log.Debug("worker received preconf tx request", "tx", ev.Tx.Hash())

			status := ev.GetStatus()
			if status == core.PreconfStatusTimeout {
				log.Warn("preconf tx request timeout", "tx", ev.Tx.Hash())
				ev.ClosePreconfResultFn()
				continue
			}

			receipt, err := miner.preconfChecker.Preconf(ev.Tx)
			if err != nil {
				// Not fatal, just trace to the log
				log.Trace("preconf failed", "tx", ev.Tx.Hash(), "err", err)
				if errors.Is(err, ErrPreconfNotAvailable) {
					log.Warn("preconf is temporary not available, tx will be handled as timeout in txpool", "tx", ev.Tx.Hash())
					continue
				}
			}
			log.Trace("worker preconf tx executed", "tx", ev.Tx.Hash(), "duration", time.Since(now))

			// set preconf status before txpool receive response, avoid successful txs not included in block
			if err == nil && receipt != nil && receipt.Status == types.ReceiptStatusSuccessful {
				status = ev.SetStatus(core.PreconfStatusWaiting, core.PreconfStatusSuccess)
			} else {
				status = ev.SetStatus(core.PreconfStatusWaiting, core.PreconfStatusFailed)
			}

			miner.txpool.SetPreconfTxStatus(ev.Tx.Hash(), status)

			if status == core.PreconfStatusTimeout {
				err := miner.preconfChecker.RevertTx(ev.Tx.Hash())
				log.Warn("preconf tx request timeout after preconf executed", "tx", ev.Tx.Hash(), "revert err", err)
				ev.ClosePreconfResultFn()
				continue
			}

			select {
			case ev.PreconfResult <- &core.PreconfResponse{Receipt: receipt, Err: err}:
				log.Debug("worker sent preconf tx response", "tx", ev.Tx.Hash(), "duration", time.Since(now))
			case <-time.After(time.Second):
				log.Warn("preconf tx response timeout, preconf result is closed?", "tx", ev.Tx.Hash())
			}
			ev.ClosePreconfResultFn()

		case <-miner.preconfTxRequestSub.Err():
			return
		}
	}
}

func (miner *Miner) IsPreconfStatusOk() bool {
	return miner.preconfChecker.PrecheckStatus() == nil
}
