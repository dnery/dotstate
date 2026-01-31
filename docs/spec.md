# dotstate spec

This doc defines the **invariants**, **guardrails**, and the current **contract** of the dotstate system.

The goal of dotstate is to make your personal OS setups reproducible and portable across:

- Windows 11 Pro (native)
- macOS (Apple Silicon)
- Arch-family Linux (CachyOS)
- WSL (NixOS‑WSL) as a first-class optional target

---

## The core idea

dotstate is two things:

1) A **git repository** that contains:
- “desired state” for user configs (dotfiles, app configs)
- “desired state” for selected system settings (later modules)
- machine-captured exports (package lists, registry exports, defaults dumps, etc.)

2) A portable **`dot` CLI** (Go binary) that orchestrates:
- capture → commit → pull/rebase → apply → push
- baseline discovery (`dot discover`)
- prerequisites checks (`dot doctor`)
- OS-specific bootstrap and scheduling (planned)

Chezmoi is used as the **copy-based file state engine**.

---

## Non-negotiable invariants

### 1) Copy-based deployment
- The system must not require symlinks.
- If symlinks exist on a platform, treat them as an implementation detail, never a requirement.

### 2) Secrets never land in git
- The repo may store *references* to secrets (1Password item/field identifiers), but never secret values.
- Secret-bearing outputs must be generated locally and written under `state/private/` (gitignored), or split into private include files.

### 3) “Edit real files” is the default workflow
- You edit destination files normally (your actual `~/.gitconfig`, app config JSON, etc.).
- `dot capture` / `dot sync` pull those edits back into the repo using Chezmoi re-add.
- No “special editor” workflow is required.

### 4) Idempotent apply
- Running `dot apply` repeatedly should converge to the same result.
- Apply steps should prefer declarative configs and drop-ins over patching monolithic files.

### 5) Safe automation by default
- Scheduled runs should avoid admin-required operations unless explicitly opted into.
- Conflicts must not be auto-resolved silently.

---

## Repository structure (current + planned)

Current directories:

- `home/`
  - Chezmoi source state (copy-based).
  - This is the **managed** set of files that participates in apply + capture.

- `state/`
  - Machine-captured exports and generated artifacts.
  - `state/private/` is reserved for secret-bearing outputs and is gitignored.

- `docs/`
  - Specs, bootstrap guides, and manual checkpoints.

Planned directories:

- `targets/`
  - OS/role specific targets (e.g., `targets/wsl/` for NixOS‑WSL flake).

- `modules/` (or `internal/modules/`)
  - OS-specific capture/apply/audit logic for deeper settings.

---

## Managed vs generated files

dotstate distinguishes three broad classes of artifacts:

### Managed artifacts (Chezmoi)
- Stored under `home/` (Chezmoi source).
- Applied to destination files via `chezmoi apply`.
- Captured from destination back into `home/` via `chezmoi re-add`.

These are the “normal” dotfiles/app configs you want to edit and sync.

### Generated / private artifacts (local-only)
- Generated at apply time from machine context and/or secrets.
- Stored under `state/private/` and **never committed**.
- Copied into destination as needed.

Use this class for:
- tokens
- signing keys
- anything that would be dangerous to auto-capture

### Captured exports (machine state)
- Stored under `state/` (not private).
- Examples: package lists, registry exports, defaults dumps.
- These may be committed, but should be curated to avoid noise.

---

## Configuration (`dot.toml`)

`dot.toml` lives at the repo root and defines:

- `[repo]`
  - `url`: remote git URL
  - `path`: where to clone to / operate from on this machine
  - `branch`: branch name

- `[sync]`
  - `interval_minutes`: default 30
  - `enable_idle`: bool
  - `enable_shutdown`: bool

- `[tools]`
  - paths to `git`, `chezmoi`, `op` (empty means “use PATH”)

- `[chex]`
  - `source_dir`: directory inside repo used as Chezmoi source (default `home`)

- `[wsl]`
  - `enable`: bool
  - `distro_name`: e.g. `nixos`
  - `flake_ref`: flake reference to apply inside WSL

---

## Core commands (contract)

### `dot doctor`
Checks prerequisites and provides actionable errors:
- config discovery (`dot.toml`)
- tool presence (`git`, `chezmoi`, `op`)
- repo sanity checks (remote configured, branch exists)

Windows-only planned checks:
- activation status (warn + manual checkpoint)
- winget availability

### `dot bootstrap --repo <url>`
Bootstraps a machine into a working state.

MVP behavior:
- clone repo into the configured directory (or default)
- print next steps

Planned behavior:
- install prerequisites (OS-specific)
- block on manual checkpoints (1Password login, activation, reboot)
- install scheduling

### `dot apply`
Applies desired state to the machine.
- Runs `chezmoi apply` against `home/` (via `--source <repo>/<chex.source_dir>`).

### `dot capture`
Captures local destination edits back into repo source state.
- Runs `chezmoi re-add`.

### `dot sync`
Runs the convergence transaction:
1. capture (`chezmoi re-add`)
2. commit (if changes)
3. pull/rebase (`git pull --rebase --autostash`)
4. apply (`chezmoi apply`)
5. push

Flags:
- `--no-apply`
- `--no-push`

### `dot discover`
Baseline onboarding command. Spec is in `docs/discover.md`.

---

## Scheduling (contract)

The intended schedule:
- full `dot sync` every 30 minutes
- full `dot sync` on idle
- shutdown flush is best-effort and must be fast

Details: `docs/scheduling.md`

---

## WSL (contract)

WSL is managed as a separate target (NixOS‑WSL + flakes), owned by this repo.

Details: `docs/wsl-nixos.md`

---

## Conflict and safety policy

- If git conflicts occur, dotstate stops and reports.
- No automatic conflict resolution.
- Scheduled runs must never silently rewrite history.
- Admin-required operations are manual unless explicitly opted into.

---

## What’s explicitly out of scope (for now)

- Full Windows hardening / debloat module (planned, but not yet locked)
- Full browser extension state automation (planned; likely hybrid of export/import + policy)
- Cross-machine merge automation beyond “stop on conflict + notify”
