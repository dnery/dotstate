# senv cache architecture

Status: current implementation

This file keeps its historical name for link compatibility. The active design is not a resident daemon. Shell/env secrets are managed by the `senv` CLI, which refreshes 1Password values into local mode-600 cache files and lets shell startup source only existing files.

## Contract

The single supported shell/env secret flow is:

1. Keep canonical secret values in 1Password Secure Notes configured in `~/.config/dotstate/senv.json`.
2. Refresh local cache files explicitly with `senv refresh --all` or `senv refresh --scope <name>`.
3. Treat `senv refresh --all` as dynamic discovery: it lists configured 1Password source vaults, reads every Secure Note whose title starts with `local/`, and writes one cache per note plus the aggregate cache.
4. Store generated cache files under `~/.local/state/dotstate/secrets/`.
5. Let shell startup source existing cache files without contacting 1Password.

Shell startup must stay quiet, deterministic, and non-interactive. It must not call `op`, perform secret refreshes, or depend on a per-command wrapper to provide normal shell variables.

## CLI surface

`senv` supports these commands:

- `inventory`: print redacted target/source metadata.
- `migrate`: create or update allowed Secure Notes from configured migration sources.
- `archive-sources`: archive migrated source items after parity is verified.
- `refresh`: read Secure Notes and write cache files. `--all` discovers every `local/...` Secure Note in configured source vaults and screams with emoji-heavy warnings when duplicate variables would override each other in the aggregate cache.
- `status`: report cache presence, mode, variable count, and mtime without printing values.
- `run -- <command>`: execute a command with the aggregate POSIX cache sourced.

Normal interactive shells should use `refresh` plus startup loaders. `run` is a fallback for a one-off child process when the parent shell has not loaded the aggregate cache.

## Config model

Default config path:

```text
~/.config/dotstate/senv.json
```

The config contains:

- `op_bin`: optional path/name for the 1Password CLI binary.
- `cache_dir`: cache output directory; defaults to `~/.local/state/dotstate/secrets/` when omitted.
- `aggregate_exclude`: variable names omitted from the aggregate cache.
- `scopes`: named 1Password Secure Notes to read and cache.
- `mutation_allowlist`: account/vault pairs that migration may mutate.
- `migration`: one-time migration source metadata.

Each scope has:

- `name`: cache basename, such as `personal`, `employee`, or `sfr3`.
- `account`: explicit 1Password account identifier.
- `vault`: 1Password vault.
- `item`: Secure Note title or item identifier.
- `section`: optional section label used to select fields.
- `mutate`: whether migration may update this target.

Every scope must define `name`, `account`, `vault`, and `item`. Mutable scopes must also match `mutation_allowlist`. The configured scopes and mutation allowlist also define which account/vault pairs `refresh --all` searches for `local/...` Secure Notes.

## Cache files

For every refreshed scope, `senv` writes three shell-specific files:

```text
~/.local/state/dotstate/secrets/<scope>.env
~/.local/state/dotstate/secrets/<scope>.fish
~/.local/state/dotstate/secrets/<scope>.ps1
```

`senv refresh --all` also writes the aggregate cache:

```text
~/.local/state/dotstate/secrets/secrets.env
~/.local/state/dotstate/secrets/secrets.fish
~/.local/state/dotstate/secrets/secrets.ps1
```

All cache files are written atomically with mode `0600`. The cache directory is created mode `0700`.

Rendered formats:

- POSIX: `export NAME='value'`
- fish: `set -gx NAME 'value'`
- PowerShell: `$env:NAME = 'value'`

Only valid shell variable names are exported. Empty values and note metadata are skipped. `local/...` notes store one environment variable per 1Password field. Secret values should use concealed/password fields; non-secret values may use text fields. The Secure Note body (`notesPlain`) is metadata only and is not parsed as dotenv. The aggregate cache merges refreshed notes in deterministic order and applies `aggregate_exclude`, so sensitive automation-only variables can remain scoped instead of becoming global shell state. If multiple `local/...` notes define the same variable, the last note wins and `senv` prints a large redacted warning with each conflicting note title.

## Shell integration

zsh/bash startup loads:

```text
~/.config/dotstate/senv.sh
```

fish startup loads:

```text
~/.config/fish/conf.d/senv.fish
```

Those loaders only source existing aggregate cache files. If a cache file is missing, startup continues without output or 1Password access.

SFR3-specific refresh uses:

```bash
sfr3-senv-refresh
```

That helper delegates to:

```bash
senv refresh --scope sfr3
```

Then it sources the SFR3 cache for the current shell.

## Migration

Migration is separate from daily refresh:

- `senv migrate --dry-run` reports intended target updates without mutating.
- `senv migrate --apply` writes only allowlisted Secure Notes.
- `senv archive-sources --apply` archives old source items only after cache parity is verified.

Migration never prints secret values. Source items and target fields are reported as metadata only.

## Failure modes

- Missing config: command exits nonzero with a read/parse error.
- Invalid scope config: validation fails before reading or writing secrets.
- Missing cache on startup: shell startup stays quiet and continues without those variables.
- Missing aggregate cache for `senv run`: command exits and tells the user to refresh first.
- Locked 1Password desktop app: refresh/migration may require desktop approval and can fail or block until the user unlocks 1Password.
- Failed refresh: temp files are cleaned up and the previous installed cache remains in place.

## Security guardrails

- Never commit generated cache files.
- Never print cache contents or full 1Password item JSON in logs.
- Keep cache files and directories private (`0600` files, `0700` directory).
- Keep `op` account selection explicit in config and helpers.
- Keep shell startup cache-only; refresh is always an explicit command.
- Exclude generated caches and auth/session state from backups and config workspaces.
