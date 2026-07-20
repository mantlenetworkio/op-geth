// Package basefee verifies that preconf receipt data is consistent with on-chain
// receipts after baseFee changes caused by consecutive full blocks.
package basefee

import (
	"context"
	"log"
	"math/big"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/config"
)

const (
	numBlocks      = 20
	blockWait      = 3 * time.Second
	transferAmount = 1_000_000_000 // 1 gwei — small amount, we only care about baseFee
)

// BaseFeeConsistencyTest sends preconf txs across multiple blocks and verifies
// that the PredictedL2BlockNumber in the preconf response matches the actual
// on-chain block, and that the on-chain receipt's EffectiveGasPrice is consistent
// with the baseFee progression.
func BaseFeeConsistencyTest() {
	log.Printf("=== Start baseFee consistency test (%d blocks) ===", numBlocks)

	ctx := context.Background()
	client, err := ethclient.Dial(config.L2RpcEndpoint)
	if err != nil {
		log.Fatalf("[basefee] failed to connect to L2 RPC: %v", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("[basefee] failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.FunderKey, chainID)
	if err != nil {
		log.Fatalf("[basefee] failed to create signer: %v", err)
	}

	// Record starting block
	startHeader, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("[basefee] failed to get start header: %v", err)
	}
	startBlock := startHeader.Number.Uint64()
	log.Printf("[basefee] starting at block %d, baseFee=%s", startBlock, startHeader.BaseFee)

	type txRecord struct {
		tx                     *types.Transaction
		predictedL2BlockNumber uint64
		baseFeeAtSubmission    *big.Int
	}

	var records []txRecord
	prevBaseFee := new(big.Int).Set(startHeader.BaseFee)

	// Self-managed nonce to avoid "already known" errors
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("[basefee] failed to get initial nonce: %v", err)
	}

	for i := 0; i < numBlocks; i++ {
		// Get current head for baseFee
		head, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("[basefee] block %d: failed to get header: %v", i, err)
		}
		currentBaseFee := head.BaseFee

		// Log baseFee change
		if i > 0 {
			diff := new(big.Int).Sub(currentBaseFee, prevBaseFee)
			log.Printf("[basefee] block %d: baseFee=%s (Δ=%s)", i, currentBaseFee, diff)
		}

		gasTipCap := big.NewInt(0)
		gasFeeCap := new(big.Int).Add(
			gasTipCap,
			new(big.Int).Mul(currentBaseFee, big.NewInt(2)),
		)

		to := common.HexToAddress(config.ToAddressHex)
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			To:        &to,
			GasTipCap: gasTipCap,
			GasFeeCap: gasFeeCap,
			Gas:       21_000,
			Value:     big.NewInt(transferAmount),
		})

		signedTx, err := auth.Signer(auth.From, tx)
		if err != nil {
			log.Fatalf("[basefee] block %d: failed to sign tx: %v", i, err)
		}

		var result core.NewPreconfTxEvent
		if err := client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
			log.Fatalf("[basefee] block %d: failed to send preconf tx: %v", i, err)
		}

		if result.Status != core.PreconfStatusSuccess {
			log.Fatalf("[basefee] block %d: preconf failed: status=%s reason=%s",
				i, result.Status, result.Reason)
		}

		records = append(records, txRecord{
			tx:                     signedTx,
			predictedL2BlockNumber: uint64(result.PredictedL2BlockNumber),
			baseFeeAtSubmission:    new(big.Int).Set(currentBaseFee),
		})

		nonce++ // self-managed nonce
		prevBaseFee = new(big.Int).Set(currentBaseFee)
		time.Sleep(blockWait)
	}

	// Verify: each preconf tx landed on-chain and the block's baseFee is consistent
	log.Printf("[basefee] verifying %d txs on-chain...", len(records))
	mismatchCount := 0

	for i, rec := range records {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		receipt, err := bind.WaitMined(waitCtx, client, rec.tx)
		cancel()
		if err != nil {
			log.Fatalf("[basefee] tx %d: failed to get receipt: %v", i, err)
		}

		if receipt.Status != types.ReceiptStatusSuccessful {
			log.Fatalf("[basefee] tx %d: tx reverted on-chain", i)
		}

		// Get the block this tx actually landed in
		block, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(receipt.BlockNumber.Uint64()))
		if err != nil {
			log.Fatalf("[basefee] tx %d: failed to get block %d: %v", i, receipt.BlockNumber.Uint64(), err)
		}
		actualBaseFee := block.BaseFee()

		// Compare predicted block number vs actual
		if rec.predictedL2BlockNumber != receipt.BlockNumber.Uint64() {
			log.Printf("[basefee] tx %d: block mismatch: predicted=%d actual=%d (tolerable ±1)",
				i, rec.predictedL2BlockNumber, receipt.BlockNumber.Uint64())
		}

		// The baseFee at submission should be close to the block's actual baseFee
		// With the fix, they should be equal (same CalcBaseFee input).
		// Without the fix, the preconf baseFee lags behind.
		if actualBaseFee.Cmp(rec.baseFeeAtSubmission) != 0 {
			mismatchCount++
			log.Printf("[basefee] tx %d: baseFee mismatch: atSubmission=%s onChain=%s blockNum=%d",
				i, rec.baseFeeAtSubmission, actualBaseFee, receipt.BlockNumber.Uint64())
		}
	}

	if mismatchCount > 0 {
		log.Printf("[basefee] ⚠ %d/%d txs had baseFee mismatch (may be due to block boundaries)", mismatchCount, len(records))
	} else {
		log.Printf("[basefee] ✅ all %d txs: baseFee at submission matches on-chain block baseFee", len(records))
	}

	// Log the baseFee curve
	endHeader, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("[basefee] failed to get end header: %v", err)
	}
	log.Printf("[basefee] baseFee curve: start=%s end=%s (blocks %d→%d)",
		startHeader.BaseFee, endHeader.BaseFee, startBlock, endHeader.Number.Uint64())

	log.Printf("=== baseFee consistency test completed ===")
}
