#!/bin/bash
set -e

VERSION_FILE=main.go
INSTALL_PATH=/usr/local/bin/pg_explain

if [ "${1}" = "dev" ]; then
    VERSION="dev-$(date +%Y%m%d%H%M%S)"
    echo "Building pg_explain ${VERSION} (local dev build)..."

    # Temporarily patch the version const for this build only, then
    # restore the file no matter how the script exits — this is a
    # local, throwaway build, not a release, so nothing here should
    # end up committed. (main.go's `VERSION` is a hardcoded const, not
    # a var, so this can't be done via -ldflags -X; see
    # relscripts/build.sh.)
    #
    # -i.bak, not bare -i: GNU sed accepts -i with no argument, but
    # BSD/macOS sed requires an explicit (possibly empty) suffix —
    # -i.bak works identically on both, and doubles as our own backup.
    trap '[ -f "${VERSION_FILE}.bak" ] && mv "${VERSION_FILE}.bak" "$VERSION_FILE"' EXIT
    sed -i.bak "s/const VERSION = \".*\"/const VERSION = \"${VERSION}\"/" "$VERSION_FILE"
    go build -o pg_explain .
else
    echo "Building pg_explain..."
    go build -o pg_explain .
fi

echo "Installing to ${INSTALL_PATH}..."
# Requires write access to /usr/local/bin — rerun with sudo if this fails.
cp pg_explain "$INSTALL_PATH"
rm pg_explain

echo "✓ Installed $("$INSTALL_PATH" version)"
