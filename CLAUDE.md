# CLAUDE.md - AI Assistant Context for dotstate

This file provides context for AI assistants working on the dotstate project.

---

## Quick Start

```bash
# Build the project
make build-local

# Run tests
make test

# Run the E2E test harness
./test/e2e/test_harness.sh ./bin/dot
```

---

## Project Overview

**dotstate** is a cross-platform tool for managing dotfiles, app configurations, and system settings. It uses:
- **Go 1.23+** as the primary language
- **Chezmoi** as the file state engine (copy-based, not symlinks)
- **Git** for version control and sync
- **1Password** (optional) for secret management

### Design Philosophy

1. **Copy semantics** - Files are copied, not symlinked
2. **Secrets never in git** - Only references to 1Password items
3. **"Edit real files" workflow** - Edit configs normally; sync captures changes
4. **Idempotent apply** - Safe to run repeatedly
5. **Safe automation** - No silent conflict resolution

---

## Essential Documentation (Read Order)

1. **[docs/architecture.md](docs/architecture.md)** - System design and components
2. **[docs/reference/cli.md](docs/reference/cli.md)** - CLI commands, flags, exit codes
3. **[docs/reference/configuration.md](docs/reference/configuration.md)** - dot.toml schema
4. **[docs/roadmap.md](docs/roadmap.md)** - Development phases and status
5. **[docs/specs/discover.md](docs/specs/discover.md)** - Discovery specification

---

## Current Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0 | Repo hygiene + docs | Complete |
| Phase 1 | Core plumbing | Complete |
| Phase 2 | `dot discover` | Complete (with bugs) |
| Phase 3+ | TUI, Sync, Scheduling | Planned |

---

## Known Bugs

Track these issues when working on the codebase:

