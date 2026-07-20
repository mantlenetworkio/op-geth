// Copyright 2024 The go-ethereum Authors
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

package types

import (
	"bytes"
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var (
	depositABI   = abi.ABI{Methods: map[string]abi.Method{"DepositEvent": depositEvent}}
	bytesT, _    = abi.NewType("bytes", "", nil)
	depositEvent = abi.NewMethod("DepositEvent", "DepositEvent", abi.Function, "", false, false, []abi.Argument{
		{Name: "pubkey", Type: bytesT, Indexed: false},
		{Name: "withdrawal_credentials", Type: bytesT, Indexed: false},
		{Name: "amount", Type: bytesT, Indexed: false},
		{Name: "signature", Type: bytesT, Indexed: false},
		{Name: "index", Type: bytesT, Indexed: false}}, nil,
	)
)

// FuzzUnpackIntoDeposit tries roundtrip packing and unpacking of deposit events.
func FuzzUnpackIntoDeposit(f *testing.F) {
	for _, tt := range []struct {
		pubkey string
		wxCred string
		amount string
		sig    string
		index  string
	}{
		{
			pubkey: "111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111111",
			wxCred: "2222222222222222222222222222222222222222222222222222222222222222",
			amount: "3333333333333333",
			sig:    "444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444444",
			index:  "5555555555555555",
		},
	} {
		f.Add(common.FromHex(tt.pubkey), common.FromHex(tt.wxCred), common.FromHex(tt.amount), common.FromHex(tt.sig), common.FromHex(tt.index))
	}

	f.Fuzz(func(t *testing.T, p []byte, w []byte, a []byte, s []byte, i []byte) {
		var (
			pubkey [48]byte
			wxCred [32]byte
			amount [8]byte
			sig    [96]byte
			index  [8]byte
		)
		copy(pubkey[:], p)
		copy(wxCred[:], w)
		copy(amount[:], a)
		copy(sig[:], s)
		copy(index[:], i)

		var enc []byte
		enc = append(enc, pubkey[:]...)
		enc = append(enc, wxCred[:]...)
		enc = append(enc, amount[:]...)
		enc = append(enc, sig[:]...)
		enc = append(enc, index[:]...)

		out, err := depositABI.Pack("DepositEvent", pubkey[:], wxCred[:], amount[:], sig[:], index[:])
		if err != nil {
			t.Fatalf("error packing deposit: %v", err)
		}
		got, err := DepositLogToRequest(out[4:])
		if err != nil {
			t.Errorf("error unpacking deposit: %v", err)
		}
		if len(got) != depositRequestSize {
			t.Errorf("wrong output size: %d, want %d", len(got), depositRequestSize)
		}
		if !bytes.Equal(enc, got) {
			t.Errorf("roundtrip failed: want %x, got %x", enc, got)
		}
	})
}

func TestDepositTx(t *testing.T) {
	depositTxStr := `{
        "blockHash": "0x13d90e1a5788116c43535ddaf6f52aa69253ed4621e8cd5247ce94f93eaf5c8f",
        "blockNumber": "0x3cafcfb",
        "ethValue": "0x0",
        "from": "0xdeaddeaddeaddeaddeaddeaddeaddeaddead0001",
        "gas": "0xf4240",
        "gasPrice": "0x0",
        "hash": "0x15ef2ac0a08b3c747ab8c938dde2164728ae3fd0c1981b0b88bee64e7c424ba8",
        "input": "0x015d8eb900000000000000000000000000000000000000000000000000000000012ecc8700000000000000000000000000000000000000000000000000000000663eebbf000000000000000000000000000000000000000000000000000000012ce52460e091984cf3d2da0b2408a6611fbde9a4a1eb6acedfe57689df496b70f0ec0e5000000000000000000000000000000000000000000000000000000000000000050000000000000000000000002f40d796917ffb642bd2e2bdd2c762a5e40fd74900000000000000000000000000000000000000000000000000000000000000bc0000000000000000000000000000000000000000000000000000000000002710",
        "mint": "0x0",
        "nonce": "0x259410",
        "r": "0x0",
        "s": "0x0",
        "sourceHash": "0xd375d8d3d4cc23e681a5c2f4737813f95b8aba4184c23842c029b3477515e6e0",
        "to": "0x4200000000000000000000000000000000000015",
        "transactionIndex": "0x0",
        "type": "0x7e",
        "v": "0x0",
        "value": "0x0"
    }`

	expectFrom := common.HexToAddress("0xdeaddeaddeaddeaddeaddeaddeaddeaddead0001")
	expectSourceHash := common.HexToHash("0xd375d8d3d4cc23e681a5c2f4737813f95b8aba4184c23842c029b3477515e6e0")
	var parsedTx = &Transaction{}
	err := json.Unmarshal([]byte(depositTxStr), &parsedTx)

	require.Equal(t, expectFrom, parsedTx.From())
	require.Equal(t, expectSourceHash, parsedTx.SourceHash())
	require.Equal(t, uint64(0), parsedTx.Mint().Uint64())

	signer := LatestSignerForChainID(big.NewInt(5000))
	from, err := signer.Sender(parsedTx)
	require.NoError(t, err)
	require.Equal(t, expectFrom, from)
}
