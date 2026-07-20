package config

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Configuration
const (
	NumTransactions     = 1000
	L1RpcEndpoint       = "http://127.0.0.1:38545"
	SequencerEndpoint   = "http://127.0.0.1:9545"
	L2RpcEndpoint       = "http://127.0.0.1:19545"
	FundKeyHex          = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
	OptimismPortalProxy = "0x5FC8d32690cc91D4c39d9d3abcBD16989F875707"
	ToAddressHex        = "0x71920E3cb420fbD8Ba9a495E6f801c50375ea127"
	BatchSize           = 10
	NonceInterval       = 20 * time.Millisecond
	Addr1Pk             = "e474bfa0d1520cf4b161b382db9f527c39ac16b6d9a8351f091bd406f739a691" // 0x6F18bEEF53452dC646C5221900F1EfE8b6B4BDc5
	Addr3Pk             = "654c6b97f400c2facec28bcb2ae04f2bf99e007bd6e41b2ce221481e30840e49" // 0x918a3880A91308279C06A89415d01ae47d64eC29
	TransferGasLimit    = 210000000
	PrintMod            = 100
	WaitTime            = 5 * time.Second
)

var (
	TestERC20    = common.HexToAddress("0x5FbDB2315678afecb367f032d93F642f64180aa3")
	TestPay      = common.HexToAddress("0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512")
	FunderKey, _ = crypto.HexToECDSA(FundKeyHex)
	Addr1Key, _  = crypto.HexToECDSA(Addr1Pk)
	Addr3Key, _  = crypto.HexToECDSA(Addr3Pk)
	FundAddr     = crypto.PubkeyToAddress(FunderKey.PublicKey)
	Addr1        = crypto.PubkeyToAddress(Addr1Key.PublicKey)
	Addr2        = common.HexToAddress(ToAddressHex)
	Addr3        = crypto.PubkeyToAddress(Addr3Key.PublicKey)
	// TestERC20 calldata
	APPROVEDATA     = fmt.Sprintf("0x095ea7b3000000000000000000000000%s00000000000000000000000000000000000000000000d3c21bcecceda0ffffff", TestPay.Hex()[2:])
	MINTDATA        = fmt.Sprintf("0x40c10f19000000000000000000000000%s0000000000000000000000000000000000000000000000000de0b6b3a7640000", Addr3.Hex()[2:])
	BALANCEOFDATA   = "0x70a08231000000000000000000000000%s"
	ALLOWANCEOFDATA = fmt.Sprintf("0xdd62ed3e000000000000000000000000%s000000000000000000000000%s", Addr3.Hex()[2:], TestPay.Hex()[2:])
	TRANSFERDATA    = "0x2ccb1b30000000000000000000000000%s%s" // e.g. 0x2ccb1b3000000000000000000000000071920E3cb420fbD8Ba9a495E6f801c50375ea1270000000000000000000000000000000000000000000000000de0b6b3a7640000
	// TestPay calldata
	PAYDATA = "0xa5f2a152000000000000000000000000%s000000000000000000000000%s%s" // e.g. 0xa5f2a15200000000000000000000000071920E3cb420fbD8Ba9a495E6f801c50375ea1270000000000000000000000000000000000000000000000000de0b6b3a7640000
	// DepositTx
	DepositAddr = common.HexToAddress(OptimismPortalProxy)
	// DepositData = "0x40c10f19000000000000000000000000%s0000000000000000000000000000000000000000000000000de0b6b3a7640000"
)

func BalanceString(balance *big.Int) string {
	return new(big.Float).Quo(new(big.Float).SetInt(balance), big.NewFloat(1e18)).String()
}

// GetBalance Get account balance
func GetBalance(ctx context.Context, client *ethclient.Client, addr common.Address) *big.Int {
	balance, err := client.BalanceAt(ctx, addr, nil)
	if err != nil {
		log.Printf("failed to get balance for %s: %v", addr.Hex(), err)
		return big.NewInt(0)
	}
	return balance
}

