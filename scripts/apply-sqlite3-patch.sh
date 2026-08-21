#!/usr/bin/env bash
# Applies the GraphJin SQLCipher ordering patch to mattn/go-sqlite3 in the
# Go module cache (the Go equivalent of `patch-package`).
#
# - Pinned to the exact version in serv/go.mod; refuses anything else.
# - Idempotent: skips if already applied.
# - Verifies anchors before applying; fails loudly on mismatch.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
PATCH_FILE="$REPO_ROOT/patches/go-sqlite3/v1.14.50-connecthook-first.patch"
WANT_VERSION="v1.14.50"
MARKER="NOTE (GraphJin patch)"

GOMODCACHE="${GOMODCACHE:-$(go env GOMODCACHE)}"
MOD_DIR="$GOMODCACHE/github.com/mattn/go-sqlite3@${WANT_VERSION}"

if [[ ! -d "$MOD_DIR" ]]; then
    echo "patch-sqlite3: module not found at $MOD_DIR" >&2
    echo "  run: go mod download github.com/mattn/go-sqlite3" >&2
    exit 1
fi
if [[ ! -w "$MOD_DIR" ]]; then
    chmod -R u+w "$MOD_DIR"
fi

TARGET="$MOD_DIR/sqlite3.go"

if grep -q "$MARKER" "$TARGET"; then
    echo "patch-sqlite3: already applied to $TARGET"
    exit 0
fi

# Sanity: the upstream block we relocate must exist exactly once pre-patch.
COUNT=$(grep -c "if d.ConnectHook != nil {" "$TARGET" || true)
if [[ "$COUNT" != "1" ]]; then
    echo "patch-sqlite3: unexpected sqlite3.go state ($COUNT hook sites; expected exactly 1)." >&2
    echo "  refusing to patch a modified or wrong-version source." >&2
    exit 1
fi

if ! patch --dry-run -s -p1 -d "$MOD_DIR" < "$PATCH_FILE" >/dev/null; then
    echo "patch-sqlite3: dry-run FAILED against $MOD_DIR" >&2
    echo "  upstream v${WANT_VERSION#want } may have changed; regenerate the patch." >&2
    exit 1
fi
patch -s -p1 -d "$MOD_DIR" < "$PATCH_FILE"

grep -q "$MARKER" "$TARGET"
echo "patch-sqlite3: applied ConnectHook-ordering patch to $TARGET"
