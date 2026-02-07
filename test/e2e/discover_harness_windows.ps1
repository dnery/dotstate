#!/usr/bin/env pwsh
[CmdletBinding()]
param(
    [string]$DotBin = ".\\bin\\dot.exe",
    [string]$OutDir = "",
    [ValidateSet("discover-fast","discover-deep","discover-interactive","capture-loop","all")]
    [string]$Scenario = "all",
    [switch]$Record,
    [switch]$Upload,
    [switch]$Strict,
    [switch]$NoDelay,
    [switch]$IncludeRaw
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Timestamp-Compact { (Get-Date).ToUniversalTime().ToString("yyyyMMddTHHmmssZ") }
function Timestamp-Human { (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ") }

if ([string]::IsNullOrWhiteSpace($OutDir)) {
    $OutDir = Join-Path "state/e2e-runs" (Timestamp-Compact)
}

New-Item -ItemType Directory -Path $OutDir -Force | Out-Null
$OutDir = (Resolve-Path $OutDir).Path

$RawDir = Join-Path $OutDir "local-raw"
$ArtifactsDir = Join-Path $OutDir "artifacts"
$CmdDir = Join-Path $ArtifactsDir "commands"
$DerivDir = Join-Path $ArtifactsDir "derivatives"

New-Item -ItemType Directory -Path $RawDir,$ArtifactsDir,$CmdDir,$DerivDir -Force | Out-Null

if ($Record -and -not $env:ASCIINEMA_REC) {
    if (-not (Get-Command asciinema -ErrorAction SilentlyContinue)) {
        throw "asciinema is required for --Record"
    }

    $CastFile = Join-Path $ArtifactsDir "session.cast"
    $ScriptPath = $MyInvocation.MyCommand.Path
    $Args = @(
        "-File", "`"$ScriptPath`"",
        "-DotBin", "`"$DotBin`"",
        "-OutDir", "`"$OutDir`"",
        "-Scenario", "`"$Scenario`""
    )
    if ($Strict) { $Args += "-Strict" }
    if ($NoDelay) { $Args += "-NoDelay" }
    if ($IncludeRaw) { $Args += "-IncludeRaw" }

    $PwshCmd = "pwsh " + ($Args -join " ")
    $env:ASCIINEMA_REC = "1"
    & asciinema rec --stdin --overwrite --title "dotstate harness $(Timestamp-Human)" --command $PwshCmd $CastFile

    if ($Upload) {
        & asciinema upload $CastFile | Tee-Object -FilePath (Join-Path $ArtifactsDir "upload.txt") | Out-Null
    }
    exit 0
}

function Sleep-IfEnabled {
    if (-not $NoDelay) { Start-Sleep -Seconds 1 }
}

function Redact-Content {
    param([string]$Content)

    $text = $Content
    $text = [regex]::Replace($text, 'gh[pors]_[A-Za-z0-9_]+', '[REDACTED_GITHUB_TOKEN]')
    $text = [regex]::Replace($text, 'glpat-[A-Za-z0-9_-]+', '[REDACTED_GITLAB_TOKEN]')
    $text = [regex]::Replace($text, 'AKIA[0-9A-Z]{16}', '[REDACTED_AWS_ACCESS_KEY]')
    $text = [regex]::Replace($text, 'ASIA[0-9A-Z]{16}', '[REDACTED_AWS_ACCESS_KEY]')
    $text = [regex]::Replace($text, '-----BEGIN [A-Z ]*PRIVATE KEY-----', '[REDACTED_PRIVATE_KEY_HEADER]')
    $text = [regex]::Replace($text, '-----END [A-Z ]*PRIVATE KEY-----', '[REDACTED_PRIVATE_KEY_FOOTER]')
    return $text
}

$RawRunLog = Join-Path $RawDir "run.log"
$SummaryMd = Join-Path $OutDir "summary.md"
$SummaryJson = Join-Path $OutDir "summary.json"
$TimelineMd = Join-Path $OutDir "timeline.md"
$EnvironmentTxt = Join-Path $OutDir "environment.txt"
$DerivIndex = Join-Path $DerivDir "index.md"

Start-Transcript -Path $RawRunLog -Force | Out-Null

$DotBinAbs = (Resolve-Path $DotBin).Path
if (-not (Test-Path $DotBinAbs)) {
    throw "dot binary not found: $DotBin"
}

$SandboxDir = Join-Path $OutDir "sandbox"
$FakeHome = Join-Path $SandboxDir "home"
$RepoDir = Join-Path $SandboxDir "repo"
$ChezmoiDefault = Join-Path $FakeHome ".local/share/chezmoi"

if (Test-Path $SandboxDir) { Remove-Item -Recurse -Force $SandboxDir }
New-Item -ItemType Directory -Path (Join-Path $FakeHome ".config/app"), (Join-Path $FakeHome ".ssh"), (Join-Path $RepoDir "home"), (Join-Path $RepoDir "state") -Force | Out-Null

@"
[user]
    name = Harness User
    email = harness@example.com
"@ | Set-Content -Path (Join-Path $FakeHome ".gitconfig")

"export EDITOR=vim" | Set-Content -Path (Join-Path $FakeHome ".bashrc")
"export EDITOR=vim" | Set-Content -Path (Join-Path $FakeHome ".zshrc")
'{"theme":"dark","fontSize":14}' | Set-Content -Path (Join-Path $FakeHome ".config/app/settings.json")

@"
-----BEGIN OPENSSH PRIVATE KEY-----
FAKE-KEY-MATERIAL
-----END OPENSSH PRIVATE KEY-----
"@ | Set-Content -Path (Join-Path $FakeHome ".ssh/id_rsa")

"Host github.com" | Set-Content -Path (Join-Path $FakeHome ".ssh/config")

& git -C $RepoDir init --initial-branch=main | Out-Null
& git -C $RepoDir config user.name "Harness User"
& git -C $RepoDir config user.email "harness@example.com"
& git -C $RepoDir config commit.gpgsign false

@"
[repo]
url = "file://$RepoDir"
path = "$RepoDir"
branch = "main"

[sync]
interval_minutes = 30
enable_idle = true
enable_shutdown = true

[tools]
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"

[wsl]
enable = false
distro_name = ""
flake_ref = ""
"@ | Set-Content -Path (Join-Path $RepoDir "dot.toml")

& git -C $RepoDir add dot.toml home state
& git -C $RepoDir commit -m "harness: initialize sandbox repo" | Out-Null

@"
# Timeline

| Start UTC | End UTC | Command | Exit |
|---|---|---|---|
"@ | Set-Content -Path $TimelineMd

$envReport = @(
    "utc=$(Timestamp-Human)",
    "os=$([System.Runtime.InteropServices.RuntimeInformation]::OSDescription)",
    "arch=$([System.Runtime.InteropServices.RuntimeInformation]::OSArchitecture)",
    "go=$((go version) 2>$null)",
    "git=$((git --version) 2>$null)",
    "asciinema=$((asciinema --version) 2>$null)",
    "dot=$((& $DotBinAbs version) 2>$null)"
)
$envReport -join [Environment]::NewLine | Set-Content -Path $EnvironmentTxt

$Checks = New-Object System.Collections.Generic.List[object]

function Add-Check {
    param(
        [string]$Name,
        [string]$Status,
        [string]$Note,
        [string]$Issue
    )
    $Checks.Add([pscustomobject]@{ name = $Name; status = $Status; note = $Note; issue = $Issue }) | Out-Null
}

function Run-CommandCapture {
    param(
        [string]$Id,
        [scriptblock]$Action
    )

    $rawFile = Join-Path $RawDir ("$Id.txt")
    $redactedFile = Join-Path $CmdDir ("$Id.txt")

    $start = Timestamp-Human
    $exitCode = 0

    try {
        & $Action *> $rawFile
        $exitCode = if ($LASTEXITCODE) { $LASTEXITCODE } else { 0 }
    } catch {
        $_ | Out-File -FilePath $rawFile -Append
        $exitCode = 1
    }

    $end = Timestamp-Human
    "| $start | $end | `$id` | $exitCode |" | Add-Content -Path $TimelineMd

    $rawContent = Get-Content -Raw -Path $rawFile
    (Redact-Content -Content $rawContent) | Set-Content -Path $redactedFile

    if ($IncludeRaw) {
        $rawArtifactDir = Join-Path $ArtifactsDir "raw"
        New-Item -ItemType Directory -Path $rawArtifactDir -Force | Out-Null
        Copy-Item -Path $rawFile -Destination (Join-Path $rawArtifactDir ("$Id.txt")) -Force
    }

    return $exitCode
}

function Get-DotArgs {
    param([string[]]$Extra)
    return @(
        "--config", (Join-Path $RepoDir "dot.toml"),
        "--repo-dir", $RepoDir
    ) + $Extra
}

function Get-ReportCount {
    param(
        [string]$ReportPath,
        [string]$Category
    )
    if (-not (Test-Path $ReportPath)) {
        return 0
    }

    $line = Get-Content -Path $ReportPath | Where-Object { $_ -match "^=== $Category \([0-9]+\) ===$" } | Select-Object -First 1
    if ([string]::IsNullOrWhiteSpace($line)) {
        return 0
    }

    $match = [regex]::Match($line, '\(([0-9]+)\)')
    if (-not $match.Success) {
        return 0
    }
    return [int]$match.Groups[1].Value
}

$env:HOME = $FakeHome
$env:XDG_CONFIG_HOME = Join-Path $FakeHome ".config"

function Run-DiscoverFast {
    $doctorCode = Run-CommandCapture -Id "doctor" -Action { & $DotBinAbs doctor @(Get-DotArgs @()) }
    if ($doctorCode -eq 0) { Add-Check "doctor exits cleanly" "PASS" "doctor command succeeded" "" } else { Add-Check "doctor exits cleanly" "FAIL" "doctor command failed" "" }

    $reportCode = Run-CommandCapture -Id "discover_report_fast" -Action { & $DotBinAbs discover @(Get-DotArgs @("--report")) }
    if ($reportCode -eq 0) { Add-Check "discover --report exits cleanly" "PASS" "discover report succeeded" "" } else { Add-Check "discover --report exits cleanly" "FAIL" "discover report failed" "" }

    $report = Get-Content -Raw -Path (Join-Path $CmdDir "discover_report_fast.txt")
    if ($report -match "~/.gitconfig|~/.zshrc|~/.bashrc") {
        Add-Check "shell dotfiles appear in report" "PASS" "report includes shell dotfiles" "#6"
    } else {
        Add-Check "shell dotfiles appear in report" "FAIL" "report missing shell dotfiles" "#6"
    }
}

function Run-DiscoverDeep {
    $code = Run-CommandCapture -Id "discover_report_deep" -Action { & $DotBinAbs discover @(Get-DotArgs @("--report","--deep")) }
    if ($code -eq 0) { Add-Check "discover --deep report exits cleanly" "PASS" "deep report succeeded" "" } else { Add-Check "discover --deep report exits cleanly" "FAIL" "deep report failed" "" }
}

function Run-DiscoverInteractive {
    $input = Join-Path $SandboxDir "interactive-input.txt"
    "n`ny" | Set-Content -Path $input
    $code = Run-CommandCapture -Id "discover_interactive" -Action {
        Get-Content $input | & $DotBinAbs discover @(Get-DotArgs @("--no-commit"))
    }
    if ($code -eq 0) {
        Add-Check "interactive flow transcript captured" "PASS" "interactive command succeeded" "#8"
    } else {
        Add-Check "interactive flow transcript captured" "FAIL" "interactive command failed" "#8"
    }
}

function Run-CaptureLoop {
    $discoverCode = Run-CommandCapture -Id "discover_yes_warning" -Action { & $DotBinAbs discover @(Get-DotArgs @("--yes","--no-commit","--secrets","warning")) }
    if ($discoverCode -eq 0) { Add-Check "discover --yes --secrets warning exits cleanly" "PASS" "discover add succeeded" "#9" } else { Add-Check "discover --yes --secrets warning exits cleanly" "FAIL" "discover add failed" "#9" }

    $repoHomeFiles = (Get-ChildItem -Recurse -File -Path (Join-Path $RepoDir "home") -ErrorAction SilentlyContinue | Measure-Object).Count
    $chezDefaultFiles = (Get-ChildItem -Recurse -File -Path $ChezmoiDefault -ErrorAction SilentlyContinue | Measure-Object).Count

    if ($repoHomeFiles -gt 0) { Add-Check "discover adds into repo home/" "PASS" "repo home file count: $repoHomeFiles" "#5" } else { Add-Check "discover adds into repo home/" "FAIL" "repo home file count: $repoHomeFiles" "#5" }
    if ($chezDefaultFiles -eq 0) { Add-Check "discover avoids default chezmoi source" "PASS" "default chezmoi file count: $chezDefaultFiles" "#5" } else { Add-Check "discover avoids default chezmoi source" "FAIL" "default chezmoi file count: $chezDefaultFiles" "#5" }

    $linkItems = @()
    $linkRoots = @((Join-Path $RepoDir "home"))
    if (Test-Path $ChezmoiDefault) {
        $linkRoots += $ChezmoiDefault
    }
    foreach ($root in $linkRoots) {
        $linkItems += Get-ChildItem -Path $root -Recurse -Force -ErrorAction SilentlyContinue | Where-Object {
            $_.Attributes -band [IO.FileAttributes]::ReparsePoint
        }
    }
    if (@($linkItems).Count -eq 0) {
        Add-Check "copy semantics (no symlinks)" "PASS" "symlink count: 0" ""
    } else {
        Add-Check "copy semantics (no symlinks)" "FAIL" "symlink count: $(@($linkItems).Count)" ""
    }

    Add-Content -Path (Join-Path $FakeHome ".gitconfig") -Value "# modified by harness at $(Timestamp-Human)"

    $captureCode = Run-CommandCapture -Id "capture" -Action { & $DotBinAbs capture @(Get-DotArgs @()) }
    if ($captureCode -eq 0) { Add-Check "capture exits cleanly" "PASS" "capture command succeeded" "" } else { Add-Check "capture exits cleanly" "FAIL" "capture command failed" "" }

    $status = (& git -C $RepoDir status --porcelain)
    if (-not [string]::IsNullOrWhiteSpace($status)) { Add-Check "capture creates repo diff after edit" "PASS" "repo has changes after capture" "" } else { Add-Check "capture creates repo diff after edit" "FAIL" "repo clean after capture" "" }
}

switch ($Scenario) {
    "discover-fast" { Run-DiscoverFast }
    "discover-deep" { Run-DiscoverDeep }
    "discover-interactive" { Run-DiscoverInteractive }
    "capture-loop" { Run-CaptureLoop }
    "all" {
        Run-DiscoverFast
        Sleep-IfEnabled
        Run-DiscoverDeep
        Sleep-IfEnabled
        Run-DiscoverInteractive
        Sleep-IfEnabled
        Run-CaptureLoop
    }
}

$derivLines = @()
$derivLines += "# Derivative Hooks"
$derivLines += ""
if (Test-Path (Join-Path $ArtifactsDir "session.cast")) {
    $derivLines += "Session cast: present"
} else {
    $derivLines += "Session cast: not present (run with --record)"
}
if (Get-Command agg -ErrorAction SilentlyContinue) { $derivLines += "- agg available for gif conversion" } else { $derivLines += "- agg not installed" }
if (Get-Command vhs -ErrorAction SilentlyContinue) { $derivLines += "- vhs available for rendering" } else { $derivLines += "- vhs not installed" }
if (Get-Command ffmpeg -ErrorAction SilentlyContinue) { $derivLines += "- ffmpeg available for post-processing" } else { $derivLines += "- ffmpeg not installed" }
$derivLines -join [Environment]::NewLine | Set-Content -Path $DerivIndex

$recommendedFast = Get-ReportCount -ReportPath (Join-Path $CmdDir "discover_report_fast.txt") -Category "Recommended"
$maybeFast = Get-ReportCount -ReportPath (Join-Path $CmdDir "discover_report_fast.txt") -Category "Maybe"
$riskyFast = Get-ReportCount -ReportPath (Join-Path $CmdDir "discover_report_fast.txt") -Category "Risky"
$recommendedDeep = Get-ReportCount -ReportPath (Join-Path $CmdDir "discover_report_deep.txt") -Category "Recommended"

$knownIssueRows = @()
foreach ($check in $Checks) {
    if ($check.status -eq "FAIL" -and -not [string]::IsNullOrWhiteSpace($check.issue)) {
        $knownIssueRows += "| $($check.name) | $($check.issue) | $($check.note) |"
    }
}

$repoDiff = (& git -C $RepoDir status --short)
$repoDiffLines = @()
if ([string]::IsNullOrWhiteSpace(($repoDiff -join ""))) {
    $repoDiffLines += "  - (no changes)"
} else {
    foreach ($line in $repoDiff) {
        $repoDiffLines += "  - $line"
    }
}

$summaryLines = @()
$summaryLines += "# Discover Harness Summary"
$summaryLines += ""
$summaryLines += "- UTC: $(Timestamp-Human)"
$summaryLines += "- Scenario: ``$Scenario``"
$summaryLines += "- Dot binary: ``$DotBinAbs``"
$summaryLines += "- Output dir: ``$OutDir``"
$summaryLines += ""
$summaryLines += "## Pass/Fail Matrix"
$summaryLines += ""
$summaryLines += "| Check | Status | Note |"
$summaryLines += "|---|---|---|"
foreach ($check in $Checks) {
    $summaryLines += "| $($check.name) | $($check.status) | $($check.note) |"
}
$summaryLines += ""
$summaryLines += "## What Changed"
$summaryLines += ""
$summaryLines += "- Fast report Recommended: $recommendedFast"
$summaryLines += "- Fast report Maybe: $maybeFast"
$summaryLines += "- Fast report Risky: $riskyFast"
$summaryLines += "- Deep report Recommended: $recommendedDeep"
$summaryLines += "- Repo home tracked files: $((Get-ChildItem -Recurse -File -Path (Join-Path $RepoDir "home") -ErrorAction SilentlyContinue | Measure-Object).Count)"
$summaryLines += "- Default chezmoi tracked files: $((Get-ChildItem -Recurse -File -Path $ChezmoiDefault -ErrorAction SilentlyContinue | Measure-Object).Count)"
$summaryLines += "- Repo diff summary:"
$summaryLines += $repoDiffLines
$summaryLines += ""
$summaryLines += "## Known Issue Detection"
$summaryLines += ""
if (@($knownIssueRows).Count -gt 0) {
    $summaryLines += "| Failing Check | Related Issue | Evidence |"
    $summaryLines += "|---|---|---|"
    $summaryLines += $knownIssueRows
} else {
    $summaryLines += "No known-issue signatures detected in failing checks."
}
$summaryLines += ""
$summaryLines += "## Artifacts"
$summaryLines += ""
$summaryLines += "- ``$EnvironmentTxt``"
$summaryLines += "- ``$TimelineMd``"
$summaryLines += "- ``$CmdDir`` (redacted command outputs)"
$summaryLines += "- ``$DerivIndex``"
if (Test-Path (Join-Path $ArtifactsDir "session.cast")) { $summaryLines += "- ``$(Join-Path $ArtifactsDir "session.cast")``" }
if ($IncludeRaw) { $summaryLines += "- ``$(Join-Path $ArtifactsDir "raw")``" } else { $summaryLines += "- raw logs kept local under ``$RawDir`` (not included in review bundle by default)" }

$summaryLines -join [Environment]::NewLine | Set-Content -Path $SummaryMd

$jsonObj = [pscustomobject]@{
    generated_at_utc = (Timestamp-Human)
    scenario = $Scenario
    output_dir = $OutDir
    checks = $Checks
}
$jsonObj | ConvertTo-Json -Depth 6 | Set-Content -Path $SummaryJson

$redactedSummary = Redact-Content -Content (Get-Content -Raw -Path $SummaryMd)
$redactedSummary | Set-Content -Path (Join-Path $ArtifactsDir "summary.redacted.md")

Get-Content -Path $SummaryMd

$failCount = @($Checks | Where-Object { $_.status -eq "FAIL" }).Count
Write-Host ""
Write-Host "Completed with $failCount failing checks."
Write-Host "Summary: $SummaryMd"

Stop-Transcript | Out-Null

if ($Strict -and $failCount -gt 0) {
    exit 1
}
