package params

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/superchain"
)

// To work with optimism op-node

var InteropCrossL2InboxAddress = common.HexToAddress("0x4200000000000000000000000000000000000022")

// ===============================================
// = protocol_params.go
// ===============================================

const (
	Bn256PairingMaxInputSizeGranite uint64 = 112687 // Maximum input size for an elliptic curve pairing check

	Bls12381G1MulMaxInputSizeIsthmus   uint64 = 513760 // Maximum input size for BLS12-381 G1 multiple-scalar-multiply operation
	Bls12381G2MulMaxInputSizeIsthmus   uint64 = 488448 // Maximum input size for BLS12-381 G2 multiple-scalar-multiply operation
	Bls12381PairingMaxInputSizeIsthmus uint64 = 235008 // Maximum input size for BLS12-381 pairing check

	Bn256PairingMaxInputSizeJovian    uint64 = 81984  // bn256Pairing limit (427 pairs)
	Bls12381G1MulMaxInputSizeJovian   uint64 = 288960 // BLS12-381 G1 MSM limit (1,806 pairs)
	Bls12381G2MulMaxInputSizeJovian   uint64 = 278784 // BLS12-381 G2 MSM limit (968 pairs)
	Bls12381PairingMaxInputSizeJovian uint64 = 156672 // BLS12-381 pairing limit (408 pairs)
)

// ===============================================
// = config.go
// ===============================================

const OPMainnetChainID = 10

func (c *ChainConfig) IsMinBaseFee(time uint64) bool {
	return c.IsJovian(time) // Replace with return false to disable
}

func (c *ChainConfig) IsDAFootprintBlockLimit(time uint64) bool {
	return c.IsJovian(time) // Replace with return false to disable
}

func (c *ChainConfig) IsOperatorFeeFix(time uint64) bool {
	return c.IsJovian(time) // Replace with return false to disable
}

// IsBedrock returns whether num is either equal to the Bedrock fork block or greater.
func (c *ChainConfig) IsBedrock(num *big.Int) bool {
	return isBlockForked(c.BedrockBlock, num)
}

func (c *ChainConfig) IsRegolith(time uint64) bool {
	return isTimestampForked(c.RegolithTime, time)
}

func (c *ChainConfig) IsCanyon(time uint64) bool {
	return isTimestampForked(c.CanyonTime, time)
}

func (c *ChainConfig) IsEcotone(time uint64) bool {
	return isTimestampForked(c.EcotoneTime, time)
}

func (c *ChainConfig) IsFjord(time uint64) bool {
	return isTimestampForked(c.FjordTime, time)
}

func (c *ChainConfig) IsGranite(time uint64) bool {
	return isTimestampForked(c.GraniteTime, time)
}

func (c *ChainConfig) IsHolocene(time uint64) bool {
	return isTimestampForked(c.HoloceneTime, time)
}

func (c *ChainConfig) IsIsthmus(time uint64) bool {
	return isTimestampForked(c.IsthmusTime, time)
}

func (c *ChainConfig) IsJovian(time uint64) bool {
	return isTimestampForked(c.JovianTime, time)
}

func (c *ChainConfig) IsInterop(time uint64) bool {
	return isTimestampForked(c.InteropTime, time)
}

// IsOptimism returns whether the node is an optimism node or not.
func (c *ChainConfig) IsOptimism() bool {
	return c.Optimism != nil
}

// IsOptimismBedrock returns true iff this is an optimism node & bedrock is active
func (c *ChainConfig) IsOptimismBedrock(num *big.Int) bool {
	return c.IsOptimism() && c.IsBedrock(num)
}

func (c *ChainConfig) IsOptimismRegolith(time uint64) bool {
	return c.IsOptimism() && c.IsRegolith(time)
}

func (c *ChainConfig) IsOptimismCanyon(time uint64) bool {
	return c.IsOptimism() && c.IsCanyon(time)
}

func (c *ChainConfig) IsOptimismEcotone(time uint64) bool {
	return c.IsOptimism() && c.IsEcotone(time)
}

