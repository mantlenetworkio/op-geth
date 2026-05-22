package types

import (
	"crypto/ecdsa"
	"math/big"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/holiman/uint256"
	"github.com/stretchr/testify/require"
)

var (
	userKey, _           = crypto.HexToECDSA("eef77acb6c6a6eebc5b363a475ac583ec7eccdb42b6481424c60f59aa326547f")
	gasFeeSponsorKey1, _ = crypto.HexToECDSA("b71c71a67e1177ad4e901695e1b4b9ee17ae16c6668d313eac2f96dbcda3f291")
	gasFeeSponsorKey2, _ = crypto.HexToECDSA("0288ef00023598499cb6c940146d050d2b1fb914198c327f76aad590bead68b6")
)

func generateMetaTxData(dynamicTx *DynamicFeeTx, expireHeight uint64, sponsorPercent uint64,
	gasFeeSponsorAddr common.Address, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	metaTxSignData := &MetaTxSignData{
		ChainID:        dynamicTx.ChainID,
		Nonce:          dynamicTx.Nonce,
		GasTipCap:      dynamicTx.GasTipCap,
		GasFeeCap:      dynamicTx.GasFeeCap,
		Gas:            dynamicTx.Gas,
		To:             dynamicTx.To,
		Value:          dynamicTx.Value,
		Data:           dynamicTx.Data,
		AccessList:     dynamicTx.AccessList,
		ExpireHeight:   expireHeight,
		SponsorPercent: sponsorPercent,
	}

	sponsorSig, err := crypto.Sign(metaTxSignData.Hash().Bytes(), privateKey)
	if err != nil {
		return nil, err
	}

	r, s, v, err := decodeSignature(sponsorSig)
	if err != nil {
		return nil, err
	}

	metaTxData := &MetaTxParams{
		ExpireHeight:   expireHeight,
		Payload:        metaTxSignData.Data,
		GasFeeSponsor:  gasFeeSponsorAddr,
		SponsorPercent: sponsorPercent,
		R:              r,
		S:              s,
		V:              v,
	}

	metaTxDataBz, err := rlp.EncodeToBytes(metaTxData)
	if err != nil {
		return nil, err
	}

	return append(MetaTxPrefix, metaTxDataBz...), nil
}

func generateMetaTxDataWithMockSig(dynamicTx *DynamicFeeTx, expireHeight uint64, sponsorPercent uint64,
	gasFeeSponsorAddr common.Address, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	metaTxSignData := &MetaTxSignData{
		ChainID:        dynamicTx.ChainID,
		Nonce:          dynamicTx.Nonce,
		GasTipCap:      dynamicTx.GasTipCap,
		GasFeeCap:      dynamicTx.GasFeeCap,
		Gas:            dynamicTx.Gas,
		To:             dynamicTx.To,
		Value:          dynamicTx.Value,
		Data:           dynamicTx.Data,
		AccessList:     dynamicTx.AccessList,
		ExpireHeight:   expireHeight,
		SponsorPercent: sponsorPercent,
	}

	sponsorSig, err := crypto.Sign(metaTxSignData.Hash().Bytes(), privateKey)
	if err != nil {
		return nil, err
	}

	sponsorSig[len(sponsorSig)-1] = sponsorSig[len(sponsorSig)-1] + 2

	r, s, v, err := decodeSignature(sponsorSig)
	if err != nil {
		return nil, err
	}
	metaTxData := &MetaTxParams{
		ExpireHeight:   expireHeight,
		Payload:        metaTxSignData.Data,
		GasFeeSponsor:  gasFeeSponsorAddr,
		SponsorPercent: sponsorPercent,
		R:              r,
		S:              s,
		V:              v,
	}

	metaTxDataBz, err := rlp.EncodeToBytes(metaTxData)
	if err != nil {
		return nil, err
	}

	return append(MetaTxPrefix, metaTxDataBz...), nil
}

