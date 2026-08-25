#!/bin/bash
set -e

VERSION=${1:-$(git describe --tags --always --dirty)}
PROJECT="pg_explain"
DIST_DIR="dist"
ARCHIVES_DIR="dist/archives"

echo "Packaging ${PROJECT} ${VERSION}..."

# Create archives directory
mkdir -p ${ARCHIVES_DIR}

# Package each binary
for BINARY in ${DIST_DIR}/${PROJECT}-${VERSION}-*; do
    # Skip if not a file or already an archive
    [ -f "$BINARY" ] || continue
    [[ "$BINARY" == *.tar.gz ]] && continue
    [[ "$BINARY" == *.zip ]] && continue

    FILENAME=$(basename "$BINARY")

    # Create temporary packaging directory
    TEMP_DIR=$(mktemp -d)
    mkdir -p "${TEMP_DIR}/bin"
    echo "Temporary Working Directory: ${TEMP_DIR}"

    # Copy binary to bin/ directory with name 'pg_explain' (or 'pg_explain.exe' for Windows)
    if [[ $FILENAME =~ windows ]]; then
        cp "$BINARY" "${TEMP_DIR}/bin/pg_explain.exe"
        ARCHIVE_NAME="${FILENAME%.exe}"
    else
        cp "$BINARY" "${TEMP_DIR}/bin/pg_explain"
        ARCHIVE_NAME="${FILENAME}"
    fi

    # Extract platform from filename and create archive with absolute path
    if [[ $FILENAME =~ windows ]]; then
        # Create zip for Windows
        ARCHIVE="$(pwd)/${ARCHIVES_DIR}/${ARCHIVE_NAME}.zip"
        echo "Creating ${ARCHIVE}..."
        (cd ${TEMP_DIR} && zip -q -r "${ARCHIVE}" ./)
    else
        # Create tar.gz for Unix
        ARCHIVE="$(pwd)/${ARCHIVES_DIR}/${ARCHIVE_NAME}.tar.gz"
        echo "Creating ${ARCHIVE}..."
        tar -czf "${ARCHIVE}" -C ${TEMP_DIR} ./
    fi

    # Clean up temp directory
    rm -rf "${TEMP_DIR}"

    echo "  ✓ $(basename ${ARCHIVE})"
done

echo ""
echo "Archives created in ${ARCHIVES_DIR}/"
ls -lh ${ARCHIVES_DIR}/
