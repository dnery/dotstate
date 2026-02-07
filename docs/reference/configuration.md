# Configuration Reference

This is the authoritative reference for dotstate configuration.

---

## Configuration File

Configuration is stored in `dot.toml` at the repository root.

### Full Schema

```toml
[repo]
url = "git@github.com:user/dotstate.git"   # Remote git URL
path = "~/.dotstate"                        # Local clone path
branch = "main"                             # Branch to track

[sync]
interval_minutes = 30                       # Sync interval (default: 30)
enable_idle = true                          # Sync on idle (default: true)
enable_shutdown = true                      # Capture on shutdown (default: true)

[tools]
git = ""                                    # Path to git (empty = use PATH)
chezmoi = ""                                # Path to chezmoi (empty = use PATH)
op = ""                                     # Path to 1Password CLI (empty = use PATH)

[chex]
source_dir = "home"                         # Chezmoi source directory in repo

[wsl]
enable = true                               # Enable WSL integration
distro_name = "nixos"                       # WSL distribution name
flake_ref = ".#wsl"                         # Flake reference for NixOS-WSL
```

### Section Details

#### `[repo]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `url` | string | - | Git remote URL (SSH or HTTPS) |
| `path` | string | `~/.dotstate` | Local repository path (supports `~` expansion) |
| `branch` | string | `main` | Branch to clone and sync |

#### `[sync]`

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `interval_minutes` | integer | `30` | Minutes between automatic syncs |
| `enable_idle` | boolean | `true` | Run sync when system becomes idle |
| `enable_shutdown` | boolean | `true` | Run capture on shutdown (best-effort) |

#### `[tools]`

Explicit paths to tools. Empty string means "find in PATH".

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `git` | string | `""` | Path to git executable |
| `chezmoi` | string | `""` | Path to chezmoi executable |
| `op` | string | `""` | Path to 1Password CLI |

#### `[chex]`

Chezmoi integration settings.

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `source_dir` | string | `home` | Directory inside repo used as Chezmoi source |

The repo should include a `.chezmoiroot` file pointing to this directory so Chezmoi can run from the repo root.

#### `[wsl]`

Windows Subsystem for Linux integration (Windows only).

| Key | Type | Default | Description |
|-----|------|---------|-------------|
| `enable` | boolean | `false` | Enable WSL target management |
| `distro_name` | string | `nixos` | WSL distribution name |
| `flake_ref` | string | `.#wsl` | NixOS flake reference |

---

## Environment Variables

Environment variables override configuration file values.

| Variable | Overrides | Example |
|----------|-----------|---------|
| `DOTSTATE_REPO_URL` | `repo.url` | `git@github.com:user/dotstate.git` |
| `DOTSTATE_REPO_PATH` | `repo.path` | `~/Projects/dotstate` |
| `DOTSTATE_REPO_BRANCH` | `repo.branch` | `dev` |

### Precedence Order

1. Default values (built-in)
2. `dot.toml` configuration file
3. Environment variables (highest priority)

---

## Configuration Loading

The configuration system:

1. Loads platform-specific defaults (XDG paths on Linux, `~/Library` on macOS, `%APPDATA%` on Windows)
2. Reads `dot.toml` if present in the repository root
3. Applies environment variable overrides
4. Expands `~` and `$ENV_VAR` in paths
5. Validates required fields

---

## Repository Structure

The configuration assumes this repository layout:

```
.
├── dot.toml                      # Configuration file
├── home/                         # Chezmoi source directory (managed files)
│   ├── dot_config/...
│   ├── dot_gitconfig
│   └── ...
├── manifests/                    # Package manifests per OS (planned)
│   ├── windows/winget.json
│   ├── macos/Brewfile
│   └── linux/pacman-explicit.txt
├── system/                       # System-level artifacts (planned)
│   ├── windows/registry/*.reg
│   ├── macos/defaults/*.sh
│   └── linux/sudoers.d/...
├── state/                        # Local state (gitignored)
│   ├── private/                  # Secret material (generated)
│   ├── cache/
│   ├── logs/
│   └── subrepos.toml            # Sub-repository references
└── docs/                         # Documentation
```

### Gitignore Policy

The following must be gitignored:
- `state/**` - All local state
- Generated secret-bearing outputs
- `state/private/**` - Secret material

---

## Platform-Specific Defaults

### macOS

```toml
[repo]
path = "~/.dotstate"
```

Config roots for discovery: `~/.config`, shell configs in `$HOME`

### Windows

```toml
[repo]
path = "%USERPROFILE%\\.dotstate"
```

Config roots for discovery: `%APPDATA%`, curated app folders

### Linux

```toml
[repo]
path = "~/.dotstate"
```

Config roots for discovery: `~/.config` (XDG standard), shell configs
