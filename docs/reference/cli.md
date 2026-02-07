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

Applies managed state to destination.

### `dot capture`

Captures live-file edits back into managed state.

### `dot sync`

Runs capture -> commit -> pull/rebase -> apply -> push.

Flags:
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
