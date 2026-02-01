# dotstate technical design and roadmap (source of truth)

**Status:** Living document (update as implementation evolves)
**Last updated:** 2026-02-01
**Audience:** You (developer/operator) working primarily from the terminal
**Repo:** `dnery/dotstate` (this doc should live at `docs/technical-design.md` or similar)

> **Implementation Status:** Phase 0, 1, and 2 are complete. The CLI has core plumbing (runner, config, logging, platform, errors) and the discover command for baseline onboarding.

---

## 0. How to use this document

This document is the *one place* that defines:

- what the system is (architecture + invariants),
- what the CLI must do (contracts),
- what is already decided vs still open,
- and the ordered development plan.

It intentionally links to deeper per-feature specs (e.g., `docs/discover.md`). Those specs are normative for their feature; this document is normative for **system-wide** behavior and priorities.

**Sections marked “EDIT AS YOU GO”** are expected to change during implementation; keep them current.

---

## 1. Problem statement

Maintain a single, GitHub-hosted repository that can:

- reproduce a **desired state** across:
  - Windows 11 Pro (latest stable),
  - macOS (latest stable) on Apple Silicon,
  - CachyOS (latest stable; Arch-family),
  - and optionally WSL (NixOS-WSL preferred) on Windows,
- and also **capture changes** made *directly to live files/settings* on any machine, pushing them back into the repo.

The system must be:

- **portable + compatible**: minimal dependencies, cross-platform entry point,
- **copy-based**: do not rely on symlinks for core dotfiles/config management,
- **Git-first**: everything trackable in a repo; secrets never exposed,
- **bootstrappable**: 1–2 commands on a fresh machine to apply desired state (plus clearly documented manual checkpoints when unavoidable),
- **incremental**: can adopt an already-customized machine via `discover` + selective tracking.

---

## 2. Non-negotiable constraints and invariants

### 2.1 Copy semantics
- Managed files are applied by **copying** into their destination paths.
- No requirement that destination supports symlinks.

### 2.2 GitHub-safe secret handling
- **No secret material is ever committed**.
- Secrets are retrieved at apply-time from **1Password**.
- The repo stores only **secret references** (item IDs/fields) and/or templates that call `op`.

### 2.3 “Edit real files” workflow
- You edit `~/.gitconfig`, Zed settings, etc. *in place*.
- A background process periodically captures those edits back into the repo **without requiring a special edit command**.

### 2.4 Deterministic apply
- `dot apply` should be idempotent and converge the machine to the repo’s desired state.
- `dot sync` should be a safe transaction (see §7).

### 2.5 Safe automation
- Automation must **not** attempt heroic conflict resolution.
- On divergence/conflicts, automation stops and surfaces the problem.

### 2.6 Explicit privilege boundaries
- Operations requiring admin/sudo must be isolated and obvious.
- Windows: UAC stays on; elevation happens only when a module requires it.

---

## 3. High-level architecture

### 3.1 Components

1) **`dot` CLI (Go, per-OS builds)**
- Primary entry point.
- Orchestrates:
  - Chezmoi actions (file state engine),
  - Git operations,
  - OS-specific modules (packages/settings),
  - Scheduling/automation,
  - 1Password integration.

2) **Chezmoi (file state engine)**
- Manages copy-based dotfile/config application.
- Provides:
  - `unmanaged` for discovery,
  - `add/re-add` for capturing live edits into source state,
  - `apply` to update destination files.

3) **Git**
- Repo is canonical; remote on GitHub.
- `dot sync` coordinates pull/rebase/apply/push.

4) **1Password**
- **SSH agent** for Git auth.
- **CLI (`op`)** for secret material retrieval.
- Secret material is written only to local, gitignored locations or to destination files generated from templates.

5) **OS tooling (modules)**
- Windows: `winget`, `reg`, optional LGPO tooling, scheduled tasks, Windows Terminal settings.
- macOS: `brew bundle`, `defaults`, launchd.
- Linux: pacman package lists, sudoers drop-ins, systemd user units.
- WSL: NixOS-WSL setup + flake apply.

### 3.2 Data model: managed vs generated vs local state

- **Managed state:** tracked by Chezmoi under the repo’s Chezmoi source directory (default `home/`).
- **Generated/private state:** produced locally at apply-time (often secret-bearing), stored in gitignored paths (e.g. `state/private/`), or written directly to destination.
- **Local runtime state:** caches, logs, locks, timestamps (gitignored).

---

## 4. Repository layout (EDIT AS YOU GO)

