package sort

import (
	"context"
	"log"
	"math/big"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

func SortTest() {
	sortTest(config.SequencerEndpoint)
	sortTest(config.L2RpcEndpoint)
}

// sortTest Order verification test
func sortTest(endpoint string) {
	log.Printf("SortTest %s starting ...\n", endpoint)
	defer log.Printf("SortTest %s completed\n", endpoint)

	ctx := context.Background()

	l2client, err := ethclient.Dial(endpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
	}
	defer l2client.Close()

	// Get L2 chain ID
	l2ChainID, err := l2client.NetworkID(ctx)
	if err != nil {
		log.Fatalf("failed to get L2 chain ID: %v", err)
	}

	// Create transaction signers
	addr1Auth, err := bind.NewKeyedTransactorWithChainID(config.Addr1Key, l2ChainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr1 signer: %v", err)
	}
	addr3Auth, err := bind.NewKeyedTransactorWithChainID(config.Addr3Key, l2ChainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr3 signer: %v", err)
	}

	// Initialize funds
	oneMNT := big.NewInt(1e18)
	transferAmount := new(big.Int).Mul(big.NewInt(config.NumTransactions), oneMNT)
	fundAmount := new(big.Int).Mul(transferAmount, big.NewInt(10)) // Extra for gas
	if err := config.FundAccount(ctx, l2client, config.Addr1, fundAmount); err != nil {
		log.Fatalf("failed to fund config.Addr1: %v", err)
	}
	if err := config.FundAccount(ctx, l2client, config.Addr3, fundAmount); err != nil {
		log.Fatalf("failed to fund config.Addr3: %v", err)
	}

	// Record initial balances
	startBalances := map[common.Address]*big.Int{
		config.Addr1: config.GetBalance(ctx, l2client, config.Addr1),
		config.Addr2: config.GetBalance(ctx, l2client, config.Addr2),
		config.Addr3: config.GetBalance(ctx, l2client, config.Addr3),
	}
	log.Printf("Initial balances - config.Addr1: %s MNT, config.Addr2: %s MNT, config.Addr3: %s MNT", config.BalanceString(startBalances[config.Addr1]), config.BalanceString(startBalances[config.Addr2]), config.BalanceString(startBalances[config.Addr3]))

	// Get starting height
	startHeight, err := l2client.BlockNumber(ctx)
	if err != nil {
		log.Fatalf("failed to get starting height: %v", err)
	}

	// Send batch transactions
	var wg sync.WaitGroup
	var addr1Txs, addr3Txs []*types.Transaction
	wg.Add(2)

	go func() {
		defer wg.Done()
		sendBatchPreconfTxs(ctx, l2client, addr1Auth, config.Addr2, oneMNT, config.NumTransactions, &addr1Txs)
		for _, tx := range addr1Txs {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, l2client, tx)
			if err == nil && receipt != nil {
				// fmt.Printf("Transaction %s confirmed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
				break
			}
		}
	}()

	go func() {
		defer wg.Done()
		sendBatchTxs(ctx, l2client, addr3Auth, config.Addr2, oneMNT, config.NumTransactions, &addr3Txs)
		for _, tx := range addr3Txs {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, l2client, tx)
			if err == nil && receipt != nil {
				// fmt.Printf("Transaction %s confirmed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
				break
			}
		}
	}()

	// Wait for transactions to complete
	wg.Wait()
	time.Sleep(5 * time.Second)

	// Get ending height
	endHeight, err := l2client.BlockNumber(ctx)
	if err != nil {
		log.Fatalf("failed to get ending height: %v", err)
	}
	log.Printf("Block range: %d -> %d", startHeight, endHeight)

	// Verify transaction order
	signer := types.LatestSignerForChainID(l2ChainID)
	for i := startHeight + 1; i <= endHeight; i++ {
		block, err := l2client.BlockByNumber(ctx, big.NewInt(int64(i)))
		if err != nil {
			log.Printf("failed to get block %d: %v", i, err)
			continue
		}

		var lastFrom common.Address
		for j, tx := range block.Transactions() {
			// Verify order: deposit transaction must be before config.Addr1's
			if tx.IsDepositTx() {
				// log.Printf("Block %d, tx %d: %s (DepositTx)", i, j, tx.Hash().Hex())
				if lastFrom != (common.Address{}) {
					log.Fatalf("Block %d: tx %d Order error, DepositTx (%s) after %s", i, j, tx.Hash().Hex(), lastFrom.Hex())
				}
				continue
			}
			from, _ := types.Sender(signer, tx)
			// log.Printf("Block %d, tx %d: %s (from: %s, to: %s)", i, j, tx.Hash().Hex(), from.Hex(), tx.To().Hex())

			// Verify order: config.Addr1's transaction must be before config.Addr3's
			if lastFrom == config.Addr3 && from == config.Addr1 {
				log.Fatalf("Block %d: Order error, config.Addr1 (%s) after config.Addr3 (%s)", i, from.Hex(), lastFrom.Hex())
			}
			lastFrom = from
		}
	}

	// Verify final balances
	endBalances := map[common.Address]*big.Int{
		config.Addr1: config.GetBalance(ctx, l2client, config.Addr1),
		config.Addr2: config.GetBalance(ctx, l2client, config.Addr2),
		config.Addr3: config.GetBalance(ctx, l2client, config.Addr3),
	}
	log.Printf("Final balances - config.Addr1: %s MNT, config.Addr2: %s MNT, config.Addr3: %s MNT", config.BalanceString(endBalances[config.Addr1]), config.BalanceString(endBalances[config.Addr2]), config.BalanceString(endBalances[config.Addr3]))

	expectedAddr2 := new(big.Int).Add(startBalances[config.Addr2], new(big.Int).Mul(transferAmount, big.NewInt(2)))
	if endBalances[config.Addr2].Cmp(expectedAddr2) != 0 {
		log.Printf("config.Addr2 balance error, expected: %s MNT, actual: %s MNT", config.BalanceString(expectedAddr2), config.BalanceString(endBalances[config.Addr2]))
	} else {
		log.Printf("config.Addr2 balance correct ✅\n")
	}
}

// sendBatchTxs Send batch transactions
func sendBatchTxs(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, count int, txs *[]*types.Transaction) {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Printf("failed to get nonce for %s: %v", auth.From.Hex(), err)
		return
	}

	for i := 0; i < count; i++ {
		tx, err := config.SendMNT(ctx, client, auth, to, amount, nonce+uint64(i))
		if err != nil {
			log.Printf("failed to send transaction %d: %v", i, err)
			continue
		}
		*txs = append(*txs, tx)
		time.Sleep(config.NonceInterval)
	}
}

// sendBatchPreconfTxs Send batch pre-confirmed transactions
func sendBatchPreconfTxs(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, count int, txs *[]*types.Transaction) {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Printf("failed to get nonce for %s: %v", auth.From.Hex(), err)
		return
	}

	for i := 0; i < count; i++ {
		tx, err := config.SendMNTWithPreconf(ctx, client, auth, to, amount, nonce+uint64(i))
		if err != nil {
			log.Printf("failed to send transaction %d: %v", i, err)
			continue
		}
		*txs = append(*txs, tx)
		time.Sleep(config.NonceInterval)
	}
}
