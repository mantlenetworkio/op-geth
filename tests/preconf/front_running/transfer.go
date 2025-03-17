package frontrunning

import (
	"context"
	"log"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/core/txpool"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
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
	fundAmount := new(big.Int).Mul(big.NewInt(1e4), oneMNT) // 10000 MNT
	if err := config.FundAccount(ctx, client, config.Addr1, fundAmount); err != nil {
		log.Fatalf("failed to fund config.Addr1: %v", err)
	}

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
	if nonce < 50 {
		log.Fatalf("config.Addr1 nonce is less than 50")
	}

	balance := config.GetBalance(ctx, client, config.Addr1)
	log.Printf("config.Addr1 nonce: %d, balance: %s MNT", nonce, config.BalanceString(balance))

	// transferAmount = (balance - 100*gasfee)/100
	gasFee := new(big.Int).Mul(big.NewInt(config.TransferGasLimit), config.GasPrice)
	_100TimesGasFee := new(big.Int).Mul(gasFee, big.NewInt(int64(100+config.NumTransactions)))
	transferAmount := new(big.Int).Div(new(big.Int).Sub(balance, _100TimesGasFee), big.NewInt(int64(config.NumTransactions)))
	log.Printf("transferAmount: %s MNT, gasFee: %s MNT\n", config.BalanceString(transferAmount), config.BalanceString(gasFee))

	// consumer all the balance of config.Addr1 using config.NumTransactions
	sendNonce := nonce - 50
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
				if strings.Contains(err.Error(), txpool.ErrOverdraft.Error()) { // balance is not enough
					// fmt.Printf("expected error: %v, now: %d\n", err, currentNonce)
					continue
				}
				log.Fatalf("should be no error: %v", err)
			}
			txs = append(txs, tx)
			// fmt.Printf("valid nonce in range [%d, %d), no error, now: %d, tx: %s\n", nonce-50, nonce+config.NumTransactions, currentNonce, tx.Hash().Hex())
		}
	}

	for _, tx := range txs {
		ctx, cancel := context.WithTimeout(ctx, config.PrintMod)
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
