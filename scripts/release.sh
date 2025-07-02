#!/bin/bash

# Release script for Tunn
# Usage: ./scripts/release.sh [version]
# Example: ./scripts/release.sh v1.0.0

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Function to print colored output
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check if version is provided
if [ -z "$1" ]; then
    print_error "Version not provided"
    echo "Usage: $0 <version>"
    echo "Example: $0 v1.0.0"
    exit 1
fi

VERSION="$1"

# Validate version format
if [[ ! $VERSION =~ ^v[0-9]+\.[0-9]+\.[0-9]+(-.*)?$ ]]; then
    print_error "Invalid version format. Use vX.Y.Z or vX.Y.Z-suffix"
    exit 1
fi

print_info "Preparing release $VERSION"

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    print_error "Not in a git repository"
    exit 1
fi

# Check if working directory is clean
if ! git diff-index --quiet HEAD --; then
    print_error "Working directory is not clean. Please commit or stash changes."
    exit 1
fi

# Check if we're on main/master branch
CURRENT_BRANCH=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CURRENT_BRANCH" != "main" && "$CURRENT_BRANCH" != "master" ]]; then
    print_warning "You are not on main/master branch (current: $CURRENT_BRANCH)"
    read -p "Continue? (y/N): " -n 1 -r
    echo
    if [[ ! $REPLY =~ ^[Yy]$ ]]; then
        print_info "Release cancelled"
        exit 0
    fi
fi

# Check if tag already exists
if git rev-parse "$VERSION" >/dev/null 2>&1; then
    print_error "Tag $VERSION already exists"
    exit 1
fi

# Update version in main.go if version variable exists
if grep -q "var Version" main.go 2>/dev/null; then
    print_info "Updating version in main.go"
    sed -i "s/var Version = .*/var Version = \"$VERSION\"/" main.go
    git add main.go
    git commit -m "chore: bump version to $VERSION" || true
fi

# Create and push tag
print_info "Creating and pushing tag $VERSION"
git tag -a "$VERSION" -m "Release $VERSION"

print_info "Pushing tag to origin..."
git push origin "$VERSION"

print_info "Release $VERSION has been created!"
print_info "GitHub Actions will automatically build and create the release."
print_info "You can monitor the progress at: https://github.com/$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/\1/')/actions"

# Open release page if possible
if command -v xdg-open > /dev/null; then
    REPO_URL=$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/https:\/\/github.com\/\1/')
    xdg-open "$REPO_URL/releases/tag/$VERSION" 2>/dev/null || true
elif command -v open > /dev/null; then
    REPO_URL=$(git config --get remote.origin.url | sed 's/.*github.com[:/]\(.*\)\.git/https:\/\/github.com\/\1/')
    open "$REPO_URL/releases/tag/$VERSION" 2>/dev/null || true
fi
