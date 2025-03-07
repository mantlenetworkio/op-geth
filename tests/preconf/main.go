package main

import (
	"context"
	"fmt"
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
	"golang.org/x/sync/semaphore"
)

// Configuration
const (
	NumTransactions = 1000
	RpcEndpoint     = "http://127.0.0.1:10545"
	PrivateKeyHex   = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80"
	ToAddressHex    = "0x71920E3cb420fbD8Ba9a495E6f801c50375ea127"
	BatchSize       = 10
	NonceInterval   = 30 * time.Millisecond
)

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

	toAddress := common.HexToAddress(ToAddressHex)
	value := big.NewInt(1e14)      // 0.0001 ETH
	gasPrice := big.NewInt(2e9)    // 2 gwei
	gasLimit := uint64(2100000000) // Standard gas limit for ETH transfer

	tx := types.NewTransaction(
		nonce,
		toAddress,
		value,
		gasLimit,
		gasPrice,
		nil,
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		fmt.Printf("Error signing transaction %d: %v\n", iteration, err)
		return TxResult{}
	}

	startTime := float64(time.Now().UnixNano()) / 1e6 // ms
	var result ethapi.PreconfTransactionResult
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	endTime := float64(time.Now().UnixNano()) / 1e6
	responseTime := endTime - startTime

	if err != nil {
		fmt.Printf("Error sending transaction %d: %v\n", iteration, err)
		return TxResult{ResponseTime: responseTime, StartTime: startTime, EndTime: endTime}
	}

	txHash := signedTx.Hash()
	if result.TxHash != txHash {
		fmt.Printf("Transaction hash mismatch: %v != %v\n", result.TxHash, txHash)
		panic("Transaction hash mismatch")
	}

	if result.Status == "failed" {
		fmt.Printf("Transaction %d failed: %s\n", iteration, result.Reason)
	}

	// Background confirmation
	go func() {
		for {
			receipt, err := client.TransactionReceipt(ctx, txHash)
			if err == nil && receipt != nil {
				fmt.Printf("Transaction %d confirmed - Status: %d, Expected Block: %d, Actual Block: %d\n", iteration, receipt.Status, uint64(result.BlockHeight), receipt.BlockNumber.Uint64())

				if result.BlockHeight.String() != hexutil.EncodeBig(receipt.BlockNumber) {
					fmt.Printf("TxHash: %s, Block height mismatch: %d != %d, reason: %s\n", result.TxHash, result.BlockHeight, receipt.BlockNumber.Uint64(), result.Reason)
				}

				break
			}
			time.Sleep(1 * time.Second)
		}
	}()

	return TxResult{ResponseTime: responseTime, TxHash: txHash, StartTime: startTime, EndTime: endTime}
}

func stressTest() {
	ctx := context.Background()
	client, err := ethclient.Dial(RpcEndpoint)
	if err != nil {
		fmt.Printf("Failed to connect to RPC: %v\n", err)
		return
	}
	defer client.Close()

	privateKey, err := crypto.HexToECDSA(PrivateKeyHex)
	if err != nil {
		fmt.Printf("Failed to parse private key: %v\n", err)
		return
	}

	publicKey := privateKey.PublicKey
	fromAddress := crypto.PubkeyToAddress(publicKey)
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		fmt.Printf("Failed to get chain ID: %v\n", err)
		return
	}

	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		fmt.Printf("Failed to create transactor: %v\n", err)
		return
	}

	fmt.Printf("Starting batched stress test with %d transactions, batch size %d...\n", NumTransactions, BatchSize)
	fmt.Printf("From: %s\n", fromAddress.Hex())
	fmt.Printf("To: %s\n", ToAddressHex)

	toBalanceBefore, err := client.BalanceAt(ctx, common.HexToAddress(ToAddressHex), nil)
	if err != nil {
		fmt.Printf("Failed to get initial To balance: %v\n", err)
		return
	}
	fmt.Printf("To balance before stress test: %s ETH\n", new(big.Float).Quo(new(big.Float).SetInt(toBalanceBefore), big.NewFloat(1e18)).String())

	baseNonce, err := client.PendingNonceAt(ctx, fromAddress)
	if err != nil {
		fmt.Printf("Failed to get base nonce: %v\n", err)
		return
	}

	var responseTimes []float64
	var mu sync.Mutex

	for batchStart := 0; batchStart < NumTransactions; batchStart += BatchSize {
		batchEnd := min(batchStart+BatchSize, NumTransactions)
		fmt.Printf("\nPreparing batch: Transactions %d to %d\n", batchStart+1, batchEnd)

		var wg sync.WaitGroup
		sem := semaphore.NewWeighted(int64(BatchSize)) // Limit concurrency to batch size

		for i := batchStart; i < batchEnd; i++ {
			if err := sem.Acquire(ctx, 1); err != nil {
				fmt.Printf("Failed to acquire semaphore: %v\n", err)
				break
			}

			// prevent nonce too high
			time.Sleep(NonceInterval)

			wg.Add(1)
			go func(iteration int, nonce uint64) {
				defer sem.Release(1)
				result := sendRawTransactionWithPreconf(ctx, client, auth, iteration, nonce, &wg)
				mu.Lock()
				responseTimes = append(responseTimes, result.ResponseTime)
				// fmt.Printf(
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

		fmt.Println("\nStress Test Results:")
		fmt.Printf("Shortest response time: %.2f ms\n", shortest)
		fmt.Printf("Longest response time: %.2f ms\n", longest)
		fmt.Printf("Average response time: %.2f ms\n", average)
		fmt.Printf("Total transactions: %d\n", len(responseTimes))
	} else {
		fmt.Println("No successful transactions to analyze.")
	}

	toBalanceAfter, err := client.BalanceAt(ctx, common.HexToAddress(ToAddressHex), nil)
	if err != nil {
		fmt.Printf("Failed to get final To balance: %v\n", err)
		return
	}
	fmt.Printf("\nTo balance after stress test: %s ETH\n", new(big.Float).Quo(new(big.Float).SetInt(toBalanceAfter), big.NewFloat(1e18)).String())

	expectedIncrease := new(big.Int).Mul(big.NewInt(1e14), big.NewInt(int64(NumTransactions))) // 0.0001 ETH * NumTransactions
	fmt.Printf("Expected increase in To balance: %s ETH\n", new(big.Float).Quo(new(big.Float).SetInt(expectedIncrease), big.NewFloat(1e18)).String())

	actualIncrease := new(big.Int).Sub(toBalanceAfter, toBalanceBefore)
	fmt.Printf("Actual increase in To balance: %s ETH\n", new(big.Float).Quo(new(big.Float).SetInt(actualIncrease), big.NewFloat(1e18)).String())
	if actualIncrease.Cmp(expectedIncrease) != 0 {
		fmt.Println("WARNING: To balance change is incorrect!")
	}
}

func main() {
	stressTest()
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
