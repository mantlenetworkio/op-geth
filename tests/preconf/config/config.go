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
	L1RpcEndpoint       = "http://127.0.0.1:8545"
	SequencerEndpoint   = "http://127.0.0.1:9545"
	L2RpcEndpoint       = "http://127.0.0.1:10545"
	FundKeyHex          = "ac0974bec39a17e36ba4a6b4d238ff944bacb478cbed5efcae784d7bf4f2ff80" // 0xf39Fd6e51aad88F6F4ce6aB8827279cffFb92266
	OptimismPortalProxy = "0xa513E6E4b8f2a923D98304ec87F64353C4D5C853"
	ToAddressHex        = "0x71920E3cb420fbD8Ba9a495E6f801c50375ea127"
	BatchSize           = 10
	NonceInterval       = 10 * time.Millisecond
	Addr1Pk             = "e474bfa0d1520cf4b161b382db9f527c39ac16b6d9a8351f091bd406f739a691" // 0x6F18bEEF53452dC646C5221900F1EfE8b6B4BDc5
	Addr3Pk             = "654c6b97f400c2facec28bcb2ae04f2bf99e007bd6e41b2ce221481e30840e49" // 0x918a3880A91308279C06A89415d01ae47d64eC29
	TestERC20Bytecodes  = "0x6080604052348015600e575f80fd5b50610ce18061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610060575f3560e01c8063095ea7b31461006457806323b872dd146100945780632ccb1b30146100c457806340c10f19146100f457806370a0823114610110578063dd62ed3e14610140575b5f80fd5b61007e60048036038101906100799190610846565b610170565b60405161008b919061089e565b60405180910390f35b6100ae60048036038101906100a991906108b7565b6101f8565b6040516100bb919061089e565b60405180910390f35b6100de60048036038101906100d99190610846565b6104f7565b6040516100eb919061089e565b60405180910390f35b61010e60048036038101906101099190610846565b6106ad565b005b61012a60048036038101906101259190610907565b610781565b6040516101379190610941565b60405180910390f35b61015a6004803603810190610155919061095a565b610795565b6040516101679190610941565b60405180910390f35b5f8160015f3373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205f8573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20819055506001905092915050565b5f8060015f8673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205f3373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20549050828110156102b8576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016102af906109f2565b60405180910390fd5b82816102c49190610a3d565b60015f8773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205f3373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20819055505f805f8673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f2054905080848261038d9190610a70565b10156103ce576040517f08c379a00000000000000000000000000000000000000000000000000000000081526004016103c590610aed565b60405180910390fd5b83816103da9190610a70565b5f808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20819055505f805f8873ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205490508481101561049d576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161049490610b55565b60405180910390fd5b84816104a99190610a3d565b5f808973ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f2081905550600193505050509392505050565b5f805f803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205490508281101561057b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161057290610bbd565b60405180910390fd5b82816105879190610a3d565b5f803373ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20819055505f805f8673ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f205490508084826106149190610a70565b1015610655576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161064c90610c25565b60405180910390fd5b83816106619190610a70565b5f808773ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f208190555060019250505092915050565b5f815f808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f20546106f69190610a70565b90508181101561073b576040517f08c379a000000000000000000000000000000000000000000000000000000000815260040161073290610c8d565b60405180910390fd5b805f808573ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff1681526020019081526020015f2081905550505050565b5f602052805f5260405f205f915090505481565b6001602052815f5260405f20602052805f5260405f205f91509150505481565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f6107e2826107b9565b9050919050565b6107f2816107d8565b81146107fc575f80fd5b50565b5f8135905061080d816107e9565b92915050565b5f819050919050565b61082581610813565b811461082f575f80fd5b50565b5f813590506108408161081c565b92915050565b5f806040838503121561085c5761085b6107b5565b5b5f610869858286016107ff565b925050602061087a85828601610832565b9150509250929050565b5f8115159050919050565b61089881610884565b82525050565b5f6020820190506108b15f83018461088f565b92915050565b5f805f606084860312156108ce576108cd6107b5565b5b5f6108db868287016107ff565b93505060206108ec868287016107ff565b92505060406108fd86828701610832565b9150509250925092565b5f6020828403121561091c5761091b6107b5565b5b5f610929848285016107ff565b91505092915050565b61093b81610813565b82525050565b5f6020820190506109545f830184610932565b92915050565b5f80604083850312156109705761096f6107b5565b5b5f61097d858286016107ff565b925050602061098e858286016107ff565b9150509250929050565b5f82825260208201905092915050565b7f616c6c6f77616e636520696e73756666696369656e74000000000000000000005f82015250565b5f6109dc601683610998565b91506109e7826109a8565b602082019050919050565b5f6020820190508181035f830152610a09816109d0565b9050919050565b7f4e487b71000000000000000000000000000000000000000000000000000000005f52601160045260245ffd5b5f610a4782610813565b9150610a5283610813565b9250828203905081811115610a6a57610a69610a10565b5b92915050565b5f610a7a82610813565b9150610a8583610813565b9250828201905080821115610a9d57610a9c610a10565b5b92915050565b7f6f766572666c6f772062616c616e636520726563697069656e740000000000005f82015250565b5f610ad7601a83610998565b9150610ae282610aa3565b602082019050919050565b5f6020820190508181035f830152610b0481610acb565b9050919050565b7f756e646572666c6f772062616c616e63652073656e64657200000000000000005f82015250565b5f610b3f601883610998565b9150610b4a82610b0b565b602082019050919050565b5f6020820190508181035f830152610b6c81610b33565b9050919050565b7f696e73756666696369656e742062616c616e63650000000000000000000000005f82015250565b5f610ba7601483610998565b9150610bb282610b73565b602082019050919050565b5f6020820190508181035f830152610bd481610b9b565b9050919050565b7f726563697069656e742062616c616e6365206f766572666c6f770000000000005f82015250565b5f610c0f601a83610998565b9150610c1a82610bdb565b602082019050919050565b5f6020820190508181035f830152610c3c81610c03565b9050919050565b7f6f766572666c6f772062616c616e6365000000000000000000000000000000005f82015250565b5f610c77601083610998565b9150610c8282610c43565b602082019050919050565b5f6020820190508181035f830152610ca481610c6b565b905091905056fea26469706673582212203b8726afc841875993e31291524430791eb00558888ef1c031360f572730e83264736f6c634300081a0033"
	TestPayBytecodes    = "0x6080604052348015600e575f80fd5b5061038d8061001c5f395ff3fe608060405234801561000f575f80fd5b5060043610610034575f3560e01c8063a5f2a15214610038578063f7e94bbb14610068575b5f80fd5b610052600480360381019061004d9190610201565b610084565b60405161005f919061026b565b60405180910390f35b610082600480360381019061007d9190610284565b61012e565b005b5f805f9054906101000a900473ffffffffffffffffffffffffffffffffffffffff1673ffffffffffffffffffffffffffffffffffffffff166323b872dd8585856040518463ffffffff1660e01b81526004016100e2939291906102cd565b6020604051808303815f875af11580156100fe573d5f803e3d5ffd5b505050506040513d601f19601f82011682018060405250810190610122919061032c565b50600190509392505050565b805f806101000a81548173ffffffffffffffffffffffffffffffffffffffff021916908373ffffffffffffffffffffffffffffffffffffffff16021790555050565b5f80fd5b5f73ffffffffffffffffffffffffffffffffffffffff82169050919050565b5f61019d82610174565b9050919050565b6101ad81610193565b81146101b7575f80fd5b50565b5f813590506101c8816101a4565b92915050565b5f819050919050565b6101e0816101ce565b81146101ea575f80fd5b50565b5f813590506101fb816101d7565b92915050565b5f805f6060848603121561021857610217610170565b5b5f610225868287016101ba565b9350506020610236868287016101ba565b9250506040610247868287016101ed565b9150509250925092565b5f8115159050919050565b61026581610251565b82525050565b5f60208201905061027e5f83018461025c565b92915050565b5f6020828403121561029957610298610170565b5b5f6102a6848285016101ba565b91505092915050565b6102b881610193565b82525050565b6102c7816101ce565b82525050565b5f6060820190506102e05f8301866102af565b6102ed60208301856102af565b6102fa60408301846102be565b949350505050565b61030b81610251565b8114610315575f80fd5b50565b5f8151905061032681610302565b92915050565b5f6020828403121561034157610340610170565b5b5f61034e84828501610318565b9150509291505056fea264697066735822122006413856350c8f5b892784fe9200e18a4a10d1c5d78ee669db75ec31ed4e238764736f6c634300081a0033"
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
	args := []interface{}{
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

	tx := types.NewTransaction(nonce, DepositAddr, big.NewInt(0), gas, big.NewInt(1e12), calldata)
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
	l1client, err := ethclient.Dial(L1RpcEndpoint)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to connect to L1 RPC: %v", err)
	}
	defer l1client.Close()
	l1chainID, err := l1client.NetworkID(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to get L1 chain ID: %v", err)
	}
	auth, err := bind.NewKeyedTransactorWithChainID(privateKey, l1chainID)
	if err != nil {
		log.Fatalf("failed to create %s signer: %v", crypto.PubkeyToAddress(privateKey.PublicKey).Hex(), err)
	}
	return l1client, auth, nil
}
