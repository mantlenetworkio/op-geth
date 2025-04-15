package types

import (
	"encoding/json"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/stretchr/testify/require"
)

var (
	setCodeTx = `
			{
                "blockHash": "0x9e77f369a3415aef97a2577ee10d341572d6343f5204551a1ff0f0c2b57e7b3f",
                "blockNumber": "0x86236",
                "from": "0x1220d2767171ea3a6f4a545eff23efaad4c80221",
                "gas": "0xf4240",
                "gasPrice": "0xe078996",
                "maxFeePerGas": "0xe078996",
                "maxPriorityFeePerGas": "0xe07898f",
                "hash": "0xa61cdf2dde1acc00b4fcd9e550d660b07ffe77082b411425cddd5fe6cc824e5f",
                "input": "0xbeabacc8000000000000000000000000792c9e49df017c00cc265cc9c09d4130c0eaddc6000000000000000000000000cdd5134b4be40f679eac600fae02600004c94e0a0000000000000000000000000000000000000000000000000de0b6b3a7640000",
                "nonce": "0x6",
                "to": "0x13023d2f562b656b3c35e65956cf527c893d5fdf",
                "transactionIndex": "0x0",
                "value": "0x0",
                "type": "0x4",
                "accessList": [],
                "chainId": "0x1a5ee289c",
                "authorizationList": [
                    {
                        "chainId": "0x1a5ee289c",
                        "address": "0xd7eb2c2b3c979fb14b94a523a102bc2d593b9080",
                        "nonce": "0x2",
                        "v": "0x0",
                        "r": "0x928c8a2a359bd28d3b94ab450589fc34f27677d605dc37c9300572844d059275",
                        "s": "0x4b9b4025c47e9ef04c99ad702aeb8056bc4f08c960c69226c3605466f316ca27"
                    }
                ],
                "v": "0x0",
                "r": "0x733c3cf5e701e6873b72c65273b290a993dc4b0612545cdc5b41950848182874",
                "s": "0x306ce694afab15d06344334e1e89960834ec691e863753d9aea53a22902f2cdd",
                "yParity": "0x0"
            }
`
)

func TestUnmarshalSetCodeTx(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "UnmarshalSetCodeTx",
			json:          setCodeTx,
			expectedError: "",
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var parsedTx = &Transaction{}
			err := json.Unmarshal([]byte(test.json), &parsedTx)
			if test.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}

func TestTransactionUnmarshalJsonDeposit(t *testing.T) {
	tx := NewTx(&DepositTx{
		SourceHash:          common.HexToHash("0x1234"),
		IsSystemTransaction: true,
		Mint:                big.NewInt(34),
	})
	json, err := tx.MarshalJSON()
	require.NoError(t, err, "Failed to marshal tx JSON")

	got := &Transaction{}
	err = got.UnmarshalJSON(json)
	require.NoError(t, err, "Failed to unmarshal tx JSON")
	require.Equal(t, tx.Hash(), got.Hash())
}

func TestTransactionUnmarshalJSON(t *testing.T) {
	tests := []struct {
		name          string
		json          string
		expectedError string
	}{
		{
			name:          "No gas",
			json:          `{"type":"0x7e","nonce":null,"gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","from":"0x0000000000000000000000000000000000000001","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			expectedError: "missing required field 'gas'",
		},
		{
			name:          "No value",
			json:          `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","from":"0x0000000000000000000000000000000000000001","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			expectedError: "missing required field 'value'",
		},
		{
			name:          "No input",
			json:          `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","from":"0x0000000000000000000000000000000000000001","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			expectedError: "missing required field 'input'",
		},
		{
			name:          "No from",
			json:          `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			expectedError: "missing required field 'from'",
		},
		{
			name:          "No sourceHash",
			json:          `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"from":"0x0000000000000000000000000000000000000001","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			expectedError: "missing required field 'sourceHash'",
		},
		{
			name: "No mint",
			json: `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","from":"0x0000000000000000000000000000000000000001","isSystemTx":false,"hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			// Allowed
		},
		{
			name: "No IsSystemTx",
			json: `{"type":"0x7e","nonce":null,"gas": "0x1234", "gasPrice":null,"maxPriorityFeePerGas":null,"maxFeePerGas":null,"value":"0x1","input":"0x616263646566","v":null,"r":null,"s":null,"to":null,"sourceHash":"0x0000000000000000000000000000000000000000000000000000000000000000","from":"0x0000000000000000000000000000000000000001","hash":"0xa4341f3db4363b7ca269a8538bd027b2f8784f84454ca917668642d5f6dffdf9"}`,
			// Allowed
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var parsedTx = &Transaction{}
			err := json.Unmarshal([]byte(test.json), &parsedTx)
			if test.expectedError == "" {
				require.NoError(t, err)
			} else {
				require.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}
