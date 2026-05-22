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
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/holiman/uint256"
)

// ─────────────────────────────────────────────────────────────────────────
// Case 1: pre-Everest MetaTx
// ─────────────────────────────────────────────────────────────────────────

// TestReplayMainnetMetaTx_5d1d3888 byte-equal replays mantle mainnet block
// 63634683 pre-Everest MetaTx
// 0x5d1d3888f2a19d6716b7571b9391a02ca0d48596cf30b5cd516fed7f74309c43.
//
// Asserts:
//   - codec parity (reconstructed tx hash matches mainnet)
//   - receipt.Status / GasUsed / L1Fee / L1GasUsed byte-equal mainnet
//   - sender nonce 0 -> 1
//   - user balance stays 0 (SponsorPercent=100 -> sponsor pays all fees)
//
// Regression guard for:
//   - pre-Arsia L1 fee model (rollupDataGas + overhead) * l1BaseFee * scalar * tokenRatio
//   - pre-Everest MetaTx decoding + buyGas sponsor accounting
//   - L1Block / GasOracle storage layout
//   - merged gasUsed model (L2 gas + L1 cost converted to L2 gas)
func TestReplayMainnetMetaTx_5d1d3888(t *testing.T) {
	var (
		user    = common.HexToAddress("0x13023d2f562b656b3c35e65956cf527c893d5fdf")
		sponsor = common.HexToAddress("0x1220d2767171ea3a6f4a545eff23efaad4c80221")
		to      = common.HexToAddress("0x792c9e49df017c00cc265cc9c09d4130c0eaddc6")

		// 50 ETH (eth_getBalance @ block 63634682).
		sponsorBalancePre = uint256.MustFromHex("0x2b5e3af16b1880000")
	)

	// Raw input = 32-byte MetaTxPrefix + RLP-encoded MetaTxParams.
	input := hexutil.MustDecode("0x00000000000000000000000000004d616e746c654d6574615478507265666978f8838403cafd5c64a46057361d0000000000000000000000000000000000000000000000000000000000000003941220d2767171ea3a6f4a545eff23efaad4c802211ca092d216b2bb7771cdd639075f96b8648e14d7da5f76dc5e67b2df053e788306d6a03e90d53160c1a556a792a4eb5b231e83a6d01e98e9a6f457f1b0e5be6290ebf4")

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:   big.NewInt(5000),
		Nonce:     0,
		GasTipCap: big.NewInt(0),
		GasFeeCap: big.NewInt(0x1312d00),
		Gas:       0x68eb300,
		To:        &to,
		Value:     big.NewInt(0),
		Data:      input,
		V:         big.NewInt(0),
		R:         hexutil.MustDecodeBig("0x6fe7e518f984cc33af1b48720d4b87ffa8dbb1638580c11fad4bdc54e5e05a2f"),
		S:         hexutil.MustDecodeBig("0x865472d6a790c5149afff442d821bafa3daec476eba0c6792d7732a5c9126f3"),
	})

	(&ReplayCase{
		Name:       "metatx-5d1d3888",
		Tx:         tx,
		ExpectHash: common.HexToHash("0x5d1d3888f2a19d6716b7571b9391a02ca0d48596cf30b5cd516fed7f74309c43"),
		Header: &types.Header{
			Number:   new(big.Int).SetUint64(63634683),
			Time:     1715399678,
			GasLimit: 200_000_000_000,
			BaseFee:  big.NewInt(0x1312d00),
			Coinbase: common.HexToAddress("0x4200000000000000000000000000000000000011"),
		},
		Sender: user,
		PreState: PreState{
			Balances: map[common.Address]*uint256.Int{
				sponsor: sponsorBalancePre,
			},
			// L1Block slots from eth_getStorageAt @ block 63634682.
			L1Block: &L1BlockState{
				L1BaseFee: big.NewInt(0x12ce52460),
				Overhead:  big.NewInt(0xbc),
				Scalar:    big.NewInt(0x2710),
			},
			TokenRatio: big.NewInt(0xb48), // 2888
		},
		Golden: GoldenReceipt{
			Status:    types.ReceiptStatusSuccessful,
			GasUsed:   0x5667a70, // 90,602,096
			L1Fee:     new(big.Int).SetUint64(0x216195c2bada3),
			L1GasUsed: big.NewInt(0xfbc), // 4028 = rollupDataGas + overhead
		},
		PostNonce:       map[common.Address]uint64{user: 1},
		PostBalanceZero: []common.Address{user},
	}).Run(t, MantleMainnetConfig())
}

// ─────────────────────────────────────────────────────────────────────────
// Case 2: pre-Arsia plain EOA-to-EOA transfer
// ─────────────────────────────────────────────────────────────────────────

