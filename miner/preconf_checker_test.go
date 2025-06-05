package miner

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/preconf"
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 16},
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 11}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 26},
			},
			want: true,
		},
		{
			name: "Partial Growth",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},                                        // no change
				HeadL1:           preconf.L1BlockRef{Number: 16},                                        // growth
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}}, // no change
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},                                        // no change
			},
			want: true,
		},
		{
			name: "Reorg CurrentL1",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 9}, // decreased
				HeadL1:           preconf.L1BlockRef{Number: 16},
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 9}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 26},
			},
			want: false,
		},
		{
			name: "Reorg HeadL1",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 14}, // decreased
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 11}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 26},
			},
			want: false,
		},
		{
			name: "Reorg UnsafeL2",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 16},
				UnsafeL2:         preconf.L2BlockRef{Number: 19, L1Origin: preconf.BlockID{Number: 11}}, // decreased
				EngineSyncTarget: preconf.L2BlockRef{Number: 26},
			},
			want: false,
		},
		{
			name: "Reorg EngineSyncTarget",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 16},
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 11}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 24}, // decreased
			},
			want: false,
		},
		{
			name: "No Change",
			currentStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 13},
				UnsafeL2:         preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 30},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 13},
				UnsafeL2:         preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 30},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 14},
				UnsafeL2:         preconf.L2BlockRef{Number: 30, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 30},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 9}, // Decreased
				HeadL1:           preconf.L1BlockRef{Number: 16},
				UnsafeL2:         preconf.L2BlockRef{Number: 21, L1Origin: preconf.BlockID{Number: 9}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 26},
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
				CurrentL1:        preconf.L1BlockRef{Number: 10},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 20, L1Origin: preconf.BlockID{Number: 10}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
			},
			newStatus: &preconf.OptimismSyncStatus{
				CurrentL1:        preconf.L1BlockRef{Number: 11},
				HeadL1:           preconf.L1BlockRef{Number: 15},
				UnsafeL2:         preconf.L2BlockRef{Number: 26, L1Origin: preconf.BlockID{Number: 11}},
				EngineSyncTarget: preconf.L2BlockRef{Number: 25},
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
