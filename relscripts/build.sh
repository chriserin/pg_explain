#!/bin/bash
set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
PROJECT="pg_explain"
DIST_DIR="dist"

echo "Building ${PROJECT} ${VERSION}..."

# Clean dist directory
rm -rf ${DIST_DIR}
mkdir -p ${DIST_DIR}

# Platforms to build
PLATFORMS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
)

for PLATFORM in "${PLATFORMS[@]}"; do
    GOOS=${PLATFORM%/*}
    GOARCH=${PLATFORM#*/}

    OUTPUT_NAME="${PROJECT}-${VERSION}-${GOOS}-${GOARCH}"

    if [ $GOOS = "windows" ]; then
        OUTPUT_NAME+='.exe'
    fi

    echo "Building ${GOOS}/${GOARCH}..."

    # No -X version injection here: main.go's `VERSION` is a hardcoded
    # const, not a var, so ldflags -X can't set it — version.sh already
    # bakes VERSION into that source file via sed before this script
    # runs.
    GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="-s -w" \
        -o "${DIST_DIR}/${OUTPUT_NAME}" \
        .

    echo "  ✓ ${OUTPUT_NAME}"
done

echo ""
echo "Build complete! Binaries in ${DIST_DIR}/"
ls -lh ${DIST_DIR}/