func (c *ChainConfig) IsOptimismFjord(time uint64) bool {
	return c.IsOptimism() && c.IsFjord(time)
}

func (c *ChainConfig) IsOptimismGranite(time uint64) bool {
	return c.IsOptimism() && c.IsGranite(time)
}

func (c *ChainConfig) IsOptimismHolocene(time uint64) bool {
	return c.IsOptimism() && c.IsHolocene(time)
}

func (c *ChainConfig) IsOptimismIsthmus(time uint64) bool {
	return c.IsOptimism() && c.IsIsthmus(time)
}

func (c *ChainConfig) IsOptimismJovian(time uint64) bool {
	return c.IsOptimism() && c.IsJovian(time)
}

// IsOptimismPreBedrock returns true iff this is an optimism node & bedrock is not yet active
func (c *ChainConfig) IsOptimismPreBedrock(num *big.Int) bool {
	return c.IsOptimism() && !c.IsBedrock(num)
}

func (c *ChainConfig) HasOptimismWithdrawalsRoot(blockTime uint64) bool {
	return c.IsOptimismIsthmus(blockTime)
}

// CheckOptimismValidity checks for OP Stack chains:
// - the EIP159 params are set
// - the Ethereum forks are set to the same time as the OP Stack forks that imply them
func (c *ChainConfig) CheckOptimismValidity() error {
	if c.Optimism == nil {
		return nil
	}

	if c.Optimism.EIP1559Denominator == 0 {
		return errors.New("zero EIP1559Denominator")
	}
	if c.Optimism.EIP1559Elasticity == 0 {
		return errors.New("zero EIP1559Elasticity")
	}

	if !equalPtrValues(c.CancunTime, c.MantleSkadiTime) {
		return fmt.Errorf("CancunTime (%s) must equal MantleSkadiTime (%s)", ptrValueString(c.CancunTime), ptrValueString(c.MantleSkadiTime))
	}
	if !equalPtrValues(c.PragueTime, c.MantleSkadiTime) {
		return fmt.Errorf("PragueTime (%s) must equal MantleSkadiTime (%s)", ptrValueString(c.PragueTime), ptrValueString(c.MantleSkadiTime))
	}

	return nil
}

func equalPtrValues[T comparable](a, b *T) bool {
	// also captures nil == nil
	return a == b || (a != nil && b != nil && *a == *b)
}

func ptrValueString[T any](t *T) string {
	if t == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%v", *t)
}

// ===============================================
// = superchain.go
// ===============================================

var OPStackSupport = ProtocolVersionV0{Build: [8]byte{}, Major: 9, Minor: 0, Patch: 0, PreRelease: 0}.Encode()

// uint64ptr is a weird helper to allow 1-line constant pointer creation.
func uint64ptr(n uint64) *uint64 {
	return &n
}

