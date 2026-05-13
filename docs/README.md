# Documentation

This directory is the canonical documentation set for `dotstate`.

## Quick Start

- [Architecture](architecture.md)
- [Roadmap](roadmap.md)
- [CLI Reference](reference/cli.md)
- [Configuration Reference](reference/configuration.md)

## Specs

- [Discover](specs/discover.md)
- [Module and State Contract](specs/modules.md)
- [macOS State Surfaces](specs/macos-state-surfaces.md)
- [Sync](specs/sync.md)
- [Scheduling](specs/scheduling.md)
- [Secrets](specs/secrets.md)
- [Advanced Tracking](specs/advanced-tracking.md)

## Platform Guides

- [Bootstrap macOS](guides/bootstrap-macos.md)
- [Bootstrap Windows](guides/bootstrap-windows.md)
- [Bootstrap Linux](guides/bootstrap-linux.md)
- [WSL NixOS](guides/wsl-nixos.md)

## Verification

- [Platform Verification Matrix](verification/matrix.md)

## Source Of Truth Rules

1. CLI contracts live in `reference/cli.md`.
2. `dot.toml` schema and overrides live in `reference/configuration.md`.
3. Feature behavior lives in `specs/*.md`.
4. Platform setup lives in `guides/*.md`.
5. Roadmap and implementation status live in `roadmap.md`.

Legacy files in `docs/*.md` remain as compatibility pointers for one release cycle.
