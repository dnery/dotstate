# Architecture

This document describes the high-level architecture of dotstate.

---

## Overview

dotstate consists of two components:

1. **A git repository** containing:
   - Desired state for user configs (dotfiles, app configs)
   - Desired state for system settings (via modules)
   - Machine-captured exports (package lists, registry dumps)

2. **A `dot` CLI** (Go binary) that orchestrates:
   - Capture, commit, pull, apply, push transactions
   - Configuration discovery (`dot discover`)
   - Prerequisites checks (`dot doctor`)
   - Platform-specific bootstrap and scheduling

**Chezmoi** is used as the copy-based file state engine.

---

## Design Principles

### Non-Negotiable Invariants

1. **Copy semantics** - Managed files are applied by copying. No symlink requirement.

2. **Secrets never in git** - The repo stores only references (1Password item/field identifiers), never secret values.

3. **"Edit real files" workflow** - Edit `~/.gitconfig`, app configs, etc. in place. Background sync captures edits back to the repo.

4. **Idempotent apply** - Running `dot apply` repeatedly converges to the same result.

5. **Safe automation** - Automation never attempts heroic conflict resolution. On divergence, it stops and surfaces the problem.

6. **Explicit privilege boundaries** - Operations requiring admin/sudo are isolated and obvious.

---

## Components

### `dot` CLI (Go)

The primary entry point. Orchestrates:
- Chezmoi actions (file state engine)
- Git operations
- OS-specific modules (packages/settings)
- Scheduling/automation
- 1Password integration

### Chezmoi (File State Engine)

Manages copy-based dotfile/config application:
- `chezmoi apply` - Update destination files from source state
- `chezmoi re-add` - Capture edits from destinations back to source
- `chezmoi add --secrets=error` - Add files with secret guardrails

### Git

Repository is canonical; remote on GitHub.
- `dot sync` coordinates pull/rebase/apply/push
- Conflicts are surfaced, never auto-resolved

### 1Password

- **SSH agent** for Git authentication
- **CLI (`op`)** for secret retrieval at apply-time
- Secrets written only to gitignored locations or destination files

### OS Modules (Planned)

| OS | Package Management | System Settings |
|----|-------------------|-----------------|
| Windows | winget manifests | Registry, Group Policy |
| macOS | brew bundle | defaults scripts |
| Linux | pacman lists | sudoers, systemd units |
| WSL | NixOS flakes | home-manager |

---

## Data Model

### Managed Artifacts (Chezmoi)

- Stored under `home/` (Chezmoi source directory)
- Applied to destinations via `chezmoi apply`
- Captured from destinations via `chezmoi re-add`
- Examples: `.gitconfig`, editor settings, shell rc files

### Generated/Private Artifacts

- Created at apply-time from secrets or machine context
- Stored under `state/private/` (gitignored)
- Never tracked or captured
- Examples: tokens, signing keys, credentials

### Captured Exports

- Machine state exports stored under `state/`
- May be committed (curated to avoid noise)
- Examples: package lists, registry exports, defaults dumps

### Sub-Repository References

- Nested git repos tracked as references, not files
- Stored in `state/subrepos.toml`
- Examples: `~/.config/nvim`, `~/.emacs.d`

---

## Cross-Platform File Strategy

### Plain Managed Files (Preferred)

- Safe for capture loop (`chezmoi re-add`)
- OS divergence handled via `.chezmoiignore` templates
- Examples: `.gitconfig`, shell configs, editor settings

### Templated Files (Use Sparingly)

- Used when content depends on secrets or OS-specific logic
- Capture loop must not overwrite templates
- Examples: files with 1Password references

### Rule of Thumb

- If a file may include secrets: **generate** or **template** it
- If safe and you want live-edit capture: **plain managed**

---

## Package Structure

| Package | Purpose |
|---------|---------|
| `internal/runner` | Interface-based command execution with timeout support |
| `internal/testutil` | Test helpers including `MockRunner` |
| `internal/config` | Configuration loading, validation, env overrides |
| `internal/logging` | Structured logging via `slog` (stderr + JSON file) |
| `internal/platform` | Cross-platform OS detection and XDG paths |
| `internal/errors` | Custom error types with sysexits.h exit codes |
| `internal/chez` | Chezmoi wrapper using Runner interface |
| `internal/gitx` | Git operations wrapper |
| `internal/discover` | Discovery, classification, secret detection |
| `internal/sync` | Sync transaction logic |

### Design Pattern

Interface-based architecture enables testing without external dependencies:

```go
type Runner interface {
    Run(ctx context.Context, cmd string, args ...string) (*CmdResult, error)
}

// Production: ExecRunner (runs real commands)
// Testing: MockRunner (returns configured responses)
```

---

## Sync Transaction

The core `dot sync` operation runs these phases in order:

1. **Capture** - `chezmoi re-add` pulls destination edits into source state
2. **Commit** - Create local commit if changes exist
3. **Pull/Rebase** - Fetch and rebase local commits on remote
4. **Apply** - `chezmoi apply` updates destinations from source
5. **Push** - Push local commits (fast-forward only)

This order is deliberate:
- Capturing first makes local edits explicit before pull
- Pulling before applying reduces cross-machine "apply thrash"

See [specs/sync.md](specs/sync.md) for full specification.

---

## Logging and Observability

Logging uses Go's `slog` package with dual output:

- **Stderr**: Human-readable text (verbose with `--verbose`)
- **JSON file**: Structured logs at `state/logs/dot.log`

Principles:
- Never log secrets or sensitive values
- Structured fields for machine parsing
- `--json` output mode planned for automation

---

## Testing Approach

- Interface-based design enables mocking external commands
- `MockRunner` allows testing git/chezmoi operations without real tools
- Unit tests for all core packages
- `make test` runs all tests with race detection

---

## Target Platforms

| Platform | Priority | Notes |
|----------|----------|-------|
| macOS (Apple Silicon) | Primary | Development target |
| Windows 11 Pro | Secondary | Native, not via WSL |
| CachyOS (Arch-family) | Secondary | |
| WSL (NixOS-WSL) | Optional | First-class but opt-in |

---

## Operational Recommendations

Habits to keep the system healthy:

- **Edit real files**: Change managed config by editing destination files directly
- **Sync immediately when needed**: Use `dot sync now` when you need changes on another machine right away
- **Be explicit about browser data**: Do not "discover" browser profile databases; use dedicated browser modules
- **Respect hardening profiles**: Treat "extreme hardening" as a separate opt-in profile with clear warnings