This layout supports both Chezmoi and non-Chezmoi assets.

```
.
├─ dot.toml                      # dot configuration (repo-level)
├─ home/                         # Chezmoi source directory (managed files)
│  ├─ dot_config/...
│  ├─ dot_gitconfig
│  └─ ... (Windows/macOS/Linux variants; see §5)
├─ manifests/                    # package manifests per OS
│  ├─ windows/winget.json
│  ├─ macos/Brewfile
│  ├─ linux/pacman-explicit.txt
│  └─ linux/pacman-foreign.txt
├─ system/                       # system-level desired state artifacts
│  ├─ windows/registry/*.reg
│  ├─ windows/policy/...
│  ├─ macos/defaults/*.sh
│  └─ linux/sudoers.d/...
├─ modules/                      # dot modules (apply/capture/audit scripts or data)
│  ├─ windows/
│  ├─ macos/
│  ├─ linux/
│  └─ browser/
├─ state/                        # local state (gitignored)
│  ├─ private/                   # secret material (generated locally)
│  ├─ cache/
│  └─ logs/
└─ docs/
   ├─ technical-design.md        # this document
   ├─ discover.md                # detailed discover spec
   ├─ sync.md                    # detailed sync spec
   ├─ scheduling.md              # detailed scheduling spec
   └─ ...
```

### 4.1 Chezmoi source directory selection
- Default: `home/` is the Chezmoi source directory.
- The repo should include `.chezmoiroot` pointing at `home/` so Chezmoi can run from repo root.

### 4.2 Git ignore policy
- `state/**` must be ignored.
- Any generated secret-bearing output paths must be ignored.

---

## 5. Cross-platform file strategy

### 5.1 Avoid templates unless necessary
To preserve `re-add` behavior (capture live edits), OS divergence should be handled using:

- multiple OS-specific **plain** files in the source state,
- plus a templated `.chezmoiignore` that selects which file applies on which OS.

Templates are reserved for:
- secret-bearing files generated via 1Password,
- small glue scripts where live-edit capture is not desired.

### 5.2 Managed file classes

1) **Plain managed files** (preferred)
- Safe for `re-add` capture loop.
- Examples: `.gitconfig`, editor settings, shell rc files.

2) **Templated managed files** (use sparingly)
- Used when file content depends on secrets or OS-only logic.
- Capture loop must not overwrite templates.

3) **Generated outputs (unmanaged)**
- Files created locally at apply-time and not tracked.
- Used for secret material and machine identity.

### 5.3 “Managed vs generated” rule of thumb
- If a file may include secrets: **generate** or **template** it.
- If it is safe and you want live-edit capture: **plain managed**.

---

## 6. Secrets and identity (1Password)

### 6.1 Source of truth
- 1Password is the only source of truth for secret material.
- The repo stores only:
  - secret references (vault/item/field),
  - templates that call `op` to fetch.

### 6.2 Authentication
- Git auth uses 1Password SSH agent (no private keys committed or placed on disk).
- `op` CLI should use desktop integration where available, with an explicit manual checkpoint during bootstrap:
  - “Unlock / sign in to 1Password, then continue.”

### 6.3 Performance considerations
- `op` calls may be slow on stable releases in some workflows.
- Provide a configurable option to install/use `op` beta if desired.
- Always cache secret reads within a single `dot apply` run.
- Never call `op` in the 30-minute background capture path unless required.

### 6.4 Local secret material handling
- Generated secrets must be written only to:
  - destination files expected to contain secrets (e.g., tokens in app config), or
  - `state/private/` (gitignored).
- Never print secret values to stdout/stderr.

---

## 7. Sync engine (core transaction)

### 7.1 Definitions
- **Capture:** update Chezmoi source state from destination files.
- **Apply:** update destination files from source state.
- **Sync:** capture + commit + pull/rebase + apply + push (safe).

### 7.2 `dot sync` transaction steps (ordered)
1) **Capture**: `chezmoi re-add` (captures changes to plain managed files).
2) **Commit**: commit any staged changes with deterministic message.
3) **Pull/Rebase**: fetch + rebase onto remote (autostash permitted).
4) **Apply**: apply desired state (Chezmoi apply + modules).
5) **Push**: push only if safe (fast-forward).
6) **Stop on conflict**: if any step results in conflict/divergence, stop and notify.

### 7.3 `dot sync now`
- Runs the same transaction immediately.
- Intended for “I need this on my other machine right away.”

