package miner

import (
	"context"
	"errors"
	"math/big"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/consensus/ethash"
	"github.com/ethereum/go-ethereum/consensus/misc/eip1559"
	"github.com/ethereum/go-ethereum/core"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/core/vm"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/params"
	"github.com/ethereum/go-ethereum/preconf"
	"github.com/holiman/uint256"
)

func TestIsSyncStatusOk(t *testing.T) {
	tests := []struct {
		name          string
		currentStatus *preconf.OptimismSyncStatus
		newStatus     *preconf.OptimismSyncStatus
		want          bool
	}{
		{
			name: "Normal Growth",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 11},
				HeadL1:    preconf.L1BlockRef{Number: 16},
				UnsafeL2:  preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 11}},
			},
			want: true,
		},
		{
			name: "Partial Growth",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},                                        // no change
				HeadL1:    preconf.L1BlockRef{Number: 16},                                        // growth
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}}, // no change
			},
			want: true,
		},
		{
			name: "Reorg CurrentL1",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 9}, // decreased
				HeadL1:    preconf.L1BlockRef{Number: 16},
				UnsafeL2:  preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 9}},
			},
			want: false,
		},
		{
			name: "Reorg HeadL1",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 11},
				HeadL1:    preconf.L1BlockRef{Number: 14}, // decreased
				UnsafeL2:  preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 11}},
			},
			want: false,
		},
		{
			name: "Reorg UnsafeL2",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 11},
				HeadL1:    preconf.L1BlockRef{Number: 16},
				UnsafeL2:  preconf.L2BlockRef{Number: 19, L1Origin: preconf.BlockID{Number: 11}}, // decreased
			},
			want: false,
		},
		{
			name: "No Change",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &preconfChecker{
				optimismSyncStatus: tt.currentStatus,
			}
			got := c.isSyncStatusOk(tt.newStatus)
			if got != tt.want {
				t.Errorf("isSyncStatusOk() = %v, want %v", got, tt.want)
			}
		})
	}
}

type mockLogFilterer struct {
	FilterLogsResult struct {
		Logs []types.Log
		Err  error
	}
	SubscribeFilterLogsResult struct {
		Sub ethereum.Subscription
		Err error
	}
	WaitTime time.Duration
}

func (m *mockLogFilterer) FilterLogs(ctx context.Context, q ethereum.FilterQuery) ([]types.Log, error) {
	time.Sleep(m.WaitTime)
	return m.FilterLogsResult.Logs, m.FilterLogsResult.Err
}

func (m *mockLogFilterer) SubscribeFilterLogs(ctx context.Context, q ethereum.FilterQuery, ch chan<- types.Log) (ethereum.Subscription, error) {
	time.Sleep(m.WaitTime)
	return m.SubscribeFilterLogsResult.Sub, m.SubscribeFilterLogsResult.Err
}

