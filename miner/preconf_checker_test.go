package miner

import (
	"testing"

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
