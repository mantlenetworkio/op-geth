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

// Package mantletest is a namespace anchor for Mantle-specific test suites.
//
// Direct test code lives in subdirectories:
//
//   - replay/  — byte-equal mainnet transaction replay (build tag: mantle_replay).
//     Run with:  go test -tags mantle_replay ./tests/mantletest/replay/...
//
//   - preconf/ — preconfirmation end-to-end runner (package main).
//     Build with: go build -o ./bin/preconf ./tests/mantletest/preconf/
//
// This file itself contains no test code; it exists so that
// `go list ./tests/mantletest/...` and IDE tooling can resolve the namespace
// directory without "no Go files" errors.
package mantletest
