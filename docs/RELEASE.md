# Release Process Documentation

This document describes the automated release process for Tunn using GitHub Actions.

## Overview

The project includes several GitHub Actions workflows that automate the build and release process:

1. **CI Workflow** (`ci.yml`) - Runs on every push and pull request
2. **Release Workflow** (`release.yml`) - Manual release creation with multi-platform builds
3. **Auto Release Workflow** (`auto-release.yml`) - Automatic release creation when version tags are pushed

## Supported Platforms

The release process builds binaries for the following platforms:

### Windows
- `tunn-windows-amd64.zip` - Windows 64-bit (Intel/AMD)
- `tunn-windows-386.zip` - Windows 32-bit
- `tunn-windows-arm64.zip` - Windows ARM64

### Linux
- `tunn-linux-amd64.tar.gz` - Linux 64-bit (Intel/AMD)
- `tunn-linux-386.tar.gz` - Linux 32-bit
- `tunn-linux-arm64.tar.gz` - Linux ARM64
- `tunn-linux-arm.tar.gz` - Linux ARM

### macOS
- `tunn-darwin-amd64.tar.gz` - macOS Intel
- `tunn-darwin-arm64.tar.gz` - macOS Apple Silicon (M1/M2)

### FreeBSD
- `tunn-freebsd-amd64.tar.gz` - FreeBSD 64-bit

## Creating a Release

### Method 1: Automatic Release (Recommended)

1. **Create and push a version tag:**
   ```bash
   # Using the provided script (Linux/macOS)
   ./scripts/release.sh v1.0.0
   
   # Using PowerShell script (Windows)
   .\scripts\release.ps1 v1.0.0
   
   # Manual method
   git tag -a v1.0.0 -m "Release v1.0.0"
   git push origin v1.0.0
   ```

2. **Monitor the build:**
   - Go to the Actions tab in your GitHub repository
   - Watch the "Auto Release" workflow complete
   - The release will be automatically created with all platform binaries

### Method 2: Manual Release via GitHub Actions

1. Go to the Actions tab in your GitHub repository
2. Select "Release" workflow
3. Click "Run workflow"
4. Enter the version tag (e.g., `v1.0.0`)
5. Click "Run workflow"

### Method 3: Local Build and Manual Upload

1. **Build locally:**
   ```bash
   # Build for all platforms
   make release
   
   # Or build specific platforms
   make build-all
   make package
   make checksums
   ```

2. **Create release manually:**
   - Go to GitHub Releases
   - Click "Create a new release"
   - Choose or create a tag
   - Upload the files from `dist/packages/`
   - Publish the release

## Version Naming Convention

Follow semantic versioning (SemVer):
- `v1.0.0` - Major release
- `v1.1.0` - Minor release (new features)
- `v1.0.1` - Patch release (bug fixes)
- `v1.0.0-beta.1` - Pre-release (will be marked as pre-release)
- `v1.0.0-rc.1` - Release candidate

## Workflow Details

### CI Workflow (`ci.yml`)
- **Triggers:** Push to main/master/develop, Pull requests
- **Actions:**
  - Run tests with coverage
  - Format checking
  - Code vetting
  - Build for multiple platforms
  - Upload build artifacts (7-day retention)

### Release Workflow (`release.yml`)
- **Triggers:** Manual dispatch with version input
- **Actions:**
  - Build for all supported platforms
  - Create compressed archives
  - Generate SHA256 checksums
  - Create GitHub release with detailed description
  - Upload all assets

### Auto Release Workflow (`auto-release.yml`)
- **Triggers:** Push of version tags (v*.*.*)
- **Actions:**
  - Generate changelog from git commits
  - Create GitHub release (draft for pre-releases)
  - Build and upload platform-specific binaries
  - Generate and upload checksums

## Release Scripts

### Linux/macOS: `scripts/release.sh`
```bash
# Create and push a release tag
./scripts/release.sh v1.0.0
```

Features:
- Validates version format
- Checks git repository status
- Updates version in code if applicable
- Creates and pushes tag
- Opens release page in browser

### Windows: `scripts/release.ps1`
```powershell
# Create and push a release tag
.\scripts\release.ps1 v1.0.0
```

Same features as the bash script, adapted for PowerShell.

## Makefile Targets

```bash
# Build for all platforms
make build-all

# Create release packages
make package

# Generate checksums
make checksums

# Complete local release
make release

# Create GitHub release tag
make github-release TAG=v1.0.0

# Clean build artifacts
make clean
```

## Binary Verification

All releases include a `checksums.txt` file with SHA256 hashes. Verify downloads:

```bash
# Linux/macOS
sha256sum -c checksums.txt

# Windows (PowerShell)
Get-FileHash .\tunn-windows-amd64.zip -Algorithm SHA256
```

## Installation Instructions

1. Download the appropriate binary for your platform
2. Extract the archive:
   ```bash
   # Linux/macOS
   tar -xzf tunn-linux-amd64.tar.gz
   
   # Windows (extract zip file)
   ```
3. Move binary to PATH:
   ```bash
   # Linux/macOS
   sudo mv tunn /usr/local/bin/
   chmod +x /usr/local/bin/tunn
   
   # Windows: Move tunn.exe to a directory in your PATH
   ```

## Troubleshooting

### Failed Builds
- Check the Actions tab for error details
- Ensure all tests pass before releasing
- Verify Go version compatibility

### Missing Assets
- Check if all platform builds completed successfully
- Re-run failed jobs if needed
- Verify upload permissions

### Version Conflicts
- Ensure the tag doesn't already exist
- Delete and recreate tags if necessary:
  ```bash
  git tag -d v1.0.0
  git push origin :refs/tags/v1.0.0
  ```

## Security Considerations

- All builds run in isolated GitHub Actions runners
- Binaries are built from source for each release
- Checksums provide integrity verification
- No external dependencies in build process
