#!/bin/bash
set -e

VERSION=${1}
ARCHIVES_DIR="dist/archives"

if [ -z "$VERSION" ]; then
    echo "Usage: ./relscripts/release.sh v0.1.0"
    exit 1
fi

# Check if gh CLI is installed
if ! command -v gh &>/dev/null; then
    echo "Error: GitHub CLI (gh) is not installed"
    echo "Install from: https://cli.github.com/"
    exit 1
fi

echo "Creating GitHub release ${VERSION}..."

# Check if tag exists
if ! git rev-parse ${VERSION} >/dev/null 2>&1; then
    echo "Error: Tag ${VERSION} does not exist."
    echo "Run ./relscripts/version.sh ${VERSION} first to update version and create tag."
    exit 1
fi

# Check if tag has been pushed to remote
if ! git ls-remote --tags origin | grep -q "refs/tags/${VERSION}"; then
    read -p "Push tag ${VERSION} to GitHub? (y/n) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        git push origin main
        git push origin ${VERSION}
    else
        echo "Warning: Tag not pushed to GitHub. Release may fail."
    fi
fi

# Generate release notes from conventional commits
echo "Generating release notes..."
NOTES=$(./relscripts/release-notes.sh ${VERSION})

# Create release
echo "Creating release on GitHub..."
gh release create ${VERSION} \
    --title "Release ${VERSION}" \
    --notes "$NOTES" \
    ${ARCHIVES_DIR}/*.tar.gz \
    ${ARCHIVES_DIR}/*.zip \
    ${ARCHIVES_DIR}/checksums.txt

echo ""
echo "✓ Release ${VERSION} created successfully!"
echo "View at: https://github.com/$(gh repo view --json nameWithOwner -q .nameWithOwner)/releases/tag/${VERSION}"
