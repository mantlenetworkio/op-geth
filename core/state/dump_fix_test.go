package state

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/rawdb"
	"github.com/ethereum/go-ethereum/core/tracing"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/triedb"
	"github.com/holiman/uint256"
)

func TestIterativeDumpIncludesHashedStorageWhenSlotPreimageMissing(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb := NewDatabase(trieDB, nil)

	stateDB, _ := New(types.EmptyRootHash, tdb)
	address := common.HexToAddress("0x00000000000000000000000000000000000000aa")
	slot := common.HexToHash("0x1")
	value := common.HexToHash("0x2")
	stateDB.SetState(address, slot, value)
	root, _ := stateDB.Commit(0, false, false)
	if err := trieDB.Commit(root, false); err != nil {
		t.Fatalf("failed to commit trie: %v", err)
	}

	hashedSlot := crypto.Keccak256Hash(slot.Bytes())
	preimageKey := append(append([]byte{}, rawdb.PreimagePrefix...), hashedSlot.Bytes()...)
	if err := db.Delete(preimageKey); err != nil {
		t.Fatalf("failed to delete preimage: %v", err)
	}
	if rawdb.ReadPreimage(db, hashedSlot) != nil {
		t.Fatal("expected storage slot preimage to be deleted")
	}

	trieDB = triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb = NewDatabase(trieDB, nil)
	var err error
	stateDB, err = New(root, tdb)
	if err != nil {
		t.Fatalf("failed to open state at root %s: %v", root.Hex(), err)
	}
	var out bytes.Buffer
	stateDB.IterativeDump(nil, json.NewEncoder(&out))

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected dump line count: %d", len(lines))
	}

	var account DumpAccount
	if err := json.Unmarshal([]byte(lines[1]), &account); err != nil {
		t.Fatalf("failed to parse account line: %v", err)
	}

	if account.Address == nil || *account.Address != address {
		t.Fatalf("unexpected account address: %v", account.Address)
	}
	if len(account.Storage) != 0 {
		t.Fatalf("expected no plain storage entries, got=%d", len(account.Storage))
	}

	got, ok := account.StorageHashed[hashedSlot]
	if !ok {
		t.Fatalf("expected hashed slot %s in storage_hashed", hashedSlot.Hex())
	}
	if got != "02" {
		t.Fatalf("unexpected hashed slot value: got=%s want=02", got)
	}
}

func TestIterativeDumpIncludesIncompleteAccountWhenAddressPreimageMissing(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb := NewDatabase(trieDB, nil)

	stateDB, _ := New(types.EmptyRootHash, tdb)
	address := common.HexToAddress("0x00000000000000000000000000000000000000bb")
	stateDB.SetBalance(address, uint256.NewInt(11), tracing.BalanceChangeUnspecified)
	root, _ := stateDB.Commit(0, false, false)
	if err := trieDB.Commit(root, false); err != nil {
		t.Fatalf("failed to commit trie: %v", err)
	}

	hashedAddress := crypto.Keccak256Hash(address.Bytes())
	preimageKey := append(append([]byte{}, rawdb.PreimagePrefix...), hashedAddress.Bytes()...)
	if err := db.Delete(preimageKey); err != nil {
		t.Fatalf("failed to delete address preimage: %v", err)
	}
	if rawdb.ReadPreimage(db, hashedAddress) != nil {
		t.Fatal("expected address preimage to be deleted")
	}

	trieDB = triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb = NewDatabase(trieDB, nil)
	var err error
	stateDB, err = New(root, tdb)
	if err != nil {
		t.Fatalf("failed to open state at root %s: %v", root.Hex(), err)
	}

	var out bytes.Buffer
	stateDB.IterativeDump(&DumpConfig{OnlyWithAddresses: false}, json.NewEncoder(&out))
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected dump line count: %d", len(lines))
	}

	var account DumpAccount
	if err := json.Unmarshal([]byte(lines[1]), &account); err != nil {
		t.Fatalf("failed to parse account line: %v", err)
	}

	if account.Address != nil {
		t.Fatalf("expected account address to be omitted, got=%s", account.Address.Hex())
	}
	if account.AddressHash == nil {
		t.Fatal("expected account key to be present")
	}
	if common.BytesToHash(account.AddressHash) != hashedAddress {
		t.Fatalf("unexpected account key: got=%x want=%s", account.AddressHash, hashedAddress.Hex())
	}
	if account.Balance != "11" {
		t.Fatalf("unexpected balance: got=%s want=11", account.Balance)
	}
}

func TestIterativeDumpOnlyIncompletesIncludesMissingAddressAccountStorage(t *testing.T) {
	db := rawdb.NewMemoryDatabase()
	trieDB := triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb := NewDatabase(trieDB, nil)

	stateDB, _ := New(types.EmptyRootHash, tdb)
	addressWithPreimage := common.HexToAddress("0x00000000000000000000000000000000000000cc")
	addressWithoutPreimage := common.HexToAddress("0x00000000000000000000000000000000000000dd")
	slot := common.HexToHash("0x01")
	value := common.HexToHash("0x02")

	stateDB.SetBalance(addressWithPreimage, uint256.NewInt(1), tracing.BalanceChangeUnspecified)
	stateDB.SetState(addressWithPreimage, slot, value)
	stateDB.SetBalance(addressWithoutPreimage, uint256.NewInt(2), tracing.BalanceChangeUnspecified)
	stateDB.SetState(addressWithoutPreimage, slot, value)

	root, _ := stateDB.Commit(0, false, false)
	if err := trieDB.Commit(root, false); err != nil {
		t.Fatalf("failed to commit trie: %v", err)
	}

	missingAddrHash := crypto.Keccak256Hash(addressWithoutPreimage.Bytes())
	preimageKey := append(append([]byte{}, rawdb.PreimagePrefix...), missingAddrHash.Bytes()...)
	if err := db.Delete(preimageKey); err != nil {
		t.Fatalf("failed to delete address preimage: %v", err)
	}
	if rawdb.ReadPreimage(db, missingAddrHash) != nil {
		t.Fatal("expected address preimage to be deleted")
	}

	trieDB = triedb.NewDatabase(db, &triedb.Config{Preimages: true})
	tdb = NewDatabase(trieDB, nil)
	var err error
	stateDB, err = New(root, tdb)
	if err != nil {
		t.Fatalf("failed to open state at root %s: %v", root.Hex(), err)
	}

	var out bytes.Buffer
	stateDB.IterativeDump(
		&DumpConfig{
			OnlyWithAddresses: false,
			OnlyIncompletes:   true,
		},
		json.NewEncoder(&out),
	)
	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("unexpected dump line count: %d", len(lines))
	}

	var account DumpAccount
	if err := json.Unmarshal([]byte(lines[1]), &account); err != nil {
		t.Fatalf("failed to parse account line: %v", err)
	}
	if account.Address != nil {
		t.Fatalf("expected account address to be omitted, got=%s", account.Address.Hex())
	}
	if account.AddressHash == nil {
		t.Fatal("expected account key to be present")
	}
	if common.BytesToHash(account.AddressHash) != missingAddrHash {
		t.Fatalf("unexpected account key: got=%x want=%s", account.AddressHash, missingAddrHash.Hex())
	}
	if got := account.Storage[slot]; got != "02" {
		t.Fatalf("unexpected storage value: got=%s want=02", got)
	}
}
