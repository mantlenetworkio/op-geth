package types

type BlockConfig struct {
	IsMantleSkadiEnabled bool
}

func (bc *BlockConfig) IsOptimismWithSkadi(blockTime uint64) bool {
	return bc.IsMantleSkadiEnabled
}

var (
	DefaultBlockConfig     = &BlockConfig{IsMantleSkadiEnabled: false}
	MantleSkadiBlockConfig = &BlockConfig{IsMantleSkadiEnabled: true}
)
