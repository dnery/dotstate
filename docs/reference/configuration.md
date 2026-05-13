# Configuration Reference

This file is the canonical `dot.toml` contract.

## Location

`dot.toml` at repository root.

## Config Discovery

`dot` resolves the active config file in this order:

1. `--config <path>`
2. `DOTSTATE_CONFIG=<path>`
3. Walk upward from the current directory looking for `dot.toml`
4. `DOTSTATE_REPO_PATH/dot.toml`

That means either of these lets you run `dot` from outside the repo:

```sh
export DOTSTATE_CONFIG="$HOME/Projects/dotstate/dot.toml"
# or
export DOTSTATE_REPO_PATH="$HOME/Projects/dotstate"
```

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

## Environment Variables

Config discovery:

- `DOTSTATE_CONFIG`

Config value overrides:

- `DOTSTATE_REPO_URL`
- `DOTSTATE_REPO_PATH`
- `DOTSTATE_REPO_BRANCH`

## Config Value Resolution Order

1. Built-in defaults.
2. `dot.toml` values.
3. Environment overrides.
