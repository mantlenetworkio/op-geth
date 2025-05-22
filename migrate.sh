#!/bin/bash
set -e

banner() {
    echo "+------------------------------------------+"
    printf "| %-40s |\n" "$(date)"
    echo "|                                          |"
    printf "|$(tput bold) %-40s $(tput sgr0)|\n" "$@"
    echo "+------------------------------------------+"
}

echo '  __  __ _  _ _____ __  __ _               _   _             '
echo ' |  \/  | \| |_   _|  \/  (_)__ _ _ _ __ _| |_(_)___ _ _     '
echo ' | |\/| | .` | | | | |\/| | / _` | '\''_/ _` |  _| / _ \ '\'' \    '
echo ' |_|  |_|_|\_| |_| |_|  |_|_\__, |_| \__,_|\__|_\___/_||_|   '
echo '                           |___/                             '

LOG_DIR="migration-log"

if [[ -n "$1" ]]; then
    MANTLE_START_BLOCK_NUM=$1
else
    echo "Mantle start block number not set."
fi

if [[ -n "$2" ]]; then
    GETH_DATA_DIR=$2
else
    echo "data directory not set."
    exit 1
fi
echo "data dir set to $GETH_DATA_DIR"

mkdir -p "$LOG_DIR"

ARTIFACT_PATH="/tmp/migration-artifact"
if [[ -n "$3" ]]; then
    ARTIFACT_PATH=$3
fi
echo "artifact path set to $ARTIFACT_PATH"
mkdir -p "$ARTIFACT_PATH"

banner "Init"
# boot up geth with new database to supply chain config
# this is only needed for new database
./build/bin/geth --datadir="$GETH_DATA_DIR" --http 2> "$LOG_DIR"/start.log &
PID=$!
sleep 10
curl --location 'localhost:8545/' \
--header 'Content-Type: application/json' \
--data '{
	"jsonrpc":"2.0",
	"method":"eth_blockNumber",
	"params":[],
	"id":83
}'
kill $PID
sleep 10

banner "Export Blocks"
time ./build/bin/geth --datadir="$GETH_DATA_DIR" export "$ARTIFACT_PATH"/blocks.rlp 0 "$MANTLE_START_BLOCK_NUM" 2> "$LOG_DIR"/export_blocks.log

banner "Export Total Difficulty"
time ./build/bin/geth --datadir="$GETH_DATA_DIR" export-totaldifficulty "$ARTIFACT_PATH"/totaldifficulty.rlp 0 "$MANTLE_START_BLOCK_NUM" 2> "$LOG_DIR"/export_totaldifficulty.log

banner "Export Receipts"
time ./build/bin/geth --datadir="$GETH_DATA_DIR" export-receipts "$ARTIFACT_PATH"/receipts.rlp 0 "$MANTLE_START_BLOCK_NUM" 2> "$LOG_DIR"/export_receipts.log

banner "Export State"
time ./build/bin/geth --datadir="$GETH_DATA_DIR" dump --iterative "$MANTLE_START_BLOCK_NUM" 1> "$ARTIFACT_PATH"/world_trie_state.jsonl 2> "$LOG_DIR"/export_state.log

banner "Export done"