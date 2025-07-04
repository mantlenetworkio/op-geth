package main

import (
	"context"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
	frontrunning "github.com/ethereum/go-ethereum/tests/preconf/front_running"
	"github.com/ethereum/go-ethereum/tests/preconf/sort"
	"github.com/ethereum/go-ethereum/tests/preconf/stress"
)

// There are three tests that require manual modification of op-geth configuration to cover:
// 1. Set txpool.preconftimeout to a very small value (e.g. 1ms) to test timeout handling.
// 2. Manually restart op-geth while processing a large number of preconfirmation transactions to test journal handling.
// 3. Manually modify op-geth's gaslimit upper bound (e.g. 200000000000) to test handling when preconfirmation transactions fill up the block.
func main() {
	precheck()
	checkPreconfRPCValid()
	stress.StressTest()
	frontrunning.TransferTest()
	sort.SortTest()
	frontrunning.ERC20Test()
	// reorg.L1ReorgDetection(common.HexToHash("0xe3f60268eb85440e5b2212cb748b3ea3df4cac7973a846ea16f7fa85c68a5eda"))
}

func precheck() {
	ctx := context.Background()
	sequencerClient, err := ethclient.Dial(config.SequencerEndpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
	}
	defer sequencerClient.Close()

	rpcClient, err := ethclient.Dial(config.L2RpcEndpoint)
	if err != nil {
		log.Fatalf("failed to connect to L2 RPC: %v", err)
	}
	defer rpcClient.Close()

	checkERC20(ctx, sequencerClient)
	checkERC20(ctx, rpcClient)

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
	foundAmount := big.NewInt(0).Mul(big.NewInt(config.NumTransactions*2), big.NewInt(1e18))
	config.FundAccount(ctx, client, config.Addr1, foundAmount)
	config.FundAccount(ctx, client, config.Addr3, foundAmount)

	// todo - go auto deploy TestERC20/TestPay
	// 1. Deploy TestERC20/TestPay
	// 2. setERC20Address in TestPay
	// 3. set TestPay address in ToPreconfs of op-geth
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
