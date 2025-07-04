package frontrunning

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/miner"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

/**
// contractAddr is the deployed TestERC20 contract address
contractAddr := common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3")
// user address
userAddr := common.HexToAddress("0x918a3880A91308279C06A89415d01ae47d64eC29")

// call getBalance function
// balanceOf mapping slot is 0 (assuming it's the first variable in the contract)
slot := big.NewInt(0)

// calculate storage position: keccak256(userAddr, slot)
key := common.BytesToHash(userAddr.Bytes()) // user address, padded to 32 bytes
base := common.BigToHash(slot)              // slot 0, converted to 32 bytes
data := append(key[:], base[:]...)          // concatenate userAddr and slot
hash := crypto.Keccak256Hash(data)          // calculate keccak256 hash

// get storage value from stateDB
balanceRaw := statedb.GetState(contractAddr, hash)

// convert return value to big.Int
balance := new(big.Int).SetBytes(balanceRaw.Bytes())
fmt.Printf("beforetx %s Balance of %s: %s\n", tx.Hash().Hex(), userAddr.Hex(), balance.String())
defer func() {
	balanceRaw := statedb.GetState(contractAddr, hash)

	// convert return value to big.Int
	balance := new(big.Int).SetBytes(balanceRaw.Bytes())
	fmt.Printf("after tx %s Balance of %s: %s\n", tx.Hash().Hex(), userAddr.Hex(), balance.String())
}()
*/

func ERC20Test() {
	time.Sleep(30 * time.Second) // wait for sequencer to sync
	erc20Test(config.SequencerEndpoint)
	time.Sleep(30 * time.Second) // wait for sequencer to sync
	erc20Test(config.L2RpcEndpoint)
}

