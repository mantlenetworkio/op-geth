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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/internal/ethapi"
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

	publicKey := config.FunderKey.PublicKey
	fromAddress := crypto.PubkeyToAddress(publicKey)
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

	log.Printf("Starting batched stress test with %d transactions, batch size %d...\n", config.NumTransactions, config.BatchSize)

	toBalanceBefore, err := client.BalanceAt(ctx, config.Addr2, nil)
	if err != nil {
		log.Printf("Failed to get initial To balance: %v\n", err)
		return
	}
	log.Printf("To balance before stress test: %s MNT\n", config.BalanceString(toBalanceBefore))

	baseNonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		log.Printf("Failed to get base nonce: %v\n", err)
		return
	}

	var responseTimes []float64
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
	time.Sleep(15 * time.Second)

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
		log.Println("WARNING: To balance change is incorrect!")
	} else {
		log.Println("To balance change is correct ✅")
	}
}

type TxResult struct {
	ResponseTime float64
	TxHash       common.Hash
	StartTime    float64
	EndTime      float64
}

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

	tx := types.NewTransaction(
		nonce,
		config.Addr2,
		value,
		config.TransferGasLimit,
		config.GasPrice,
		nil,
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		log.Printf("Error signing transaction %d: %v\n", iteration, err)
		return TxResult{}
	}

	startTime := float64(time.Now().UnixNano()) / 1e6 // ms
	var result ethapi.PreconfTransactionResult
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	endTime := float64(time.Now().UnixNano()) / 1e6
	responseTime := endTime - startTime

	if err != nil {
		log.Printf("Error sending transaction %d: %v\n", iteration, err)
		return TxResult{ResponseTime: responseTime, StartTime: startTime, EndTime: endTime}
	}

	txHash := signedTx.Hash()
	if result.TxHash != txHash {
		log.Printf("Transaction hash mismatch: %v != %v\n", result.TxHash, txHash)
		panic("Transaction hash mismatch")
	}

	if result.Status == "failed" {
		log.Printf("Transaction %d failed: %s\n", iteration, result.Reason)
	}

	// Background confirmation
	go func() {
		for {
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil && receipt != nil {
				// log.Printf("Transaction %d confirmed - Status: %d, Expected Block: %d, Actual Block: %d\n", iteration, receipt.Status, uint64(result.BlockHeight), receipt.BlockNumber.Uint64())

				if result.BlockHeight.String() != hexutil.EncodeBig(receipt.BlockNumber) {
					log.Printf("TxHash: %s, Block height mismatch: %d != %d, reason: %s\n", result.TxHash, result.BlockHeight, receipt.BlockNumber.Uint64(), result.Reason)
				}

				break
			}
			time.Sleep(1 * time.Second)
		}
	}()

	return TxResult{ResponseTime: responseTime, TxHash: txHash, StartTime: startTime, EndTime: endTime}
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