### 7.4 Background sync policy
- Background job runs every 30 minutes and on idle.
- Shutdown hook runs **capture + commit only** (no pull/apply/push) because shutdown time is unreliable.

### 7.5 Conflict policy
- Never auto-resolve merge conflicts unattended.
- Provide guidance and a clear status output so you can resolve quickly.

---

## 8. Scheduling/automation

### 8.1 Desired behavior
- Run capture/sync at:
  - **every 30 minutes**
  - **idle**
  - **shutdown (capture+commit only)**
- Provide `dot schedule install|status|remove`.

### 8.2 OS strategies
- Windows: Task Scheduler tasks with triggers:
  - time-based repetition
  - idle trigger
  - shutdown event-based trigger (best-effort)
- Linux: systemd user timer(s)
- macOS: launchd agent(s)

---

## 9. Bootstrap (fresh machine)

### 9.1 Principles
- Minimal steps: 1–2 commands plus explicit manual checkpoints.
- Dependencies installed by bootstrap where possible.

### 9.2 Manual checkpoints (explicit and blocking)
- 1Password sign-in/unlock (QR or system auth).
- Windows activation status:
  - if not activated, pause and instruct to activate legitimately, then re-run.

### 9.3 Bootstrap outputs
- Installs/configures:
  - `dot` itself (preferably from GitHub releases),
  - Chezmoi (pinned version or minimal required version),
  - Git (if missing),
  - 1Password CLI integration (if required for apply),
  - scheduling tasks/timers (optional switch).

---

## 10. WSL strategy (NixOS-WSL preferred)

### 10.1 Roles
- Windows host remains the primary environment.
- WSL is first-class but optional; treated as its own target.

### 10.2 NixOS-WSL integration goals
- Provide `dot wsl install` to:
  - enable WSL,
  - install NixOS-WSL image,
  - apply a flake-based configuration,
  - integrate Docker Desktop support where desired.

### 10.3 1Password + WSL
- Prefer official integration model:
  - SSH uses Windows `ssh.exe` and 1Password agent from host.
- Manage SSH config on Windows; WSL uses forwarded agent.

---

## 11. OS modules (planned; partial)

### 11.1 Package management
- Windows: winget export/import manifest (install from JSON).
- macOS: `brew bundle` via Brewfile (optionally include MAS apps).
- CachyOS (Arch): explicit pacman package lists; foreign/AUR list.

### 11.2 System settings
- Windows:
  - Registry bundles (`.reg`) for HKCU/HKLM
  - Local Group Policy backups (optional; if tooling is included)
  - Scheduled tasks/services adjustments
- macOS:
  - curated `defaults write` scripts
- Linux:
  - sudoers drop-ins (`/etc/sudoers.d/...`)
  - shell, user services, and selected `/etc` drop-ins

### 11.3 Windows hardening (“gaming-safe” default)
- Two profiles:
  - **Gaming-safe** (default)
  - **Extreme** (opt-in with clear warnings)
- Must include ongoing **audit** capability to detect drift after updates.
- Must not break common gaming requirements by default.

---

## 12. Browser strategy (planned; Firefox-first)

### 12.1 Firefox canonical profile
- Assume one canonical Firefox profile per machine.
- Manage:
  - `user.js` for deterministic prefs
  - `chrome/` directory for userChrome/userContent tweaks
- Extension state:
  - Use export/import artifacts per extension where possible.
  - Sidebery snapshot exports are high priority.

### 12.2 Chrome/Chromium
- Secondary requirement.
- Prefer policies and minimal “enablement” rather than full profile replication.

---

## 13. CLI surface area (EDIT AS YOU GO)

The CLI is intentionally small and composable. Each command must be scriptable and have machine-readable exit codes.

### 13.1 Implemented commands

**`dot version`**
- Shows version, commit, build date, Go version, and platform

**`dot doctor`**
- Validates dependencies, repo state, auth, env
- Checks: git, chezmoi, op (optional), repo root, config file

**`dot bootstrap --repo <url>`**
- Clones repo to configured path
- Flags: `--repo`, `--branch`, `--path`

**`dot apply`**
- Applies desired state via `chezmoi apply`

**`dot capture`**
- Captures live edits via `chezmoi re-add`

**`dot sync`**
- Full transaction: capture → commit → pull/rebase → apply → push

**`dot discover`** *(Phase 2 - implemented)*
- Scans system for untracked config files
- Classifies as Recommended/Maybe/Risky/Ignored
- Detects secrets and nested git repositories (sub-repos)
- Flags:
  - `--yes, -y` — Auto-accept recommended files
  - `--dry-run` — Show what would be added without adding
  - `--deep` — Scan additional directories (Library, AppData)
  - `--report` — Print report only, no interactive prompts

