#!/usr/bin/env bash
# Regenerates sqlite3.{c,h} in this directory from the pinned SQLCipher
# submodule commit (./cipher). Maintainers only — downstream go-get users
# build from the committed amalgamation directly.
#
# Deterministic: refuses to run against any commit other than the pin below;
# update PIN when intentionally upgrading SQLCipher.
set -euo pipefail

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SUB="$DIR/cipher"
PIN="63697beb0fafcb61faa7a3e6fd267036548ab11b" # v4.18.0 / SQLite 3.53.4

git -C "$SUB" submodule update --init --recursive >/dev/null 2>&1 || true

CURRENT="$(git -C "$SUB" rev-parse HEAD)"
if [[ "$CURRENT" != "$PIN" ]]; then
    echo "gen.sh: cipher submodule is at $CURRENT, expected $PIN" >&2
    echo "  checkout the pinned commit (or update PIN deliberately) first." >&2
    exit 1
fi

cd "$SUB"
./configure --with-tempstore=yes --fts5 \
    CFLAGS="-DSQLITE_HAS_CODEC -DSQLITE_TEMP_STORE=2 -DSQLITE_THREADSAFE=1 -DSQLITE_EXTRA_INIT=sqlcipher_extra_init -DSQLITE_EXTRA_SHUTDOWN=sqlcipher_extra_shutdown" \
    LDFLAGS="-lcrypto" >/dev/null 2>&1
make sqlite3.c >/dev/null

cp sqlite3.c sqlite3.h "$DIR/"
echo "gen.sh: regenerated $DIR/sqlite3.{c,h} from $PIN"
