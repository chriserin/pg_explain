#!/bin/bash
set -e

ARCHIVES_DIR="dist/archives"
CHECKSUMS_FILE="${ARCHIVES_DIR}/checksums.txt"

echo "Generating checksums..."

cd ${ARCHIVES_DIR}
sha256sum *.tar.gz *.zip 2>/dev/null > checksums.txt || true

echo ""
echo "Checksums:"
cat checksums.txt

echo ""
echo "Checksums saved to ${CHECKSUMS_FILE}"
