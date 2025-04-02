package sort

import (
	"context"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/miner"
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
	config.FundAccount(ctx, l2client, config.Addr1, fundAmount)
	config.FundAccount(ctx, l2client, config.Addr3, fundAmount)

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
	wg.Add(3)

	// send deposit tx
	go func() {
		defer wg.Done()
		// addr1 and addr3 send deposit tx to addr2
		l1client, l1Addr1Auth, err := config.GetL1Auth(ctx, config.Addr1Key)
		if err != nil {
			log.Fatalf("failed to get L1 auth: %v", err)
		}
		_, l1Addr3Auth, err := config.GetL1Auth(ctx, config.Addr3Key)
		if err != nil {
			log.Fatalf("failed to get L1 auth: %v", err)
		}
		fundAmount := new(big.Int).Mul(big.NewInt(config.NumTransactions*5), oneMNT)
		config.FundAccount(ctx, l1client, config.Addr1, fundAmount)
		config.FundAccount(ctx, l1client, config.Addr3, fundAmount)

		addr1DepositTxs := sendBatchDepositTxs(ctx, l1client, l1Addr1Auth, config.Addr2, oneMNT, config.NumTransactions/20+1)
		addr3DepositTxs := sendBatchDepositTxs(ctx, l1client, l1Addr3Auth, config.Addr2, oneMNT, config.NumTransactions/20+1)
		for i, tx := range append(addr1DepositTxs, addr3DepositTxs...) {
			ctx, cancel := context.WithTimeout(ctx, 6*config.WaitTime)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, l1client, tx)
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					log.Printf("failed to send deposit tx, ending deposit tx test: %v", err)
					return
				}
				log.Fatalf("failed to wait for deposit tx %d: %v, tx: %s", i, err, tx.Hash().Hex())
			}
			if receipt.Status != types.ReceiptStatusSuccessful {
				log.Printf("deposit tx %d failed: %v, tx: %s", i, receipt.Status, tx.Hash().Hex())
			}
		}
	}()

	// send pre-confirmed tx
	go func() {
		defer wg.Done()
		time.Sleep(12 * time.Second) // wait for deposit tx to be sent
		sendBatchPreconfTxs(ctx, l2client, addr1Auth, config.Addr2, oneMNT, config.NumTransactions, &addr1Txs)
		for i, tx := range addr1Txs {
			ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, l2client, tx)
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					log.Printf("pre-confirmed tx replaced by deposit tx, from: %s, nonce: %d, tx: %s", addr1Auth.From.Hex(), tx.Nonce(), tx.Hash().Hex())
					continue
				}
				log.Fatalf("failed to wait for pre-confirmed tx %d: %v, tx: %s", i, err, tx.Hash().Hex())
			}
			if receipt.Status != types.ReceiptStatusSuccessful {
				log.Fatalf("pre-confirmed tx %d failed: %v, tx: %s", i, receipt.Status, tx.Hash().Hex())
			}
		}
	}()

	// send transfer tx
	go func() {
		defer wg.Done()
		time.Sleep(12 * time.Second) // wait for deposit tx to be sent
		sendBatchTxs(ctx, l2client, addr3Auth, config.Addr2, oneMNT, config.NumTransactions, &addr3Txs)
		for i, tx := range addr3Txs {
			ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, l2client, tx)
			if err != nil {
				if strings.Contains(err.Error(), "context deadline exceeded") {
					log.Printf("transfer tx replaced by deposit tx, from: %s, nonce: %d, tx: %s", addr3Auth.From.Hex(), tx.Nonce(), tx.Hash().Hex())
					continue
				}
				log.Fatalf("failed to wait for transfer tx %d: %v, tx: %s", i, err, tx.Hash().Hex())
			}
			if receipt.Status != types.ReceiptStatusSuccessful {
				log.Fatalf("transfer tx %d failed: %v, tx: %s", i, receipt.Status, tx.Hash().Hex())
			}
		}
	}()

	// Wait for transactions to complete
	wg.Wait()
	time.Sleep(15 * time.Second)

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
				log.Fatalf("Block %d: Order error, config.Addr1 (%s) after config.Addr3 (%s), tx: %s", i, from.Hex(), lastFrom.Hex(), tx.Hash().Hex())
			}
			lastFrom = from
		}
	}
	log.Printf("SortTest %s completed ✅\n", endpoint)
}

// sendBatchTxs Send batch transactions
func sendBatchTxs(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, count int, txs *[]*types.Transaction) {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Printf("failed to get nonce for %s: %v", auth.From.Hex(), err)
		return
	}

	for i := 0; i < count; i++ {
		if i%config.PrintMod == 0 {
			log.Printf("sending MNT %d", i)
		}
		tx, err := config.SendNativeToken(ctx, client, auth, to, amount, nonce+uint64(i))
		if err != nil {
			log.Printf("failed to send mnt %d: %v", i, err)
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
		if i%config.PrintMod == 0 {
			log.Printf("sending MNT with preconf %d", i)
		}
		tx, err := config.SendMNTWithPreconf(ctx, client, auth, to, amount, nonce+uint64(i))
		if err != nil {
			if strings.Contains(err.Error(), "nonce too low") {
				log.Printf("preconf tx replaced by deposit tx, from: %s, nonce: %d, tx: %s", auth.From.Hex(), nonce+uint64(i), tx.Hash().Hex())
				continue
			}
			if strings.Contains(err.Error(), miner.ErrEnvBlockNumberAndEngineSyncTargetBlockNumberDistanceTooLarge.Error()) { // env block number and engine sync target block number distance too large
				log.Printf("env block number and engine sync target block number distance too large, wait for new preconf tx: %s", tx.Hash().Hex())
				time.Sleep(config.WaitTime)
				continue
			}

			log.Fatalf("failed to send mnt with preconf %d: %v, tx: %s", i, err, tx.Hash().Hex())
		}
		*txs = append(*txs, tx)
		time.Sleep(config.NonceInterval)
	}
}

// sendBatchDepositTxs Send batch deposit transactions
func sendBatchDepositTxs(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, count int) []*types.Transaction {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Printf("failed to get nonce for %s: %v", auth.From.Hex(), err)
		return nil
	}

	var txs []*types.Transaction
	for i := 0; i < count; i++ {
		if i%config.PrintMod == 0 {
			log.Printf("sending DepositTx %d", i)
		}
		tx, err := config.SendDepositTx(ctx, client, auth, to, "0x", amount, nonce+uint64(i))
		if err != nil {
			log.Printf("failed to send deposit tx %d: %v", i, err)
			continue
		}
		txs = append(txs, tx)
		time.Sleep(config.NonceInterval)
	}

	return txs
}
