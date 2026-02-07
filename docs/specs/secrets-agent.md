# secrets-env cache architecture

Status: current implementation

This file keeps its historical name for link compatibility. The active design is not a resident daemon. Shell/env secrets are managed by the `secrets-env` CLI, which refreshes 1Password values into local mode-600 cache files and lets shell startup source only existing files.

## Contract

The single supported shell/env secret flow is:

1. Keep canonical secret values in 1Password Secure Notes configured in `~/.config/dotstate/secrets-env.json`.
2. Refresh local cache files explicitly with `secrets-env refresh --all` or `secrets-env refresh --scope <name>`.
3. Store generated cache files under `~/.local/state/dotstate/secrets/`.
4. Let shell startup source existing cache files without contacting 1Password.

Shell startup must stay quiet, deterministic, and non-interactive. It must not call `op`, perform secret refreshes, or depend on a per-command wrapper to provide normal shell variables.

## CLI surface

`secrets-env` supports these commands:

- `inventory`: print redacted target/source metadata.
- `migrate`: create or update allowed Secure Notes from configured migration sources.
- `archive-sources`: archive migrated source items after parity is verified.
- `refresh`: read configured Secure Notes and write cache files.
- `status`: report cache presence, mode, variable count, and mtime without printing values.
- `run -- <command>`: execute a command with the aggregate POSIX cache sourced.

Normal interactive shells should use `refresh` plus startup loaders. `run` is a fallback for a one-off child process when the parent shell has not loaded the aggregate cache.

## Config model

Default config path:

```text
~/.config/dotstate/secrets-env.json
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

Every scope must define `name`, `account`, `vault`, and `item`. Mutable scopes must also match `mutation_allowlist`.

## Cache files

For every refreshed scope, `secrets-env` writes three shell-specific files:

```text
~/.local/state/dotstate/secrets/<scope>.env
~/.local/state/dotstate/secrets/<scope>.fish
~/.local/state/dotstate/secrets/<scope>.ps1
```

`secrets-env refresh --all` also writes the aggregate cache:

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

Only valid shell variable names are exported. Empty values and note metadata are skipped. The aggregate cache merges refreshed scopes and applies `aggregate_exclude`, so sensitive automation-only variables can remain scoped instead of becoming global shell state.

## Shell integration

zsh/bash startup loads:

```text
~/.config/dotstate/secrets-env.sh
```

fish startup loads:

```text
~/.config/fish/conf.d/secrets-env.fish
```

Those loaders only source existing aggregate cache files. If a cache file is missing, startup continues without output or 1Password access.

SFR3-specific refresh uses:

```bash
sfr3-secrets-refresh
```

That helper delegates to:

```bash
secrets-env refresh --scope sfr3
```

Then it sources the SFR3 cache for the current shell.

## Migration

Migration is separate from daily refresh:

- `secrets-env migrate --dry-run` reports intended target updates without mutating.
- `secrets-env migrate --apply` writes only allowlisted Secure Notes.
- `secrets-env archive-sources --apply` archives old source items only after cache parity is verified.

Migration never prints secret values. Source items and target fields are reported as metadata only.

## Failure modes

- Missing config: command exits nonzero with a read/parse error.
- Invalid scope config: validation fails before reading or writing secrets.
- Missing cache on startup: shell startup stays quiet and continues without those variables.
- Missing aggregate cache for `secrets-env run`: command exits and tells the user to refresh first.
- Locked 1Password desktop app: refresh/migration may require desktop approval and can fail or block until the user unlocks 1Password.
- Failed refresh: temp files are cleaned up and the previous installed cache remains in place.

## Security guardrails

- Never commit generated cache files.
- Never print cache contents or full 1Password item JSON in logs.
- Keep cache files and directories private (`0600` files, `0700` directory).
- Keep `op` account selection explicit in config and helpers.
- Keep shell startup cache-only; refresh is always an explicit command.
- Exclude generated caches and auth/session state from backups and config workspaces.
