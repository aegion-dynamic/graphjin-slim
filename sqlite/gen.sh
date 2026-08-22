#!/usr/bin/env bash
# Regenerates ALL vendored C code in this directory from pinned upstream
# releases. Maintainers only - downstream go-get users build from the
# committed amalgamations directly.
#
#   sqlite3.c / sqlite3.h   <- SQLCipher v4.18.0  (SQLite 3.53.4)
#
# Upstream sources are cloned into upstream/ (gitignored) at the exact pinned
# commits. Deterministic: refuses to run against any commit other than the
# pins below. Update the pins deliberately when upgrading.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UPSTREAM="$DIR/upstream"
PIN_SQLCIPHER="63697beb0fafcb61faa7a3e6fd267036548ab11b" # v4.18.0 / SQLite 3.53.4

fetch_pinned() {
    local dst=$1 url=$2 pin=$3 ref=$4
    if [[ -d "$dst" ]]; then
        local cur
        cur="$(git -C "$dst" rev-parse HEAD)"
        if [[ "$cur" != "$pin" ]]; then
            echo "gen.sh: $(basename "$dst") is at $cur, expected $pin" >&2
            echo "  delete it and re-run, or update PIN deliberately." >&2
            exit 1
        fi
        return 0
    fi
    mkdir -p "$(dirname "$dst")"
    git clone --quiet "$url" "$dst"
    git -C "$dst" fetch --quiet --depth 1 origin "refs/tags/$ref"
    git -C "$dst" checkout --quiet --detach "$pin"
    local cur
    cur="$(git -C "$dst" rev-parse HEAD)"
    if [[ "$cur" != "$pin" ]]; then
        echo "gen.sh: fetched $(basename "$dst") at $cur, expected $pin" >&2
        exit 1
    fi
}

SQLCIPHER_SRC="$UPSTREAM/sqlcipher"
fetch_pinned "$SQLCIPHER_SRC" \
    "https://github.com/sqlcipher/sqlcipher.git" \
    "$PIN_SQLCIPHER" "v4.18.0"

cd "$SQLCIPHER_SRC"
./configure --with-tempstore=yes --fts5 \
    CFLAGS="-DSQLITE_HAS_CODEC -DSQLITE_TEMP_STORE=2 -DSQLITE_THREADSAFE=1 -DSQLITE_EXTRA_INIT=sqlcipher_extra_init -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown" \
    LDFLAGS="-lcrypto" >/dev/null 2>&1
make sqlite3.c >/dev/null
cp sqlite3.c sqlite3.h "$DIR/"
cd "$DIR"
echo "gen.sh: regenerated sqlite3.{c,h} from SQLCipher v4.18.0 ($PIN_SQLCIPHER)"
