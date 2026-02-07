# Development Roadmap

This document tracks the development phases, their status, and open decisions.

**Last updated:** 2026-02-01

---

## Implementation Status

| Phase | Description | Status |
|-------|-------------|--------|
| Phase 0 | Repo hygiene + docs | Complete |
| Phase 1 | Core plumbing | Complete |
| Phase 2 | `dot discover` (baseline) | Complete |
| Phase 3 | `dot discover` TUI | Planned |
| Phase 4 | Capture & Apply MVP | Planned |
| Phase 5 | `dot sync` (safe transaction) | Planned |
| Phase 6 | Scheduling automation | Planned |
| Phase 7 | 1Password integration MVP | Planned |
| Phase 8 | Windows dev UX | Planned |
| Phase 9 | WSL integration | Planned |
| Phase 10 | Package manifests | Planned |
| Phase 11 | Windows hardening | Planned |
| Phase 12 | Browser modules | Planned |
| Phase 13 | Notifications | Planned |
| Phase 14 | CI/CD and releases | Planned |

---

## Completed Phases

### Phase 0 - Repo Hygiene + Docs

**Deliverables:**
- Technical design documentation in repo
- Spec index established
- `dot doctor` command for basic validation

### Phase 1 - Core Plumbing

**Implemented:**
- `internal/runner`: Interface-based command execution with `Runner` interface and `ExecRunner` implementation
- `internal/testutil`: Test helpers including `MockRunner` for unit testing
- `internal/config`: Configuration loading from `dot.toml` with defaults, validation, env overrides
- `internal/logging`: Structured logging via `slog` (stderr + JSON file)
- `internal/platform`: Cross-platform OS detection and XDG-compliant paths
- `internal/errors`: Custom error types with sysexits.h exit codes
- `internal/chez`: Chezmoi wrapper using Runner interface
- `internal/gitx`: Git operations wrapper
- `.gitleaks.toml`: Secret scanning configuration
- `Makefile`: Enhanced with test, lint, secrets targets

### Phase 2 - `dot discover` (Baseline)

**Implemented:**
- `internal/discover/scanner.go`: Scans common config locations
- `internal/discover/classifier.go`: Classifies files as Recommended/Maybe/Risky/Ignored
- `internal/discover/secrets.go`: 30+ regex patterns for secret detection
- `internal/discover/subrepo.go`: Detects nested git repositories
- `internal/discover/prompt.go`: Interactive selection with `--yes` flag
- `internal/discover/discoverer.go`: Main orchestrator

**CLI flags:** `--yes`, `--dry-run`, `--deep`, `--report`, `--no-commit`, `--secrets`

**Key decisions made:**
- Sub-repos tracked as references (URL + branch) in `state/subrepos.toml`
- Secret detection uses regex pre-scan AND `chezmoi add --secrets=error`
- Interactive by default, `--yes` for automation

---

## Planned Phases

### Phase 3 - `dot discover` TUI

**Goal:** Interactive Bubble Tea UI for file selection

**Scope:**
- Multi-select with search
- Category groups (Recommended/Maybe/Risky)
- Preview small text files
- Confirm -> add -> commit flow

**Acceptance criteria:**
- UX is fast and not noisy
- No accidental browser DB tracking

### Phase 4 - Capture & Apply MVP

**Goal:** Basic file synchronization working

**Scope:**
- `dot capture`: `chezmoi re-add` + optional `--commit`
- `dot apply`: `chezmoi apply` + module hook placeholders

**Acceptance criteria:**
- Edit real file -> `dot capture` -> change visible in repo
- Clone repo -> `dot apply` -> files deployed

### Phase 5 - `dot sync` (Safe Transaction)

**Goal:** Full sync transaction with conflict handling

**Scope:**
- Transaction steps: capture -> commit -> pull/rebase -> apply -> push
- Safe push policy (fast-forward only)
- Conflict detection and clear status

**Acceptance criteria:**
- Two machines editing different files: syncs cleanly
- Same file edited on both: stops with conflict status

### Phase 6 - Scheduling Automation

**Goal:** Automated background sync

