package stress

import (
	"context"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
	"golang.org/x/sync/semaphore"
)

func StressTest() {
	stress(config.SequencerEndpoint)
	stress(config.L2RpcEndpoint)
}

func stress(rawurl string) {
	log.Printf("StressTest %s starting ...\n", rawurl)
	defer log.Printf("StressTest %s completed\n", rawurl)

	ctx := context.Background()
	client, err := ethclient.Dial(rawurl)
	if err != nil {
		log.Printf("Failed to connect to RPC: %v\n", err)
		return
	}
	defer client.Close()

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		log.Printf("Failed to get chain ID: %v\n", err)
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.FunderKey, chainID)
	if err != nil {
		log.Printf("Failed to create transactor: %v\n", err)
		return
	}

	baseNonce := config.GetNonce(ctx, client, config.FundAddr)
	log.Printf("Starting batched stress test with %d transactions, batch size %d, base nonce %d...\n", config.NumTransactions, config.BatchSize, baseNonce)

	toBalanceBefore := config.GetBalance(ctx, client, config.Addr2)
	log.Printf("To balance before stress test: %s MNT\n", config.BalanceString(toBalanceBefore))

	var responseTimes []float64
	var txResult []TxResult
	var mu sync.Mutex

	for batchStart := 0; batchStart < config.NumTransactions; batchStart += config.BatchSize {
		batchEnd := min(batchStart+config.BatchSize, config.NumTransactions)
		// log.Printf("Preparing batch: Transactions %d to %d\n", batchStart+1, batchEnd)

		var wg sync.WaitGroup
		sem := semaphore.NewWeighted(int64(config.BatchSize)) // Limit concurrency to batch size

		for i := batchStart; i < batchEnd; i++ {
			if err := sem.Acquire(ctx, 1); err != nil {
				log.Printf("Failed to acquire semaphore: %v\n", err)
				break
			}

			// prevent nonce too high
			time.Sleep(config.NonceInterval)

			wg.Add(1)
			go func(iteration int, nonce uint64) {
				defer sem.Release(1)
				result := sendRawTransactionWithPreconf(ctx, client, auth, iteration, nonce, &wg)
				mu.Lock()
				txResult = append(txResult, result)
				responseTimes = append(responseTimes, result.ResponseTime)
				// log.Printf(
				// 	"Tx %d: Start %.2f ms, End %.2f ms, Response time %.2f ms\n",
				// 	iteration, result.StartTime, result.EndTime, result.ResponseTime,
				// )
				mu.Unlock()
			}(i+1, baseNonce+uint64(i))
		}

		wg.Wait() // Wait for batch to complete
	}

	// Wait for confirmations (adjust as needed)
	for _, result := range txResult {
		// Background confirmation
		ctx, cancel := context.WithTimeout(ctx, 1*time.Minute)
		defer cancel()
		receipt, err := bind.WaitMined(ctx, client, result.Tx)
		if err != nil {
			log.Fatalf("Transaction %s not confirmed yet\n", result.TxHash.Hex())
		}
		if receipt == nil {
			log.Fatalf("Transaction %s not mined\n", result.TxHash.Hex())
		}

		// log.Printf("Transaction %d:%s %d confirmed - Status: %d, Expected Block: %d, Actual Block: %d\n", iteration, txHash.Hex(), tx.Nonce(), receipt.Status, uint64(result.BlockHeight), receipt.BlockNumber.Uint64())
		if result.PredictedL2BlockNumber.String() != hexutil.EncodeBig(receipt.BlockNumber) {
			if result.PredictedL2BlockNumber+1 == hexutil.Uint64(receipt.BlockNumber.Uint64()) {
				log.Printf("TxHash: %s, Block height mismatch: predicted %d != receipt %d\n", result.TxHash, result.PredictedL2BlockNumber, receipt.BlockNumber.Uint64())
			} else if result.PredictedL2BlockNumber != 0 {
				log.Fatalf("TxHash: %s, Block height mismatch: predicted %d != receipt %d\n", result.TxHash, result.PredictedL2BlockNumber, receipt.BlockNumber.Uint64())
			}
		}
		if receipt.Status == types.ReceiptStatusFailed {
			log.Fatalf("TxHash: %s, preconf success but transaction failed\n", result.TxHash)
		}
	}

	if len(responseTimes) > 0 {
		shortest := minFloat(responseTimes)
		longest := maxFloat(responseTimes)
		average := sumFloat(responseTimes) / float64(len(responseTimes))

		log.Println("Stress Test Results:")
		log.Printf("Shortest response time: %.2f ms\n", shortest)
		log.Printf("Longest response time: %.2f ms\n", longest)
		log.Printf("Average response time: %.2f ms\n", average)
		log.Printf("Total transactions: %d\n", len(responseTimes))
	} else {
		log.Println("No successful transactions to analyze.")
	}

	toBalanceAfter, err := client.BalanceAt(ctx, config.Addr2, nil)
	if err != nil {
		log.Printf("Failed to get final To balance: %v\n", err)
		return
	}
	log.Printf("To balance after stress test: %s MNT\n", config.BalanceString(toBalanceAfter))

	expectedIncrease := new(big.Int).Mul(big.NewInt(1e14), big.NewInt(int64(config.NumTransactions))) // 0.0001 MNT * NumTransactions
	log.Printf("Expected increase in To balance: %s MNT\n", config.BalanceString(expectedIncrease))

	actualIncrease := new(big.Int).Sub(toBalanceAfter, toBalanceBefore)
	log.Printf("Actual increase in To balance: %s MNT\n", config.BalanceString(actualIncrease))
	if actualIncrease.Cmp(expectedIncrease) != 0 {
		log.Fatal("To balance change is incorrect!")
	} else {
		log.Println("To balance change is correct ✅")
	}
}

