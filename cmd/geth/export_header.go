package main

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"slices"

	"github.com/ethereum/go-ethereum/cmd/utils"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/internal/flags"
	"github.com/ethereum/go-ethereum/log"
	"github.com/ethereum/go-ethereum/rlp"
	"github.com/urfave/cli/v2"
)

var (
	exportHeaderCommand = &cli.Command{
		Action: doExportHeader,
		Name:   "export-header",
		Usage:  "Export header RLP data",
		Flags: slices.Concat([]cli.Flag{
			BlockNumberFlag,
			BlockHashFlag,
			OutputFlag,
		}, utils.DatabaseFlags),
		Description: `
This command exports the header RLP data for a given block (or latest, if none provided).
`,
	}

	// General settings
	BlockNumberFlag = &cli.Int64Flag{
		Name:     "block",
		Usage:    "Block number to encode",
		Value:    0,
		Category: flags.EthCategory,
	}
	BlockHashFlag = &cli.StringFlag{
		Name:     "hash",
		Usage:    "Block hash to encode",
		Value:    "",
		Category: flags.EthCategory,
	}
	OutputFlag = &cli.StringFlag{
		Name:     "output",
		Usage:    "Output file name (default: header.rlp)",
		Value:    "header.rlp",
		Category: flags.EthCategory,
	}
)

type rlpConfig struct {
	BlockNumber *big.Int
	BlockHash   common.Hash
	Output      string
}

func doExportHeader(ctx *cli.Context) error {
	stack, _ := makeConfigNode(ctx)
	defer stack.Close()

	db := utils.MakeChainDatabase(ctx, stack, true)
	defer db.Close()

	conf, err := parseRLPConfig(ctx)
	if err != nil {
		return err
	}

	var header *types.Header
	if conf.BlockNumber != nil {
		hash := rawdb.ReadCanonicalHash(db, conf.BlockNumber.Uint64())
		header = rawdb.ReadHeader(db, hash, conf.BlockNumber.Uint64())
	} else if conf.BlockHash != (common.Hash{}) {
		number := rawdb.ReadHeaderNumber(db, conf.BlockHash)
		if number == nil {
			return fmt.Errorf("block %x not found", conf.BlockHash)
		}
		header = rawdb.ReadHeader(db, conf.BlockHash, *number)
	} else {
		// Use latest
		header = rawdb.ReadHeadHeader(db)
		log.Info("no block number or block hash provided, using latest block")
	}
	if header == nil {
		return errors.New("no head block found")
	}

	// Encode to file
	if err = encodeToFile(header, conf.Output); err != nil {
		return err
	}

	log.Info("RLP encoded to file", "output", conf.Output, "block", header.Number, "hash", header.Hash().Hex(), "total difficulty", header.Difficulty.String())

	return nil
}

func parseRLPConfig(ctx *cli.Context) (*rlpConfig, error) {
	conf := &rlpConfig{}
	conf.Output = ctx.String(OutputFlag.Name)
	if ctx.IsSet(BlockNumberFlag.Name) {
		conf.BlockNumber = new(big.Int)
		conf.BlockNumber.SetString(ctx.String(BlockNumberFlag.Name), 10)
	}
	if ctx.IsSet(BlockHashFlag.Name) {
		conf.BlockHash = common.HexToHash(ctx.String(BlockHashFlag.Name))
	}

	log.Info("RLP configured", "block", conf.BlockNumber, "hash", conf.BlockHash, "output", conf.Output)

	return conf, nil
}

func encodeToFile(header *types.Header, output string) error {
	file, err := os.Create(output)
	if err != nil {
		return err
	}
	defer file.Close()

	return rlp.Encode(file, header)
}