func GetNonce(ctx context.Context, client *ethclient.Client, addr common.Address) uint64 {
	nonce, err := client.PendingNonceAt(ctx, addr)
	if err != nil {
		log.Printf("failed to get nonce for %s: %v", addr.Hex(), err)
		return 0
	}
	return nonce
}
func SendDepositTx(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, data string, l2MsgValue *big.Int, nonce uint64) (*types.Transaction, error) {
	if nonce == 0 {
		var err error
		nonce, err = client.PendingNonceAt(ctx, auth.From)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
	}

	abiJSON := `[
		{
			"name": "depositTransaction",
			"type": "function",
			"inputs": [
				{"name": "_ethTxValue", "type": "uint256"},
				{"name": "_mntValue", "type": "uint256"},
				{"name": "_to", "type": "address"},
				{"name": "_mntTxValue", "type": "uint256"},
				{"name": "_gasLimit", "type": "uint64"},
				{"name": "_isCreation", "type": "bool"},
				{"name": "_data", "type": "bytes"}
			]
		}
	]`

	// Parse ABI
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		log.Fatalf("Failed to parse ABI: %v\n", err)
	}

	// Prepare parameters
	args := []any{
		big.NewInt(0),            // uint256: 0
		big.NewInt(0),            // uint256: 0
		to,                       // address
		l2MsgValue,               // uint256: 0
		uint64(100000),           // uint64: 210000000
		false,                    // bool: false
		hexutil.MustDecode(data), // bytes
	}

	// Encode calldata
	calldata, err := parsedABI.Pack("depositTransaction", args...)
	if err != nil {
		log.Fatalf("failed to pack calldata: %v", err)
	}

	// Output result (hexadecimal)
	// log.Printf("Calldata: 0x%x\n", calldata)

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &DepositAddr,
		Data:  calldata,
		Value: big.NewInt(0),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %v", err)
	}
	// log.Println("deposit tx gas", gas)

	// gasPrice, err := client.SuggestGasPrice(ctx)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to suggest gas price: %v", err)
	// }

	tx := types.NewTransaction(nonce, DepositAddr, big.NewInt(0), gas*120/100, big.NewInt(1e12), calldata)
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	return signedTx, nil
}

// SendNativeToken Send MNT/ETH transaction
func SendNativeToken(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, nonce uint64) (*types.Transaction, error) {
	if nonce == 0 {
		var err error
		nonce, err = client.PendingNonceAt(ctx, auth.From)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
	}
	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %v", err)
	}

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  auth.From,
		To:    &to,
		Value: amount,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to estimate gas: %v", err)
	}
	// log.Println("send native token gas", gas)

	tx := types.NewTransaction(nonce, to, amount, gas, gasPrice, nil)
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}
	if err := client.SendTransaction(ctx, signedTx); err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}
	return signedTx, nil
}

// SendMNTWithPreconf Send MNT transaction with pre-confirmed
func SendMNTWithPreconf(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, to common.Address, amount *big.Int, nonce uint64) (*types.Transaction, error) {
	if nonce == 0 {
		var err error
		nonce, err = client.PendingNonceAt(ctx, auth.From)
		if err != nil {
			return nil, fmt.Errorf("failed to get nonce: %v", err)
		}
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to suggest gas price: %v", err)
	}

	tx := types.NewTransaction(nonce, to, amount, TransferGasLimit, gasPrice, nil)
	signedTx, err := auth.Signer(auth.From, tx)
	if err != nil {
		return nil, fmt.Errorf("failed to sign transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	if err := client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		return signedTx, fmt.Errorf("failed to send transaction with pre-confirmed: %v", err)
	}
	if result.Status == core.PreconfStatusFailed {
		return signedTx, fmt.Errorf("transaction pre-confirmed failed: %s, %s", result.Reason, result.TxHash)
	}
	return signedTx, nil
}

// FundAccount Fund the account with initial amount
func FundAccount(ctx context.Context, client *ethclient.Client, to common.Address, amount *big.Int) error {
	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("failed to get L2 chain ID: %v", err)
	}

	funderAuth, err := bind.NewKeyedTransactorWithChainID(FunderKey, chainID)
	if err != nil {
		log.Fatalf("failed to create funder signer: %v", err)
	}

	tx, err := SendNativeToken(ctx, client, funderAuth, to, amount, 0)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, WaitTime)
	defer cancel()
	_, err = bind.WaitMined(ctx, client, tx)
	if err != nil {
		return fmt.Errorf("failed to wait for send native token transaction %s confirmation: %v", tx.Hash().Hex(), err)
	}
	log.Printf("Funded account %s with %s MNT", to.Hex(), BalanceString(amount))
	return nil
}

func GetL1Auth(ctx context.Context, privateKey *ecdsa.PrivateKey) (*ethclient.Client, *bind.TransactOpts, error) {
	return GetAuth(ctx, L1RpcEndpoint, privateKey)
}

func GetAuth(ctx context.Context, rpc string, privateKey *ecdsa.PrivateKey) (*ethclient.Client, *bind.TransactOpts, error) {
	client, err := ethclient.Dial(rpc)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to L1 RPC: %v", err)
	}
	defer client.Close()
	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get L1 chain ID: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, chainID)
	if err != nil {
		log.Fatalf("failed to create %s signer: %v", crypto.PubkeyToAddress(privateKey.PublicKey).Hex(), err)
	}
	return client, auth, nil
}