func TestUpdateOptimismSyncStatus(t *testing.T) {
	log := []types.Log{
		{
			Address: common.HexToAddress("0xa513e6e4b8f2a923d98304ec87f64353c4d5c853"),
			Topics: []common.Hash{
				common.HexToHash("0xb3813568d9991fc951961fcb4c784893574240a28925604d09fc577c55bb7c32"),
				common.HexToHash("0x0000000000000000000000001276878a594ca255338adfa4d48449f69242fca0"),
				common.HexToHash("0x0000000000000000000000004200000000000000000000000000000000000007"),
				common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
			},
			Data:        common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000024d0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000024ef1200ff8daf150001000000000000000000000000000000000000000000000000000000000000000000000000000000000000dc64a140aa3e981100a9beca4e685f962f0cf6c900000000000000000000000042000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001e848000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000a4f407a99e000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb922660000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
			BlockHash:   common.HexToHash("0x123"),
			BlockNumber: 10,
			TxHash:      common.HexToHash("0x123"),
			TxIndex:     0,
		},
	}
	filterLogsResult := struct {
		Logs []types.Log
		Err  error
	}{
		Logs: log,
		Err:  nil,
	}
	tests := []struct {
		name                   string
		currentStatus          *preconf.OptimismSyncStatus
		newStatus              *preconf.OptimismSyncStatus
		expectStatusUpdate     bool
		expectDepositTxsUpdate bool
		mockLogFilterer        *mockLogFilterer
	}{
		{
			name:          "Initial Status",
			currentStatus: nil,
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 13},
				UnsafeL2:  preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
			},
			expectStatusUpdate:     true,
			expectDepositTxsUpdate: true,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         10 * time.Millisecond,
			},
		},
		{
			name: "L1 Block Changed",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 13},
				UnsafeL2:  preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 11},
				HeadL1:    preconf.L1BlockRef{Number: 14},
				UnsafeL2:  preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
			},
			expectStatusUpdate:     true,
			expectDepositTxsUpdate: true,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         10 * time.Millisecond,
			},
		},
		{
			name: "No Change",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			expectStatusUpdate:     false,
			expectDepositTxsUpdate: false,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         10 * time.Millisecond,
			},
		},
		{
			name: "No L1 Block Change",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 10}},
			},
			expectStatusUpdate:     true,
			expectDepositTxsUpdate: false,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         10 * time.Millisecond,
			},
		},
		{
			name: "L1 Reorg",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 9}, // Decreased
				HeadL1:    preconf.L1BlockRef{Number: 16},
				UnsafeL2:  preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 9}},
			},
			expectStatusUpdate:     false,
			expectDepositTxsUpdate: false,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         10 * time.Millisecond,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			c := &preconfChecker{
				optimismSyncStatus:   tt.currentStatus,
				optimismSyncStatusOk: true,
				env:                  &environment{},
				envUpdatedAt:         time.Now(),
				depositTxs:           []*types.Transaction{},
				unSealedPreconfTxsCh: make(chan []*types.Transaction),
				minerConfig:          &preconf.DefaultMinerConfig,
			}

			c.l1ethclient = &mockLogFilterer{
				FilterLogsResult: tt.mockLogFilterer.FilterLogsResult,
				WaitTime:         tt.mockLogFilterer.WaitTime,
			}

			originalDepositTxs := c.depositTxs
			originalOptimismSyncStatus := c.optimismSyncStatus

			// Call the function
			c.UpdateOptimismSyncStatus(tt.newStatus)

			// Check if status was updated
			if tt.expectStatusUpdate && (c.optimismSyncStatus == originalOptimismSyncStatus || c.optimismSyncStatus != tt.newStatus) {
				t.Fatalf("UpdateOptimismSyncStatus() did not update status, expected %p, got %p", tt.newStatus, c.optimismSyncStatus)
			}

			if !tt.expectStatusUpdate && reflect.DeepEqual(c.optimismSyncStatus, originalDepositTxs) {
				t.Fatalf("UpdateOptimismSyncStatus() updated status when it shouldn't have, expected %p, got %p, new %p", originalOptimismSyncStatus, c.optimismSyncStatus, tt.newStatus)
			}

			// Check if depositTxs were updated
			if tt.expectDepositTxsUpdate && reflect.DeepEqual(c.depositTxs, originalDepositTxs) {
				t.Fatalf("UpdateOptimismSyncStatus() depositTxs update mismatch, expected update: %v", tt.expectDepositTxsUpdate)
			}
		})
	}
}

