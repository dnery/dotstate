# dotstate (skeleton)

This is a starter skeleton for a cross-platform "OS state" repo + tool.

**Design goals**
- One portable entry point: a `dot` binary (Go).
- Copy-based config management (no symlink dependency).
- Safe secret handling: secrets come from 1Password (never committed to git).
- "Edit real files" workflow: you edit live config files; a background sync captures them back into the repo.
- Works across:
  - Windows 11 (native)
  - macOS (latest stable)
  - Arch-family Linux (CachyOS)
  - WSL (NixOS-WSL) as a first-class target

**Status**
This repository is a scaffold. The Go tool compiles and wires up the core commands,
but OS modules are intentionally stubs.

## Quickstart (conceptual)

1) Clone your repo to a known location (default: `~/.dotstate`), or run:

```sh
dot bootstrap --repo <git-url>
```

2) Apply desired state:

```sh
dot apply
```

3) Edit real files normally (e.g. `~/.gitconfig`), then sync:

```sh
dot sync now
```

The long-term goal is to install an automated schedule:
- every 30 minutes
- on idle
- on shutdown / restart (best-effort)

## Repo structure (proposed)

- `home/`:
  Chezmoi source state (copy-based). This is where dotfiles/configs live.
- `state/`:
  Machine-captured exports (packages lists, registry exports, macOS defaults dumps, etc.)
- `modules/`:
  OS-specific capture/apply logic.
- `docs/`:
  Bootstrap checkpoints and manual steps.

## Secrets

Secrets are pulled from 1Password at apply-time.

- The repo stores only **references** to secrets (e.g. item/field identifiers).
- Rendered secret-bearing outputs should be written to `state/private/` and gitignored.

## License

Choose your own.
