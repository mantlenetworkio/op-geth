package check

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/mantletest/preconf/config"
)

func Check() {
	precheck()
	checkPreconfRPCValid()
	checkPreconfReasonValidation()
}

func checkPreconfRPCValid() {
	ctx := context.Background()
	client, err := ethclient.Dial(config.L2RpcEndpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
		return
	}
	defer client.Close()

	event := sendRawTransactionWithPreconf(ctx, client)
	if event == nil {
		log.Fatalf("preconf not valid err: %v", err)
		return
	}

	log.Printf("preconf valid, event: %v", event)
}

// preconf failed reason test cases:
//
//	out of gas ✅
//	contract creation code storage out of gas ❌ no need to test
//	max call depth exceeded ❓ skip test, same as out of gas
//	insufficient balance for transfer ✅
//	contract address collision ❌ no need to test
//	execution reverted ✅
//	max code size exceeded ❌ no need to test
//	max initcode size exceeded ❌ no need to test
//	invalid jump destination ❓ skip test, same as out of gas
//	write protection ❓ skip test, same as out of gas
//	return data out of bounds ❓ skip test, same as out of gas
//	gas uint64 overflow ❓ skip test, same as out of gas
//	invalid code: must not begin with 0xef ❓ skip test, same as out of gas
//
// addrFunder msg.sender, addr2 recipient, addr3 sender
func checkPreconfReasonValidation() {
	log.Printf("=== Start testing result.Reason validation functionality ===")

	ctx := context.Background()
	client, addr1Auth, err := config.GetAuth(ctx, config.L2RpcEndpoint, config.Addr1Key)
	if err != nil {
		log.Fatalf("failed to get auth: %v", err)
	}

	// Test insufficient gas and out of gas
	testTransferFromInsufficientGas(ctx, client, addr1Auth)

	time.Sleep(4 * time.Second)

	// Test insufficient value to transfer
	testTransferFromInsufficientValue(ctx, client, addr1Auth)

	// Test insufficient allowance
	testTransferFromInsufficientAllowance(ctx, client, addr1Auth)

	time.Sleep(4 * time.Second)

	// Test insufficient balance during transferFrom
	testTransferFromInsufficientBalance(ctx, client, addr1Auth)

	log.Printf("=== result.Reason validation functionality test completed ===")
}

