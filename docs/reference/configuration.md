# Configuration Reference

This file is the canonical `dot.toml` contract.

## Location

`dot.toml` at repository root.

## Schema

```toml
[repo]
url = "https://github.com/dnery/dotstate"
path = "~/Projects/dotstate"
branch = "master"

[sync]
interval_minutes = 30
enable_idle = true
enable_shutdown = true

[tools]
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"

[wsl]
enable = true
distro_name = "nixos"
flake_ref = ".#wsl"
```

## Sections

### `[sync]`

- `interval_minutes`: cadence used by `dot schedule install` when rendering the macOS LaunchAgent. `30` means launchd `StartInterval = 1800` seconds.
- `enable_idle`: retained for future platform-specific idle scheduling. macOS user LaunchAgent idle integration is not implemented yet.
- `enable_shutdown`: retained for future platform-specific shutdown behavior. macOS intentionally does not install a shutdown hook; use `dot sync now` for explicit manual flushes.

## Environment Overrides

- `DOTSTATE_REPO_URL`
- `DOTSTATE_REPO_PATH`
- `DOTSTATE_REPO_BRANCH`

## Resolution Order

1. Built-in defaults.
2. `dot.toml` values.
3. Environment overrides.