**Scope:**
- Windows: Task Scheduler (interval, idle, shutdown)
- macOS: launchd agent
- Linux: systemd user timer
- `dot schedule install|status|remove` commands

**Acceptance criteria:**
- Tasks/timers install and run correctly
- Shutdown task does not block for long

### Phase 7 - 1Password Integration MVP

**Goal:** Secret retrieval at apply-time

**Scope:**
- `dot doctor` checks 1Password integration
- `dot apply` supports secret retrieval for templates
- Caching of `op` calls per run
- Manual checkpoint messaging

**Acceptance criteria:**
- First run prompts for unlock
- Subsequent runs are fast
- No secrets leak into logs

### Phase 8 - Windows Developer UX

**Goal:** PowerShell 7 + Windows Terminal setup

**Scope:**
- Install PS7 and Windows Terminal
- Configure Terminal default profile
- Track Terminal settings in repo

### Phase 9 - WSL Integration

**Goal:** NixOS-WSL as managed target

**Scope:**
- `dot wsl install` (guided + checkpointed)
- `dot wsl apply` for flake configuration
- 1Password SSH bridging documentation

### Phase 10 - Package Manifests

**Goal:** Portable package lists

**Scope:**
- `dot packages capture` writes manifests
- `dot packages apply` installs from manifests
- Integrate into `dot apply` optionally

**Per-OS:**
- Windows: winget JSON
- macOS: Brewfile
- Linux: pacman explicit + foreign lists

### Phase 11 - Windows Hardening

**Goal:** Reproducible security baselines

**Scope:**
- `dot windows harden apply --profile gaming-safe`
- `dot windows harden audit`
- Registry + policy bundles
- Two profiles: gaming-safe (default), extreme (opt-in)

### Phase 12 - Browser Modules (Firefox-first)

**Goal:** Track browser customizations

**Scope:**
- Firefox profile detection
- Manage `user.js` and `chrome/`
- Extension export/import workflows
- Sidebery snapshot strategy

### Phase 13 - Notifications

**Goal:** Alert on conflicts or manual checkpoints

**Scope:**
- Webhook notifications (Pushcut/Telegram)
- Message format and rate limiting

### Phase 14 - CI/CD and Releases

**Goal:** Automated builds and distribution

**Scope:**
- Build per OS target
- GitHub Releases publishing
- Bootstrap scripts download correct binary

---

## Open Decisions

### Notifications Channel (Phase 13)
- Pushcut vs Telegram vs both
- Message format, rate limiting, secrets handling

### Windows Hardening Policies (Phase 11)
- Need curated, versioned policy/registry set
- Must be tested against gaming requirements

### Browser Extension Automation (Phase 12)
For each extension (uBO, Sidebery, Stylus, Violentmonkey, SponsorBlock):
- Determine best export/import format
- Determine which settings are reliably restorable
- Decide what stays manual

### NixOS-WSL Flake Design (Phase 9)
- Flake inside dotstate repo or separate repo/submodule
- WSL dotfiles shared with CachyOS or separate variants

### CachyOS Package Capture (Phase 10)
- AUR package manifest format
- Which AUR helper to assume

---

## Decided Items

### Secrets Boundary Enforcement
**Decision:** Use BOTH pre-commit scanning AND runtime detection:
1. `.gitleaks.toml` for pre-commit/CI scanning
2. Built-in regex patterns in `internal/discover/secrets.go`
3. `chezmoi add --secrets=error` as final guardrail

### Chezmoi Integration
**Decision:** Currently user-managed installation; auto-install planned.
- `dot doctor` checks for chezmoi presence
- Future: `dot bootstrap` installs if missing

### Sub-Repository Handling
**Decision:** Track nested git repos as references, not files.
- Manifest in `state/subrepos.toml`
- On apply: clone if missing, skip if present

### Platform Priority
**Decision:** macOS first -> Windows 11 -> CachyOS/Arch Linux
- Development and testing prioritizes macOS (Apple Silicon)

---

## Maintenance Notes

Update this document when:
- A phase completes (move to "Completed" section with details)
- An open decision is resolved (move to "Decided" section)
- New phases are added or existing phases are rescoped
