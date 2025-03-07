package preconf

var (
	DefaultMinerConfig = MinerConfig{
		OptimismNodeHTTP: "http://localhost:7545",
		L1RPCHTTP:        "http://localhost:8545",
		L1DepositAddress: "0xa513E6E4b8f2a923D98304ec87F64353C4D5C853",
	}
)

type MinerConfig struct {
	OptimismNodeHTTP string
	L1RPCHTTP        string
	L1DepositAddress string
}
