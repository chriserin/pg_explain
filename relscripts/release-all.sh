#!/bin/bash
set -e

VERSION=${1}

if [ -z "$VERSION" ]; then
    echo "Usage: ./relscripts/release-all.sh v0.1.0"
    exit 1
fi

echo "======================================"
echo "  Full Release Process for ${VERSION}"
echo "======================================"
echo ""

# Run tests first - exit if they fail
echo "Step 1/7: Running tests..."
if ! go test ./...; then
    echo "✗ Tests failed - aborting release"
    exit 1
fi
echo "✓ Tests passed"
echo ""

# Update version in main.go and create tag
echo "Step 2/7: Updating version and creating tag..."
./relscripts/version.sh ${VERSION}
echo ""

# Build binaries
echo "Step 3/7: Building binaries..."
./relscripts/build.sh ${VERSION}
echo ""

# Package archives
echo "Step 4/7: Creating archives..."
./relscripts/package.sh ${VERSION}
echo ""

# Generate checksums
echo "Step 5/7: Generating checksums..."
./relscripts/checksums.sh
echo ""

# Preview release notes
echo "Step 6/7: Generating release notes..."
echo ""
echo "======================================"
./relscripts/release-notes.sh ${VERSION}
echo "======================================"
echo ""
read -p "Continue with GitHub release? (y/n) " -n 1 -r
echo
if [[ ! $REPLY =~ ^[Yy]$ ]]; then
    echo "Release cancelled."
    exit 0
fi

# Create GitHub release
echo "Step 7/7: Creating GitHub release..."
./relscripts/release.sh ${VERSION}

echo ""
echo "======================================"
echo "  Release complete! 🎉"
echo "======================================"
