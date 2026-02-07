# CLI Reference

This is the authoritative reference for the `dot` command-line interface.

---

## Commands

### `dot version`

Shows version information including commit hash, build date, Go version, and platform.

```bash
dot version
```

### `dot doctor`

Validates dependencies and environment, providing actionable errors.

**Checks performed:**
- Configuration file (`dot.toml`) presence and validity
- Tool presence: `git`, `chezmoi`, `op` (optional)
- Repository state (remote configured, branch exists)

**Planned checks (Windows):**
- Activation status
- winget availability

```bash
dot doctor
```

### `dot bootstrap`

Bootstraps a machine by cloning the dotstate repository.

**Flags:**
- `--repo <url>` (required) - Git URL of the dotstate repository
- `--branch <name>` - Branch to clone (default: `main`)
- `--path <path>` - Local path for the repository

```bash
dot bootstrap --repo https://github.com/user/dotstate
dot bootstrap --repo git@github.com:user/dotstate.git --branch dev
```

### `dot apply`

Applies desired state from the repository to the machine.

```bash
dot apply
```

Runs `chezmoi apply` against the configured source directory.

### `dot capture`

Captures local edits back into the repository source state.

```bash
dot capture
```

Runs `chezmoi re-add` to update the repository with changes made to destination files.

### `dot sync`

Runs the full convergence transaction: capture, commit, pull/rebase, apply, push.

**Flags:**
- `--no-apply` - Skip the apply step
- `--no-push` - Skip the push step

**Subcommands:**
- `dot sync now` - Alias for `dot sync` (for "I need this on another machine right away")

```bash
dot sync
dot sync --no-push
dot sync now
```

### `dot discover`

Scans the system for configuration files not yet tracked in the repository.

**Flags:**
- `--yes, -y` - Auto-accept recommended files without prompting
- `--dry-run` - Show what would be added without adding
- `--deep` - Scan additional directories (Library, AppData)
- `--report` - Print classification report only (no prompts)
- `--no-commit` - Skip commit after adding files
- `--secrets {error|warning|ignore}` - Secret handling mode (default: `error`)

**Classification categories:**
- **Recommended** - High-confidence config files (preselected)
- **Maybe** - Potentially useful but uncertain
- **Risky** - May contain secrets (requires explicit selection)
- **Ignored** - Caches, logs, browser data (excluded)

```bash
dot discover                    # Interactive mode
dot discover --yes              # Auto-accept recommended
dot discover --report           # Report only, no changes
dot discover --deep --dry-run   # Deep scan preview
```

Full specification: [specs/discover.md](../specs/discover.md)

---

## Planned Commands

These commands are designed but not yet implemented:

- `dot schedule install|status|remove` - Manage automated sync scheduling
- `dot wsl install|apply|status` - WSL/NixOS-WSL management
- `dot packages capture|apply` - Package manifest management
- `dot subrepo status` - Check status of tracked sub-repositories
- `dot windows harden apply|audit` - Windows hardening profiles

---

## Global Flags

- `--verbose, -v` - Enable verbose output
- `--help, -h` - Show help for any command

---

## Exit Codes

Following [sysexits.h](https://man.freebsd.org/cgi/man.cgi?query=sysexits) conventions:

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | `ExitOK` | Success |
| 1 | `ExitError` | Generic error |
| 64 | `ExitUsage` | Bad command-line arguments |
| 65 | `ExitDataErr` | Invalid input data |
| 69 | `ExitUnavailable` | Service/dependency unavailable |
| 75 | `ExitConflict` | Conflict detected (git/chezmoi) |
| 78 | `ExitConfig` | Configuration error |

All commands are scriptable and return appropriate exit codes for automation.