func erc20Test(endpoint string) {
	// consumer all the balance of config.Addr1 using config.NumTransactions
	log.Printf("ERC20Test %s starting ...\n", endpoint)
	defer log.Printf("ERC20Test %s completed\n", endpoint)

	ctx := context.Background()

	client, err := ethclient.Dial(endpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
	}
	defer client.Close()

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		log.Fatalf("failed to get L2 chain ID: %v", err)
	}

	addr1Auth, err := bind.NewKeyedTransactorWithChainID(config.Addr1Key, chainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr1 signer: %v", err)
	}

	addr3Auth, err := bind.NewKeyedTransactorWithChainID(config.Addr3Key, chainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr3 signer: %v", err)
	}

	// approve TestPay using addr3
	err = sendERC20Tx(ctx, client, addr3Auth, config.APPROVEDATA)
	if err != nil {
		log.Fatalf("failed to send approve transaction: %v", err)
	}

	// mint 1e18 ERC20 to addr3
	time.Sleep(time.Second)
	err = sendERC20Tx(ctx, client, addr3Auth, config.MINTDATA)
	if err != nil {
		log.Fatalf("failed to send mint transaction: %v", err)
	}

	log.Printf("addr1 balance: %s MNT", config.BalanceString(config.GetBalance(ctx, client, config.Addr1)))

	// check addresses erc20 balance
	addrFundBeforeBalance := erc20Balance(ctx, client, config.FundAddr)
	addr1BeforeBalance := erc20Balance(ctx, client, config.Addr1)
	addr2BeforeBalance := erc20Balance(ctx, client, config.Addr2)
	addr3BeforeBalance := erc20Balance(ctx, client, config.Addr3)

	// check addr3 erc20 allowance
	allowance := callERC20(ctx, client, config.ALLOWANCEOFDATA)
	var allowanceInt big.Int
	allowanceInt.SetBytes(allowance)
	log.Printf("addr3 erc20 allowance: %s TestERC20", config.BalanceString(&allowanceInt))

	// balance/numtransactions
	transferAmount := big.NewInt(0).Div(big.NewInt(1e18), big.NewInt(config.NumTransactions))
	fmt.Println("transferAmount", config.BalanceString(transferAmount))

	// Send batch transactions
	var wg sync.WaitGroup
	var addr1Txs, addr1PreconfFailedTx, addr3Txs []*types.Transaction
	wg.Add(3)

	// 3 deposit to funder
	go func() {
		defer wg.Done()

		l1client, l1Addr3Auth, err := config.GetL1Auth(ctx, config.Addr3Key)
		if err != nil {
			log.Fatalf("failed to get L1 auth: %v", err)
		}
		fundAmount := big.NewInt(0).Mul(big.NewInt(1e18), big.NewInt(config.NumTransactions*5))
		config.FundAccount(ctx, l1client, config.Addr3, fundAmount)
		time.Sleep(12 * time.Second) // wait for funder tx to be sent

		depositTxs := make([]*types.Transaction, 0)
		for i := 0; i < config.NumTransactions/20+1; i++ {
			if i%(config.PrintMod/20) == 0 {
				log.Printf("depositTx %d", i)
			}
			datastring := fmt.Sprintf(config.TRANSFERDATA, config.FundAddr.Hex()[2:], hex.EncodeToString(common.LeftPadBytes(transferAmount.Bytes(), 32)))
			tx, err := config.SendDepositTx(ctx, l1client, l1Addr3Auth, config.TestERC20, datastring, big.NewInt(0), 0)
			if err != nil {
				log.Fatalf("failed to send deposit transaction: %v", err)
			}
			depositTxs = append(depositTxs, tx)

			time.Sleep(300 * time.Millisecond)
		}
		time.Sleep(3 * config.WaitTime)
		fmt.Println("deposit txs", len(depositTxs))
		var wg1 sync.WaitGroup
		wg1.Add(len(depositTxs))
		sem := make(chan struct{}, 8)
		for _, tx := range depositTxs {
			sem <- struct{}{}
			go func() {
				defer wg1.Done()
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(ctx, 6*config.WaitTime)
				defer cancel()
				receipt, err := bind.WaitMined(ctx, l1client, tx)
				if err != nil {
					log.Fatalf("failed to wait for deposit transaction: tx: %s, %v", tx.Hash().Hex(), err)
				}
				if receipt.Status != types.ReceiptStatusSuccessful {
					log.Fatalf("deposit transaction failed: %v, tx: %s", receipt.Status, tx.Hash().Hex())
				}
				// fmt.Println("deposit tx success", tx.Hash().Hex(), "block number", receipt.BlockNumber.Uint64())
			}()
		}
		wg1.Wait()
		fmt.Println("deposit txs done")
	}()

	// 3 pay 1e18 to 1
	go func() {
		defer wg.Done()

		log.Printf("waiting for deposit tx and user transfer go first, and then pay")
		time.Sleep(50 * time.Second) // let deposit tx and user transfer go first
		nonce := config.GetNonce(ctx, client, addr1Auth.From)
		for i := 0; i < config.NumTransactions; i++ {
			if i%config.PrintMod == 0 {
				log.Printf("paying %d", i)
			}
			if err := pay(ctx, client, addr1Auth, i, nonce+uint64(i), transferAmount, &addr1Txs, &addr1PreconfFailedTx); err != nil {
				if strings.Contains(err.Error(), miner.ErrEnvBlockNumberAndEngineSyncTargetBlockNumberDistanceTooLarge.Error()) {
					log.Println("so many preconf txs, wait for new preconf tx")
					time.Sleep(config.WaitTime)
					continue
				}
				if strings.Contains(err.Error(), "transaction preconf failed") {
					continue
				}
				log.Fatalf("failed to pay: %v", err)
			}
		}
		time.Sleep(config.WaitTime)
		fmt.Println("pay txs", len(addr1Txs))
		var wg1 sync.WaitGroup
		wg1.Add(len(addr1Txs))
		sem := make(chan struct{}, 8)
		for _, tx := range addr1Txs {
			sem <- struct{}{}
			go func() {
				defer wg1.Done()
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
				defer cancel()
				receipt, err := bind.WaitMined(ctx, client, tx)
				if err == nil && receipt != nil {
					if receipt.Status != types.ReceiptStatusSuccessful {
						log.Fatalf("Preconf Transaction %s failed but preconf succeed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
					}
				}
			}()
		}
		wg1.Wait()
		fmt.Println("pay txs done")

		fmt.Println("preconf failed txs", len(addr1PreconfFailedTx))
		var wg2 sync.WaitGroup
		wg2.Add(len(addr1PreconfFailedTx))
		sem = make(chan struct{}, 8)
		for _, tx := range addr1PreconfFailedTx {
			sem <- struct{}{}
			go func() {
				defer wg2.Done()
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
				defer cancel()
				receipt, err := bind.WaitMined(ctx, client, tx)
				if err == nil && receipt != nil {
					if receipt.Status == types.ReceiptStatusSuccessful {
						log.Printf("Preconf Transaction %s succeed but preconf failed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
					}
				}
			}()
		}
		wg2.Wait()
		fmt.Println("preconf failed txs done")
	}()

	// 3 transfer 1e18 to 2
	go func() {
		defer wg.Done()

		log.Printf("waiting for deposit tx go first, and then transfer")
		time.Sleep(48 * time.Second) // let deposit tx go first
		nonce := config.GetNonce(ctx, client, addr3Auth.From)
		for i := 0; i < config.NumTransactions; i++ {
			if i%config.PrintMod == 0 {
				log.Printf("transferring %d", i)
			}
			err := transfer(ctx, client, nonce+uint64(i), transferAmount, addr3Auth, &addr3Txs)
			if err != nil {
				if strings.Contains(err.Error(), "execution reverted: insufficient balance") {
					continue
				}
				log.Fatalf("failed to transfer: %v", err)
			}
		}

		// wait for 2 minute to make sure all the txs are in the txpool
		time.Sleep(config.WaitTime)
		fmt.Println("transfer txs", len(addr3Txs))
		var wg1 sync.WaitGroup
		wg1.Add(len(addr3Txs))
		sem := make(chan struct{}, 8)
		for _, tx := range addr3Txs {
			sem <- struct{}{}
			go func() {
				defer wg1.Done()
				defer func() { <-sem }()
				ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
				defer cancel()
				_, err := bind.WaitMined(ctx, client, tx)
				if err != nil {
					if strings.Contains(err.Error(), "context deadline exceeded") {
						log.Printf("transfer tx replaced by deposit tx, from: %s, nonce: %d, tx: %s", addr3Auth.From.Hex(), tx.Nonce(), tx.Hash().Hex())
						return
					}
					log.Fatalf("failed to wait for transfer transaction: %v, tx: %s", err, tx.Hash().Hex())
				}
			}()
		}
		wg1.Wait()
		fmt.Println("transfer txs done")
	}()

	// Wait for transactions to complete
	wg.Wait()
	time.Sleep(5 * time.Second)

	// check addresses erc20 balance
	addrFundAfterBalance := erc20Balance(ctx, client, config.FundAddr)
	addr1AfterBalance := erc20Balance(ctx, client, config.Addr1)
	addr2AfterBalance := erc20Balance(ctx, client, config.Addr2)
	addr3AfterBalance := erc20Balance(ctx, client, config.Addr3)

	addrFundAfterBalance.Sub(addrFundAfterBalance, addrFundBeforeBalance)
	addr1AfterBalance.Sub(addr1AfterBalance, addr1BeforeBalance)
	addr2AfterBalance.Sub(addr2AfterBalance, addr2BeforeBalance)
	addr3AfterBalance.Sub(addr3BeforeBalance, addr3AfterBalance)

	add := big.NewInt(0).Add(addr1AfterBalance, addr2AfterBalance)
	add.Add(add, addrFundAfterBalance)
	if add.Cmp(addr3AfterBalance) != 0 {
		log.Fatalf("addrFund + addr1 + addr2 is not equal to addr3 sub, %s + %s + %s != %s", addrFundAfterBalance.String(), addr1AfterBalance.String(), addr2AfterBalance.String(), addr3AfterBalance.String())
	}
	fmt.Println("erc20 test completed✅")
}

func erc20Balance(ctx context.Context, client *ethclient.Client, addr common.Address) *big.Int {
	balance := callERC20(ctx, client, fmt.Sprintf(config.BALANCEOFDATA, addr.Hex()[2:]))
	var balanceInt big.Int
	balanceInt.SetBytes(balance)
	fmt.Println("addr", addr.Hex(), "balance", balanceInt.String())
	log.Printf("addr %s erc20 balance: %s TestERC20", addr.Hex(), config.BalanceString(&balanceInt))
	return &balanceInt
}

func sendERC20Tx(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, data string) error {
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &config.TestERC20,
		Data:  hexutil.MustDecode(data),
		Value: big.NewInt(0),
	})
	if err != nil {
		return fmt.Errorf("failed to estimate gas: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to suggest gas price: %v", err)
	}

	tx := types.NewTransaction(
		config.GetNonce(ctx, client, auth.From),
		config.TestERC20,
		big.NewInt(0),
		gas,
		gasPrice,
		hexutil.MustDecode(data),
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return fmt.Errorf("signing transaction: %v", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %v", err)
	}

	ctx, cancel := context.WithTimeout(ctx, config.WaitTime)
	defer cancel()
	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return fmt.Errorf("failed to wait for send erc20transaction: %v, tx: %s", err, signedTx.Hash().Hex())
	}
	if receipt.Status != types.ReceiptStatusSuccessful {
		return fmt.Errorf("transaction failed, tx: %s", signedTx.Hash().Hex())
	}

	return nil
}

