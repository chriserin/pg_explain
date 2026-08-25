#!/bin/bash
set -e

VERSION=${1}
PREVIOUS_TAG=${2}

if [ -z "$VERSION" ]; then
    echo "Usage: ./relscripts/release-notes.sh v0.1.0 [previous-tag]"
    exit 1
fi

# Auto-detect previous tag if not provided
if [ -z "$PREVIOUS_TAG" ]; then
    PREVIOUS_TAG=$(git describe --tags --abbrev=0 ${VERSION}^ 2>/dev/null || echo "")
fi

# Get commits between tags
if [ -n "$PREVIOUS_TAG" ]; then
    COMMITS=$(git log ${PREVIOUS_TAG}..${VERSION} --pretty=format:"%s" --reverse)
    RANGE="${PREVIOUS_TAG}..${VERSION}"
else
    COMMITS=$(git log ${VERSION} --pretty=format:"%s" --reverse)
    RANGE="Initial release"
fi

# Arrays to hold categorized commits
declare -a FEATURES
declare -a FIXES
declare -a DOCS
declare -a CHORES
declare -a OTHER

# Parse conventional commits
while IFS= read -r commit; do
    # Extract type, scope, and message
    if [[ $commit =~ ^feat(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            FEATURES+=("**${scope}:** ${message}")
        else
            FEATURES+=("${message}")
        fi
    elif [[ $commit =~ ^fix(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            FIXES+=("**${scope}:** ${message}")
        else
            FIXES+=("${message}")
        fi
    elif [[ $commit =~ ^docs(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            DOCS+=("**${scope}:** ${message}")
        else
            DOCS+=("${message}")
        fi
    elif [[ $commit =~ ^chore(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            CHORES+=("**${scope}:** ${message}")
        else
            CHORES+=("${message}")
        fi
    elif [[ $commit =~ ^refactor(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            OTHER+=("**${scope}:** ${message}")
        else
            OTHER+=("Refactor: ${message}")
        fi
    elif [[ $commit =~ ^test(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            OTHER+=("**${scope}:** ${message}")
        else
            OTHER+=("Test: ${message}")
        fi
    elif [[ $commit =~ ^perf(\(([^\)]+)\))?:\ (.+)$ ]]; then
        scope="${BASH_REMATCH[2]}"
        message="${BASH_REMATCH[3]}"
        if [ -n "$scope" ]; then
            OTHER+=("**${scope}:** ${message}")
        else
            OTHER+=("Performance: ${message}")
        fi
    else
        # Non-conventional commit
        OTHER+=("$commit")
    fi
done <<<"$COMMITS"

# Build release notes
NOTES=""

if [ -n "$PREVIOUS_TAG" ]; then
    NOTES+="## Changes since ${PREVIOUS_TAG}\n\n"
else
    NOTES+="## Initial Release\n\n"
fi

# Features section
if [ ${#FEATURES[@]} -gt 0 ]; then
    NOTES+="### Features\n\n"
    for feature in "${FEATURES[@]}"; do
        NOTES+="- ${feature}\n"
    done
    NOTES+="\n"
fi

# Fixes section
if [ ${#FIXES[@]} -gt 0 ]; then
    NOTES+="### Bug Fixes\n\n"
    for fix in "${FIXES[@]}"; do
        NOTES+="- ${fix}\n"
    done
    NOTES+="\n"
fi

# Documentation section
if [ ${#DOCS[@]} -gt 0 ]; then
    NOTES+="### Documentation\n\n"
    for doc in "${DOCS[@]}"; do
        NOTES+="- ${doc}\n"
    done
    NOTES+="\n"
fi

# Other changes section
if [ ${#OTHER[@]} -gt 0 ]; then
    NOTES+="### Other Changes\n\n"
    for other in "${OTHER[@]}"; do
        NOTES+="- ${other}\n"
    done
    NOTES+="\n"
fi

# Add full changelog link if we have a previous tag
if [ -n "$PREVIOUS_TAG" ]; then
    REPO_URL=$(git remote get-url origin | sed 's/\.git$//' | sed 's/git@github.com:/https:\/\/github.com\//')
    NOTES+="**Full Changelog**: ${REPO_URL}/compare/${PREVIOUS_TAG}...${VERSION}"
fi

# Output the notes
echo -e "$NOTES"