func TestUpdateOptimismSyncStatusDelay(t *testing.T) {
	log := []types.Log{
		{
			Address: common.HexToAddress("0xa513e6e4b8f2a923d98304ec87f64353c4d5c853"),
			Topics: []common.Hash{
				common.HexToHash("0xb3813568d9991fc951961fcb4c784893574240a28925604d09fc577c55bb7c32"),
				common.HexToHash("0x0000000000000000000000001276878a594ca255338adfa4d48449f69242fca0"),
				common.HexToHash("0x0000000000000000000000004200000000000000000000000000000000000007"),
				common.HexToHash("0x0000000000000000000000000000000000000000000000000000000000000001"),
			},
			Data:        common.Hex2Bytes("0000000000000000000000000000000000000000000000000000000000000020000000000000000000000000000000000000000000000000000000000000024d0000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000024ef1200ff8daf150001000000000000000000000000000000000000000000000000000000000000000000000000000000000000dc64a140aa3e981100a9beca4e685f962f0cf6c900000000000000000000000042000000000000000000000000000000000000100000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000001e848000000000000000000000000000000000000000000000000000000000000000e000000000000000000000000000000000000000000000000000000000000000a4f407a99e000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb92266000000000000000000000000f39fd6e51aad88f6f4ce6ab8827279cfffb922660000000000000000000000000000000000000000000000000000000000000001000000000000000000000000000000000000000000000000000000000000008000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000000"),
			BlockHash:   common.HexToHash("0x123"),
			BlockNumber: 10,
			TxHash:      common.HexToHash("0x123"),
			TxIndex:     0,
		},
	}
	filterLogsResult := struct {
		Logs []types.Log
		Err  error
	}{
		Logs: log,
		Err:  nil,
	}
	tests := []struct {
		name                   string
		currentStatus          *preconf.OptimismSyncStatus
		newStatus              *preconf.OptimismSyncStatus
		expectStatusUpdate     bool
		expectDepositTxsUpdate bool
		mockLogFilterer        *mockLogFilterer
	}{
		{
			name: "L1 Delay",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 10},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1: preconf.L1BlockRef{Number: 11},
				HeadL1:    preconf.L1BlockRef{Number: 15},
				UnsafeL2:  preconf.L2BlockRef{Number: 26, L1Origin: preconf.BlockID{Number: 11}},
			},
			expectStatusUpdate:     true,
			expectDepositTxsUpdate: false,
			mockLogFilterer: &mockLogFilterer{
				FilterLogsResult: filterLogsResult,
				WaitTime:         2 * time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			c := &preconfChecker{
				optimismSyncStatus:   tt.currentStatus,
				optimismSyncStatusOk: true,
				env:                  &environment{},
				envUpdatedAt:         time.Now(),
				depositTxs:           []*types.Transaction{},
				unSealedPreconfTxsCh: make(chan []*types.Transaction),
				minerConfig:          &preconf.DefaultMinerConfig,
			}

			c.l1ethclient = &mockLogFilterer{
				FilterLogsResult: tt.mockLogFilterer.FilterLogsResult,
				WaitTime:         tt.mockLogFilterer.WaitTime,
			}

			originalDepositTxs := c.depositTxs
			originalOptimismSyncStatus := c.optimismSyncStatus

			// Call the function
			c.UpdateOptimismSyncStatus(tt.newStatus)

			// Check if status was updated
			if tt.expectStatusUpdate && (c.optimismSyncStatus == originalOptimismSyncStatus || c.optimismSyncStatus != tt.newStatus) {
				t.Fatalf("UpdateOptimismSyncStatus() did not update status, expected %p, got %p", tt.newStatus, c.optimismSyncStatus)
			}

			if !tt.expectStatusUpdate && reflect.DeepEqual(c.optimismSyncStatus, originalDepositTxs) {
				t.Fatalf("UpdateOptimismSyncStatus() updated status when it shouldn't have, expected %p, got %p, new %p", originalOptimismSyncStatus, c.optimismSyncStatus, tt.newStatus)
			}

			// Check if depositTxs were updated
			if tt.expectDepositTxsUpdate && reflect.DeepEqual(c.depositTxs, originalDepositTxs) {
				t.Fatalf("UpdateOptimismSyncStatus() depositTxs update mismatch, expected update: %v", tt.expectDepositTxsUpdate)
			}
		})
	}
}

// --- Test helpers for baseFee / block-full tests ---

func newU64(v uint64) *uint64 { return &v }

// newArsiaBlockchain creates a *core.BlockChain with MantleArsiaTime=0 (always active)
// and Arsia EIP-1559 params encoded in genesis ExtraData.
func newArsiaBlockchain(t *testing.T) *core.BlockChain {
	t.Helper()
	zero := uint64(0)
	chainConfig := &params.ChainConfig{
		ChainID:             big.NewInt(1337),
		HomesteadBlock:      big.NewInt(0),
		EIP150Block:         big.NewInt(0),
		EIP155Block:         big.NewInt(0),
		EIP158Block:         big.NewInt(0),
		ByzantiumBlock:      big.NewInt(0),
		ConstantinopleBlock: big.NewInt(0),
		PetersburgBlock:     big.NewInt(0),
		IstanbulBlock:       big.NewInt(0),
		MuirGlacierBlock:    big.NewInt(0),
		BerlinBlock:         big.NewInt(0),
		LondonBlock:         big.NewInt(0),
		Ethash:              new(params.EthashConfig),
		MantleArsiaTime:     &zero,
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:        50,
			EIP1559Denominator:       50,
			EIP1559DenominatorCanyon: newU64(50),
		},
	}
	genesis := &core.Genesis{
		Config:     chainConfig,
		ExtraData:  eip1559.EncodeMinBaseFeeExtraData(50, 50, 0),
		GasLimit:   30_000_000,
		BaseFee:    big.NewInt(params.InitialBaseFee),
		Difficulty: big.NewInt(1),
	}
	chainDB := rawdb.NewMemoryDatabase()
	bc, err := core.NewBlockChain(chainDB, genesis, ethash.NewFaker(), nil)
	if err != nil {
		t.Fatalf("NewBlockChain: %v", err)
	}
	t.Cleanup(bc.Stop)
	return bc
}