func generateMetaTxDataV2(dynamicTx *DynamicFeeTx, msgSender common.Address, expireHeight uint64, sponsorPercent uint64,
	gasFeeSponsorAddr common.Address, privateKey *ecdsa.PrivateKey) ([]byte, error) {
	metaTxSignData := &MetaTxSignDataV2{
		From:           msgSender,
		ChainID:        dynamicTx.ChainID,
		Nonce:          dynamicTx.Nonce,
		GasTipCap:      dynamicTx.GasTipCap,
		GasFeeCap:      dynamicTx.GasFeeCap,
		Gas:            dynamicTx.Gas,
		To:             dynamicTx.To,
		Value:          dynamicTx.Value,
		Data:           dynamicTx.Data,
		AccessList:     dynamicTx.AccessList,
		ExpireHeight:   expireHeight,
		SponsorPercent: sponsorPercent,
	}

	sponsorSig, err := crypto.Sign(metaTxSignData.Hash().Bytes(), privateKey)
	if err != nil {
		return nil, err
	}

	r, s, v, err := decodeSignature(sponsorSig)
	if err != nil {
		return nil, err
	}

	metaTxData := &MetaTxParams{
		ExpireHeight:   expireHeight,
		Payload:        metaTxSignData.Data,
		GasFeeSponsor:  gasFeeSponsorAddr,
		SponsorPercent: sponsorPercent,
		R:              r,
		S:              s,
		V:              v,
	}

	metaTxDataBz, err := rlp.EncodeToBytes(metaTxData)
	if err != nil {
		return nil, err
	}

	return append(MetaTxPrefix, metaTxDataBz...), nil
}

func TestDecodeMetaTxParams(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey1.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)
	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4700000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata,
		AccessList: nil,
	}

	metaTxData := &MetaTxParams{
		ExpireHeight:   expireHeight,
		Payload:        depositABICalldata,
		GasFeeSponsor:  gasFeeSponsorAddr,
		SponsorPercent: 50,
	}

	metaTxDataBz, err := rlp.EncodeToBytes(metaTxData)
	require.NoError(t, err)

	dynamicTx.Data = append(MetaTxPrefix, metaTxDataBz...)

	metaTxParams, err := DecodeMetaTxParams(dynamicTx.Data)
	require.NoError(t, err)

	require.Equal(t, gasFeeSponsorAddr.String(), metaTxParams.GasFeeSponsor.String())
	require.Equal(t, hexutil.Encode(depositABICalldata), hexutil.Encode(metaTxParams.Payload))

	metaTxData = &MetaTxParams{
		ExpireHeight:   expireHeight,
		Payload:        depositABICalldata,
		GasFeeSponsor:  gasFeeSponsorAddr,
		SponsorPercent: 101,
	}

	metaTxDataBz, err = rlp.EncodeToBytes(metaTxData)
	require.NoError(t, err)

	dynamicTx.Data = append(MetaTxPrefix, metaTxDataBz...)

	metaTxParams, err = DecodeMetaTxParams(dynamicTx.Data)
	require.Equal(t, ErrInvalidSponsorPercent, err)

}

func TestDecodeAndVerifyMetaTxParams(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey1.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)
	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4700000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata,
		AccessList: nil,
	}

	payload, err := generateMetaTxData(dynamicTx, expireHeight, 50, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx := NewTx(dynamicTx)
	signer := LatestSignerForChainID(chainId)
	txSignature, err := crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err := tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// test normal metaTx
	metaTxParams, err := DecodeAndVerifyMetaTxParams(signedTx, false, false, false)
	require.NoError(t, err)

	require.Equal(t, gasFeeSponsorAddr.String(), metaTxParams.GasFeeSponsor.String())
	require.Equal(t, hexutil.Encode(depositABICalldata), hexutil.Encode(metaTxParams.Payload))

	// Test ErrInvalidGasFeeSponsorSig
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxDataWithMockSig(dynamicTx, expireHeight, 100, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, false, false, false)
	require.Equal(t, err, ErrInvalidGasFeeSponsorSig)

	// Test ErrGasFeeSponsorMismatch
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxData(dynamicTx, expireHeight, 80, gasFeeSponsorAddr, gasFeeSponsorKey2)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, true, false, false)
	require.Equal(t, err, ErrGasFeeSponsorMismatch)

	// Test ErrGasFeeSponsorMismatch
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxData(dynamicTx, expireHeight, 101, gasFeeSponsorAddr, gasFeeSponsorKey2)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, false, false, false)
	require.Equal(t, err, ErrInvalidSponsorPercent)
}

