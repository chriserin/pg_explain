#!/usr/bin/env bash
# Builds pg_explain with a dev version stamp (dev-MMDDYY) and installs it
# to /usr/local/bin.
set -euo pipefail

cd "$(dirname "$0")"

VERSION="dev-$(date +%m%d%y)"

sed -i.bak -E "s/^const VERSION = \".*\"/const VERSION = \"${VERSION}\"/" main.go
rm -f main.go.bak

go build -o pg_explain .

cp pg_explain /usr/local/bin/pg_explain

echo "Deployed pg_explain ${VERSION} to /usr/local/bin/pg_explain"
