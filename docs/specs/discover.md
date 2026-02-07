# `dot discover` spec

`dot discover` is the "kick‑off" feature for onboarding an **already‑customized** machine.

It scans for high‑signal configuration files that exist on the machine but are **not yet tracked** in the repo, lets you review/preview them, and then adds the selected ones to the repo's managed state (Chezmoi source under `home/`).

> **Implementation Status:** Phase 2 complete. Core discovery, classification, secret detection, and sub-repo handling are implemented. Interactive TUI planned for Phase 3.

The intent is that you can run this on:
- macOS (your M4 Max MacBook Pro) — **primary target**
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
   - Prefer "clearly a config" over "might be interesting".
   - When unsure, classify as **Maybe** (not preselected).

3. **Safe by default**
   - Secrets must not be accidentally committed.
   - Anything likely to contain secrets is **Risky** and not preselected.
   - Built-in secret detection with 30+ regex patterns
   - Adding uses `chezmoi add --secrets=error` by default so secret‑looking content is rejected. (This is a guardrail, not a guarantee.)

4. **Copy semantics**
   - Added files are copied into the repo's source state (Chezmoi).
   - No reliance on symlinks.

5. **"Edit real files" workflow compatible**
   - `dot sync`/`dot capture` rely on `chezmoi re-add` to pull future edits back into the repo.
   - Therefore, `discover` should add mostly **plain managed files**, not templates.

6. **Sub-repo aware**
   - Nested git repositories (e.g., `~/.config/nvim`) are tracked as references, not files.
   - Prevents conflicts with existing git workflows.

---

## Terminology

- **Repo root**: directory containing `dot.toml`.
- **Chezmoi source dir**: `repo/<chex.source_dir>` (default `home/`).
- **Destination file**: the real file on disk that the OS/app actually reads.
- **Managed**: tracked under Chezmoi source; participates in apply/capture.
- **Generated / private**: created locally at apply time (often secret‑bearing) and stored under `state/private/` (gitignored). Not managed, not discovered, not captured.
- **Sub-repo**: a directory that is itself a git repository, tracked by reference rather than content.

---

## UX contract

### Default behavior (interactive)

Running:

```bash
dot discover
```

Must:

1. Compute the scan roots (fast set).
2. Scan for files under those roots.
3. Apply filtering + classification.
4. Detect nested git repositories (sub-repos).
5. Run secret detection on candidates.
6. Present an interactive selection:
   - Group by **Recommended / Maybe / Risky**
   - Show file path and classification reason
   - Allow selection/deselection
7. On confirm, add selected paths to repo with:
   - `chezmoi add --secrets=error <paths...>`
8. Optionally commit:
   - Default: **commit after successful add**
   - Message: `dot discover on <hostname> at <timestamp>`
9. Print next steps:
   - Run `dot apply` if you want the repo state to immediately be the source of truth
   - Run `dot sync now` if you want to propagate to other machines immediately

### Auto-accept mode (implemented)

```bash
dot discover --yes
```

Automatically accepts all **Recommended** files without prompting. Useful for:
- Headless environments
- CI/CD pipelines
- Initial setup scripts

### Report mode (implemented)

```bash
dot discover --report
```

Prints a classification report without adding or prompting:
- Lists all candidates grouped by category
- Shows secret warnings
- Shows sub-repos detected
- Does not modify any files

This mode is for debugging / CI / scripting.

---

## CLI interface (implemented)

### Core command

```bash
dot discover [flags]
```

### Flags

- `--yes, -y`
  - Auto-accept recommended files without prompting
  - Adds all Recommended category files
  - Skips Maybe and Risky

- `--dry-run`
  - Computes candidates and shows what would be added
  - Does not actually add or commit

- `--deep`
  - Adds broader roots (AppData/Library) and increases scan scope
  - Applies stricter filtering on expanded directories

- `--report`
  - Print classification report only (no interactive UI)
  - Does not add files unless combined with `--yes`