func LoadOPStackChainConfig(chConfig *superchain.ChainConfig) (*ChainConfig, error) {
	hardforks := chConfig.Hardforks
	genesisActivation := uint64(0)
	out := &ChainConfig{
		ChainID:                 new(big.Int).SetUint64(chConfig.ChainID),
		HomesteadBlock:          common.Big0,
		DAOForkBlock:            nil,
		DAOForkSupport:          false,
		EIP150Block:             common.Big0,
		EIP155Block:             common.Big0,
		EIP158Block:             common.Big0,
		ByzantiumBlock:          common.Big0,
		ConstantinopleBlock:     common.Big0,
		PetersburgBlock:         common.Big0,
		IstanbulBlock:           common.Big0,
		MuirGlacierBlock:        common.Big0,
		BerlinBlock:             common.Big0,
		LondonBlock:             common.Big0,
		ArrowGlacierBlock:       common.Big0,
		GrayGlacierBlock:        common.Big0,
		MergeNetsplitBlock:      common.Big0,
		ShanghaiTime:            hardforks.CanyonTime,  // Shanghai activates with Canyon
		CancunTime:              hardforks.EcotoneTime, // Cancun activates with Ecotone
		PragueTime:              hardforks.IsthmusTime, // Prague activates with Isthmus
		BedrockBlock:            common.Big0,
		RegolithTime:            &genesisActivation,
		CanyonTime:              hardforks.CanyonTime,
		EcotoneTime:             hardforks.EcotoneTime,
		FjordTime:               hardforks.FjordTime,
		GraniteTime:             hardforks.GraniteTime,
		HoloceneTime:            hardforks.HoloceneTime,
		IsthmusTime:             hardforks.IsthmusTime,
		JovianTime:              hardforks.JovianTime,
		InteropTime:             hardforks.InteropTime,
		TerminalTotalDifficulty: common.Big0,
		Ethash:                  nil,
		Clique:                  nil,
	}

	if chConfig.Optimism != nil {
		out.Optimism = &OptimismConfig{
			EIP1559Elasticity:  chConfig.Optimism.EIP1559Elasticity,
			EIP1559Denominator: chConfig.Optimism.EIP1559Denominator,
		}
		if chConfig.Optimism.EIP1559DenominatorCanyon != nil {
			out.Optimism.EIP1559DenominatorCanyon = uint64ptr(*chConfig.Optimism.EIP1559DenominatorCanyon)
		}
	}

	// special overrides for OP-Stack chains with pre-Regolith upgrade history
	switch chConfig.ChainID {
	case OPMainnetChainID:
		out.BerlinBlock = big.NewInt(3950000)
		out.LondonBlock = big.NewInt(105235063)
		out.ArrowGlacierBlock = big.NewInt(105235063)
		out.GrayGlacierBlock = big.NewInt(105235063)
		out.MergeNetsplitBlock = big.NewInt(105235063)
		out.BedrockBlock = big.NewInt(105235063)
	}

	return out, nil
}

// ProtocolVersion encodes the OP-Stack protocol version. See OP-Stack superchain-upgrade specification.
type ProtocolVersion [32]byte

func (p ProtocolVersion) MarshalText() ([]byte, error) {
	return common.Hash(p).MarshalText()
}

func (p *ProtocolVersion) UnmarshalText(input []byte) error {
	return (*common.Hash)(p).UnmarshalText(input)
}

func (p ProtocolVersion) Parse() (versionType uint8, build [8]byte, major, minor, patch, preRelease uint32) {
	versionType = p[0]
	if versionType != 0 {
		return
	}
	// bytes 1:8 reserved for future use
	copy(build[:], p[8:16])                        // differentiates forks and custom-builds of standard protocol
	major = binary.BigEndian.Uint32(p[16:20])      // incompatible API changes
	minor = binary.BigEndian.Uint32(p[20:24])      // identifies additional functionality in backwards compatible manner
	patch = binary.BigEndian.Uint32(p[24:28])      // identifies backward-compatible bug-fixes
	preRelease = binary.BigEndian.Uint32(p[28:32]) // identifies unstable versions that may not satisfy the above
	return
}

func (p ProtocolVersion) String() string {
	versionType, build, major, minor, patch, preRelease := p.Parse()
	if versionType != 0 {
		return "v0.0.0-unknown." + common.Hash(p).String()
	}
	ver := fmt.Sprintf("v%d.%d.%d", major, minor, patch)
	if preRelease != 0 {
		ver += fmt.Sprintf("-%d", preRelease)
	}
	if build != ([8]byte{}) {
		if humanBuildTag(build) {
			ver += fmt.Sprintf("+%s", strings.TrimRight(string(build[:]), "\x00"))
		} else {
			ver += fmt.Sprintf("+0x%x", build)
		}
	}
	return ver
}