func TestDecodeAndVerifyMetaTxParamsV2(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey1.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)
	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4700000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata,
		AccessList: nil,
	}

	payload, err := generateMetaTxDataV2(dynamicTx, crypto.PubkeyToAddress(userKey.PublicKey), expireHeight, 50, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx := NewTx(dynamicTx)
	signer := LatestSignerForChainID(chainId)
	txSignature, err := crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err := tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// test normal metaTx
	metaTxParams, err := DecodeAndVerifyMetaTxParams(signedTx, true, false, false)
	require.NoError(t, err)

	require.Equal(t, gasFeeSponsorAddr.String(), metaTxParams.GasFeeSponsor.String())
	require.Equal(t, hexutil.Encode(depositABICalldata), hexutil.Encode(metaTxParams.Payload))

	// Test ErrInvalidGasFeeSponsorSig
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxDataWithMockSig(dynamicTx, expireHeight, 100, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, true, false, false)
	require.Equal(t, err, ErrInvalidGasFeeSponsorSig)

	// Test ErrGasFeeSponsorMismatch
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxDataV2(dynamicTx, crypto.PubkeyToAddress(userKey.PublicKey), expireHeight, 80, gasFeeSponsorAddr, gasFeeSponsorKey2)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, true, false, false)
	require.Equal(t, err, ErrGasFeeSponsorMismatch)

	// Test ErrGasFeeSponsorMismatch
	dynamicTx.Data = depositABICalldata
	payload, err = generateMetaTxData(dynamicTx, expireHeight, 101, gasFeeSponsorAddr, gasFeeSponsorKey2)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	_, err = DecodeAndVerifyMetaTxParams(signedTx, false, false, false)
	require.Equal(t, err, ErrInvalidSponsorPercent)
}

func TestDecodeAndVerifyMetaTxParamsV3(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey1.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)
	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4700000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata,
		AccessList: nil,
	}

	userPrivateKey := gasFeeSponsorKey1
	msgSender := crypto.PubkeyToAddress(userPrivateKey.PublicKey)
	payload, err := generateMetaTxDataV2(dynamicTx, msgSender, expireHeight, 50, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx := NewTx(dynamicTx)
	signer := LatestSignerForChainID(chainId)
	txSignature, err := crypto.Sign(signer.Hash(tx).Bytes(), userPrivateKey)
	require.NoError(t, err)
	signedTx, err := tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// test normal metaTx
	metaTxParams, err := DecodeAndVerifyMetaTxParams(signedTx, true, false, false)
	require.NoError(t, err)

	require.Equal(t, gasFeeSponsorAddr.String(), metaTxParams.GasFeeSponsor.String())
	require.Equal(t, hexutil.Encode(depositABICalldata), hexutil.Encode(metaTxParams.Payload))

	dynamicTx.Nonce = dynamicTx.Nonce + 1
	payload, err = generateMetaTxDataV2(dynamicTx, msgSender, expireHeight, 50, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx = NewTx(dynamicTx)
	txSignature, err = crypto.Sign(signer.Hash(tx).Bytes(), userPrivateKey)
	require.NoError(t, err)
	signedTx, err = tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// test normal metaTx
	metaTxParams, err = DecodeAndVerifyMetaTxParams(signedTx, true, true, false)
	require.Error(t, err, ErrSponsorMustNotEqualToSender)
}

// TestMetaTxPrefix_Format pins the exact magic prefix bytes used to detect
// MetaTx transactions in calldata. Any byte-level change here would silently
// break compatibility with all historical pre-Everest MetaTx txs on mainnet
// (archive replay would diverge), so this guards the on-chain ABI contract.
//
// The expected value is hardcoded INLINE (not derived from MetaTxPrefix or
// MetaTxPrefixLength) so that any change to either constant — even a paired
// update — is caught.
func TestMetaTxPrefix_Format(t *testing.T) {
	// Locked, hex-literal byte sequence. DO NOT replace with a reference to
	// MetaTxPrefix — that would make the assertion self-referential and
	// useless against accidental edits.
	const expectedHex = "0x00000000000000000000000000004D616E746C654D6574615478507265666978"
	const expectedLength = 32

	expectedBytes, err := hexutil.Decode(expectedHex)
	require.NoError(t, err, "expectedHex literal must be valid hex")
	require.Equal(t, expectedLength, len(expectedBytes),
		"hardcoded expectation drift: expectedHex must decode to %d bytes", expectedLength)

	// ① Length must match the hardcoded 32 — catches any change to
	//    MetaTxPrefix length even if MetaTxPrefixLength is updated in sync.
	require.Equal(t, expectedLength, len(MetaTxPrefix),
		"MetaTxPrefix length drifted: got %d, want %d (would break archive replay)",
		len(MetaTxPrefix), expectedLength)

	// ② Byte-for-byte equality with the hardcoded sequence — catches any
	//    content change to MetaTxPrefix.
	require.Equal(t, expectedBytes, MetaTxPrefix,
		"MetaTxPrefix byte sequence drifted: got %x, want %s\n"+
			"This would break archive replay of historical pre-Everest MetaTx.",
		MetaTxPrefix, expectedHex)

	// ③ The const MetaTxPrefixLength must stay synchronized with the real
	//    length of MetaTxPrefix. Independent constants drifting apart causes
	//    silent off-by-N bugs in callers (DecodeMetaTxParams, MetaTxCheck).
	require.Equal(t, expectedLength, MetaTxPrefixLength,
		"MetaTxPrefixLength constant drifted: got %d, want %d", MetaTxPrefixLength, expectedLength)

	// ④ Sanity: padding (first 14 bytes) compared to hardcoded zero literal.
	for i := 0; i < 14; i++ {
		require.Equal(t, byte(0), MetaTxPrefix[i],
			"MetaTxPrefix byte %d must be zero padding", i)
	}

	// ⑤ Sanity: ASCII suffix compared to hardcoded literal string.
	require.Equal(t, "MantleMetaTxPrefix", string(MetaTxPrefix[14:]),
		"MetaTxPrefix suffix must be ASCII 'MantleMetaTxPrefix'")
}

// TestMetaTxCheck covers the stateless guard used by txpool.ValidateTransaction
// to reject MetaTx after MantleEverest activation. The guard must:
//   - pass through any data shorter than or equal to the prefix length;
//   - pass through non-MetaTx calldata of any length;
//   - reject any calldata that starts with MetaTxPrefix.
func TestMetaTxCheck(t *testing.T) {
	testCases := []struct {
		name    string
		txData  []byte
		wantErr error
	}{
		{
			name:    "data shorter than prefix length",
			txData:  []byte{0x01, 0x02, 0x03},
			wantErr: nil,
		},
		{
			name:    "data equal to prefix length but all zero",
			txData:  make([]byte, MetaTxPrefixLength),
			wantErr: nil,
		},
		{
			name:    "non-MetaTx calldata (ERC-20 transfer)",
			txData:  hexutil.MustDecode("0xa9059cbb000000000000000000000000aabbccddeeff00112233445566778899aabbccdd0000000000000000000000000000000000000000000000000000000000000064"),
			wantErr: nil,
		},
		{
			name:    "MetaTx prefix with payload",
			txData:  append(append([]byte{}, MetaTxPrefix...), 0xde, 0xad, 0xbe, 0xef),
			wantErr: ErrMetaTxDisabled,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.wantErr, MetaTxCheck(tc.txData))
		})
	}
}