// TestReplayMainnetTransfer_4b0a92d6 byte-equal replays mantle mainnet block
// 94,028,673 pre-Arsia EOA-to-EOA transfer
// 0x4b0a92d6309e254e568c4b992445d17da5bc908978e1ed00ae6d26bae70b9322.
//
// Why this case matters (different signal than the MetaTx case):
//   - Plain DynamicFeeTx (type-2), input=0x, no MetaTx layering — exercises
//     the bog-standard pre-Arsia fee path (no sponsor accounting).
//   - Real positive non-zero msg.value (10 ETH) — exercises balance transfer
//     plus gas/L1-fee deduction on the same account.
//   - Block-internal index=1 (tx_0 is a deposit setL1BlockValues) — proves
//     L1Block/GasOracle slots queried at block-1 are stable for slots 1/5/6
//     under the mantle setL1BlockValues invariant.
//
// Asserts the same byte-equal mainnet invariants as the MetaTx case minus
// PostBalanceZero (sender keeps a residual balance after transfer + fee).
func TestReplayMainnetTransfer_4b0a92d6(t *testing.T) {
	var (
		sender = common.HexToAddress("0x5adf20216a9f9b61d47584b68b497b5e82df2eee")
		to     = common.HexToAddress("0x1220d2767171ea3a6f4a545eff23efaad4c80221")

		// eth_getBalance @ block 94,028,672 (block-1).
		senderBalancePre = uint256.MustFromHex("0xad78ebc5ac6200000")
		toBalancePre     = uint256.MustFromHex("0xb1baf6a0fd506542")
	)

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(5000),
		Nonce:      0,
		GasTipCap:  big.NewInt(0x186a0),   // 100,000
		GasFeeCap:  big.NewInt(0x16fbca0), // 24,100,000
		Gas:        0x5176a5a,             // 85,432,410
		To:         &to,
		Value:      hexutil.MustDecodeBig("0x8ac7230489e80000"), // 10 ETH
		Data:       nil,
		AccessList: nil,
		V:          big.NewInt(0),
		R:          hexutil.MustDecodeBig("0xf9299fa2b2bb5e1e57475b7184d83da80dd17714187301e93c5291ccf88a5499"),
		S:          hexutil.MustDecodeBig("0x502d69955e02c7dca497e85d4893013561fbe6a39e4da382a499f969599cedf8"),
	})

	(&ReplayCase{
		Name:       "transfer-4b0a92d6",
		Tx:         tx,
		ExpectHash: common.HexToHash("0x4b0a92d6309e254e568c4b992445d17da5bc908978e1ed00ae6d26bae70b9322"),
		Header: &types.Header{
			Number:   new(big.Int).SetUint64(94028673),
			Time:     0x69dcbd0a, // 1,775,533,322
			GasLimit: 0x2e90edd000,
			BaseFee:  big.NewInt(0x1312d00),
			Coinbase: common.HexToAddress("0x4200000000000000000000000000000000000011"),
		},
		Sender: sender,
		PreState: PreState{
			Balances: map[common.Address]*uint256.Int{
				sender: senderBalancePre,
				to:     toBalancePre,
			},
			// L1Block slots from eth_getStorageAt @ block 94,028,672.
			L1Block: &L1BlockState{
				L1BaseFee: big.NewInt(0x171ca62d), // 388,990,509
				Overhead:  big.NewInt(0xbc),       // 188
				Scalar:    big.NewInt(0x2710),     // 10,000
			},
			TokenRatio: big.NewInt(0xce7), // 3303 (matches receipt.tokenRatio)
		},
		Golden: GoldenReceipt{
			Status:    types.ReceiptStatusSuccessful,
			GasUsed:   0x436aecd,                                  // 70,692,557
			L1Fee:     hexutil.MustDecodeBig("0x18467146780e"),    // 26,690,827,221,006
			L1GasUsed: big.NewInt(0x824),                          // 2,084 = rollupDataGas + overhead
		},
		PostNonce: map[common.Address]uint64{sender: 1},
	}).Run(t, MantleMainnetConfig())
}

// ─────────────────────────────────────────────────────────────────────────
// Case 3: post-Arsia self-transfer (vanilla-eth gas model, Ecotone L1 fee,
//         operator-fee infrastructure active)
// ─────────────────────────────────────────────────────────────────────────