### 13.2 Planned commands
- `dot schedule install|status|remove`
- `dot wsl install|apply|status` (when WSL phase begins)

### 13.3 Global flags
- `--verbose, -v` — Enable verbose output
- `--help, -h` — Show help

### 13.4 Exit codes (implemented)

Following sysexits.h conventions:

| Code | Constant | Meaning |
|------|----------|---------|
| 0 | `ExitOK` | Success |
| 1 | `ExitError` | Generic error |
| 64 | `ExitUsage` | Usage error (bad arguments) |
| 65 | `ExitDataErr` | Data error (invalid input) |
| 69 | `ExitUnavailable` | Service unavailable (dependency missing) |
| 75 | `ExitConflict` | Conflict detected (git/chezmoi conflict) |
| 78 | `ExitConfig` | Configuration error |

---

## 14. Logging and observability (EDIT AS YOU GO)

### 14.1 Implementation (Phase 1)

Logging uses Go's `slog` package with dual output:

- **Stderr**: Human-readable text, controlled by `--verbose` flag
- **JSON file**: Structured logs at `state/logs/dot.log` (gitignored)

The `internal/logging` package provides:

```go
type Logger struct {
    slog    *slog.Logger
    file    *os.File
    verbose bool
}

// Usage
log.Info("operation complete", "files", count)
log.Debug("detailed info", "path", path)  // Only shown with --verbose
log.Error("failed", "error", err)
```

### 14.2 Principles

- All commands support `--verbose` for detailed output
- `--json` output mode planned for automation
- Logs go to `state/logs/` (gitignored)
- Never log secrets or sensitive values
- Structured fields for machine parsing

---

## 15. Development plan (ordered phases)

Each phase has deliverables + acceptance criteria.

### Phase 0 — Repo hygiene + docs ✅ COMPLETE
**Deliverables**
- This technical design doc is in repo.
- Spec index is current.
- `dot doctor` exists and reports basic info (repo root, OS, arch, paths).

**Acceptance criteria**
- Running `dot doctor` on macOS and Windows yields a clear report.

---

### Phase 1 — Core plumbing: runners + configuration ✅ COMPLETE
**Implemented**
- `internal/runner`: Interface-based command execution with `Runner` interface and `ExecRunner` implementation. Supports timeouts, captures stdout/stderr, returns structured `CmdResult`.
- `internal/testutil`: Test helpers including `MockRunner` for unit testing command execution.
- `internal/config`: Configuration loading from `dot.toml` with defaults, validation, and environment variable overrides (`DOTSTATE_REPO_URL`, `DOTSTATE_REPO_PATH`, `DOTSTATE_REPO_BRANCH`).
- `internal/logging`: Structured logging via `slog` with dual output (stderr text + JSON file).
- `internal/platform`: Cross-platform OS detection and XDG-compliant path handling.
- `internal/errors`: Custom error types with sysexits.h-compliant exit codes.
- `internal/chez`: Chezmoi wrapper using Runner interface.
- `internal/gitx`: Git operations wrapper.
- `.gitleaks.toml`: Secret scanning configuration.
- `Makefile`: Enhanced with `test`, `test-cover`, `lint`, `secrets` targets.

**Acceptance criteria** ✅
- `dot doctor` verifies git, chezmoi, op presence
- All packages have unit tests
- Interface-based design enables testing without external dependencies

---

### Phase 2 — `dot discover` v0 (report + auto-accept) ✅ COMPLETE
**Implemented**
- `internal/discover/scanner.go`: Scans common config locations (`~/.config`, `~/Library`, `%APPDATA%`, etc.)
- `internal/discover/classifier.go`: Classifies files as Recommended/Maybe/Risky/Ignored based on:
  - File extension (config-ish: .json, .toml, .yaml, etc.)
  - Known config names (settings.json, starship.toml, etc.)
  - Path patterns (risky: tokens, secrets, private keys)
  - File size (>2MB demoted to Maybe or excluded)
- `internal/discover/secrets.go`: 30+ regex patterns for detecting secrets (AWS keys, GitHub tokens, private keys, etc.)
- `internal/discover/subrepo.go`: Detects nested git repositories and extracts remote URL/branch for reference tracking
- `internal/discover/prompt.go`: Interactive selection with `--yes` flag for automation
- `internal/discover/discoverer.go`: Main orchestrator combining all components