// TestDecodeAndVerifyMetaTxParams_PostEverestDisabled ensures that once
// MantleEverest is active the stateless guard rejects MetaTx before any RLP
// decode or signature recovery, regardless of V2/V3 flags. Critical to verify
// the post-Everest forward compatibility branch in the merged v1.17.3 code.
func TestDecodeAndVerifyMetaTxParams_PostEverestDisabled(t *testing.T) {
	gasFeeSponsorPublicKey := gasFeeSponsorKey1.Public()
	pubKeyECDSA, _ := gasFeeSponsorPublicKey.(*ecdsa.PublicKey)
	gasFeeSponsorAddr := crypto.PubkeyToAddress(*pubKeyECDSA)

	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	expireHeight := uint64(20_000_010)

	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4_700_000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata,
		AccessList: nil,
	}

	// Build a syntactically valid MetaTx that would succeed pre-Everest.
	payload, err := generateMetaTxData(dynamicTx, expireHeight, 50, gasFeeSponsorAddr, gasFeeSponsorKey1)
	require.NoError(t, err)

	dynamicTx.Data = payload
	tx := NewTx(dynamicTx)
	signer := LatestSignerForChainID(chainId)
	txSignature, err := crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err := tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// Pre-Everest: legacy behavior preserved (essential for archive replay).
	metaTxParams, err := DecodeAndVerifyMetaTxParams(signedTx, false, false, false)
	require.NoError(t, err)
	require.NotNil(t, metaTxParams)
	require.Equal(t, gasFeeSponsorAddr, metaTxParams.GasFeeSponsor)

	// Post-Everest: must be rejected with ErrMetaTxDisabled before any decode.
	_, err = DecodeAndVerifyMetaTxParams(signedTx, false, false, true)
	require.ErrorIs(t, err, ErrMetaTxDisabled)

	// Everest guard takes precedence over V2/V3 flag combinations.
	_, err = DecodeAndVerifyMetaTxParams(signedTx, true, true, true)
	require.ErrorIs(t, err, ErrMetaTxDisabled)
}

