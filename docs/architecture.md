# Architecture

`dotstate` is a repository plus a CLI that keeps machine configuration in sync across supported targets.

## Core Principles

1. Copy semantics over symlinks.
2. Secrets never committed to git.
3. Edit real destination files, then capture back.
4. Deterministic apply and safe sync.
5. Explicit privilege boundaries.

## Main Components

- `dot` CLI (`cmd/dot`, `internal/cli`): orchestration and UX.
- Chezmoi wrapper (`internal/chez`): apply, add, re-add, managed list.
- Git wrapper (`internal/gitx`): commit/pull/rebase/push operations.
- Discover engine (`internal/discover`): scan, classify, secret detection, prompting.
- Platform/config/logging (`internal/platform`, `internal/config`, `internal/logging`).

## Data Classes

- Managed state: tracked through Chezmoi source dir (`home/` by default).
- Generated private state: local-only under `state/private/`.
- Runtime state: logs/cache/temporary state under `state/`.

## Current Target Priority

1. macOS Tahoe on Apple Silicon (M4 Max).
2. Windows 11 on Zen4.
3. Linux and WSL follow-on.
