# dotstate

Cross-platform OS state management for dotfiles, app configs, and machine setup.

## Current Focus

Primary support targets right now:
- macOS Tahoe (Apple Silicon, M4 Max)
- Windows 11 (Zen4)

## Quick Start

```sh
# Build local binary
make build-local

# Check environment
./bin/dot doctor

# Discover unmanaged config files
./bin/dot discover --report

# Run e2e discover/capture harness
make test-e2e
```

## Canonical Documentation

- [Documentation Index](docs/README.md)
- [Architecture](docs/architecture.md)
- [Roadmap](docs/roadmap.md)
- [CLI Reference](docs/reference/cli.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Discover Spec](docs/specs/discover.md)
- [Module and State Contract](docs/specs/modules.md)
- [macOS State Surfaces](docs/specs/macos-state-surfaces.md)

Legacy docs remain as compatibility pointers for one release cycle.

## Repository Layout

```text
.
├── cmd/                      # CLI entrypoint
├── internal/                 # core packages
├── home/                     # chezmoi source state
├── state/                    # runtime state, logs, captures
├── docs/                     # canonical documentation
└── test/e2e/                 # harnesses and reviewers' artifacts
```

## Development Commands

```sh
# unit/integration tests
make test

# docs consistency
make docs-check

# e2e harness (all scenarios, sandbox + summary bundle)
make test-e2e

# e2e harness capture-loop only
make test-e2e-capture

# e2e harness with asciinema recording (local cast)
make test-e2e-record
```
