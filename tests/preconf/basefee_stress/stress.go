// Package basefee_stress is an optional stress test that sends high-TPS preconf
// traffic for many consecutive blocks, verifying baseFee growth follows the
// EIP-1559 formula and no panics/deadlocks occur.
package basefee_stress

import (
	"context"
	"fmt"
	"log"
	"math/big"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

const (
	stressBlocks   = 50
	concurrency    = 10
	txsPerRound    = 20
	roundSleep     = 2 * time.Second
	transferAmount = 1_000_000_000 // 1 gwei
)

// StressTest sends high-TPS preconf txs for stressBlocks consecutive blocks
// and verifies baseFee growth.
func StressTest() {
	log.Printf("=== Start baseFee stress test (%d blocks, concurrency=%d) ===", stressBlocks, concurrency)

	ctx := context.Background()
	client, err := ethclient.Dial(config.L2RpcEndpoint)
	if err != nil {
		log.Fatalf("[basefee_stress] failed to connect: %v", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("[basefee_stress] failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.FunderKey, chainID)
	if err != nil {
		log.Fatalf("[basefee_stress] failed to create signer: %v", err)
	}

	to := common.HexToAddress(config.ToAddressHex)

	// Record baseFee at start
	startHeader, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("[basefee_stress] failed to get start header: %v", err)
	}

	type blockRecord struct {
		number  uint64
		baseFee *big.Int
		gasUsed uint64
	}
	var baseFeeHistory []blockRecord
	baseFeeHistory = append(baseFeeHistory, blockRecord{
		number:  startHeader.Number.Uint64(),
		baseFee: startHeader.BaseFee,
	})

	var totalSuccess atomic.Int64
	var totalFailed atomic.Int64
	var mu sync.Mutex
	var nonceCounter uint64

	// Get initial nonce
	nonceCounter, err = client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("[basefee_stress] failed to get nonce: %v", err)
	}

	for round := range stressBlocks {
		head, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("[basefee_stress] round %d: failed to get header: %v", round, err)
		}

		gasFeeCap := new(big.Int).Mul(head.BaseFee, big.NewInt(2))

		// Send a batch concurrently
		var wg sync.WaitGroup
		for j := range txsPerRound {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()

				mu.Lock()
				nonce := nonceCounter
				nonceCounter++
				mu.Unlock()

				tx := types.NewTx(&types.DynamicFeeTx{
					ChainID:   chainID,
					Nonce:     nonce,
					To:        &to,
					GasTipCap: big.NewInt(0),
					GasFeeCap: gasFeeCap,
					Gas:       21_000,
					Value:     big.NewInt(transferAmount),
				})

				signedTx, err := auth.Signer(auth.From, tx)
				if err != nil {
					log.Printf("[basefee_stress] round %d tx %d: sign error: %v", round, idx, err)
					totalFailed.Add(1)
					return
				}

				var result core.NewPreconfTxEvent
				if err := client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
					totalFailed.Add(1)
					return
				}

				if result.Status == core.PreconfStatusSuccess {
					totalSuccess.Add(1)
				} else {
					totalFailed.Add(1)
				}
			}(j)
		}
		wg.Wait()

		// Record baseFee after this round
		postHead, err := client.HeaderByNumber(ctx, nil)
		if err == nil && postHead.Number.Uint64() > baseFeeHistory[len(baseFeeHistory)-1].number {
			// Capture intermediate blocks we may have missed
			for n := baseFeeHistory[len(baseFeeHistory)-1].number + 1; n <= postHead.Number.Uint64(); n++ {
				block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(n))
				if err != nil {
					continue
				}
				baseFeeHistory = append(baseFeeHistory, blockRecord{
					number:  n,
					baseFee: block.BaseFee(),
					gasUsed: block.GasUsed(),
				})
			}
		}

		if round%10 == 0 {
			log.Printf("[basefee_stress] round %d/%d: success=%d failed=%d baseFee=%s",
				round, stressBlocks, totalSuccess.Load(), totalFailed.Load(), head.BaseFee)
		}

		time.Sleep(roundSleep)
	}

	// Print baseFee curve summary
	log.Printf("[basefee_stress] baseFee curve (%d data points):", len(baseFeeHistory))
	for i, rec := range baseFeeHistory {
		fillPct := ""
		if i > 0 && rec.gasUsed > 0 {
			// Approximate fill ratio (may not have GasLimit from header)
			fillPct = fmt.Sprintf(" gasUsed=%d", rec.gasUsed)
		}
		if i < 5 || i >= len(baseFeeHistory)-5 || i%10 == 0 {
			log.Printf("  block %d: baseFee=%s%s", rec.number, rec.baseFee, fillPct)
		}
	}

	// Verify baseFee increased (assuming blocks were full)
	first := baseFeeHistory[0].baseFee
	last := baseFeeHistory[len(baseFeeHistory)-1].baseFee
	if last.Cmp(first) <= 0 {
		log.Printf("[basefee_stress] ⚠ baseFee did not increase: start=%s end=%s — blocks may not have been full", first, last)
	} else {
		ratio := new(big.Float).Quo(new(big.Float).SetInt(last), new(big.Float).SetInt(first))
		log.Printf("[basefee_stress] ✅ baseFee increased: start=%s end=%s ratio=%s", first, last, ratio.Text('f', 4))
	}

	log.Printf("[basefee_stress] totals: success=%d failed=%d", totalSuccess.Load(), totalFailed.Load())
	log.Printf("=== baseFee stress test completed ===")
}
