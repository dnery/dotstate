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

## Environment Overrides

- `DOTSTATE_REPO_URL`
- `DOTSTATE_REPO_PATH`
- `DOTSTATE_REPO_BRANCH`

## Resolution Order

1. Built-in defaults.
2. `dot.toml` values.
3. Environment overrides.