- `--no-commit`
  - Skip the commit step after adding files
  - Useful when you want to review changes before committing

- `--secrets {error|warning|ignore}`
  - Default: `error`
  - Controls how secrets are handled during add
  - Passed through to `chezmoi add --secrets=...`

### Planned flags (future)

- `--roots <path1,path2,...>`
  - Override root set entirely (advanced)

- `--max-file-size <bytes>`
  - Default: 2 MiB
  - Files larger than this are classified as Maybe or excluded

---

## Root sets (implemented)

### Fast roots (default)

These are chosen for speed and high yield.

**All OS**
- `$HOME/.config`
- `$HOME/.ssh` (classified as Risky)

**macOS**
- `$HOME/.zshrc`, `$HOME/.bashrc`, etc. (shell configs)

**Windows**
- `%APPDATA%\` (Roaming) **only for curated app folders**
- Curated single files:
  - Windows Terminal `settings.json`
  - PowerShell profile(s)

**Linux**
- `$HOME/.config` (XDG standard)
- Shell configs in `$HOME`

### Deep roots (`--deep`)

**macOS**
- `$HOME/Library/Application Support`
- `$HOME/Library/Preferences`

**Windows**
- `%APPDATA%` (Roaming) - full scan
- `%LOCALAPPDATA%` (Local)

Deep mode must be strict about ignores (see below).

---

## Filtering rules (implemented)

### Hard excludes (never shown)

Exclude any path that matches:

**Caches / logs / crash dumps:**
- `Cache`, `Caches`, `GPUCache`, `Code Cache`, `ShaderCache`
- `Crash`, `Crashpad`, `Crashes`
- `Logs`, `log`, `Trace`, `traces`
- `Temp`, `tmp`

**"Clearly not config" bulk:**
- `node_modules`
- `Steam`, `Epic`, game install directories
- Large media directories

**Browser data (tracked via dedicated modules later):**
- Firefox profile trees (`Firefox/Profiles`, `Mozilla/Firefox/Profiles`)
- Chrome/Edge profile trees (`Chrome/User Data`, `Microsoft/Edge/User Data`)

### File size rules

- Default max file size: 2 MiB
- If a file is > max size:
  - if extension is config-ish: classify **Maybe**
  - otherwise: exclude

### Classification logic (implemented)

Files are classified into four categories based on scoring:

**Recommended** (score ≥ 3):
- Located under `.config` or known config roots
- Has config-ish extension (`.json`, `.toml`, `.yaml`, `.yml`, `.ini`, `.conf`, `.lua`, `.plist`)
- Basename is a known config name (`settings.json`, `config.json`, `starship.toml`, etc.)
- Matches curated app config patterns

**Maybe** (score 1-2):
- Medium confidence
- Large config-ish files
- Unknown but plausible config files

**Risky** (has risky indicators):
- Filename/path contains: `id_rsa`, `id_ed25519`, `.pem`, `.p12`, `.pfx`, `.key`
- Contains: `token`, `secret`, `password`, `credentials`, `kubeconfig`
- Lives under password manager directories
- Has secret findings from regex scan

**Ignored** (score 0 or excluded):
- Matches hard exclude patterns
- Binary files
- Too large and not config-ish

---

## Secret detection (implemented)

### Built-in patterns

The `internal/discover/secrets.go` package includes 30+ regex patterns:

**Cloud credentials:**
- AWS access keys (`AKIA...`)
- AWS secret keys
- GCP service account keys
- Azure connection strings

**API tokens:**
- GitHub tokens (`ghp_`, `gho_`, `ghs_`, `ghr_`)
- GitLab tokens (`glpat-`)
- Slack tokens (`xox[baprs]-`)
- Discord webhooks
- Stripe keys (`sk_live_`, `rk_live_`)

**Private keys:**
- RSA/DSA/EC private key headers
- OpenSSH private keys
- PGP private keys
- Age secret keys

**Database credentials:**
- PostgreSQL connection strings
- MySQL connection strings
- MongoDB connection strings
- Redis URLs

**Other secrets:**
- JWT tokens
- Basic auth in URLs
- Generic password/secret/token assignments

### Detection flow

1. **Pre-scan**: Before classification, files are scanned for secret patterns
2. **Classification impact**: Files with findings are marked as **Risky**
3. **Warnings displayed**: Secret findings shown in report and prompts
4. **Final guardrail**: `chezmoi add --secrets=error` catches remaining secrets

---

## Sub-repository handling (implemented)

### Detection

The discover command detects nested git repositories by looking for `.git` directories. When found:

1. Directory is excluded from file tracking
2. Remote URL and branch are extracted from `.git/config`
3. Information stored in `state/subrepos.toml`

### Manifest format

```toml
[[subrepo]]
path = ".config/nvim"
url = "https://github.com/user/nvim-config"
branch = "main"

