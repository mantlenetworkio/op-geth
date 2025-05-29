package miner

import (
	"math/big"
	"sync"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/beacon/engine"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/preconf"
)

func testBuildPayloadWithPreconf(t *testing.T, cfg *preconf.TxPoolConfig, txs []*types.Transaction, expectTxs, expectPreconfTxs int, expectStatus core.PreconfStatus, init bool) time.Duration {
	var (
		db        = rawdb.NewMemoryDatabase()
		recipient = common.HexToAddress("0xdeadbeef")
	)
	testTxPoolConfig.Preconf = cfg
	testTxPoolConfig.NoLocals = true
	w, b := newPreconfTestWorker(t, params.TestChainConfig, ethash.NewFaker(), db, 0)
	defer w.close()

	timestamp := uint64(time.Now().Unix())
	gasLimit := uint64(2_000_000_000_000)
	args := &BuildPayloadArgs{
		Parent:       b.chain.CurrentBlock().Hash(),
		Timestamp:    timestamp,
		Random:       common.Hash{},
		FeeRecipient: recipient,
		BaseFee:      big.NewInt(1e9),
		NoTxPool:     true,
		GasLimit:     &gasLimit,
	}
	w.preconfChecker.UpdateOptimismSyncStatus(&preconf.OptimismSyncStatus{
		CurrentL1:        preconf.L1BlockRef{Number: 1, Time: timestamp},
		HeadL1:           preconf.L1BlockRef{Number: 1, Time: timestamp},
		EngineSyncTarget: preconf.L2BlockRef{Number: 1, Time: timestamp},
		UnsafeL2:         preconf.L2BlockRef{Number: 1, Time: timestamp},
	})

	emptyPayload, err := w.buildPayload(args)
	if err != nil {
		t.Fatalf("Failed to build payload %v", err)
	}

	if init {
		env, err := w.makeEnv(b.chain.CurrentBlock(), emptyPayload.empty.Header(), recipient)
		if err != nil {
			t.Fatalf("Failed to make env %v", err)
		}
		if env.gasPool == nil {
			env.gasPool = new(core.GasPool).AddGas(gasLimit)
		}
		w.preconfChecker.PausePreconf()
		w.preconfChecker.UnpausePreconf(env, w.eth.TxPool().PreconfReady)
	}

	start := time.Now()

	preconfTxCh := make(chan core.NewPreconfTxEvent, 100)
	defer close(preconfTxCh)
	sub := b.txPool.SubscribeNewPreconfTxEvent(preconfTxCh)
	defer sub.Unsubscribe()

	for _, tx := range txs {
		b.txPool.AddLocals([]*types.Transaction{tx})
		time.Sleep(2 * time.Millisecond)
	}

	args.NoTxPool = false
	payload, err := w.buildPayload(args)
	if err != nil {
		t.Fatalf("Failed to build payload %v", err)
	}
	verify := func(outer *engine.ExecutionPayloadEnvelope, txs int, timestamp uint64) {
		payload := outer.ExecutionPayload
		if payload.ParentHash != b.chain.CurrentBlock().Hash() {
			t.Fatal("Unexpect parent hash")
		}
		if payload.Random != (common.Hash{}) {
			t.Fatal("Unexpect random value")
		}
		if payload.Timestamp != timestamp {
			t.Fatal("Unexpect timestamp")
		}
		if payload.FeeRecipient != recipient {
			t.Fatal("Unexpect fee recipient")
		}
		if len(payload.Transactions) != txs {
			t.Fatal("Unexpect transaction set")
		}
	}

	empty := payload.ResolveEmpty()
	verify(empty, 0, timestamp)

	full := payload.ResolveFull()
	verify(full, expectTxs, timestamp)

	preconfTxSet := make(map[common.Hash]struct{}, expectTxs)
	for _, preconfTx := range preconfTxs {
		preconfTxSet[preconfTx.Hash()] = struct{}{}
	}

	receivedPreconfTxs := make([]core.NewPreconfTxEvent, 0, expectPreconfTxs)
	for i := 0; i < expectPreconfTxs; i++ {
		select {
		case ret := <-preconfTxCh:
			if _, ok := preconfTxSet[ret.TxHash]; ok {
				if ret.Status != core.PreconfStatusSuccess && ret.Status != expectStatus {
					t.Fatalf("Expected %v status %s, but got %s", ret.TxHash, expectStatus, ret.Status)
				}
				receivedPreconfTxs = append(receivedPreconfTxs, ret)
			}
		case <-time.After(10 * time.Millisecond):
			t.Fatal("Preconf txs should not be ready")
		}
	}
	if len(receivedPreconfTxs) != expectPreconfTxs {
		t.Fatalf("Expected %d success txs, but got %d", expectPreconfTxs, len(receivedPreconfTxs))
	}

	// Check that the same transactions appear in the same order in both collections
	if expectPreconfTxs > 0 {
		// Build an index map of tx hashes by position in the txs array
		txsOrder := make(map[common.Hash]int)
		for i, tx := range payload.full.Transactions() {
			txHash := tx.Hash()
			if _, exists := preconfTxSet[txHash]; exists {
				txsOrder[txHash] = i
			}
		}

		// Verify preconf txs order
		for i := 0; i < len(receivedPreconfTxs); i++ {
			if txsOrder[receivedPreconfTxs[i].TxHash] != i {
				t.Fatalf("Transaction order mismatch: tx %v came before %v in receivedPreconfTxs, but after in txs",
					receivedPreconfTxs[i].TxHash, receivedPreconfTxs[i+1].TxHash)
			}
		}
	}

	return time.Since(start)
}

