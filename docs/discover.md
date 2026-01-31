# `dot discover` spec

`dot discover` is the “kick‑off” feature for onboarding an **already‑customized** machine.

It scans for high‑signal configuration files that exist on the machine but are **not yet tracked** in the repo, lets you review/preview them, and then adds the selected ones to the repo’s managed state (Chezmoi source under `home/`).

The intent is that you can run this on:
- macOS (your M4 Max MacBook Pro)
- Windows 11 Pro desktop
- (later) CachyOS / Arch-family Linux
- (later) WSL user environment

…and quickly converge on a tracked baseline without turning the repo into a landfill of caches, logs, and browser databases.

---

## Design principles

1. **Fast by default, deep when asked**
   - Default scan should finish quickly and produce a shortlist.
   - `--deep` expands to broader OS data roots that require stronger filtering.

2. **High signal, low regret**
   - Prefer “clearly a config” over “might be interesting”.
   - When unsure, classify as **Maybe** (not preselected).

3. **Safe by default**
   - Secrets must not be accidentally committed.
   - Anything likely to contain secrets is **Risky** and not preselected.
   - Adding uses `chezmoi add --secrets=error` by default so secret‑looking content is rejected. (This is a guardrail, not a guarantee.)

4. **Copy semantics**
   - Added files are copied into the repo’s source state (Chezmoi).
   - No reliance on symlinks.

5. **“Edit real files” workflow compatible**
   - `dot sync`/`dot capture` rely on `chezmoi re-add` to pull future edits back into the repo.
   - Therefore, `discover` should add mostly **plain managed files**, not templates.

---

## Terminology

- **Repo root**: directory containing `dot.toml`.
- **Chezmoi source dir**: `repo/<chex.source_dir>` (default `home/`).
- **Destination file**: the real file on disk that the OS/app actually reads.
- **Managed**: tracked under Chezmoi source; participates in apply/capture.
- **Generated / private**: created locally at apply time (often secret‑bearing) and stored under `state/private/` (gitignored). Not managed, not discovered, not captured.

---

## UX contract

### Default behavior (interactive)

Running:

```bash
dot discover
```

Must:

1. Compute the scan roots (fast set).
2. Ask Chezmoi for unmanaged paths under those roots.
3. Apply filtering + scoring + classification.
4. Present an interactive selection UI:
   - searchable list
   - multi-select (toggle)
   - group by **Recommended / Maybe / Risky / Ignored**
   - optional preview pane for small text files
   - show size + path + “why this is recommended / why risky”
5. On confirm, add selected paths to repo with:
   - `chezmoi add --secrets=error <paths...>`
6. Optionally commit:
   - Default: **commit after successful add**
   - Message: `dot discover on <hostname> at <timestamp>`
7. Print next steps:
   - Run `dot apply` if you want the repo state to immediately be the source of truth
   - Run `dot sync now` if you want to propagate to other machines immediately

### Non-interactive mode

```bash
dot discover --no-tui
```

Must print a deterministic report:
- category
- score
- size
- absolute path
- reason tags

This mode is for debugging / CI / scripting. It should not add files unless `--accept` is provided.

---

## CLI interface

### Core command

```bash
dot discover [flags]
```

### Flags

- `--deep`
  - Adds broader roots (AppData/Library) and increases filtering strictness.

- `--dry-run`
  - Computes candidates and renders UI/report, but does not add or commit.

- `--no-tui`
  - Print report only (no interactive UI).

- `--accept <selector>`
  - Non-interactive selection mechanism.
  - Selector formats (v1):
    - `--accept recommended` (add all Recommended)
    - `--accept <comma-separated indexes>` (when combined with `--no-tui` output indexes)

- `--commit` / `--no-commit`
  - Default: `--commit` in interactive mode.
  - Default: `--no-commit` in report-only mode unless `--accept` is used.

- `--secrets {error|warning|ignore}`
  - Default: `error`.
  - Passed through to `chezmoi add --secrets=...`.
  - Only allow weakening this intentionally.

- `--roots <path1,path2,...>`
  - Override root set entirely (advanced).
  - When set, `--deep` is ignored.

- `--max-file-size <bytes>`
  - Default: 2 MiB.
  - Files larger than this are shown as **Maybe** (not preselected) or excluded, depending on type.

---

## Root sets

### Fast roots (default)

These are chosen for speed and high yield.

**All OS**
- `$HOME/.config`
- `$HOME/.ssh`

**macOS**
- (no additional default roots — `.config` already captures most developer tooling)

