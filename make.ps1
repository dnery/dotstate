param (
    [Parameter(Mandatory=$false)]
    [ValidateSet("build", "test", "clean", "all", "check")]
    [string]$Target = "all"
)

# 1. Variables (Equivalent to Makefile variables)
$RequiredCommands = @("go", "op", "chezmoi")
$ProjectName = "MyProject"
$BuildDir = "./bin"

# 2. Targets (Equivalent to Makefile recipes)
function Target-Build {
    Write-Host "--- Building $ProjectName ---" -ForegroundColor Cyan
    if (!(Test-Path $BuildDir)) { New-Item -ItemType Directory -Path $BuildDir }
    # Run your compiler here (e.g., dotnet build, gcc, etc.)
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'linux', 'amd64', 0; go build -trimpath -ldflags "-s -w" -o bin/linux/dot ./cmd/dot
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'darwin', 'arm64', 0; go build -trimpath -ldflags "-s -w" -o bin/darwin/dot ./cmd/dot
    $env:GOOS, $env:GOARCH, $env:CGO_ENABLED = 'windows', 'amd64', 0; go build -trimpath -ldflags "-s -w" -o bin/windows/dot ./cmd/dot
    Write-Host "Done."
}

function Target-Test {
    Write-Host "--- Running Tests ---" -ForegroundColor Yellow
    # Invoke-Pester or your test runner
}

function Target-Clean {
    Write-Host "--- Cleaning Build Artifacts ---" -ForegroundColor Red
    if (Test-Path $BuildDir) { Remove-Item -Recurse -Force $BuildDir }
}

function Assert-Dependencies {
    Write-Host "Checking for required tools..." -ForegroundColor Gray
    $missing = @()

    foreach ($cmd in $RequiredCommands) {
        # Get-Command -ErrorAction SilentlyContinue is the standard way to check existence
        if (!(Get-Command $cmd -ErrorAction SilentlyContinue)) {
            $missing += $cmd
            Write-Host "  [X] $cmd - Not found" -ForegroundColor Red
        } else {
            # Optional: Log the version for debugging
            Write-Host "  [√] $cmd - Found" -ForegroundColor Green
        }
    }

    if ($missing.Count -gt 0) {
        Write-Error "Missing dependencies: $($missing -join ', '). Please install them and try again."
        exit 1
    }
}

# 3. Execution Logic (Equivalent to Makefile default goal)
switch ($Target) {
    "build" { Target-Build }
    "test"  { Target-Test }
    "clean" { Target-Clean }
    "check" { Assert-Dependencies }
    "all"   { Assert-Dependencies; Target-Clean; Target-Build; Target-Test }
}
