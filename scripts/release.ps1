# Release script for Tunn (PowerShell version)
# Usage: .\scripts\release.ps1 [version]
# Example: .\scripts\release.ps1 v1.0.0

param(
    [Parameter(Mandatory=$true)]
    [string]$Version
)

# Function to print colored output
function Write-Info {
    param([string]$Message)
    Write-Host "[INFO] $Message" -ForegroundColor Green
}

function Write-Warning {
    param([string]$Message)
    Write-Host "[WARNING] $Message" -ForegroundColor Yellow
}

function Write-Error {
    param([string]$Message)
    Write-Host "[ERROR] $Message" -ForegroundColor Red
}

# Validate version format
if ($Version -notmatch '^v\d+\.\d+\.\d+(-.*)?$') {
    Write-Error "Invalid version format. Use vX.Y.Z or vX.Y.Z-suffix"
    exit 1
}

Write-Info "Preparing release $Version"

# Check if we're in a git repository
try {
    git rev-parse --git-dir | Out-Null
} catch {
    Write-Error "Not in a git repository"
    exit 1
}

# Check if working directory is clean
$gitStatus = git status --porcelain
if ($gitStatus) {
    Write-Error "Working directory is not clean. Please commit or stash changes."
    exit 1
}

# Check if we're on main/master branch
$currentBranch = git rev-parse --abbrev-ref HEAD
if ($currentBranch -notin @("main", "master")) {
    Write-Warning "You are not on main/master branch (current: $currentBranch)"
    $response = Read-Host "Continue? (y/N)"
    if ($response -notmatch '^[Yy]$') {
        Write-Info "Release cancelled"
        exit 0
    }
}

# Check if tag already exists
try {
    git rev-parse $Version | Out-Null
    Write-Error "Tag $Version already exists"
    exit 1
} catch {
    # Tag doesn't exist, which is what we want
}

# Update version in main.go if version variable exists
if (Test-Path "main.go") {
    $mainGoContent = Get-Content "main.go"
    if ($mainGoContent -match "var Version") {
        Write-Info "Updating version in main.go"
        $mainGoContent = $mainGoContent -replace 'var Version = .*', "var Version = `"$Version`""
        $mainGoContent | Set-Content "main.go"
        git add main.go
        try {
            git commit -m "chore: bump version to $Version"
        } catch {
            # Commit might fail if no changes, that's ok
        }
    }
}

# Create and push tag
Write-Info "Creating and pushing tag $Version"
git tag -a $Version -m "Release $Version"

Write-Info "Pushing tag to origin..."
git push origin $Version

Write-Info "Release $Version has been created!"
Write-Info "GitHub Actions will automatically build and create the release."

# Get repository URL for monitoring
try {
    $originUrl = git config --get remote.origin.url
    $repoPath = if ($originUrl -match 'github\.com[:/](.+)\.git') { $matches[1] } else { $null }
    if ($repoPath) {
        Write-Info "You can monitor the progress at: https://github.com/$repoPath/actions"
        
        # Try to open release page
        $releaseUrl = "https://github.com/$repoPath/releases/tag/$Version"
        try {
            Start-Process $releaseUrl
        } catch {
            # Ignore if we can't open browser
        }
    }
} catch {
    # Ignore errors in URL parsing
}