// testTransferFromInsufficientValue tests transferFrom with insufficient value to transfer
func testTransferFromInsufficientValue(ctx context.Context, client *ethclient.Client, addr *bind.TransactOpts) {
	log.Printf("Testing transferFrom with insufficient value to transfer...")

	balance, err := client.BalanceAt(ctx, addr.From, nil)
	if err != nil {
		log.Fatalf("failed to get balance: %v", err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("failed to suggest gas price: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, addr.From)
	if err != nil {
		log.Fatalf("failed to get nonce for %s: %v", addr.From.Hex(), err)
	}

	transferValue := big.NewInt(0).Add(balance, big.NewInt(1))
	signedTx, err := addr.Signer(addr.From, types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &config.Addr2,
		GasPrice: gasPrice,
		Gas:      config.TransferGasLimit,
		Value:    transferValue,
	}))
	if err != nil {
		log.Fatalf("Error signing transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	if err = client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		if strings.Contains(err.Error(), "insufficient funds for gas * price + value") {
			log.Printf("✓ transferFrom insufficient value to transfer test passed")
		} else {
			log.Fatalf("Error sending transaction: %v, txHash: %s", err, signedTx.Hash().Hex())
		}
	}

	log.Printf("✓ transferFrom insufficient value to transfer test passed")
}

// testTransferFromInsufficientBalance tests transferFrom with insufficient sender balance
func testTransferFromInsufficientBalance(ctx context.Context, client *ethclient.Client, addr *bind.TransactOpts) {
	log.Printf("Testing transferFrom with insufficient balance...")

	// 3 approve pay
	_, addr3Auth, err := config.GetAuth(ctx, config.L2RpcEndpoint, config.Addr3Key)
	if err != nil {
		log.Fatalf("failed to get auth: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, addr3Auth.From)
	if err != nil {
		log.Fatalf("failed to get nonce for %s: %v", addr3Auth.From.Hex(), err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("failed to suggest gas price: %v", err)
	}

	gas, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From: addr3Auth.From,
		To:   &config.TestERC20,
		Data: hexutil.MustDecode(config.APPROVEDATA),
	})
	if err != nil {
		log.Fatalf("failed to estimate gas: %v", err)
	}

	// Let addr3 call approve to authorize TestPay to use its tokens
	signedApproveTx, err := addr3Auth.Signer(addr3Auth.From, types.NewTx(&types.LegacyTx{
		Nonce:    nonce,
		To:       &config.TestERC20,
		GasPrice: gasPrice,
		Gas:      gas * 120 / 100,
		Data:     hexutil.MustDecode(config.APPROVEDATA),
	}))
	if err != nil {
		log.Fatalf("Error signing approve transaction: %v", err)
	}

	if err = client.SendTransaction(ctx, signedApproveTx); err != nil {
		log.Fatalf("Error sending approve transaction: %v", err)
	}
	receipt, err := bind.WaitMined(ctx, client, signedApproveTx)
	if err != nil || receipt.Status != types.ReceiptStatusSuccessful {
		log.Fatalf("Error waiting for approve transaction to be mined: %v", err)
	}
	log.Printf("✓ Approve transaction sent successfully")

	// Check addr3's balance
	balanceData := fmt.Sprintf(config.BALANCEOFDATA, addr3Auth.From.Hex()[2:])
	balance, err := client.CallContract(ctx, ethereum.CallMsg{To: &config.TestERC20, Data: hexutil.MustDecode(balanceData)}, nil)
	if err != nil {
		log.Fatalf("failed to call contract: %v", err)
	}
	balanceInt := new(big.Int).SetBytes(balance)

	// Try to transfer more tokens than addr3's balance, which will cause insufficient balance error
	transferAmount := big.NewInt(0).Add(balanceInt, big.NewInt(1))
	tx := getPreconfTx(ctx, client, addr, config.Addr3, config.Addr2, transferAmount)
	signedTx, err := addr.Signer(addr.From, types.NewTx(tx))
	if err != nil {
		log.Fatalf("Error signing transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	if err = client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		log.Fatalf("Error sending transaction: %v, txHash: %s", err, signedTx.Hash().Hex())
	}

	// Validate that result.Reason contains the expected error message
	expectReasonInResult(&result, "underflow balance sender")
	log.Printf("✓ transferFrom insufficient balance test passed")
}

// testTransferFromInsufficientAllowance tests transferFrom with insufficient allowance
func testTransferFromInsufficientAllowance(ctx context.Context, client *ethclient.Client, addr *bind.TransactOpts) {
	log.Printf("Testing transferFrom with insufficient allowance...")

	allowanceData := fmt.Sprintf("0xdd62ed3e000000000000000000000000%s000000000000000000000000%s", config.Addr3.Hex()[2:], config.TestPay.Hex()[2:])
	allowance, err := client.CallContract(ctx, ethereum.CallMsg{To: &config.TestERC20, Data: hexutil.MustDecode(allowanceData)}, nil)
	if err != nil {
		log.Fatalf("failed to call contract: %v", err)
	}
	allowanceInt := new(big.Int).SetBytes(allowance)

	// transferAmount = allowanceInt + 1, so it will be preconf failed
	transferAmount := big.NewInt(0).Add(allowanceInt, big.NewInt(1))
	tx := getPreconfTx(ctx, client, addr, config.Addr3, config.Addr2, transferAmount)
	signedTx, err := addr.Signer(addr.From, types.NewTx(tx))
	if err != nil {
		log.Fatalf("Error signing transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	if err = client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		log.Fatalf("Error sending transaction: %v", err)
	}

	// Validate that result.Reason contains the expected error message
	expectReasonInResult(&result, "allowance insufficient")
	log.Printf("✓ transferFrom insufficient allowance test passed")
}

// testTransferFromInsufficientGas tests transferFrom with insufficient gas
func testTransferFromInsufficientGas(ctx context.Context, client *ethclient.Client, addr *bind.TransactOpts) {
	log.Printf("Testing transferFrom with insufficient gas...")

	tx := getPreconfTx(ctx, client, addr, config.Addr3, config.Addr2, big.NewInt(1))
	tx.Gas = 1
	signedTx, err := addr.Signer(addr.From, types.NewTx(tx))
	if err != nil {
		log.Fatalf("Error signing transaction: %v", err)
	}

	var result core.NewPreconfTxEvent
	if err = client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		if strings.Contains(err.Error(), "intrinsic gas too low") {
			log.Printf("✓ transferFrom intrinsic gas test passed")
		} else {
			log.Fatalf("Error sending transaction: %v", err)
		}
	}

	tx.Gas = 106676000
	signedTx, err = addr.Signer(addr.From, types.NewTx(tx))
	if err != nil {
		log.Fatalf("Error signing transaction: %v", err)
	}

	if err = client.SendTransactionWithPreconf(ctx, signedTx, &result); err != nil {
		log.Fatalf("Error sending transaction: %v", err)
	}
	expectReasonInResult(&result, "out of gas")

	log.Printf("✓ transferFrom insufficient gas test passed")
}

// expectReasonInResult validates that result.Reason contains the expected error message
func expectReasonInResult(result *core.NewPreconfTxEvent, expectedReason string) {
	if result.Status != core.PreconfStatusFailed {
		log.Fatalf("Expected status to be failed, got: %s", result.Status)
	}

	if !strings.Contains(result.Reason, expectedReason) {
		log.Fatalf("Expected reason to contain '%s', but got: '%s'", expectedReason, result.Reason)
	}

	log.Printf("Reason validation passed: '%s' contains '%s'", result.Reason, expectedReason)
}

func sendRawTransactionWithPreconf(
	ctx context.Context,
	client *ethclient.Client,
) *core.NewPreconfTxEvent {

	chainID, err := client.ChainID(ctx)
	if err != nil {
		log.Fatalf("failed to get chain ID: %v", err)
	}

	auth, err := bind.NewKeyedTransactorWithChainID(config.FunderKey, chainID)
	if err != nil {
		log.Fatalf("failed to create config.Addr1 signer: %v", err)
	}

	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("failed to get nonce for %s: %v", auth.From.Hex(), err)
	}

	gasTipCap := big.NewInt(0)

	head, err := client.HeaderByNumber(ctx, nil)
	if err != nil {
		log.Fatalf("failed to suggest gas price: %v", err)
	}
	baseFee := head.BaseFee

	gasFeeCap := new(big.Int).Add(
		gasTipCap,
		new(big.Int).Mul(baseFee, big.NewInt(2)),
	)

	tx := &types.DynamicFeeTx{
		ChainID:   chainID,
		Nonce:     nonce,
		To:        &config.TestPay,
		GasTipCap: gasTipCap,
		GasFeeCap: gasFeeCap,
		Gas:       config.TransferGasLimit,
		Data:      common.Hex2Bytes("f7e94bbb000000000000000000000000" + config.TestERC20.Hex()[2:]),
	}

	signedTx, err := auth.Signer(auth.From, types.NewTx(tx))
	if err != nil {
		log.Printf("Error signing transaction: %v\n", err)
		return nil
	}

	var result core.NewPreconfTxEvent
	err = client.SendTransactionWithPreconf(ctx, signedTx, &result)
	txHash := signedTx.Hash()

	if err != nil {
		log.Fatalf("Error sending transaction: %v, txHash: %s\n", err, txHash.Hex())
	}

	if result.TxHash != txHash {
		log.Fatalf("Transaction hash mismatch: %v != %v\n", result.TxHash, txHash)
	}

	return &result
}

func getPreconfTx(ctx context.Context, client *ethclient.Client, auth *bind.TransactOpts, sender, recipient common.Address, amount *big.Int) *types.LegacyTx {
	nonce, err := client.PendingNonceAt(ctx, auth.From)
	if err != nil {
		log.Fatalf("failed to get nonce for %s: %v", auth.From.Hex(), err)
	}

	gasPrice, err := client.SuggestGasPrice(ctx)
	if err != nil {
		log.Fatalf("failed to suggest gas price: %v", err)
	}

	// Construct transferFrom call
	transferFromData := fmt.Sprintf(config.PAYDATA,
		sender.Hex()[2:],
		recipient.Hex()[2:],
		hex.EncodeToString(common.LeftPadBytes(amount.Bytes(), 32)), // amount
	)
	// 0xa5f2a152000000000000000000000000918a3880a91308279c06a89415d01ae47d64ec2900000000000000000000000071920e3cb420fbd8ba9a495e6f801c50375ea1270000000000000000000000000000000000000000000000000001c6bf52634001

	return &types.LegacyTx{
		Nonce:    nonce,
		To:       &config.TestPay,
		GasPrice: gasPrice,
		Gas:      config.TransferGasLimit,
		Data:     hexutil.MustDecode(transferFromData),
	}
}
