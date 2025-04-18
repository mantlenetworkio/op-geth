// Copyright 2019 The go-ethereum Authors
// This file is part of go-ethereum.
//
// go-ethereum is free software: you can redistribute it and/or modify
// it under the terms of the GNU General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// go-ethereum is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU General Public License for more details.
//
// You should have received a copy of the GNU General Public License
// along with go-ethereum. If not, see <http://www.gnu.org/licenses/>.

// Package utils contains internal helper functions for go-ethereum commands.
package utils

import (
	"flag"
	"fmt"
	"reflect"
	"testing"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/txpool/legacypool"
	"github.com/ethereum/go-ethereum/preconf"
	"github.com/urfave/cli/v2"
)

func Test_SplitTagsFlag(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args string
		want map[string]string
	}{
		{
			"2 tags case",
			"host=localhost,bzzkey=123",
			map[string]string{
				"host":   "localhost",
				"bzzkey": "123",
			},
		},
		{
			"1 tag case",
			"host=localhost123",
			map[string]string{
				"host": "localhost123",
			},
		},
		{
			"empty case",
			"",
			map[string]string{},
		},
		{
			"garbage",
			"smth=smthelse=123",
			map[string]string{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := SplitTagsFlag(tt.args); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("splitTagsFlag() = %v, want %v", got, tt.want)
			}
		})
	}
}

func Test_setPreconfCfg(t *testing.T) {
	app := cli.NewApp()
	flags := []cli.Flag{
		&cli.StringFlag{Name: TxPoolFromPreconfsFlag.Name},
		&cli.StringFlag{Name: TxPoolToPreconfsFlag.Name},
		&cli.BoolFlag{Name: TxPoolAllPreconfsFlag.Name},
		&cli.DurationFlag{Name: TxPoolPreconfTimeoutFlag.Name},
	}
	app.Flags = flags

	tests := []struct {
		name      string
		args      []string
		config    *preconf.TxPoolConfig
		want      *preconf.TxPoolConfig
		wantFatal bool
		wantMsg   string
	}{
		{
			name:   "Set FromPreconfs",
			args:   []string{"--txpool.frompreconfs", "0x1111111111111111111111111111111111111111, 0x2222222222222222222222222222222222222222"},
			config: &preconf.TxPoolConfig{},
			want: &preconf.TxPoolConfig{
				FromPreconfs: []common.Address{
					common.HexToAddress("0x1111111111111111111111111111111111111111"),
					common.HexToAddress("0x2222222222222222222222222222222222222222"),
				},
			},
			wantFatal: false,
		},
		{
			name:   "Set ToPreconfs",
			args:   []string{"--txpool.topreconfs", "0x3333333333333333333333333333333333333333"},
			config: &preconf.TxPoolConfig{},
			want: &preconf.TxPoolConfig{
				ToPreconfs: []common.Address{
					common.HexToAddress("0x3333333333333333333333333333333333333333"),
				},
			},
			wantFatal: false,
		},
		{
			name:   "Set AllPreconfs",
			args:   []string{"--txpool.allpreconfs"},
			config: &preconf.TxPoolConfig{},
			want: &preconf.TxPoolConfig{
				AllPreconfs: true,
			},
			wantFatal: false,
		},
		{
			name:   "Set PreconfTimeout",
			args:   []string{"--txpool.preconftimeout", "2s"},
			config: &preconf.TxPoolConfig{},
			want: &preconf.TxPoolConfig{
				PreconfTimeout: 2 * time.Second,
			},
			wantFatal: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := cli.NewContext(app, NewStringSet(app.Flags, tt.args), nil)
			cfg := tt.config

			setPreconfCfg(ctx, &legacypool.Config{Preconf: cfg})

			if !reflect.DeepEqual(cfg, tt.want) {
				t.Errorf("setPreconfCfg() cfg = %v, want %v", cfg, tt.want)
			}
		})
	}
}

func NewStringSet(flags []cli.Flag, args []string) *flag.FlagSet {
	fs := flag.NewFlagSet("test", flag.ContinueOnError)
	for _, f := range flags {
		f.Apply(fs)
	}
	err := fs.Parse(args)
	if err != nil {
		panic(fmt.Sprintf("Failed to parse flags: %v", err))
	}
	return fs
}