// humanBuildTag identifies which build tag we can stringify for human-readable versions
func humanBuildTag(v [8]byte) bool {
	for i, c := range v { // following semver.org advertised regex, alphanumeric with '-' and '.', except leading '.'.
		if c == 0 { // trailing zeroed are allowed
			for _, d := range v[i:] {
				if d != 0 {
					return false
				}
			}
			return true
		}
		if !((c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' || (c == '.' && i > 0)) {
			return false
		}
	}
	return true
}

// ProtocolVersionComparison is used to identify how far ahead/outdated a protocol version is relative to another.
// This value is used in metrics and switch comparisons, to easily identify each type of version difference.
// Negative values mean the version is outdated.
// Positive values mean the version is up-to-date.
// Matching versions have a 0.
type ProtocolVersionComparison int

const (
	AheadMajor         ProtocolVersionComparison = 4
	OutdatedMajor      ProtocolVersionComparison = -4
	AheadMinor         ProtocolVersionComparison = 3
	OutdatedMinor      ProtocolVersionComparison = -3
	AheadPatch         ProtocolVersionComparison = 2
	OutdatedPatch      ProtocolVersionComparison = -2
	AheadPrerelease    ProtocolVersionComparison = 1
	OutdatedPrerelease ProtocolVersionComparison = -1
	Matching           ProtocolVersionComparison = 0
	DiffVersionType    ProtocolVersionComparison = 100
	DiffBuild          ProtocolVersionComparison = 101
	EmptyVersion       ProtocolVersionComparison = 102
	InvalidVersion     ProtocolVersionComparison = 103
)

func (p ProtocolVersion) Compare(other ProtocolVersion) (cmp ProtocolVersionComparison) {
	if p == (ProtocolVersion{}) || (other == (ProtocolVersion{})) {
		return EmptyVersion
	}
	aVersionType, aBuild, aMajor, aMinor, aPatch, aPreRelease := p.Parse()
	bVersionType, bBuild, bMajor, bMinor, bPatch, bPreRelease := other.Parse()
	if aVersionType != bVersionType {
		return DiffVersionType
	}
	if aBuild != bBuild {
		return DiffBuild
	}
	// max values are reserved, consider versions with these values invalid
	if aMajor == ^uint32(0) || aMinor == ^uint32(0) || aPatch == ^uint32(0) || aPreRelease == ^uint32(0) ||
		bMajor == ^uint32(0) || bMinor == ^uint32(0) || bPatch == ^uint32(0) || bPreRelease == ^uint32(0) {
		return InvalidVersion
	}
	fn := func(a, b uint32, ahead, outdated ProtocolVersionComparison) ProtocolVersionComparison {
		if a == b {
			return Matching
		}
		if a > b {
			return ahead
		}
		return outdated
	}
	if aPreRelease != 0 { // if A is a pre-release, then decrement the version before comparison
		if aPatch != 0 {
			aPatch -= 1
		} else if aMinor != 0 {
			aMinor -= 1
			aPatch = ^uint32(0) // max value
		} else if aMajor != 0 {
			aMajor -= 1
			aMinor = ^uint32(0) // max value
		}
	}
	if bPreRelease != 0 { // if B is a pre-release, then decrement the version before comparison
		if bPatch != 0 {
			bPatch -= 1
		} else if bMinor != 0 {
			bMinor -= 1
			bPatch = ^uint32(0) // max value
		} else if bMajor != 0 {
			bMajor -= 1
			bMinor = ^uint32(0) // max value
		}
	}
	if c := fn(aMajor, bMajor, AheadMajor, OutdatedMajor); c != Matching {
		return c
	}
	if c := fn(aMinor, bMinor, AheadMinor, OutdatedMinor); c != Matching {
		return c
	}
	if c := fn(aPatch, bPatch, AheadPatch, OutdatedPatch); c != Matching {
		return c
	}
	return fn(aPreRelease, bPreRelease, AheadPrerelease, OutdatedPrerelease)
}

type ProtocolVersionV0 struct {
	Build                           [8]byte
	Major, Minor, Patch, PreRelease uint32
}

func (v ProtocolVersionV0) Encode() (out ProtocolVersion) {
	copy(out[8:16], v.Build[:])
	binary.BigEndian.PutUint32(out[16:20], v.Major)
	binary.BigEndian.PutUint32(out[20:24], v.Minor)
	binary.BigEndian.PutUint32(out[24:28], v.Patch)
	binary.BigEndian.PutUint32(out[28:32], v.PreRelease)
	return
}
