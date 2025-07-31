package preconf

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"

	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/eth"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/node"
	"github.com/ethereum/go-ethereum/preconf"
	"github.com/ethereum/go-ethereum/rpc"
)

func Register(stack *node.Node, backend *eth.Ethereum) {
	log.Warn("PreConf API enabled", "protocol", "eth")
	stack.RegisterAPIs([]rpc.API{
		{
			Namespace:     "preconf",
			Service:       NewPreConfAPI(backend),
			Authenticated: true,
		},
	})
}

const (
	defaultPreConfTimeout = 1 * time.Second
)

var errBlockNumberInvalid = errors.New("block number is invalid")

var caps = []string{
	"preconf_sendRawTransactionWithPreConfV2",
}

type PreConfAPI struct {
	eth *eth.Ethereum

	parentHash        common.Hash
	lastBlockNumber   uint64
	lastBlockGasLimit uint64

	sendRawTransactionLock sync.Mutex
}

func NewPreConfAPI(eth *eth.Ethereum) *PreConfAPI {
	return &PreConfAPI{eth: eth}
}

func (api *PreConfAPI) SendRawTransactionWithPreConfV2(params *preconf.Params) (preconf.Response, error) {
	// SendRawTransactionWithPreConfV2 processes multiple transactions with preconfirmation.
	//
	// Timeout handling:
	// - User can specify a timeout via params.Timeout
	// - We use a timer-based approach instead of context to avoid internal 2s limit conflicts
	// - Each transaction is processed with its own context, but we respect the overall user timeout
	// - We can process transactions efficiently without being limited by internal timeouts
	//
	// Response status:
	// - SUCCESS: All transactions processed successfully
	// - FAILED: One or more transactions failed (non-timeout errors)
	// - TIMEOUT: Processing timed out (user timeout exceeded)
	// - INVALID: Invalid parameters (block number, timeout format, etc.)

	api.sendRawTransactionLock.Lock()
	defer api.sendRawTransactionLock.Unlock()

	if api.lastBlockNumber == 0 {
		api.reset(params)
	}

	if api.lastBlockNumber == params.BlockNumber {
		if api.parentHash != params.ParentHash {
			return preconf.STATUS_REOGR, nil
		}
	} else if api.lastBlockNumber > params.BlockNumber {
		if !params.ForkBlock {
			return preconf.STATUS_INVALID, nil
		}
		api.reset(params)
	} else if api.lastBlockNumber == params.BlockNumber-1 {
		api.reset(params)
	} else {
		// verify parent hash
		block := api.eth.BlockChain().GetBlockByNumber(params.BlockNumber - 1)
		if block != nil {
			if api.parentHash == block.Hash() {
				api.reset(params)
			} else {
				return preconf.STATUS_REOGR, nil
			}
		}
		return preconf.STATUS_INVALID, nil
	}

	var (
		err     error
		timeout = defaultPreConfTimeout
	)
	response := &preconf.Response{
		Status:   preconf.WAITING,
		Receipts: make([]hexutil.Bytes, 0),
	}
	log.Trace("PreConf API request received", "method", "SendRawTransactionWithPreConfV2", "head", params.BlockNumber)

	// Parse timeout if provided
	if params.Timeout != "" {
		if timeout, err = time.ParseDuration(params.Timeout); err != nil {
			return preconf.STATUS_INVALID, preconf.InvalidTimeOut
		}
	}

	// Validate block number
	if params.BlockNumber != api.eth.BlockChain().CurrentBlock().Number.Uint64() {
		return preconf.STATUS_INVALID, preconf.InvalidBlockNumber
	}

	// Check if transactions are provided
	if len(params.Transactions) == 0 {
		log.Warn("No transactions provided in params")
		response.Status = preconf.FAILED
		return *response, nil
	}

	log.Trace("PreConf processing started",
		"timeout", timeout,
		"transaction_count", len(params.Transactions))

	// check gas limit
	gaslimit := api.eth.BlockChain().GasLimit()
	txs, gasUsed, err := checkTransactions(params.Transactions)
	if err != nil {
		response.Status = preconf.FAILED
		return *response, err
	}
	if api.lastBlockGasLimit+gasUsed > gaslimit {
		return preconf.STATUS_INVALID, preconf.InvalidGasLimitFlow
	}
	api.lastBlockGasLimit += gasUsed

	// Channel to collect results
	resultCh := make(chan preconf.Response, 1)

	// Start processing in goroutine with timer-based timeout
	go func() {
		defer close(resultCh)

		var receipts []hexutil.Bytes
		var hasError bool
		var errorReason string

		// Create timer for overall timeout
		timer := time.NewTimer(timeout)
		defer timer.Stop()

		// Process each transaction
		for i, tx := range txs {
			// Check if we've exceeded the overall timeout
			select {
			case <-timer.C:
				log.Warn("PreConf processing timeout", "processed", i, "total", len(params.Transactions))
				resultCh <- preconf.Response{
					Status:   preconf.TIMEOUT,
					Receipts: receipts,
				}
				return
			default:
				// Continue processing
			}

			// Create a context with a reasonable timeout for this single transaction
			// We use a shorter timeout than the internal 2s to avoid conflicts
			txCtx, txCancel := context.WithTimeout(context.Background(), 1500*time.Millisecond)

			//txpool add moho tx
			signer := types.LatestSigner(api.eth.APIBackend.ChainConfig())
			from, _ := types.Sender(signer, tx)
			txHash := tx.Hash()
			api.eth.APIBackend.AddMoHoPreconfTxHash(from, txHash)

			// Send transaction with preconf
			preconfEvent, err := api.eth.APIBackend.SendTxWithPreconf(txCtx, tx)
			txCancel() // Always cancel the transaction context

			if err != nil {
				log.Error("Failed to send transaction with preconf", "tx", tx.Hash().Hex(), "error", err)
				hasError = true
				errorReason = err.Error()
				break
			}

			// Check preconf status
			switch preconfEvent.Status {
			case core.PreconfStatusSuccess:
				// Transaction was successful, add receipt if available
				if preconfEvent.Receipt.Logs != nil && len(preconfEvent.Receipt.Logs) > 0 {
					// Convert receipt to hexutil.Bytes (simplified for this example)
					receiptBytes := hexutil.Bytes(tx.Hash().Bytes())
					receipts = append(receipts, receiptBytes)
				}
				log.Trace("Transaction preconf successful", "tx", tx.Hash().Hex(), "index", i, "progress", fmt.Sprintf("%d/%d", i+1, len(params.Transactions)))

			case core.PreconfStatusFailed:
				log.Error("Transaction preconf failed", "tx", tx.Hash().Hex(), "reason", preconfEvent.Reason, "index", i)
				hasError = true
				errorReason = preconfEvent.Reason
				break

			case core.PreconfStatusTimeout:
				log.Warn("Transaction preconf timeout", "tx", tx.Hash().Hex(), "index", i)
				hasError = true
				errorReason = "preconf timeout"
				break

			default:
				log.Warn("Unknown preconf status", "tx", tx.Hash().Hex(), "status", preconfEvent.Status, "index", i)
				hasError = true
				errorReason = "unknown preconf status"
				break
			}

			// If we encountered an error, stop processing
			if hasError {
				break
			}
		}

		// Determine final response status
		var finalStatus string
		if hasError {
			if errorReason == "preconf timeout" {
				finalStatus = preconf.TIMEOUT
			} else {
				finalStatus = preconf.FAILED
			}
		} else {
			finalStatus = preconf.SUCCESS
		}

		// Send final response
		resultCh <- preconf.Response{
			Status:   finalStatus,
			Receipts: receipts,
		}
	}()

	// Wait for result or timeout
	select {
	case result := <-resultCh:
		return result, nil
	case <-time.After(timeout):
		// This should not happen as the goroutine should handle the timeout
		log.Error("Unexpected timeout in main function")
		return preconf.Response{
			Status:   preconf.TIMEOUT,
			Receipts: make([]hexutil.Bytes, 0),
		}, nil
	}
}

func (api *PreConfAPI) reset(params *preconf.Params) {
	api.parentHash = params.ParentHash
	api.lastBlockNumber = params.BlockNumber
	api.lastBlockGasLimit = 0
}

// checkTransactions converts hexutil.Bytes to types.Transaction and calculates total gas limit
func checkTransactions(txs []hexutil.Bytes) ([]*types.Transaction, uint64, error) {
	var (
		transactions = make([]*types.Transaction, 0, len(txs))
		totalGas     uint64
	)

	for i, txData := range txs {
		// Unmarshal transaction
		tx := new(types.Transaction)
		if err := tx.UnmarshalBinary(txData); err != nil {
			return nil, 0, fmt.Errorf("failed to unmarshal transaction at index %d: %w", i, err)
		}

		// Add to transactions slice
		transactions = append(transactions, tx)

		// Accumulate gas limit
		totalGas += tx.Gas()
	}

	return transactions, totalGas, nil
}