// newTestEnv creates a minimal *environment backed by the genesis state of bc.
// gasPoolGas controls the available gas (pass header.GasLimit for a full pool).
func newTestEnv(t *testing.T, bc *core.BlockChain, header *types.Header, gasPoolGas uint64) *environment {
	t.Helper()
	statedb, err := bc.StateAt(bc.Genesis().Header())
	if err != nil {
		t.Fatalf("StateAt: %v", err)
	}
	chainConfig := bc.Config()
	hdr := types.CopyHeader(header)
	blockCtx := core.NewEVMBlockContext(hdr, bc, nil, chainConfig, statedb)
	evm := vm.NewEVM(blockCtx, statedb, chainConfig, vm.Config{})
	gp := core.NewGasPool(gasPoolGas)
	return &environment{
		signer:  types.MakeSigner(chainConfig, hdr.Number, hdr.Time),
		state:   statedb,
		evm:     evm,
		gasPool: gp,
		header:  hdr,
	}
}

// --- Block-full tests: verify virtual block advance is removed ---

func TestApplyTxWithResetEnv_GasLimitReturnsBlockFull(t *testing.T) {
	bc := newArsiaBlockchain(t)

	blobGasUsed := uint64(0)
	header := &types.Header{
		Number:      big.NewInt(11),
		GasLimit:    30_000_000,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		Time:        0,
		Extra:       eip1559.EncodeMinBaseFeeExtraData(50, 50, 0),
		BlobGasUsed: &blobGasUsed,
	}
	// gasPool = 0 → any tx will trigger ErrGasLimitReached
	env := newTestEnv(t, bc, header, 0)
	originalNumber := new(big.Int).Set(env.header.Number)

	checker := &preconfChecker{
		blockchain:  bc,
		env:         env,
		minerConfig: &preconf.DefaultMinerConfig,
	}

	// Create a simple tx (gas required > 0 but pool has 0 gas)
	key, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(key.PublicKey)
	// Fund sender so it passes balance check and hits the gas pool check
	env.state.AddBalance(sender, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)
	signer := types.NewEIP155Signer(bc.Config().ChainID)
	tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
		Nonce:    0,
		Gas:      21_000,
		GasPrice: big.NewInt(params.InitialBaseFee),
	}), signer, key)

	_, _, err := checker.applyTxWithResetEnv(env, tx)

	// Must get ErrPreconfBlockFull (wrapping ErrGasLimitReached)
	if !errors.Is(err, ErrPreconfBlockFull) {
		t.Fatalf("expected ErrPreconfBlockFull, got %v", err)
	}
	if !errors.Is(err, core.ErrGasLimitReached) {
		t.Fatalf("expected wrapped ErrGasLimitReached, got %v", err)
	}
	// header.Number must NOT have advanced (no virtual block advance)
	if env.header.Number.Cmp(originalNumber) != 0 {
		t.Fatalf("header.Number must not advance: got %v, want %v", env.header.Number, originalNumber)
	}
}

func TestApplyTxWithResetEnv_DASpaceReturnsBlockFull(t *testing.T) {
	bc := newArsiaBlockchain(t)

	blobGasUsed := uint64(30_000_000) // DA completely saturated
	header := &types.Header{
		Number:      big.NewInt(11),
		GasLimit:    30_000_000,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		Time:        0,
		Extra:       eip1559.EncodeMinBaseFeeExtraData(50, 50, 0),
		BlobGasUsed: &blobGasUsed,
	}
	// Gas pool is fine, but DA is exhausted
	env := newTestEnv(t, bc, header, header.GasLimit)
	env.daFootprintGasScalar = 1 // activate DA check (scalar=0 disables it)
	originalNumber := new(big.Int).Set(env.header.Number)

	checker := &preconfChecker{
		blockchain:  bc,
		env:         env,
		minerConfig: &preconf.DefaultMinerConfig,
	}

	key, _ := crypto.GenerateKey()
	signer := types.NewEIP155Signer(bc.Config().ChainID)
	tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
		Nonce:    0,
		Gas:      21_000,
		GasPrice: big.NewInt(params.InitialBaseFee),
	}), signer, key)

	_, _, err := checker.applyTxWithResetEnv(env, tx)

	// Must get ErrPreconfBlockFull (wrapping ErrDAFootprintLimitReached)
	if !errors.Is(err, ErrPreconfBlockFull) {
		t.Fatalf("expected ErrPreconfBlockFull, got %v", err)
	}
	if !errors.Is(err, core.ErrDAFootprintLimitReached) {
		t.Fatalf("expected wrapped ErrDAFootprintLimitReached, got %v", err)
	}
	// header.Number must NOT have advanced
	if env.header.Number.Cmp(originalNumber) != 0 {
		t.Fatalf("header.Number must not advance: got %v, want %v", env.header.Number, originalNumber)
	}
}

