package miner

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/preconf"
)

// Errors
var (
	ErrEnvNil                                                                 = errors.New("env is nil")
	ErrOptimismSyncNil                                                        = errors.New("optimism sync status is nil")
	ErrOptimismSyncNotOk                                                      = errors.New("optimism sync status is not ok")
	ErrEnvTooOld                                                              = errors.New("env is too old")
	ErrCurrentL1BlockTooOld                                                   = errors.New("current l1 block is too old")
	ErrHeadL1BlockTooOld                                                      = errors.New("head l1 block is too old")
	ErrCurrentL1NumberAndHeadL1NumberDistanceTooLarge                         = errors.New("current l1 number and head l1 number distance is too large")
	ErrEnvBlockNumberLessThanEngineSyncTargetBlockNumberOrUnsafeL2BlockNumber = errors.New("env block number is less than engine sync target block number or unsafe l2 block number")
)

const (
	RequestTimeout = 5 * time.Second
)

type preconfChecker struct {
	mu sync.RWMutex

	chainConfig *params.ChainConfig
	chain       *core.BlockChain
	minerConfig *preconf.MinerConfig

	// clients
	opnodeClient *http.Client
	l1ethclient  *ethclient.Client

	env          *environment
	envUpdatedAt time.Time

	optimismSyncStatus   *preconf.OptimismSyncStatus
	optimismSyncStatusOk bool

	depositTxs []*types.Transaction
}

func NewPreconfChecker(chainConfig *params.ChainConfig, chain *core.BlockChain, minerConfig *preconf.MinerConfig) *preconfChecker {
	checker := &preconfChecker{
		chainConfig:  chainConfig,
		chain:        chain,
		minerConfig:  minerConfig,
		opnodeClient: &http.Client{Timeout: RequestTimeout},
	}
	log.Info("preconf checker", "minner.config", checker.minerConfig.String())
	go checker.loop()
	return checker
}

func (c *preconfChecker) loop() {
	if !c.minerConfig.EnablePreconfChecker {
		log.Debug("preconf checker is disabled, skip loop")
		return
	}
	for {
		if err := c.syncOptimismStatus(); err != nil {
			log.Error("Failed to sync optimism status", "err", err)
		}

		preconf.MetricsOpNodeSyncStatus(c.optimismSyncStatus, c.optimismSyncStatusOk)

		time.Sleep(1 * time.Second)
	}
}

func (c *preconfChecker) syncOptimismStatus() error {
	resp, err := c.opnodeClient.Post(c.minerConfig.OptimismNodeHTTP, "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}`))
	if err != nil {
		return fmt.Errorf("failed to get optimism sync status from opNode: %w", err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read optimism sync status from opNode: %w", err)
	}
	response := &preconf.OptimismSyncStatusResponse{}
	if err := json.Unmarshal(body, response); err != nil {
		return fmt.Errorf("failed to unmarshal optimism sync status: %w", err)
	}

	if response.Error != nil {
		return fmt.Errorf("failed to get optimism sync status from opNode: %v", response.Error)
	}

	// update optimism sync status
	c.UpdateOptimismSyncStatus(response.Result)
	return nil
}

func (c *preconfChecker) GetDepositTxs(start, end uint64) ([]*types.Transaction, error) {
	if c.l1ethclient == nil {
		dialCtx, dialCancel := context.WithTimeout(context.Background(), RequestTimeout)
		defer dialCancel()
		l1ethclient, err := ethclient.DialContext(dialCtx, c.minerConfig.L1RPCHTTP)
		if err != nil {
			return nil, fmt.Errorf("failed to dial l1 rpc: %w", err)
		}
		c.l1ethclient = l1ethclient
	}

	// filter deposit logs from start to end, but not include end
	filterCtx, filterCancel := context.WithTimeout(context.Background(), RequestTimeout)
	defer filterCancel()
	logs, err := c.l1ethclient.FilterLogs(filterCtx, ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(start)),
		ToBlock:   big.NewInt(int64(end)),
		Addresses: []common.Address{common.HexToAddress(c.minerConfig.L1DepositAddress)},
		Topics:    [][]common.Hash{{preconf.DepositEventABIHash}},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}
	log.Trace("filter deposit tx logs", "start", start, "end", end, "logs", len(logs))

	depositTxs := make([]*types.Transaction, 0)
	for _, log := range logs {
		depositTx, err := preconf.UnmarshalDepositLogEvent(&log)
		if err != nil {
			return nil, fmt.Errorf("failed to unmarshal deposit log event: %w", err)
		}
		depositTxs = append(depositTxs, types.NewTx(depositTx))
	}

	return depositTxs, nil
}

func (c *preconfChecker) UpdateOptimismSyncStatus(newOptimismSyncStatus *preconf.OptimismSyncStatus) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// Initialization
	if c.optimismSyncStatus == nil {
		c.optimismSyncStatus = newOptimismSyncStatus
		c.optimismSyncStatusOk = true
		c.updateDepositTxs(newOptimismSyncStatus.CurrentL1.Number, newOptimismSyncStatus.HeadL1.Number)
		return
	}

	log.Debug("update optimism sync status", "current_l1.number", c.optimismSyncStatus.CurrentL1.Number, "head_l1.number", c.optimismSyncStatus.HeadL1.Number,
		"unsafe_l2.number", c.optimismSyncStatus.UnsafeL2.Number, "engine_sync_target.number", c.optimismSyncStatus.EngineSyncTarget.Number,
		"new_current_l1.number", newOptimismSyncStatus.CurrentL1.Number, "new_head_l1.number", newOptimismSyncStatus.HeadL1.Number,
		"new_unsafe_l2.number", newOptimismSyncStatus.UnsafeL2.Number, "new_engine_sync_target.number", newOptimismSyncStatus.EngineSyncTarget.Number)

	// check optimism sync status
	if c.isSyncStatusOk(newOptimismSyncStatus) {
		// if l1 block changes, update depositTxs
		if c.optimismSyncStatus.CurrentL1.Number != newOptimismSyncStatus.CurrentL1.Number ||
			c.optimismSyncStatus.HeadL1.Number != newOptimismSyncStatus.HeadL1.Number {
			c.updateDepositTxs(newOptimismSyncStatus.CurrentL1.Number, newOptimismSyncStatus.HeadL1.Number)
		}
		c.optimismSyncStatus = newOptimismSyncStatus
		c.optimismSyncStatusOk = true
	} else {
		c.optimismSyncStatusOk = false
		log.Error("optimism sync status is not ok, l1 reorg?", "old", c.optimismSyncStatus, "new", newOptimismSyncStatus)
	}
}