// TestDecodeAndVerifyMetaTxParams_PostEverestNonMetaTxPassthrough verifies
// that post-Everest the guard only rejects calldata starting with
// MetaTxPrefix; ordinary EIP-1559 dynamic-fee txs must continue to flow
// without being treated as disabled MetaTx.
func TestDecodeAndVerifyMetaTxParams_PostEverestNonMetaTxPassthrough(t *testing.T) {
	chainId := big.NewInt(1)
	depositABICalldata, _ := hexutil.Decode("0xd0e30db0")
	to := common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")

	dynamicTx := &DynamicFeeTx{
		ChainID:    chainId,
		Nonce:      100,
		GasTipCap:  big.NewInt(1e9),
		GasFeeCap:  big.NewInt(1e15),
		Gas:        4_700_000,
		To:         &to,
		Value:      big.NewInt(1e18),
		Data:       depositABICalldata, // Normal calldata, no MetaTxPrefix.
		AccessList: nil,
	}

	tx := NewTx(dynamicTx)
	signer := LatestSignerForChainID(chainId)
	txSignature, err := crypto.Sign(signer.Hash(tx).Bytes(), userKey)
	require.NoError(t, err)
	signedTx, err := tx.WithSignature(signer, txSignature)
	require.NoError(t, err)

	// Post-Everest with all guards on: non-MetaTx tx passes through as nil/nil.
	metaTxParams, err := DecodeAndVerifyMetaTxParams(signedTx, true, true, true)
	require.NoError(t, err)
	require.Nil(t, metaTxParams)
}

// TestCalculateSponsorPercentAmount validates the sponsor/self split helper.
// Used both in pre-Arsia state_transition for gas accounting and in txpool
// balance check; a wrong split would mis-charge sponsor or sender accounts
// during archive replay of pre-Everest historical MetaTx blocks.
func TestCalculateSponsorPercentAmount(t *testing.T) {
	testCases := []struct {
		name            string
		mxParams        *MetaTxParams
		amount          *uint256.Int
		expectedSponsor *uint256.Int
		expectedSelf    *uint256.Int
	}{
		{
			name:            "nil params returns nil/nil",
			mxParams:        nil,
			amount:          uint256.NewInt(1000),
			expectedSponsor: nil,
			expectedSelf:    nil,
		},
		{
			name:            "50% split on 1000",
			mxParams:        &MetaTxParams{SponsorPercent: 50},
			amount:          uint256.NewInt(1000),
			expectedSponsor: uint256.NewInt(500),
			expectedSelf:    uint256.NewInt(500),
		},
		{
			name:            "100% sponsor on 1000",
			mxParams:        &MetaTxParams{SponsorPercent: 100},
			amount:          uint256.NewInt(1000),
			expectedSponsor: uint256.NewInt(1000),
			expectedSelf:    uint256.NewInt(0),
		},
		{
			name:            "1% sponsor on 1000 (low boundary)",
			mxParams:        &MetaTxParams{SponsorPercent: 1},
			amount:          uint256.NewInt(1000),
			expectedSponsor: uint256.NewInt(10),
			expectedSelf:    uint256.NewInt(990),
		},
		{
			name:            "33% rounding-down on 100",
			mxParams:        &MetaTxParams{SponsorPercent: 33},
			amount:          uint256.NewInt(100),
			expectedSponsor: uint256.NewInt(33),
			expectedSelf:    uint256.NewInt(67),
		},
		{
			name:            "50% on uint64 max",
			mxParams:        &MetaTxParams{SponsorPercent: 50},
			amount:          uint256.NewInt(^uint64(0)),
			expectedSponsor: uint256.NewInt(^uint64(0) / 2),
			expectedSelf:    new(uint256.Int).Sub(uint256.NewInt(^uint64(0)), uint256.NewInt(^uint64(0)/2)),
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			sponsor, self := CalculateSponsorPercentAmount(tc.mxParams, tc.amount)
			if tc.expectedSponsor == nil {
				require.Nil(t, sponsor)
				require.Nil(t, self)
				return
			}
			require.NotNil(t, sponsor)
			require.NotNil(t, self)
			require.True(t, sponsor.Eq(tc.expectedSponsor),
				"sponsor amount mismatch: got %s, want %s", sponsor.String(), tc.expectedSponsor.String())
			require.True(t, self.Eq(tc.expectedSelf),
				"self amount mismatch: got %s, want %s", self.String(), tc.expectedSelf.String())
			// Conservation invariant: sponsor + self == amount.
			sum := new(uint256.Int).Add(sponsor, self)
			require.True(t, sum.Eq(tc.amount),
				"conservation violated: sponsor+self=%s, amount=%s", sum.String(), tc.amount.String())
		})
	}
}
