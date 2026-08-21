#!/bin/sh

# Updates every internal graphjin-slim module requirement in every go.mod to
# the given version. Called by the auto-release workflow; harmless to run
# locally (it only edits require lines of our own /v3 modules).
#
# Usage: release.sh <version>   e.g. release.sh 3.1.0

if [ -z "$1" ] || ! echo "$1" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
    echo "Usage: $0 <version> (e.g. 3.1.0)"
    exit 1
fi

new_version=$1
export new_version

find . -name 'go.mod' -not -path './bench/*' -exec sh -c '
    for file do
        echo "Processing $file"
        # bump require lines of our own /v3 modules to the new version;
        # replace directives carry no version and are left untouched
        sed -i"" -e "/github.com\/aegion-dynamic\/graphjin-slim\/.*\/v[0-9]/s/ v[0-9.-]*[^ ]*/ v$new_version/" "$file"
    done
' sh {} +
