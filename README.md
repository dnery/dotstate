# dotstate

A cross-platform tool for managing dotfiles, app configurations, and system settings.

## What it does

- Maintains a single GitHub repository as the source of truth for your OS state
- Applies configuration consistently across macOS, Windows, and Linux
- Captures live edits automatically back into the repository
- Handles secrets safely via 1Password (never committed to git)

## Quick Start

```bash
# Build from source
make build

# Check system status
./bin/dot doctor

# Discover existing configs
./bin/dot discover

# Apply configuration
./bin/dot apply

# Sync with remote
./bin/dot sync
```

## Prerequisites

- Go 1.23+
- Git
- [Chezmoi](https://www.chezmoi.io/)
- 1Password CLI (`op`) - optional, for secret management

## Current Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0 | Repo hygiene + docs | Complete |
| Phase 1 | Core plumbing | Complete |
| Phase 2 | `dot discover` | Complete |
| Phase 3+ | TUI, Sync, Scheduling | Planned |

## Supported Platforms

- **macOS** (Apple Silicon) - primary target
- **Windows 11** (native)
- **Linux** (CachyOS / Arch-family)
- **WSL** (NixOS-WSL) - optional

## Documentation

- **[Architecture](docs/architecture.md)** - How dotstate works
- **[CLI Reference](docs/reference/cli.md)** - Commands, flags, exit codes
- **[Configuration](docs/reference/configuration.md)** - `dot.toml` schema

### Specifications

- [Discover](docs/specs/discover.md) - Configuration discovery
- [Sync](docs/specs/sync.md) - Sync transaction
- [Scheduling](docs/specs/scheduling.md) - Automation
- [Secrets](docs/specs/secrets.md) - 1Password integration

### Bootstrap Guides

- [macOS](docs/guides/bootstrap-macos.md)
- [Windows](docs/guides/bootstrap-windows.md)
- [Linux](docs/guides/bootstrap-linux.md)
- [WSL](docs/guides/wsl-nixos.md)

See **[docs/roadmap.md](docs/roadmap.md)** for the full development plan.

## Development

```bash
make test        # Run tests
make test-cover  # Coverage report
make lint        # Lint code
make secrets     # Scan for secrets
```

## License

Choose your own.
