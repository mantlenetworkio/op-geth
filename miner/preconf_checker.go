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

type preconfChecker struct {
	mu sync.RWMutex

	chainConfig *params.ChainConfig
	chain       *core.BlockChain
	minerConfig *preconf.MinerConfig
	l1ethclient *ethclient.Client

	env          *environment
	envUpdatedAt time.Time

	optimismSyncStatus   *preconf.OptimismSyncStatus
	optimismSyncStatusOk bool

	depositTxs []*types.Transaction
}

func NewPreconfChecker(chainConfig *params.ChainConfig, chain *core.BlockChain, minerConfig *preconf.MinerConfig) *preconfChecker {
	checker := &preconfChecker{
		chainConfig: chainConfig,
		chain:       chain,
		minerConfig: minerConfig,
	}
	log.Info("preconf checker", "minner.config", checker.minerConfig.String())
	go checker.loop()
	return checker
}

func (c *preconfChecker) loop() {
	for {
		time.Sleep(1 * time.Second)

		if err := c.syncOptimismStatus(); err != nil {
			log.Error("Failed to sync optimism status", "err", err)
		}

		preconf.MetricsOpNodeSyncStatus(c.optimismSyncStatus, c.optimismSyncStatusOk)
	}
}

func (c *preconfChecker) syncOptimismStatus() error {
	resp, err := http.Post(c.minerConfig.OptimismNodeHTTP, "application/json", strings.NewReader(`{"jsonrpc":"2.0","method":"optimism_syncStatus","params":[],"id":1}`))
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
		l1ethclient, err := ethclient.Dial(c.minerConfig.L1RPCHTTP)
		if err != nil {
			return nil, fmt.Errorf("failed to dial l1 rpc: %w", err)
		}
		c.l1ethclient = l1ethclient
	}

	logs, err := c.l1ethclient.FilterLogs(context.Background(), ethereum.FilterQuery{
		FromBlock: big.NewInt(int64(start)),
		ToBlock:   big.NewInt(int64(end)),
		Addresses: []common.Address{common.HexToAddress(c.minerConfig.L1DepositAddress)},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to filter logs: %w", err)
	}

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
	c.optimismSyncStatusOk = false
	if c.optimismSyncStatus == nil {
		c.optimismSyncStatus = newOptimismSyncStatus
	}

	log.Debug("update optimism sync status", "current_l1.number", c.optimismSyncStatus.CurrentL1.Number, "head_l1.number", c.optimismSyncStatus.HeadL1.Number,
		"unsafe_l2.number", c.optimismSyncStatus.UnsafeL2.Number, "engine_sync_target.number", c.optimismSyncStatus.EngineSyncTarget.Number,
		"new_current_l1.number", newOptimismSyncStatus.CurrentL1.Number, "new_head_l1.number", newOptimismSyncStatus.HeadL1.Number,
		"new_unsafe_l2.number", newOptimismSyncStatus.UnsafeL2.Number, "new_engine_sync_target.number", newOptimismSyncStatus.EngineSyncTarget.Number)
	// current_l1.number normal growth
	// head_l1.number normal growth
	// unsafe_l2.number normal growth
	// engine_sync_target.number normal growth
	if c.optimismSyncStatus.CurrentL1.Number <= newOptimismSyncStatus.CurrentL1.Number &&
		c.optimismSyncStatus.HeadL1.Number <= newOptimismSyncStatus.HeadL1.Number &&
		c.optimismSyncStatus.UnsafeL2.Number <= newOptimismSyncStatus.UnsafeL2.Number &&
		c.optimismSyncStatus.EngineSyncTarget.Number <= newOptimismSyncStatus.EngineSyncTarget.Number {
		// update optimism sync status
		c.optimismSyncStatus = newOptimismSyncStatus

		// update optimism sync status ok
		c.optimismSyncStatusOk = true
	} else {
		log.Error("optimism sync status is not ok, l1 reorg?", "old", c.optimismSyncStatus, "new", newOptimismSyncStatus)
	}

	// update deposit txs if current_l1.number or head_l1.number is changed and optimism sync status is ok
	if c.optimismSyncStatusOk &&
		(c.optimismSyncStatus.CurrentL1.Number != newOptimismSyncStatus.CurrentL1.Number ||
			c.optimismSyncStatus.HeadL1.Number != newOptimismSyncStatus.HeadL1.Number) {
		depositTxs, err := c.GetDepositTxs(c.optimismSyncStatus.CurrentL1.Number, c.optimismSyncStatus.HeadL1.Number-2)
		if err != nil {
			log.Error("failed to get deposit txs", "err", err, "start", c.optimismSyncStatus.CurrentL1.Number, "end", c.optimismSyncStatus.HeadL1.Number-2)
			preconf.MetricsL1Deposit(false, 0)
		} else {
			c.depositTxs = depositTxs
			preconf.MetricsL1Deposit(true, len(depositTxs))
		}

		log.Debug("update deposit txs", "current_l1.number", c.optimismSyncStatus.CurrentL1.Number, "head_l1.number", c.optimismSyncStatus.HeadL1.Number,
			"new_current_l1.number", newOptimismSyncStatus.CurrentL1.Number, "new_head_l1.number", newOptimismSyncStatus.HeadL1.Number,
			"deposit_txs", len(depositTxs))
	}
}

func (c *preconfChecker) Preconf(tx *types.Transaction) (*types.Receipt, error) {
	defer preconf.MetricsPreconfExecuteCost(time.Now())

	c.mu.Lock()
	defer c.mu.Unlock()

	if err := c.precheck(); err != nil {
		return nil, err
	}

	return c.applyTx(c.env, tx)
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

	// Not more than 5 seconds from the last L2Block.
	if time.Since(c.envUpdatedAt) > 5*time.Second {
		return ErrEnvTooOld
	}

	// Not more than 30 seconds from the last L1Block.
	currentL1BlockTime := time.Unix(int64(c.optimismSyncStatus.CurrentL1.Time), 0)
	if time.Since(currentL1BlockTime) > 30*time.Second {
		return ErrCurrentL1BlockTooOld
	}
	headL1BlockTime := time.Unix(int64(c.optimismSyncStatus.HeadL1.Time), 0)
	if time.Since(headL1BlockTime) > 30*time.Second {
		return ErrHeadL1BlockTooOld
	}

	// The distance between current_l1.number and head_l1.number should not exceed 5
	if c.optimismSyncStatus.HeadL1.Number-c.optimismSyncStatus.CurrentL1.Number > 5 {
		return ErrCurrentL1NumberAndHeadL1NumberDistanceTooLarge
	}

	// env block number should be greater than engine sync target block number and unsafe l2 block number
	envBlockNumber := c.env.header.Number.Uint64()
	engineSyncTargetBlockNumber := c.optimismSyncStatus.EngineSyncTarget.Number
	unsafeL2BlockNumber := c.optimismSyncStatus.UnsafeL2.Number
	if envBlockNumber < engineSyncTargetBlockNumber || envBlockNumber < unsafeL2BlockNumber {
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
	return receipt, nil
}

func (c *preconfChecker) PausePreconf() {
	c.mu.Lock()
}

func (c *preconfChecker) UnpausePreconf(env *environment) {
	defer c.mu.Unlock()
	c.env = env
	c.envUpdatedAt = time.Now()

	// LoadDepositTxs
	c.env.txs = append(c.env.txs, c.depositTxs...)

	// Metrics
	preconf.MetricsOpGethEnvBlockNumber(env.header.Number.Int64())
}
