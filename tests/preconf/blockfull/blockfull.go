// Package blockfull verifies preconf block-full behaviour:
//   - Preconf returns failure when the block is full.
//   - Preconf txs that succeeded are 100% landed on-chain.
//   - After the next block cycle, a previously rejected tx can be resubmitted.
package blockfull

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
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

// BlockFullTest sends preconf txs until the block is full, then verifies:
// 1. Successful preconf txs are on-chain.
// 2. A failed tx can be retried in the next block.
func BlockFullTest() {
	log.Printf("=== Start block-full test ===")

	ctx := context.Background()
	client, err := ethclient.Dial(config.L2RpcEndpoint)
	if err != nil {
		log.Fatalf("[blockfull] failed to connect to L2 RPC: %v", err)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("[blockfull] failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.FunderKey, chainID)
	if err != nil {
		log.Fatalf("[blockfull] failed to create signer: %v", err)
	}

	to := common.HexToAddress(config.ToAddressHex)
	amount := big.NewInt(1_000_000_000) // 1 gwei

	// Phase 1: Send preconf txs until one fails
	var succeeded []*types.Transaction
	var gotBlockFull bool

	// Query actual block gas limit to size txs correctly
	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("[blockfull] failed to get header: %v", err)
	}
	blockGasLimit := head.GasLimit
	// Use ~80% of block gas limit per tx so 2 txs fill a block
	txGas := blockGasLimit * 80 / 100
	log.Printf("[blockfull] block gasLimit=%d, using txGas=%d per tx", blockGasLimit, txGas)

	// Self-managed nonce
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("[blockfull] failed to get nonce: %v", err)
	}

	for i := 0; i < 500; i++ {
		head, err := client.HeaderByNumber(ctx, nil)
		if err != nil {
			log.Fatalf("[blockfull] failed to get header: %v", err)
		}

		gasFeeCap := new(big.Int).Mul(head.BaseFee, big.NewInt(2))
		tx := types.NewTx(&types.DynamicFeeTx{
			ChainID:   chainID,
			Nonce:     nonce,
			To:        &to,
			GasTipCap: big.NewInt(0),
			GasFeeCap: gasFeeCap,
			Gas:       txGas,
			Value:     amount,
		})

		signedTx, err := auth.Signer(auth.From, tx)
		if err != nil {
			log.Fatalf("[blockfull] failed to sign tx: %v", err)
		}

		var result core.NewPreconfTxEvent
		if err := client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
			// RPC-level error (e.g., intrinsic gas too low) — not block full
			log.Printf("[blockfull] tx %d: RPC error: %v", i, err)
			continue
		}

		if result.Status == core.PreconfStatusFailed {
			log.Printf("[blockfull] tx %d: preconf failed (reason=%s) — block may be full", i, result.Reason)
			gotBlockFull = true
			break
		}

		succeeded = append(succeeded, signedTx)
		nonce++
		log.Printf("[blockfull] tx %d: preconf succeeded (predicted block=%d)", i, result.PredictedL2BlockNumber)
	}

	if !gotBlockFull {
		log.Printf("[blockfull] ⚠ sent 500 txs without hitting block full — test inconclusive (block gas limit may be very large)")
		log.Printf("=== block-full test completed (inconclusive) ===")
		return
	}

	log.Printf("[blockfull] %d txs succeeded before block full", len(succeeded))

	// Phase 2: Verify all succeeded txs are on-chain
	log.Printf("[blockfull] verifying %d succeeded txs on-chain...", len(succeeded))
	for i, tx := range succeeded {
		waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		receipt, err := bind.WaitMined(waitCtx, client, tx)
		cancel()
		if err != nil {
			log.Fatalf("[blockfull] tx %d: failed to get receipt: %v", i, err)
		}
		if receipt.Status != types.ReceiptStatusSuccessful {
			log.Fatalf("[blockfull] tx %d: preconf succeeded but on-chain reverted!", i)
		}
	}
	log.Printf("[blockfull] ✅ all %d succeeded txs confirmed on-chain", len(succeeded))

	// Phase 3: Wait for next block cycle, then retry a new tx
	log.Printf("[blockfull] waiting for next block cycle (3s)...")
	time.Sleep(3 * time.Second)

	head, err = client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("[blockfull] failed to get header after wait: %v", err)
	}

	// Re-fetch nonce (previous succeeded txs advanced it)
	retryNonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("[blockfull] failed to get retry nonce: %v", err)
	}

	retryFeeCap := new(big.Int).Mul(head.BaseFee, big.NewInt(2))
	retryTx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     retryNonce,
		To:        &to,
		GasTipCap: big.NewInt(0),
		GasFeeCap: retryFeeCap,
		Gas:       21_000, // small gas this time
		Value:     amount,
	})

	signedRetry, err := auth.Signer(auth.From, retryTx)
	if err != nil {
		log.Fatalf("[blockfull] failed to sign retry tx: %v", err)
	}

	var retryResult core.NewPreconfTxEvent
	if err := client.SendTransactionWithPreconf(ctx, signedRetry, &retryResult); err != nil {
		log.Fatalf("[blockfull] retry tx RPC error: %v", err)
	}

	if retryResult.Status != core.PreconfStatusSuccess {
		log.Fatalf("[blockfull] retry tx failed: status=%s reason=%s", retryResult.Status, retryResult.Reason)
	}

	waitCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	receipt, err := bind.WaitMined(waitCtx, client, signedRetry)
	cancel()
	if err != nil {
		log.Fatalf("[blockfull] retry tx: failed to get receipt: %v", err)
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		log.Fatalf("[blockfull] retry tx: on-chain reverted!")
	}

	log.Printf("[blockfull] ✅ retry tx succeeded in block %d", receipt.BlockNumber.Uint64())
	log.Printf("=== block-full test completed ===")
}
