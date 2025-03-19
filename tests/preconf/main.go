package main

import (
	"context"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/tests/preconf/config"
	frontrunning "github.com/ethereum/go-ethereum/tests/preconf/front_running"
	"github.com/ethereum/go-ethereum/tests/preconf/sort"
	"github.com/ethereum/go-ethereum/tests/preconf/stress"
)

func main() {
	precheck()
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