[[subrepo]]
path = ".emacs.d"
url = "https://github.com/user/emacs-config"
```

### Rationale

Many users have config directories that are their own git repos:
- `~/.config/nvim` cloned from a personal/fork
- `~/.emacs.d` with Spacemacs/Doom
- `~/.oh-my-zsh` with custom fork

These should NOT be tracked as files because:
- They have their own git history
- Updates come from their upstream, not dotstate
- Including them would create conflicts

Instead, dotstate tracks the reference and can clone on new machines.

### Future: Apply behavior

When `dot apply` encounters a sub-repo entry:
- If directory doesn't exist: clone from URL
- If directory exists: skip (user manages)
- Future: `dot subrepo status` to check all sub-repos

---

## Add semantics

When user confirms selection:

1. `discover` runs:
   - `chezmoi add --secrets=error <paths...>`

2. If any add fails due to secret detection:
   - Do not partially commit
   - Show the list of rejected files
   - Suggest patterns:
     - Split secret parts into `*.private` include files under `state/private/`
     - Or generate secret-bearing outputs at apply-time

3. After a successful add:
   - Commit if enabled
   - Message: `dot discover on <hostname> at <timestamp>`

---

## Implementation details

### Scanner (`internal/discover/scanner.go`)

- Walks configured root directories
- Applies exclude patterns
- Respects file size limits
- Handles platform-specific paths

### Classifier (`internal/discover/classifier.go`)

- Scores files based on extension, name, and location
- Applies risky pattern detection
- Returns category and reasoning

### Secret detector (`internal/discover/secrets.go`)

- Compiles regex patterns once
- Scans file content for matches
- Returns findings with pattern name and line context

### Sub-repo detector (`internal/discover/subrepo.go`)

- Checks for `.git` directory
- Parses `.git/config` for remote URL
- Extracts current branch from `.git/HEAD`

### Prompter (`internal/discover/prompt.go`)

- Handles interactive selection
- Supports `--yes` for auto-accept
- Groups candidates by category

### Discoverer (`internal/discover/discoverer.go`)

- Orchestrates all components
- Coordinates scan → classify → detect → prompt → add flow

---

## Testing checklist

### macOS
- [x] `dot discover` should show `.config` candidates
- [x] It must not show caches or browser profiles by default
- [ ] Adding a file should create it under `home/`

### Windows
- [x] `dot discover` fast mode should show Terminal settings.json if present
- [x] `--deep` should produce more candidates but still avoid junk

### Safety
- [x] Files containing secret patterns are classified as Risky
- [x] Secret warnings are displayed in report
- [ ] `chezmoi add --secrets=error` rejects actual secrets

### Sub-repos
- [x] Nested git repos are detected
- [x] Sub-repos excluded from file candidates
- [x] Remote URL and branch extracted correctly

---

## Extension points (future)

- User-maintained ignore rules: `state/discover/ignore.txt`
- Curated candidates in repo: `state/discover/curated.toml`
- Watch mode: file system watcher for new config files
- Interactive TUI with Bubble Tea (Phase 3)
