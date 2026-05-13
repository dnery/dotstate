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

- `--repo <url>`: required unless running from configured repo.

Clones/prepares repo path.

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

### `dot discover`

Discovers candidate config files and adds selected files.

Flags:
- `--yes`, `-y`
- `--dry-run`
- `--no-commit`
- `--deep`
- `--report`
- `--secrets <error|warning|ignore>`

## Exit Codes

- `0`: success.
- `1`: generic error.
- `64`: usage error.
- `65`: data/config input error.
- `69`: unavailable dependency/service.
- `75`: conflict condition.
- `78`: configuration error.