| Issue | Description | Severity |
|-------|-------------|----------|
| [#5](https://github.com/dnery/dotstate/issues/5) | chezmoi add uses wrong source path | Critical |
| [#6](https://github.com/dnery/dotstate/issues/6) | Shell dotfiles missing from discover | Medium |
| [#7](https://github.com/dnery/dotstate/issues/7) | Sub-repo manifest never persisted | Critical |
| [#8](https://github.com/dnery/dotstate/issues/8) | Selection index boundary error | Medium |
| [#9](https://github.com/dnery/dotstate/issues/9) | --secrets modes not implemented | Medium |
| [#10](https://github.com/dnery/dotstate/issues/10) | Docs scoring thresholds wrong | Low |

---

## Development Environment Setup

### Required Tools

```bash
# Go 1.23+
go version  # Should be 1.23 or higher

# Git
git --version

# Chezmoi (file state engine)
# Install via official binary:
curl -sL "https://github.com/twpayne/chezmoi/releases/download/v2.69.3/chezmoi-linux-amd64" \
  -o /usr/local/bin/chezmoi && chmod +x /usr/local/bin/chezmoi

# Or on macOS:
brew install chezmoi

# GitHub CLI (for issue management)
# On Ubuntu/Debian:
curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | \
  sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] \
  https://cli.github.com/packages stable main" | \
  sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
sudo apt update && sudo apt install gh

# Authenticate with GitHub
echo "$GITHUB_TOKEN" | gh auth login --with-token

# asciinema (for recording test runs)
pip3 install asciinema
```

### Build Commands

```bash
make build-local   # Build for current platform → ./bin/dot
make build         # Build for linux, darwin, windows
make test          # Run tests with race detection
make test-cover    # Generate coverage report
make lint          # Run golangci-lint
make secrets       # Scan for secrets with gitleaks
```

---

## Testing

### Unit Tests

```bash
make test          # All tests
make test-v        # Verbose output
make test-cover    # With coverage
```

### E2E Test Harness

The E2E test harness creates a sandbox environment to test discover functionality:

```bash
./test/e2e/test_harness.sh ./bin/dot
```

**What it tests:**
1. Creates mock home with config files
2. Initializes dotstate repo
3. Runs `dot discover --yes`
4. Verifies files are copied (not symlinked)
5. Tests change detection via `dot capture`

**Known issues the harness exposes:**
- Files added to wrong chezmoi source path (#5)
- Shell dotfiles not discovered (#6)

### Recording Tests with asciinema

```bash
# Record a test run
asciinema rec --command "./test/e2e/test_harness.sh ./bin/dot" test.cast

# Upload to asciinema.org
asciinema upload test.cast
```

---

## Code Structure

```
├── cmd/dot/main.go           # Entry point
├── internal/
│   ├── cli/root.go           # CLI commands (478 lines)
│   ├── config/config.go      # Configuration loading
│   ├── platform/platform.go  # OS detection, paths
│   ├── runner/runner.go      # Command execution interface
│   ├── errors/errors.go      # Error types, exit codes
│   ├── logging/logging.go    # Structured logging (slog)
│   ├── chez/chez.go          # Chezmoi wrapper
│   ├── gitx/gitx.go          # Git operations
│   ├── discover/             # Discovery system
│   │   ├── scanner.go        # Filesystem scanning
│   │   ├── classifier.go     # File classification
│   │   ├── secrets.go        # Secret pattern detection
│   │   ├── subrepo.go        # Sub-repo detection
│   │   ├── prompt.go         # Interactive selection
│   │   └── discoverer.go     # Orchestration
│   └── sync/sync.go          # Sync transaction
├── docs/                     # Documentation
│   ├── architecture.md
│   ├── roadmap.md
│   ├── reference/            # CLI and config reference
│   ├── specs/                # Feature specifications
│   └── guides/               # Bootstrap guides
└── test/e2e/                 # E2E test harness
```

---

## Key Interfaces

### Runner (Command Execution)

```go
type Runner interface {
    Run(ctx context.Context, workDir string, name string, args ...string) (*CmdResult, error)
}
```

All external commands go through this interface, enabling mocking in tests.

### Configuration

```go
type Config struct {
    Repo  RepoConfig   // url, path, branch
    Sync  SyncConfig   // interval, idle, shutdown
    Tools ToolsConfig  // git, chezmoi, op paths
    Chex  ChexConfig   // source_dir
    WSL   WSLConfig    // enable, distro_name, flake_ref
}
```

Loaded from `dot.toml`, with env var overrides (`DOTSTATE_REPO_*`).

---

## Common Tasks

### Adding a New CLI Command

1. Add command function in `internal/cli/root.go`
2. Register in `init()` with `rootCmd.AddCommand()`
3. Add flags with validation
4. Update `docs/reference/cli.md`

### Adding a Secret Pattern

1. Edit `internal/discover/secrets.go`
2. Add pattern to `defaultPatterns` slice
3. Consider false positive rate
4. Test against real configs

### Fixing the chezmoi Source Path Bug (#5)

The fix is in `internal/chez/chez.go`:

```go
func (c *Chezmoi) Add(ctx context.Context, repoPath, sourceDir string, files []string, secretsError bool) error {
    args := []string{"add"}
    if sourceDir != "" {
        args = append(args, "--source", filepath.Join(repoPath, sourceDir))
    }
    // ... rest of function
}
```

Then update callers in `internal/discover/discoverer.go`.

---

## Git Workflow

- Main branch: `master`
- Feature branches: `claude/<description>-<id>` for AI-assisted work
- Commit signing may be disabled in test environments:
  ```bash
  git config commit.gpgsign false
  ```

---

## Troubleshooting

### "chezmoi add" puts files in wrong location

This is bug #5. Chezmoi uses `~/.local/share/chezmoi` by default instead of the repo's `home/` directory. The `--source` flag must be passed.

### Shell dotfiles not discovered

This is bug #6. The scanner adds dotfiles to roots but they may be classified as Ignored. Check the classifier scoring.

### Git commit signing fails in tests

Disable signing for test repos:
```bash
git config commit.gpgsign false
git config tag.gpgsign false
```

### asciinema upload fails with validation error

The env field may contain null values. Fix with:
```bash
head -1 recording.cast | python3 -c "
import json, sys
data = json.loads(sys.stdin.read())
data['env'] = {k: str(v) if v is not None else '' for k, v in data['env'].items()}
print(json.dumps(data))
" > fixed_header.json
tail -n +2 recording.cast > rest.cast
cat fixed_header.json rest.cast > fixed.cast
asciinema upload fixed.cast
```

---

## Contact

Repository: https://github.com/dnery/dotstate