// TestReplayMainnetTransfer_e0811036_PostArsia byte-equal replays mantle
// mainnet block 94,438,807 post-Arsia self-transfer
// 0xe081103e6990d27e08d88e29216e2497beecc730c18192ff41ca92bbd951b87a.
//
// Why this case matters (qualitatively different from pre-Arsia cases):
//
//   - GasLimit on the header is back to vanilla-ethereum scale (60M, not the
//     200B pre-Arsia mantle value). gasUsed is the canonical 21,000 for a
//     plain transfer.
//
//   - L1 fee is computed via the Ecotone/Fjord-style formula reading slots
//     1/3/7 of L1Block (l1BaseFee, packed l1FeeScalars, l1BlobBaseFee). The
//     pre-Arsia slot 5/6 (overhead/scalar) are no longer consulted.
//
//   - Operator fee infrastructure is active: NewOperatorCostFunc reads
//     L1Block slot 8 (packed operatorFeeScalar+operatorFeeConstant). For this
//     tx the operator fee resolves to zero (operatorFeeConstant = 0), but
//     wiring the params still exercises the post-Arsia code path.
//
//   - tokenRatio is still in GasOracle slot 0 (kept for shape parity) but it
//     no longer feeds the L1 fee formula post-Arsia — L1 fee is denominated
//     in MNT directly via the scalars.
//
// Asserts the same byte-equal mainnet invariants as pre-Arsia cases:
//   - codec parity (reconstructed tx hash matches mainnet)
//   - receipt.Status / GasUsed / L1Fee / L1GasUsed byte-equal mainnet
//   - sender nonce 0x5f -> 0x60.
func TestReplayMainnetTransfer_e0811036_PostArsia(t *testing.T) {
	// from == to: this is a self-transfer (1 MNT round-trips, only fees are net).
	addr := common.HexToAddress("0x1220d2767171ea3a6f4a545eff23efaad4c80221")

	tx := types.NewTx(&types.DynamicFeeTx{
		ChainID:    big.NewInt(5000),
		Nonce:      0x5f,                      // 95
		GasTipCap:  big.NewInt(0x186a0),       // 100,000 wei
		GasFeeCap:  big.NewInt(0x22ecb3e2a0),  // 150,000,000,000 wei (~150 gwei)
		Gas:        0x5208,                    // 21,000 vanilla-eth transfer
		To:         &addr,
		Value:      hexutil.MustDecodeBig("0xde0b6b3a7640000"), // 1 ETH
		Data:       nil,
		AccessList: nil,
		V:          big.NewInt(0),
		R:          hexutil.MustDecodeBig("0x4b995917fba6d0dd12fd61914ecafb4ad560c58e635d0fb9b54d1ccefd1d3a89"),
		S:          hexutil.MustDecodeBig("0x5fd5aa60ebf7e1bcc40d12b01dabe6f633bc29c1c5e439c6f8686f9a73bbcce8"),
	})

	(&ReplayCase{
		Name:       "transfer-e0811036-postarsia",
		Tx:         tx,
		ExpectHash: common.HexToHash("0xe081103e6990d27e08d88e29216e2497beecc730c18192ff41ca92bbd951b87a"),
		Header: &types.Header{
			Number:   new(big.Int).SetUint64(94438807),
			Time:     0x69e8dd36,               // 1,776,868,662 — past MantleArsiaTime 1,776,841,200
			GasLimit: 0x3938700,                // 60,000,000 (vanilla-eth scale)
			BaseFee:  big.NewInt(0xba43b7400),  // 50 gwei
			Coinbase: common.HexToAddress("0x4200000000000000000000000000000000000011"),
		},
		Sender: addr,
		PreState: PreState{
			Balances: map[common.Address]*uint256.Int{
				addr: uint256.MustFromHex("0x13c7e5eacc4009f42"), // 22.805769... MNT
			},
			Nonces: map[common.Address]uint64{addr: 0x5f},
			// Post-Arsia L1Block slots from eth_getStorageAt @ block 94,438,806.
			// slot 3 / slot 8 are copied verbatim (packed values).
			L1BlockArsia: &L1BlockArsiaState{
				L1BaseFee:         big.NewInt(0x2689c9888), // slot 1 — l1GasPrice 10,344,265,352
				L1FeeScalars:      common.HexToHash("0x000000000000000000000000000000000002943b000000000000000000000004"),
				L1BlobBaseFee:     hexutil.MustDecodeBig("0x12ff986549a8f43431"), // slot 7
				OperatorFeeParams: common.HexToHash("0x000000000000000000000000000000000000019005f5e1000000000000000000"),
			},
			TokenRatio: big.NewInt(0xea0), // 3744 — kept for state-shape parity
		},
		Golden: GoldenReceipt{
			Status:    types.ReceiptStatusSuccessful,
			GasUsed:   0x5208, // 21,000 — vanilla-eth transfer
			L1Fee:     hexutil.MustDecodeBig("0x253646c8b6da40"),
			L1GasUsed: big.NewInt(0x640), // 1600
		},
		PostNonce: map[common.Address]uint64{addr: 0x60},
	}).Run(t, MantleMainnetConfig())
}