type TxResult struct {
	ResponseTime           float64
	StartTime              float64
	EndTime                float64
	Tx                     *types.Transaction
	TxHash                 common.Hash
	PredictedL2BlockNumber hexutil.Uint64
}

var predictedL2BlockNumber hexutil.Uint64

func sendRawTransactionWithPreconf(
	ctx context.Context,
	client *ethclient.Client,
	auth *bind.TransactOpts,
	iteration int,
	nonce uint64,
	wg *sync.WaitGroup,
) TxResult {
	defer wg.Done()

	value := big.NewInt(1e14) // 0.0001 MNT

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("failed to suggest gas price: %v", err)
	}

	tx := types.NewTransaction(
		nonce,
		config.Addr2,
		value,
		config.TransferGasLimit,
		gasPrice,
		nil,
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		log.Printf("Error signing transaction %d: %v\n", iteration, err)
		return TxResult{}
	}

	startTime := float64(time.Now().UnixNano()) / 1e6 // ms
	var result core.NewPreconfTxEvent
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	endTime := float64(time.Now().UnixNano()) / 1e6
	responseTime := endTime - startTime
	txHash := signedTx.Hash()

	if err != nil {
		log.Fatalf("Error sending transaction %d: %v, txHash: %s\n", iteration, err, txHash.Hex())
	}

	if result.TxHash != txHash {
		log.Fatalf("Transaction hash mismatch: %v != %v\n", result.TxHash, txHash)
	}

	if predictedL2BlockNumber != result.PredictedL2BlockNumber {
		log.Printf("new predictedL2BlockNumber: %d\n", result.PredictedL2BlockNumber)
		predictedL2BlockNumber = result.PredictedL2BlockNumber
	}

	if result.Status == core.PreconfStatusFailed {
		if result.Reason == miner.ErrEnvBlockNumberAndEngineSyncTargetBlockNumberDistanceTooLarge.Error() {
			log.Printf("Transaction %d failed: %s, wait for new preconf tx\n", iteration, result.Reason)
			time.Sleep(config.WaitTime)
			return TxResult{ResponseTime: responseTime, StartTime: startTime, EndTime: endTime, Tx: signedTx, TxHash: txHash}
		}
		log.Fatalf("Transaction %d failed, txHash: %s, reason: %s\n", iteration, txHash.Hex(), result.Reason)
	}
	// log.Printf("preconf txHash: %s, nonce: %d\n", txHash.Hex(), tx.Nonce())

	return TxResult{ResponseTime: responseTime, PredictedL2BlockNumber: predictedL2BlockNumber, StartTime: startTime, EndTime: endTime, Tx: signedTx, TxHash: txHash}
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func minFloat(slice []float64) float64 {
	if len(slice) == 0 {
		return 0
	}
	min := slice[0]
	for _, v := range slice {
		if v < min {
			min = v
		}
	}
	return min
}

func maxFloat(slice []float64) float64 {
	if len(slice) == 0 {
		return 0
	}
	max := slice[0]
	for _, v := range slice {
		if v > max {
			max = v
		}
	}
	return max
}

func sumFloat(slice []float64) float64 {
	sum := 0.0
	for _, v := range slice {
		sum += v
	}
	return sum
}