// --- UnpausePreconf baseFee recalculation test ---

func TestUnpausePreconf_BaseFeeRecalculated(t *testing.T) {
	bc := newArsiaBlockchain(t)
	chainConfig := bc.Config()

	gasLimit := uint64(30_000_000)
	daUsed := gasLimit // BlobGasUsed == GasLimit → parentGasMetered = max(GasUsed, BlobGasUsed)
	arsiaExtra := eip1559.EncodeMinBaseFeeExtraData(50, 50, 0)

	sealedHeader := &types.Header{
		Number:      big.NewInt(10),
		GasLimit:    gasLimit,
		GasUsed:     gasLimit, // 100% utilisation → baseFee must increase
		BlobGasUsed: &daUsed,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		Time:        0,
		Extra:       arsiaExtra,
		Difficulty:  big.NewInt(1),
	}

	// Ground truth: what CalcBaseFee computes for the next block
	expectedBaseFee := eip1559.CalcBaseFee(chainConfig, sealedHeader)
	if expectedBaseFee.Cmp(sealedHeader.BaseFee) == 0 {
		t.Fatal("test setup error: baseFee unchanged — gas params may be wrong")
	}

	// Build environment from the sealed header
	env := newTestEnv(t, bc, sealedHeader, gasLimit)
	// BlobGasUsed must match for CalcBaseFee to read the correct parentGasMetered
	*env.header.BlobGasUsed = daUsed
	env.header.GasUsed = gasLimit

	// Set up the unsealed preconf txs channel (empty, no overflow)
	unSealedCh := make(chan []*types.Transaction, 1)
	unSealedCh <- []*types.Transaction{}

	checker := &preconfChecker{
		blockchain:           bc,
		depositTxs:           nil,
		unSealedPreconfTxsCh: unSealedCh,
		minerConfig:          &preconf.DefaultMinerConfig,
	}

	readyCalled := false
	// UnpausePreconf defers mu.Unlock(), so we must hold the lock
	checker.mu.Lock()
	checker.UnpausePreconf(env, func() { readyCalled = true })

	// preconfReady callback was called
	if !readyCalled {
		t.Error("preconfReady was not called")
	}

	// Block number advanced by 1
	wantNumber := new(big.Int).Add(sealedHeader.Number, common.Big1)
	if checker.env.header.Number.Cmp(wantNumber) != 0 {
		t.Errorf("header.Number: got %v, want %v", checker.env.header.Number, wantNumber)
	}

	// BaseFee on the header is recalculated (not stale)
	if checker.env.header.BaseFee.Cmp(expectedBaseFee) != 0 {
		t.Errorf("header.BaseFee: got %v, want %v",
			checker.env.header.BaseFee, expectedBaseFee)
	}

	// EVM BlockContext.BaseFee is also updated (EVM was rebuilt)
	if checker.env.evm.Context.BaseFee.Cmp(expectedBaseFee) != 0 {
		t.Errorf("evm.Context.BaseFee: got %v, want %v",
			checker.env.evm.Context.BaseFee, expectedBaseFee)
	}

	// BlobGasUsed reset to 0 for new block
	if *checker.env.header.BlobGasUsed != 0 {
		t.Errorf("BlobGasUsed not reset: got %d", *checker.env.header.BlobGasUsed)
	}

	// Gas pool reset to full
	if checker.env.gasPool.Gas() != gasLimit {
		t.Errorf("gasPool.Gas(): got %d, want %d", checker.env.gasPool.Gas(), gasLimit)
	}
}

// --- C1 supplement: non gas/DA errors pass through without wrapping ---

