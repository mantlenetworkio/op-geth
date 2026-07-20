// Copyright 2026 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

//go:build mantle_replay
// +build mantle_replay

package replay

import (
	"math/big"

	"github.com/ethereum/go-ethereum/params"
)

// MantleMainnetConfig returns a ChainConfig matching Mantle mainnet fork
// timestamps. Mirrors the logic in core/genesis.go that applies
// MantleMainnetUpgradeConfig onto a base post-Bedrock ChainConfig.
//
// Mantle mainnet does NOT enable Amsterdam — AmsterdamTime is left nil to
// match core/genesis.go and the live node.
func MantleMainnetConfig() *params.ChainConfig {
	cfg := &params.ChainConfig{
		ChainID:                 big.NewInt(5000),
		HomesteadBlock:          big.NewInt(0),
		EIP150Block:             big.NewInt(0),
		EIP155Block:             big.NewInt(0),
		EIP158Block:             big.NewInt(0),
		ByzantiumBlock:          big.NewInt(0),
		ConstantinopleBlock:     big.NewInt(0),
		PetersburgBlock:         big.NewInt(0),
		IstanbulBlock:           big.NewInt(0),
		MuirGlacierBlock:        big.NewInt(0),
		BerlinBlock:             big.NewInt(0),
		LondonBlock:             big.NewInt(0),
		ArrowGlacierBlock:       big.NewInt(0),
		GrayGlacierBlock:        big.NewInt(0),
		MergeNetsplitBlock:      big.NewInt(0),
		TerminalTotalDifficulty: big.NewInt(0),
		BedrockBlock:            big.NewInt(0),
		RegolithTime:            u64Ptr(0),
		Optimism: &params.OptimismConfig{
			EIP1559Elasticity:  10,
			EIP1559Denominator: 50,
		},
	}
	mu := params.MantleMainnetUpgradeConfig
	cfg.BaseFeeTime = mu.BaseFeeTime
	cfg.BVMETHMintUpgradeTime = mu.BVMETHMintUpgradeTime
	cfg.MetaTxV2UpgradeTime = mu.MetaTxV2UpgradeTime
	cfg.MetaTxV3UpgradeTime = mu.MetaTxV3UpgradeTime
	cfg.ProxyOwnerUpgradeTime = mu.ProxyOwnerUpgradeTime
	cfg.MantleEverestTime = mu.MantleEverestTime
	cfg.MantleSkadiTime = mu.MantleSkadiTime
	cfg.MantleLimbTime = mu.MantleLimbTime
	cfg.MantleArsiaTime = mu.MantleArsiaTime
	cfg.MantleElysiumTime = mu.MantleElysiumTime
	// OP forks aligned to MantleArsia.
	cfg.CanyonTime = cfg.MantleArsiaTime
	cfg.EcotoneTime = cfg.MantleArsiaTime
	cfg.FjordTime = cfg.MantleArsiaTime
	cfg.GraniteTime = cfg.MantleArsiaTime
	cfg.HoloceneTime = cfg.MantleArsiaTime
	cfg.IsthmusTime = cfg.MantleArsiaTime
	cfg.JovianTime = cfg.MantleArsiaTime
	// EVM standard forks aligned to MantleSkadi / MantleLimb.
	cfg.ShanghaiTime = cfg.MantleSkadiTime
	cfg.CancunTime = cfg.MantleSkadiTime
	cfg.PragueTime = cfg.MantleSkadiTime
	cfg.OsakaTime = cfg.MantleLimbTime
	// Mantle mainnet does NOT enable Amsterdam — keep nil to match genesis.go.
	cfg.AmsterdamTime = nil
	return cfg
}

func u64Ptr(v uint64) *uint64 { return &v }