func TestPreconfSuccess(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		AllPreconfs:    true,
		PreconfTimeout: 400 * time.Millisecond,
	}
	testBuildPayloadWithPreconf(t, cfg, preconfTxs, len(preconfTxs), len(preconfTxs), core.PreconfStatusSuccess, true)
}

func TestPreconfMixSuccess(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: 400 * time.Millisecond,
	}
	txs := []*types.Transaction{pendingTxs[0], preconfTxs[0], newTxs[0], preconfTxs[1]}
	testBuildPayloadWithPreconf(t, cfg, txs, len(txs), len(preconfTxs), core.PreconfStatusSuccess, true)
}

func TestPreconfNonceTooHighFailed(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: 400 * time.Millisecond,
	}
	txs := []*types.Transaction{pendingTxs[0], preconfTxs[1]}
	testBuildPayloadWithPreconf(t, cfg, txs, 1, 0, core.PreconfStatusFailed, true)
}

func TestPreconfNonceTooLowFailed(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: 400 * time.Millisecond,
	}
	txs := []*types.Transaction{pendingTxs[0], preconfTxs[0], preconfTxs[1], preconfTxs[0]}
	testBuildPayloadWithPreconf(t, cfg, txs, 3, 2, core.PreconfStatusFailed, true)
}

func TestPreconfTimeoutFailed(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: time.Nanosecond,
	}
	testBuildPayloadWithPreconf(t, cfg, preconfTxs, 0, 0, core.PreconfStatusTimeout, true)
}

func TestPreconfBeforeInitFailed(t *testing.T) {
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: 400 * time.Millisecond,
	}
	txs := []*types.Transaction{pendingTxs[0], preconfTxs[0]}
	testBuildPayloadWithPreconf(t, cfg, txs, 1, 0, core.PreconfStatusFailed, false)
}

func TestPreconfWithL1SyncStatusDelay(t *testing.T) {
	wg := sync.WaitGroup{}
	wg.Add(1)
	go func() {
		TestUpdateOptimismSyncStatusDelay(t)
		wg.Done()
	}()
	time.Sleep(10 * time.Millisecond)
	cfg := &preconf.TxPoolConfig{
		FromPreconfs:   []common.Address{testPreconfAddress},
		ToPreconfs:     []common.Address{preconfTo},
		PreconfTimeout: 400 * time.Millisecond,
	}
	txs := []*types.Transaction{pendingTxs[0], preconfTxs[0], newTxs[0], preconfTxs[1]}
	elapsed := testBuildPayloadWithPreconf(t, cfg, txs, len(txs), len(preconfTxs), core.PreconfStatusSuccess, true)
	if elapsed > 100*time.Second {
		t.Fatalf("TestPreconfWithL1SyncStatusDelay took %v", elapsed)
	}
	wg.Wait()
}
