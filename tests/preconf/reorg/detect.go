package reorg

import (
	"context"
	"fmt"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// Configuration
const (
	L1RpcEndpoint = "http://127.0.0.1:8545"
)

func L1ReorgDetection(blockHash common.Hash) {
	ctx := context.Background()
	client, err := ethclient.Dial(L1RpcEndpoint)
	if err != nil {
		fmt.Printf("Failed to connect to RPC: %v\n", err)
		return
	}
	defer client.Close()

	forkPoint := detect(ctx, blockHash, client)
	if forkPoint == nil {
		fmt.Println("Failed to detect fork point due to errors")
		return
	}

	fmt.Printf("Fork point found at height %d, hash %s\n", forkPoint.NumberU64(), forkPoint.Hash().Hex())
}

func detect(ctx context.Context, blockHash common.Hash, client *ethclient.Client) *types.Block {
	// Get the block by the specified hash
	blockByHash, err := client.BlockByHash(ctx, blockHash)
	if err != nil {
		fmt.Printf("Failed to get block by hash %s: %v", blockHash.Hex(), err)
		return nil
	}
	fmt.Printf("\nBlock by hash: %s, block number: %d\n", blockByHash.Hash().Hex(), blockByHash.NumberU64())

	// Get the main chain block at the same height
	blockByNumber, err := client.BlockByNumber(ctx, blockByHash.Number())
	if err != nil {
		fmt.Printf("Failed to get block by number %d: %v", blockByHash.NumberU64(), err)
		return nil
	}
	fmt.Printf("Block by number: %s, block number: %d\n", blockByNumber.Hash().Hex(), blockByNumber.NumberU64())

	// If the hashes do not match, it indicates a fork, continue to trace back the parent block
	if blockByNumber.Hash() != blockByHash.Hash() {
		fmt.Printf("Fork detected at height %d: canonical hash %s != forked hash %s",
			blockByHash.NumberU64(), blockByNumber.Hash().Hex(), blockByHash.Hash().Hex())
		return detect(ctx, blockByHash.ParentHash(), client)
	}

	// The hashes match, indicating this block is part of the main chain, return this block as the fork point
	fmt.Printf("No fork at height %d, hash %s is on canonical chain", blockByHash.NumberU64(), blockByHash.Hash().Hex())
	return blockByHash
}