**CLI flags:**
- `--yes, -y`: Auto-accept recommended files (no prompts)
- `--dry-run`: Show what would be added without adding
- `--deep`: Scan additional directories (Library, AppData)
- `--report`: Print report only, no interactive prompts

**Key decisions:**
- Sub-repos tracked as references (URL + branch) in `state/subrepos.toml`, not as files
- Secret detection uses both regex pre-scan AND `chezmoi add --secrets=error`
- Interactive by default, `--yes` for headless/automation use

**Acceptance criteria** ✅
- `dot discover --report` outputs classified candidates
- `dot discover --yes` adds recommended files and commits
- Sub-repos detected and excluded from file tracking
- Secret-bearing files flagged as Risky

---

### Phase 3 — `dot discover` v1 (interactive TUI)
**Implement**
- Bubble Tea list UI:
  - multi-select + search
  - category groups
  - preview small text files
- confirm -> add -> commit

**Acceptance criteria**
- UX is fast and not noisy; no accidental browser DB tracking.

---

### Phase 4 — Capture and Apply (MVP)
**Implement**
- `dot capture`:
  - `chezmoi re-add` (capture edits)
  - optional `dot capture --commit` (explicit)
- `dot apply`:
  - `chezmoi apply`
  - module hook placeholders (no-op initially)

**Acceptance criteria**
- Edit a real file, run `dot capture`, see change in repo.
- Clone repo on second machine, run `dot apply`, get file.

---

### Phase 5 — `dot sync` (safe transaction)
**Implement**
- transaction steps in §7
- safe push policy (fast-forward only)
- conflict detection and clear status

**Acceptance criteria**
- Two machines editing different files:
  - `dot sync` pushes/pulls cleanly
- Same file edited on both:
  - `dot sync` stops with conflict status and instructions

---

### Phase 6 — Scheduling automation
**Implement**
- Windows:
  - create scheduled tasks for:
    - interval sync (30 min)
    - idle sync
    - shutdown capture+commit
- macOS:
  - launchd agent for interval sync (start here)
- Linux:
  - systemd user timer for interval sync (start here)

**Acceptance criteria**
- After install, you can confirm tasks/timers exist and run correctly.
- Shutdown task does not block shutdown for long.

---

### Phase 7 — 1Password integration MVP
**Implement**
- `dot doctor` checks:
  - 1Password desktop integration workable
- `dot apply` supports secret retrieval for a small set of templates/generated outputs
- caching of `op` calls per run
- manual checkpoint messaging (“unlock 1Password now”)

**Acceptance criteria**
- On macOS + Windows:
  - first run prompts for unlock
  - subsequent runs are fast
- no secrets leak into logs

---

### Phase 8 — Windows developer UX (PS7 + Terminal)
**Implement**
- Windows module that:
  - installs PowerShell 7 and Windows Terminal
  - sets Terminal default profile to PS7 by managing `settings.json` under correct path
- Provide fallback doc if OS blocks some “default shell” behavior.

**Acceptance criteria**
- Open Terminal → default is pwsh 7
- Repo tracks Terminal settings and PS profile.

---

### Phase 9 — WSL (NixOS-WSL) integration
**Implement**
- `dot wsl install` (guided + checkpointed)
- `dot wsl apply` to apply flake/config
- docs for 1Password SSH bridging behavior

**Acceptance criteria**
- WSL environment reachable and can apply configuration reproducibly.

---

### Phase 10 — Package manifests (Windows/macOS/Linux)
**Implement**
- capture/export commands:
  - `dot packages capture` writes manifests
- apply/install commands:
  - `dot packages apply` installs from manifests
- integrate into `dot apply` as optional module stage.

**Acceptance criteria**
- On each OS, packages install from repo manifest.

---

### Phase 11 — Windows hardening (gaming-safe)
**Implement**
- `dot windows harden apply --profile gaming-safe`
- `dot windows harden audit`
- registry + policy bundles
- explicit opt-in for “extreme” with warnings.

**Acceptance criteria**
- Hardening is reproducible and auditable.
- Gaming baseline not broken by default.

---

### Phase 12 — Browser modules (Firefox-first)
**Implement**
- Firefox profile detection
- manage `user.js` and `chrome/`
- extension export/import artifact tracking workflows (manual steps where required)
- Sidebery snapshot export strategy

**Acceptance criteria**
- prefs + UI tweaks reproducible
- extension settings best-effort with documented manual exports.

---

