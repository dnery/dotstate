# dotstate

A cross-platform "OS state" repo + tool for managing dotfiles, app configurations, and system settings.

**Design goals**
- One portable entry point: a `dot` binary (Go).
- Copy-based config management (no symlink dependency) via Chezmoi.
- Safe secret handling: secrets come from 1Password (never committed to git).
- "Edit real files" workflow: you edit live config files; `dot sync` captures them back into the repo.
- Works across:
  - macOS (primary target, Apple Silicon)
  - Windows 11 (native)
  - Arch-family Linux (CachyOS)
  - WSL (NixOS-WSL) as a first-class target

## Current Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0 | Repo hygiene + docs | Complete |
| Phase 1 | Core plumbing (runners, config, logging) | Complete |
| Phase 2 | `dot discover` (baseline discovery) | Complete |
| Phase 3 | `dot discover` TUI (interactive selection) | Planned |
| Phase 4+ | Capture, Apply, Sync, Scheduling | Planned |

See [DOTSTATE-TECHNICAL-DESIGN.md](DOTSTATE-TECHNICAL-DESIGN.md) for the full roadmap.

## Installation

```sh
# Build from source
make build

# Or install directly
go install ./dot
```

### Prerequisites

- Go 1.23+
- Git
- [Chezmoi](https://www.chezmoi.io/) (will be auto-installed in future versions)
- 1Password CLI (`op`) - optional, for secret management

## Usage

### Check system status

```sh
dot doctor
```

Validates prerequisites and reports system info (OS, paths, tool availability).

### Discover existing configurations

```sh
# Interactive discovery (select files to add)
dot discover

# Auto-accept recommended files
dot discover --yes

# Preview only (don't add anything)
dot discover --report

# Include additional directories
dot discover --deep
```

The discover command scans your system for configuration files and helps you add them to your dotstate repo. Files are classified as:

- **Recommended**: High-confidence config files (preselected)
- **Maybe**: Potentially useful but unverified
- **Risky**: May contain secrets (requires explicit selection)
- **Ignored**: Caches, logs, browser data (excluded)

### Bootstrap a new machine

```sh
dot bootstrap --repo <git-url>
```

Clones your dotstate repo and sets up the environment.

### Apply configuration

```sh
dot apply
```

Applies the repo's desired state to your machine.

### Capture local changes

```sh
dot capture
```

Captures edits made to real files back into the repo.

### Sync with remote

```sh
dot sync
```

Full sync transaction: capture → commit → pull/rebase → apply → push.

## Repo Structure

```
.
├── dot.toml                      # dot configuration (repo-level)
├── home/                         # Chezmoi source directory (managed files)
│   ├── dot_config/...
│   ├── dot_gitconfig
│   └── ...
├── manifests/                    # Package manifests per OS (planned)
├── system/                       # System-level artifacts (planned)
├── state/                        # Local state (gitignored)
│   ├── private/                  # Secret material (generated locally)
│   ├── cache/
│   └── logs/
└── docs/
    ├── spec.md                   # System specification
    ├── discover.md               # Discover command spec
    └── ...
```

## Configuration

`dot.toml` at the repo root:

```toml
[repo]
url = "git@github.com:user/dotstate.git"
path = "~/.dotstate"
branch = "main"

[sync]
interval_minutes = 30
enable_idle = true
enable_shutdown = true

[tools]
# Empty means "use PATH"
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"
```

Environment variables override config:
- `DOTSTATE_REPO_URL`
- `DOTSTATE_REPO_PATH`
- `DOTSTATE_REPO_BRANCH`

## Architecture

### Core Packages

| Package | Purpose |
|---------|---------|
| `internal/runner` | Interface-based command execution with timeout support |
| `internal/config` | Configuration loading with defaults, validation, env overrides |
| `internal/logging` | Structured logging via slog (stderr + JSON file) |
| `internal/platform` | Cross-platform path handling and OS detection |
| `internal/errors` | Custom error types with exit codes |
| `internal/chez` | Chezmoi integration wrapper |
| `internal/gitx` | Git operations wrapper |
| `internal/discover` | Configuration discovery and classification |

### Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Generic error |
| 64 | Usage error (bad arguments) |
| 65 | Data error (invalid config) |
| 69 | Service unavailable |
| 75 | Conflict detected |
| 78 | Configuration error |

## Development

```sh
# Run tests
make test

# Run tests with coverage
make test-cover

# Lint code
make lint

# Scan for secrets
make secrets

# Build for all platforms
make build-all
```

## Secrets

Secrets are pulled from 1Password at apply-time.

- The repo stores only **references** to secrets (item/field identifiers)
- Secret detection uses both:
  - Built-in regex patterns (API keys, tokens, private keys)
  - Chezmoi's `--secrets=error` flag for additional protection
- Rendered secrets are written to `state/private/` (gitignored)

## Documentation

- [Technical Design & Roadmap](DOTSTATE-TECHNICAL-DESIGN.md)
- [System Specification](docs/spec.md)
- [Discover Command Spec](docs/discover.md)
- [Non-Trivial Tracking](docs/non-trivial-tracking.md)
- Platform Bootstrap Guides:
  - [macOS](docs/bootstrap-macos.md)
  - [Windows](docs/bootstrap-windows.md)
  - [Linux](docs/bootstrap-linux.md)

## License

Choose your own.