**Windows**
- `%APPDATA%\` (Roaming) **only for curated app folders** (not recursive across all of AppData by default)
- Curated single files (added even if not under scanned roots):
  - Windows Terminal `settings.json` (stable + preview)
  - PowerShell profile(s) (PowerShell 7)
  - Zed settings/keymap/themes folders

Rationale: scanning all of `%APPDATA%` by default is too noisy.

### Deep roots (`--deep`)

**macOS**
- `$HOME/Library/Application Support`
- `$HOME/Library/Preferences`

**Windows**
- `%APPDATA%` (Roaming)
- `%LOCALAPPDATA%` (Local)

Deep mode must be strict about ignores (see below).

---

## Filtering rules

### Hard excludes (never shown)

Exclude any path that matches (case-insensitive substring match is OK in v1):

Caches / logs / crash dumps:
- `Cache`, `Caches`, `GPUCache`, `Code Cache`, `ShaderCache`
- `Crash`, `Crashpad`, `Crashes`
- `Logs`, `log`, `Trace`, `traces`
- `Temp`, `tmp`

“Clearly not config” bulk:
- `node_modules`
- `Steam`, `Epic`, game install directories
- Large media directories (`Videos`, `Music`, etc.)

Browser swamps (tracked via dedicated browser modules later):
- Firefox profile trees (`Firefox/Profiles`, `Mozilla/Firefox/Profiles`)
- Chrome/Edge profile trees (`Chrome/User Data`, `Microsoft/Edge/User Data`, `BraveSoftware`)

### File size rules

- Default max file size: 2 MiB
- If a file is > max size:
  - if extension is config-ish (`.json`, `.toml`, `.yml`, `.ini`, `.conf`, `.plist`, `.lua`): classify **Maybe**
  - otherwise: exclude

### “Risky” classification rules

Mark as **Risky** (not preselected) if:
- filename/path contains:
  - `id_rsa`, `id_ed25519`, `.pem`, `.p12`, `.pfx`, `.key`
  - `token`, `secret`, `password`, `credentials`
  - `kubeconfig`
- or lives under:
  - password manager directories
  - browser extension storage
- or is a known “identity file”:
  - SSH private keys
  - GPG private keys (if found)

Risky items are still showable because sometimes you want to track the *non-secret* bits (e.g., SSH config), but they must never be “select all recommended”.

### “Recommended” scoring rules

Score boosts for:
- located under `.config` or other config roots
- file extension config-ish
- basename is a known config name:
  - `settings.json`, `config.json`, `keymap.json`, `starship.toml`, `alacritty.yml`, etc.
- paths matching curated app configs (Zed, Terminal, PowerShell profile, Git config, etc.)

### Final categories

- **Recommended**
  - high score, small-ish, config-ish, not risky
  - preselected in UI

- **Maybe**
  - medium score, or large config-ish, or unknown extension
  - not preselected

- **Risky**
  - likely secret-bearing or identity-bearing
  - not preselected and requires explicit toggle

---

## Add semantics

When user confirms selection:

1. `discover` runs:
   - `chezmoi add --secrets=error <paths...>`

2. If any add fails due to secret detection:
   - do not partially commit
   - show the list of rejected files
   - suggest patterns:
     - split secret parts into `*.private` include files under `state/private/`
     - or generate secret-bearing outputs at apply-time

3. After a successful add:
   - `discover` runs `dot capture` (optional; usually unnecessary right after add)
   - commit if enabled

---

## Extension points (future)

- Allow user-maintained ignore rules:
  - `state/discover/ignore.txt` (simple substring / glob list)
- Allow curated candidates defined in repo:
  - `state/discover/curated.toml` containing per-OS path templates and metadata
- Add a “watch mode”:
  - a file system watcher that detects edits and suggests “Add this new config?”

---

## Testing checklist

### macOS
- `dot discover` should show `.config` candidates and curated files (PowerShell profile if present).
- It must not show caches or browser profiles by default.
- Adding a file should create it under `home/` and `dot apply` should reproduce it.

### Windows
- `dot discover` fast mode should:
  - show Terminal settings.json if present
  - show Zed config if present
  - show PowerShell profile if present
- `--deep` should produce more candidates but still avoid obvious junk.

### Safety
- Create a file containing a token-like string in `.config` and verify:
  - `--secrets=error` rejects it (or at least classifies as risky + warns)
  - user must explicitly opt-in to add it, and docs advise splitting secret parts.