### Phase 13 — Notifications + integrations (optional)
**Implement**
- on conflict or required manual checkpoint:
  - notify via webhook (Pushcut/Telegram, chosen later)

---

### Phase 14 — CI/CD and releases
**Implement**
- build per OS target
- publish GitHub releases
- bootstrap scripts download correct binary + install dependencies.

---

## 16. Roadmap: open decisions and missing strategies

This section lists what is intentionally not fully decided yet, with status updates.

### 16.1 Notifications channel
- Pushcut vs Telegram vs both
- Message format, rate limiting, secrets handling
- **Status:** Open (Phase 13)

### 16.2 Exact Windows hardening policy list
- Need a curated, versioned policy/registry set
- Must be tested against gaming requirements
- **Status:** Open (Phase 11)

### 16.3 Browser extension automation limits
- For each extension (uBO, Sidebery, Stylus, Violentmonkey, SponsorBlock):
  - determine best export/import format
  - determine which settings are reliably restorable
  - decide what stays manual
- **Status:** Documented in `docs/non-trivial-tracking.md`, implementation deferred (Phase 12)

### 16.4 Secrets boundary enforcement ✅ DECIDED
- **Decision:** Use BOTH pre-commit secret scanning AND runtime detection:
  1. `.gitleaks.toml` configuration for pre-commit/CI scanning
  2. Built-in regex patterns in `internal/discover/secrets.go` for runtime detection
  3. `chezmoi add --secrets=error` as final guardrail
- Files with detected secrets are classified as "Risky" and require explicit selection
- **Status:** Implemented in Phase 1 and 2

### 16.5 Chezmoi integration approach ✅ DECIDED
- **Decision:** Currently requires user-managed installation; auto-install planned for future
- `dot doctor` checks for chezmoi presence
- Future: `dot bootstrap` will install chezmoi if missing (platform-specific methods)
- **Status:** Partial implementation, auto-install deferred

### 16.6 NixOS-WSL flake design
- Decide whether flake lives inside dotstate repo or as a separate repo/submodule.
- Decide whether WSL dotfiles are shared with CachyOS or separate variant sets.
- **Status:** Open (Phase 9)

### 16.7 Package capture strategy for CachyOS
- How to represent AUR packages (manifest format) and which helper to assume.
- **Status:** Open (Phase 10)

### 16.8 Sub-repository handling ✅ DECIDED
- **Decision:** Track nested git repos as references, not files
- Manifest stored in `state/subrepos.toml` with path, URL, and branch
- On apply: clone if missing, skip if present (user manages updates)
- Future: `dot subrepo status` command to check all sub-repos
- **Status:** Detection implemented in Phase 2; apply/clone deferred to Phase 4

### 16.9 Platform priority ✅ DECIDED
- **Decision:** macOS first → Windows 11 → CachyOS/Arch Linux
- Development and testing prioritizes macOS (Apple Silicon)
- Windows and Linux support follows
- **Status:** Documented, informing all implementation decisions

---

## 17. Spec index (EDIT AS YOU GO)

These docs are normative for their feature areas:

- `docs/discover.md` — discovery roots, filtering, scoring, UI contract
- `docs/non-trivial-tracking.md` — handling Firefox, sub-repos, macOS defaults, Windows registry, browser extensions
- `docs/sync.md` — sync transaction details and failure modes
- `docs/scheduling.md` — automation strategy per OS
- `docs/secrets-1password.md` — secret model and patterns
- `docs/bootstrap-*.md` — bootstrap procedures per OS
- `docs/wsl-nixos.md` — WSL target specifics

### Implementation references

- `internal/runner/runner.go` — Command execution interface
- `internal/config/config.go` — Configuration loading and validation
- `internal/logging/logging.go` — Structured logging
- `internal/platform/platform.go` — OS detection and paths
- `internal/errors/errors.go` — Error types and exit codes
- `internal/discover/*.go` — Discovery implementation

---

## 18. “Update this as we implement” checklist (EDIT AS YOU GO)

When any of the below changes, update this document:

- command list and flags in §13
- repo layout in §4
- transaction semantics in §7 (if behavior changes)
- scheduling behavior in §8
- open decisions in §16 (remove items as they’re decided)

---

## 19. Appendix: operational habits

Recommended habits to keep the system healthy:

- Prefer changing managed config by editing the real destination file.
- Use `dot sync now` when you need changes on another machine immediately.
- Keep browser modules explicit; do not “discover” browser profile DBs.
- Treat “extreme hardening” as a separate profile with warnings.

