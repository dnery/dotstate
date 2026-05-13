# CLI Reference

This file is the canonical command contract.

## Global Flags

- `--config <path>`: path to `dot.toml`.
- `--repo-dir <path>`: override repo directory.
- `--verbose`, `-v`: verbose output.

## Commands

### `dot version`

Prints version, commit, build date, and platform.

### `dot doctor`

Checks platform, config resolution, and required tools.

### `dot bootstrap`

Clones/prepares repo path and prints macOS bootstrap checkpoints.

Flags:
- `--repo <url>`: required unless running from a configured repo.
- `--skip-op-checkpoint`: omit the 1Password/op manual checkpoint text.

The command checks Xcode Command Line Tools, points missing Homebrew users to the official installer, treats 1Password/op unlock as a manual checkpoint, then prints safe validation commands: `dot doctor`, `dot apply --dry-run`, `dot sync --dry-run`, `dot macos audit --json`, and `dot schedule install`.

### `dot apply`

Applies managed state to destination through the module orchestrator. The files module remains Chezmoi-backed.

Flags:
- `--dry-run`: emit the module plan without applying changes.

### `dot capture`

Captures live-file edits back into managed state through the module orchestrator.

Flags:
- `--dry-run`: emit the module plan without mutating repo artifacts.

### `dot sync`

Runs capture -> commit -> pull/rebase -> apply -> push through the module orchestrator. `dot sync` refuses to start when the repo is already dirty so unrelated work is not swept into the sync commit.

Flags:
- `--dry-run`: emit capture/apply module plans without capture, git, apply, or push mutations.
- `--no-apply`
- `--no-push`

Subcommand:
- `dot sync now` (alias).

### `dot macos audit`

Emits a non-mutating macOS audit envelope.

Flags:
- `--json`: required; emits `dotstate.audit.v1` JSON.

The current bootstrap bridge redacts hostnames, reports pending macOS surfaces as capability diagnostics, and includes explicit Full Disk Access/TCC/SIP/MDM/Keychain guardrails. Full brew/MAS/apps/defaults collectors remain planned under the macOS audit goal.

### `dot schedule`

Manages the macOS user LaunchAgent for scheduled sync.

Subcommands:
- `dot schedule install`: write and load `~/Library/LaunchAgents/com.dnery.dotstate.sync.plist`.
- `dot schedule status`: report whether the LaunchAgent plist exists and whether launchd reports it loaded.
- `dot schedule remove`: unload best-effort and remove the dotstate-owned plist.

Install flags:
- `--dry-run`: print the LaunchAgent plan without writing or loading it.
- `--dot-bin <path>`: binary path launchd should execute; defaults to the current executable.
- `--interval <minutes>`: override `[sync].interval_minutes`.
- `--no-load`: write the plist but do not call `launchctl bootstrap`/`enable`.

macOS shutdown flush is intentionally not installed. Use interval sync and `dot sync now` for explicit manual flushes.

### `dot discover`

Discovers candidate config files and adds selected files.

Flags:
- `--yes`, `-y`
- `--dry-run`
- `--no-commit`
- `--deep`
- `--report`: prints a redacted report and a `secrets.gitleaks.unavailable` diagnostic when the external scanner is not installed.
- `--secrets <error|warning|ignore>`

## Exit Codes

- `0`: success.
- `1`: generic error.
- `64`: usage error.
- `65`: data/config input error.
- `69`: unavailable dependency/service.
- `75`: conflict condition.
- `78`: configuration error.