func TestApplyTxWithResetEnv_OtherErrorsPassThrough(t *testing.T) {
	bc := newArsiaBlockchain(t)

	blobGasUsed := uint64(0)
	header := &types.Header{
		Number:      big.NewInt(11),
		GasLimit:    30_000_000,
		BaseFee:     big.NewInt(params.InitialBaseFee),
		Time:        0,
		Extra:       eip1559.EncodeMinBaseFeeExtraData(50, 50, 0),
		BlobGasUsed: &blobGasUsed,
	}
	env := newTestEnv(t, bc, header, header.GasLimit) // gas pool is full

	checker := &preconfChecker{
		blockchain:  bc,
		env:         env,
		minerConfig: &preconf.DefaultMinerConfig,
	}

	// Create a tx with nonce=999 (way ahead of actual nonce 0) → triggers nonce-too-high error
	key, _ := crypto.GenerateKey()
	sender := crypto.PubkeyToAddress(key.PublicKey)
	env.state.AddBalance(sender, uint256.NewInt(1e18), tracing.BalanceChangeUnspecified)
	signer := types.NewEIP155Signer(bc.Config().ChainID)
	tx, _ := types.SignTx(types.NewTx(&types.LegacyTx{
		Nonce:    999, // nonce too high
		Gas:      21_000,
		GasPrice: big.NewInt(params.InitialBaseFee),
	}), signer, key)

	_, _, err := checker.applyTxWithResetEnv(env, tx)

	// Must NOT be ErrPreconfBlockFull — should be original error
	if err == nil {
		t.Fatal("expected error for nonce-too-high tx, got nil")
	}
	if errors.Is(err, ErrPreconfBlockFull) {
		t.Fatalf("non gas/DA error should not be wrapped as ErrPreconfBlockFull, got %v", err)
	}
}

// --- C2 supplement: baseFee decreases on low utilisation ---

func TestUnpausePreconf_BaseFeeDecreasesOnLowUtil(t *testing.T) {
	bc := newArsiaBlockchain(t)
	chainConfig := bc.Config()

	gasLimit := uint64(30_000_000)
	daUsed := uint64(0) // empty block
	arsiaExtra := eip1559.EncodeMinBaseFeeExtraData(50, 50, 0)

	// Start with a high baseFee so there's room to decrease
	highBaseFee := new(big.Int).Mul(big.NewInt(params.InitialBaseFee), big.NewInt(10))

	sealedHeader := &types.Header{
		Number:      big.NewInt(10),
		GasLimit:    gasLimit,
		GasUsed:     0, // 0% utilisation → baseFee must decrease
		BlobGasUsed: &daUsed,
		BaseFee:     highBaseFee,
		Time:        0,
		Extra:       arsiaExtra,
		Difficulty:  big.NewInt(1),
	}

	expectedBaseFee := eip1559.CalcBaseFee(chainConfig, sealedHeader)
	if expectedBaseFee.Cmp(sealedHeader.BaseFee) >= 0 {
		t.Fatal("test setup error: baseFee did not decrease — expected lower baseFee for empty block")
	}

	env := newTestEnv(t, bc, sealedHeader, gasLimit)
	*env.header.BlobGasUsed = daUsed
	env.header.GasUsed = 0

	unSealedCh := make(chan []*types.Transaction, 1)
	unSealedCh <- []*types.Transaction{}

	checker := &preconfChecker{
		blockchain:           bc,
		depositTxs:           nil,
		unSealedPreconfTxsCh: unSealedCh,
		minerConfig:          &preconf.DefaultMinerConfig,
	}

	checker.mu.Lock()
	checker.UnpausePreconf(env, func() {})

	// BaseFee must have decreased
	if checker.env.header.BaseFee.Cmp(highBaseFee) >= 0 {
		t.Errorf("header.BaseFee should decrease: got %v, parent was %v",
			checker.env.header.BaseFee, highBaseFee)
	}
	if checker.env.header.BaseFee.Cmp(expectedBaseFee) != 0 {
		t.Errorf("header.BaseFee: got %v, want %v",
			checker.env.header.BaseFee, expectedBaseFee)
	}

	// EVM must also reflect the decreased baseFee
	if checker.env.evm.Context.BaseFee.Cmp(expectedBaseFee) != 0 {
		t.Errorf("evm.Context.BaseFee: got %v, want %v",
			checker.env.evm.Context.BaseFee, expectedBaseFee)
	}
}