// check optimism sync status
//
// current_l1.number normal growth
// head_l1.number normal growth
// unsafe_l2.number normal growth
// engine_sync_target.number normal growth
func (c *preconfChecker) isSyncStatusOk(newStatus *preconf.OptimismSyncStatus) bool {
	return c.optimismSyncStatus.CurrentL1.Number <= newStatus.CurrentL1.Number &&
		c.optimismSyncStatus.HeadL1.Number <= newStatus.HeadL1.Number &&
		c.optimismSyncStatus.UnsafeL2.Number <= newStatus.UnsafeL2.Number &&
		c.optimismSyncStatus.EngineSyncTarget.Number <= newStatus.EngineSyncTarget.Number
}

// update depositTxs
func (c *preconfChecker) updateDepositTxs(currentL1, headL1 uint64) {
	start, end := currentL1+1, headL1-1
	depositTxs, err := c.GetDepositTxs(start, end)
	if err != nil {
		c.depositTxs = nil
		log.Error("failed to get deposit txs", "err", err, "start", start, "end", end)
		preconf.MetricsL1Deposit(false, 0)
		return
	}
	c.depositTxs = depositTxs
	preconf.MetricsL1Deposit(true, len(depositTxs))
	log.Debug("update deposit txs", "current_l1.number", currentL1, "head_l1.number", headL1, "start", start, "end", end, "deposit_txs", len(depositTxs))
}

func (c *preconfChecker) Preconf(tx *types.Transaction) (*types.Receipt, error) {
	defer preconf.MetricsPreconfExecuteCost(time.Now())

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.precheck(); err != nil {
		return nil, err
	}

	receipt, err := c.applyTx(c.env, tx)
	if err != nil {
		if errors.Is(err, core.ErrNonceTooLow) {
			// if a tx is rejected because of nonce too low, it is possible that it has already been included in a block.
			// In this case, check if there is a corresponding receipt in env, and return it if found.
			for _, receipt := range c.env.receipts {
				if receipt.TxHash == tx.Hash() {
					log.Trace("preconf tx already in block", "tx", tx.Hash().Hex())
					return receipt, nil
				}
			}
		}
		return nil, err
	}

	// new tx can only be sealed in the next block
	cpy := *receipt
	cpy.BlockNumber = new(big.Int).Add(receipt.BlockNumber, big.NewInt(1))
	return &cpy, nil
}

