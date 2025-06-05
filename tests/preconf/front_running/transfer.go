package frontrunning

import (
	"context"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

func TransferTest() {
	transferTest(config.SequencerEndpoint)
	transferTest(config.L2RpcEndpoint)
}

// consumer all the balance of config.Addr1 using config.NumTransactions
func transferTest(endpoint string) {
	log.Printf("TransferTest %s starting ...\n", endpoint)
	defer log.Printf("TransferTest %s completed\n", endpoint)

	ctx := context.Background()

	client, err := ethclient.Dial(endpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
	}
	defer client.Close()

	// Fund config.Addr1
	oneMNT := big.NewInt(1e18)
	fundAmount := new(big.Int).Mul(big.NewInt(config.NumTransactions), oneMNT) // 10000 MNT
	config.FundAccount(ctx, client, config.Addr1, fundAmount)

	// Get L2 chain ID
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		log.Fatalf("failed to get L2 chain ID: %v", err)
	}

	addr1Auth, err := bind.NewKeyedTransactorWithChainID(config.Addr1Key, chainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr1 signer: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, config.Addr1)
	if err != nil {
		log.Fatalf("failed to get config.Addr1 nonce: %v", err)
	}
	sendNonce := nonce
	if nonce > 50 {
		sendNonce = nonce - 50
	}

	balance := config.GetBalance(ctx, client, config.Addr1)
	log.Printf("config.Addr1 nonce: %d, balance: %s MNT", nonce, config.BalanceString(balance))

	transferAmount := big.NewInt(1e14)
	log.Printf("transferAmount: %s MNT\n", config.BalanceString(transferAmount))

	// consumer all the balance of config.Addr1 using config.NumTransactions

	txs := make([]*types.Transaction, 0, 100)
	for i := 0; i < 100+config.NumTransactions; i++ {
		currentNonce := sendNonce + uint64(i)
		tx, err := config.SendMNTWithPreconf(ctx, client, addr1Auth, config.Addr2, transferAmount, currentNonce)
		if currentNonce < nonce { // nonce too low
			if err == nil {
				log.Fatalf("should be error for nonce too low")
			}
			// fmt.Printf("<nonce error[%d, %d): %v, now: %d\n", nonce-50, nonce, err, currentNonce)
		} else { // valid nonce
			if err != nil {
				// if strings.Contains(err.Error(), txpool.ErrOverdraft.Error()) { // balance is not enough
				// 	// fmt.Printf("expected error: %v, now: %d\n", err, currentNonce)
				// 	continue
				// }
				if strings.Contains(err.Error(), core.ErrNonceTooLow.Error()) { // nonce too low
					log.Printf("nonce too low, bug? error: %v, tx: %s", err, tx.Hash().Hex())
					continue
				}
				if strings.Contains(err.Error(), core.ErrGasLimitReached.Error()) { // gas limit reached
					log.Printf("this tx will in the next block? %v", err)
					continue
				}
				if strings.Contains(err.Error(), miner.ErrEnvBlockNumberAndEngineSyncTargetBlockNumberDistanceTooLarge.Error()) { // env block number and engine sync target block number distance too large
					log.Printf("env block number and engine sync target block number distance too large, wait for new preconf tx: %s", tx.Hash().Hex())
					time.Sleep(config.WaitTime)
					continue
				}
				log.Fatalf("should be no error: %v", err)
			}
			txs = append(txs, tx)
			// fmt.Printf("valid nonce in range [%d, %d), no error, now: %d, tx: %s\n", nonce-50, nonce+config.NumTransactions, currentNonce, tx.Hash().Hex())
		}
	}

	for _, tx := range txs {
		ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
		defer cancel()
		receipt, err := bind.WaitMined(ctx, client, tx)
		if err != nil {
			log.Fatalf("failed to wait for transaction %s confirmation: %v", tx.Hash().Hex(), err)
		}
		// fmt.Println("tx", tx.Hash().Hex(), "receipt", receipt.Status)
		if receipt.Status == types.ReceiptStatusFailed {
			log.Fatalf("transaction %s failed", tx.Hash().Hex())
		}
	}

	nonce, _ = client.PendingNonceAt(ctx, config.Addr1)
	balance = config.GetBalance(ctx, client, config.Addr1)
	log.Printf("config.Addr1 nonce: %d, balance: %s MNT", nonce, config.BalanceString(balance))

	log.Printf("TransferTest ✅\n")
}