func callERC20(ctx context.Context, client *ethclient.Client, data string) []byte {
	result, err := client.CallContract(ctx, ethereum.CallMsg{
		To:   &config.TestERC20,
		Data: hexutil.MustDecode(data),
	}, nil)
	if err != nil {
		log.Fatalf("failed to call contract: %v", err)
	}
	return result
}

func pay(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, i int, nonce uint64, amount *big.Int, txs *[]*types.Transaction, preconfFailedTx *[]*types.Transaction) error {
	datastring := fmt.Sprintf("0xa5f2a152000000000000000000000000%s000000000000000000000000%s%s", config.Addr3.Hex()[2:], config.Addr1.Hex()[2:], hex.EncodeToString(common.LeftPadBytes(amount.Bytes(), 32)))
	data := hexutil.MustDecode(datastring)
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &config.TestPay,
		Data:  data,
		Value: big.NewInt(0),
	})
	if err != nil {
		return fmt.Errorf("failed to estimate gas: %v", err)
	}
	// fmt.Println("gas", gas) // 138875846, 302704189

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to suggest gas price: %v", err)
	}

	tx := types.NewTransaction(
		nonce,
		config.TestPay,
		big.NewInt(0),
		gas,
		gasPrice,
		data,
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return fmt.Errorf("signing transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	if err != nil {
		return fmt.Errorf("failed to send transaction with preconf: %v", err)
	}

	if result.Status == core.PreconfStatusFailed {
		*preconfFailedTx = append(*preconfFailedTx, signedTx)
		// log.Printf("transaction preconf failed, i: %d, tx: %s, reason: %v", i, signedTx.Hash().Hex(), result.Reason)
		return fmt.Errorf("transaction preconf failed, tx: %s, reason: %v", signedTx.Hash().Hex(), result.Reason)
	}

	*txs = append(*txs, signedTx)

	return nil
}

func transfer(ctx context.Context, client *ethclient.Client, nonce uint64, transferAmount *big.Int, addr3Auth *bind.TransactOpts, txs *[]*types.Transaction) error {
	datastring := fmt.Sprintf(config.TRANSFERDATA, config.Addr2.Hex()[2:], hex.EncodeToString(common.LeftPadBytes(transferAmount.Bytes(), 32)))
	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  addr3Auth.From,
		To:    &config.TestERC20,
		Data:  hexutil.MustDecode(datastring),
		Value: big.NewInt(0),
	})
	if err != nil {
		return fmt.Errorf("failed to estimate gas: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return fmt.Errorf("failed to suggest gas price: %v", err)
	}

	tx := types.NewTransaction(
		nonce,
		config.TestERC20,
		big.NewInt(0),
		gas,
		gasPrice,
		hexutil.MustDecode(datastring),
	)

	signedTx, err := addr3Auth.Signer(addr3Auth.From, tx)
	if err != nil {
		return fmt.Errorf("signing transaction: %v", err)
	}

	err = client.SendTransaction(ctx, signedTx)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %v", err)
	}

	*txs = append(*txs, signedTx)

	return nil
}
