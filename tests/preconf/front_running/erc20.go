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
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/internal/ethapi"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
)

func ERC20Test() {
	erc20Test(config.SequencerEndpoint)
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

	checkERC20(ctx, client)

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
	err = sendERC20Tx(ctx, client, addr3Auth, config.MINTDATA)
	if err != nil {
		log.Fatalf("failed to send mint transaction: %v", err)
	}

	log.Printf("addr1 balance: %s MNT", config.BalanceString(config.GetBalance(ctx, client, config.Addr1)))

	// check addr3 erc20 balance
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
	wg.Add(2)

	go func() {
		defer wg.Done()

		time.Sleep(1 * time.Second) // let user transfer go first
		nonce := config.GetNonce(ctx, client, addr1Auth.From)
		for i := 0; i < config.NumTransactions; i++ {
			if err := pay(ctx, client, addr1Auth, i, nonce+uint64(i), transferAmount, &addr1Txs, &addr1PreconfFailedTx); err != nil {
				if strings.Contains(err.Error(), "transaction preconf failed") {
					continue
				}
				log.Fatalf("failed to pay: %v", err)
			}
		}
		for _, tx := range addr1Txs {
			ctx, cancel := context.WithTimeout(ctx, 1*time.Second)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, client, tx)
			if err == nil && receipt != nil {
				if receipt.Status != types.ReceiptStatusSuccessful {
					log.Fatalf("Preconf Transaction %s failed but preconf succeed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
				}
			}
		}
		for _, tx := range addr1PreconfFailedTx {
			ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			receipt, err := bind.WaitMined(ctx, client, tx)
			if err == nil && receipt != nil {
				if receipt.Status == types.ReceiptStatusSuccessful {
					log.Fatalf("Preconf Transaction %s succeed but preconf failed - Status: %d, Actual Block: %d\n", tx.Hash(), receipt.Status, receipt.BlockNumber.Uint64())
				}
			}
		}
	}()

	go func() {
		defer wg.Done()
		nonce := config.GetNonce(ctx, client, addr3Auth.From)
		for i := 0; i < config.NumTransactions; i++ {
			err := transfer(ctx, client, nonce+uint64(i), transferAmount, addr3Auth, &addr3Txs)
			if err != nil {
				if strings.Contains(err.Error(), "execution reverted: insufficient balance") {
					continue
				}
				log.Fatalf("failed to transfer: %v", err)
			}
		}
		for _, tx := range addr3Txs {
			ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
			defer cancel()
			_, err := bind.WaitMined(ctx, client, tx)
			if err != nil {
				log.Fatalf("failed to wait for transaction: %v", err)
			}
		}
	}()

	// Wait for transactions to complete
	wg.Wait()
	time.Sleep(5 * time.Second)

	// check addr3 erc20 balance
	addr1AfterBalance := erc20Balance(ctx, client, config.Addr1)
	addr2AfterBalance := erc20Balance(ctx, client, config.Addr2)
	addr3AfterBalance := erc20Balance(ctx, client, config.Addr3)

	addr1AfterBalance.Sub(addr1AfterBalance, addr1BeforeBalance)
	addr2AfterBalance.Sub(addr2AfterBalance, addr2BeforeBalance)
	addr3AfterBalance.Sub(addr3BeforeBalance, addr3AfterBalance)

	add := big.NewInt(0).Add(addr1AfterBalance, addr2AfterBalance)
	if add.Cmp(addr3AfterBalance) != 0 {
		log.Fatalf("addr1 + addr2 is not equal to addr3 sub")
	}
	fmt.Println("erc20 test completed✅")
}

func erc20Balance(ctx context.Context, client *ethclient.Client, addr common.Address) *big.Int {
	balance := callERC20(ctx, client, fmt.Sprintf(config.BALANCEOFDATA, addr.Hex()[2:]))
	var balanceInt big.Int
	balanceInt.SetBytes(balance)
	log.Printf("addr %s erc20 balance: %s TestERC20", addr.Hex(), config.BalanceString(&balanceInt))
	return &balanceInt
}

func checkERC20(ctx context.Context, client *ethclient.Client) {
	code, err := client.CodeAt(ctx, config.TestERC20, nil)
	if err != nil {
		log.Fatalf("failed to get TestERC20 code at %s: %v", config.TestERC20.Hex(), err)
	}
	if len(code) == 0 {
		log.Fatalf("TestERC20 code is empty, deploy it first")
	}

	code, err = client.CodeAt(ctx, config.TestPay, nil)
	if err != nil {
		log.Fatalf("failed to get TestPay code at %s: %v", config.TestPay.Hex(), err)
	}
	if len(code) == 0 {
		log.Fatalf("TestPay code is empty, deploy it first")
	}

	// 1 * Number.Transactions * 1e18
	foundAmount := big.NewInt(10).Mul(big.NewInt(config.NumTransactions), big.NewInt(1e18))
	config.FundAccount(ctx, client, config.Addr1, foundAmount)
	config.FundAccount(ctx, client, config.Addr3, foundAmount)

	// todo - go auto deploy TestERC20/TestPay
	// 1. Deploy TestERC20/TestPay
	// 2. setERC20Address in TestPay
	// 3. set TestPay address in ToPreconfs of op-geth
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

	tx := types.NewTransaction(
		config.GetNonce(ctx, client, auth.From),
		config.TestERC20,
		big.NewInt(0),
		gas,
		config.GasPrice,
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

	receipt, err := bind.WaitMined(ctx, client, signedTx)
	if err != nil {
		return fmt.Errorf("failed to wait for transaction: %v", err)
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
	// gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
	// 	From:  auth.From,
	// 	To:    &config.TestPay,
	// 	Data:  data,
	// 	Value: big.NewInt(0),
	// })
	// if err != nil {
	// 	return fmt.Errorf("failed to estimate gas: %v", err)
	// }
	// fmt.Println("gas", gas) //138875846

	tx := types.NewTransaction(
		nonce,
		config.TestPay,
		big.NewInt(0),
		// gas,
		1400000000,
		config.GasPrice,
		data,
	)

	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return fmt.Errorf("signing transaction: %v", err)
	}

	var result ethapi.PreconfTransactionResult
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	if err != nil {
		return fmt.Errorf("failed to send transaction: %v", err)
	}

	if result.Status == "failed" {
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

	tx := types.NewTransaction(
		nonce,
		config.TestERC20,
		big.NewInt(0),
		gas,
		config.GasPrice,
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