func (c *preconfChecker) precheck() error {
	if c.env == nil {
		return ErrEnvNil
	}

	if c.optimismSyncStatus == nil {
		return ErrOptimismSyncNil
	}

	if !c.optimismSyncStatusOk {
		return ErrOptimismSyncNotOk
	}

	// Not more than MantleToleranceDuration(default 6s) from the last L2Block.
	if time.Since(c.envUpdatedAt) > c.minerConfig.MantleToleranceDuration() {
		log.Trace("envTooOld", "envUpdatedAt", c.envUpdatedAt, "time.Since(envUpdatedAt)", time.Since(c.envUpdatedAt), "tolerance", c.minerConfig.MantleToleranceDuration())
		return ErrEnvTooOld
	}

	// Not more than EthToleranceDuration(default 72s) from the last L1Block.
	currentL1BlockTime := time.Unix(int64(c.optimismSyncStatus.CurrentL1.Time), 0)
	if time.Since(currentL1BlockTime) > c.minerConfig.EthToleranceDuration() {
		log.Trace("currentL1BlockTooOld", "currentL1BlockTime", currentL1BlockTime, "time.Since(currentL1BlockTime)", time.Since(currentL1BlockTime), "tolerance", c.minerConfig.EthToleranceDuration())
		return ErrCurrentL1BlockTooOld
	}
	headL1BlockTime := time.Unix(int64(c.optimismSyncStatus.HeadL1.Time), 0)
	if time.Since(headL1BlockTime) > c.minerConfig.EthToleranceDuration() {
		log.Trace("headL1BlockTooOld", "headL1BlockTime", headL1BlockTime, "time.Since(headL1BlockTime)", time.Since(headL1BlockTime), "tolerance", c.minerConfig.EthToleranceDuration())
		return ErrHeadL1BlockTooOld
	}

	// The distance between current_l1.number and head_l1.number should not exceed EthToleranceBlock(default 6)
	if c.optimismSyncStatus.HeadL1.Number-c.optimismSyncStatus.CurrentL1.Number > c.minerConfig.EthToleranceBlock() {
		log.Trace("currentL1NumberAndHeadL1NumberDistanceTooLarge", "currentL1Number", c.optimismSyncStatus.CurrentL1.Number, "headL1Number", c.optimismSyncStatus.HeadL1.Number, "tolerance", c.minerConfig.EthToleranceBlock())
		return ErrCurrentL1NumberAndHeadL1NumberDistanceTooLarge
	}

	// env block number should be greater than engine sync target block number and unsafe l2 block number
	envBlockNumber := c.env.header.Number.Uint64()
	engineSyncTargetBlockNumber := c.optimismSyncStatus.EngineSyncTarget.Number
	unsafeL2BlockNumber := c.optimismSyncStatus.UnsafeL2.Number
	if envBlockNumber < engineSyncTargetBlockNumber || envBlockNumber < unsafeL2BlockNumber {
		log.Trace("envBlockNumberLessThanEngineSyncTargetBlockNumberOrUnsafeL2BlockNumber", "envBlockNumber", envBlockNumber, "engineSyncTargetBlockNumber", engineSyncTargetBlockNumber, "unsafeL2BlockNumber", unsafeL2BlockNumber)
		return ErrEnvBlockNumberLessThanEngineSyncTargetBlockNumberOrUnsafeL2BlockNumber
	}

	return nil
}

func (c *preconfChecker) applyTx(env *environment, tx *types.Transaction) (*types.Receipt, error) {
	env.state.SetTxContext(tx.Hash(), env.tcount)
	var (
		snap = env.state.Snapshot()
		gp   = env.gasPool.Gas()
	)
	receipt, err := core.ApplyTransaction(c.chainConfig, c.chain, &env.coinbase, env.gasPool, env.state, env.header, tx, &env.header.GasUsed, *c.chain.GetVMConfig())
	if err != nil {
		env.state.RevertToSnapshot(snap)
		env.gasPool.SetGas(gp)
		return nil, err
	}
	env.txs = append(env.txs, tx)
	env.receipts = append(env.receipts, receipt)
	env.tcount++
	return receipt, nil
}

func (c *preconfChecker) PausePreconf() {
	c.mu.Lock()
	log.Trace("pause preconf")
}

func (c *preconfChecker) UnpausePreconf(env *environment, preconfReady func()) {
	defer c.mu.Unlock()
	c.env = env
	c.envUpdatedAt = time.Now()
	log.Trace("unpause preconf", "env.header.Number", env.header.Number.Int64(), "envUpdatedAt", c.envUpdatedAt)

	// LoadDepositTxs
	log.Trace("apply deposit txs", "deposit_txs", len(c.depositTxs))
	for _, tx := range c.depositTxs {
		if _, err := c.applyTx(c.env, tx); err != nil {
			log.Error("failed to apply deposit tx", "err", err, "tx", tx.Hash().Hex())
			continue
		}
		log.Trace("applied deposit tx", "tx", tx.Hash().Hex(), "nonce", tx.Nonce())
	}

	// Metrics
	preconf.MetricsOpGethEnvBlockNumber(env.header.Number.Int64())

	// notify txpool that preconf is ready
	preconfReady()
}
