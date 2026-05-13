# macOS state surfaces

Status: planned implementation contract for macOS Tahoe on Apple Silicon first.

This spec maps macOS state into first-party `dotstate` modules. It depends on the common [module and state contract](modules.md) and must not introduce separate schema, capability, redaction, or UX semantics.

## Scope and defaults

`dotstate` manages macOS developer state through typed surfaces, not broad filesystem crawling. The default backend choices are:

- `files`: Chezmoi copy semantics from `home/`.
- `brew`: Homebrew Bundle for taps, formulae, and casks.
- `mas`: `mas` for Mac App Store inventory when installed and signed in.
- `apps`: bundle identifiers plus source hints for direct-download or manually installed apps.
- `defaults`: curated domain/key reads only; no full plist capture by default.
- `launchd`: user LaunchAgents and Homebrew services.
- `profiles`: configuration profile metadata and MDM posture.
- `privacy_tcc`: privacy/TCC posture and manual checkpoints; never TCC database mutation.
- `subrepos`: nested git repositories tracked by reference.
- `secrets`: secret references and generated-file checkpoints; never secret values.

The first implementation target is read-only `dot macos audit --json`. Mutating behavior must be introduced later through the plan/backup/apply/verify lifecycle in [modules.md](modules.md).

## macOS audit envelope

`dot macos audit --json` must emit `dotstate.audit.v1` with `dotstate.fact.v1` facts and `dotstate.diagnostic.v1` diagnostics. The command should exit successfully without elevated permissions when it can safely report partial state. Missing tools, locked accounts, privacy restrictions, and MDM ownership are diagnostics.

Initial surface order:

1. `files`
2. `brew`
3. `mas`
4. `apps`
5. `launchd`
6. `defaults`
7. `profiles`
8. `privacy_tcc`
9. `subrepos`
10. `secrets`

The order is for deterministic output only. It does not imply apply order.

## Common macOS capabilities and diagnostics

| Condition | Capability | Diagnostic code pattern | Expected behavior |
| --- | --- | --- | --- |
| Tool is missing (`brew`, `mas`, `profiles`, etc.) | `unsupported` or `read_only` | `macos.<surface>.tool_unavailable` | Continue other surfaces; include install/remediation text. |
| Command needs administrator rights | `requires_sudo`, `manual` | `macos.<surface>.sudo_required` | Do not prompt for sudo during audit. |
| Privacy approval blocks reads | `requires_full_disk_access`, `manual` | `macos.<surface>.full_disk_access_required` | Emit partial facts where safe and explain how to retry. |
| State is controlled by MDM/profile | `requires_mdm`, `read_only` | `macos.<surface>.mdm_managed` | Report metadata; never attempt local apply. |
| Surface has no safe API | `manual` or `unsupported` | `macos.<surface>.manual_checkpoint` | Record instructions/checkpoints instead of scraping private data. |
| Value may contain secrets | `read_only` or `manual` | `macos.<surface>.redacted` | Redact before serialization and taint sensitivity. |

## Surface contracts

### `files`

**Purpose:** manage normal dotfiles and app config files through Chezmoi copy semantics.

- Current state sources:
  - `chezmoi managed` with the configured source directory.
  - Destination file metadata for selected managed paths.
- Desired artifact:
  - `home/` Chezmoi source tree.
- Fact IDs:
  - `files:path/~/.zshrc`
  - `files:path/~/Library/Application Support/App/config.json`
- Capability defaults:
  - `read_only` during audit.
  - `auto_apply` only through the existing Chezmoi apply path with backups/checks.
- Redaction:
  - Normalize home paths to `~`.
  - Do not include file contents in audit facts.
  - Secret-looking managed files must produce diagnostics rather than content.
- Risk notes:
  - High risk for private keys, SSH material, browser databases, cookies, and app caches; these should remain excluded or manual.

### `brew`

**Purpose:** model Homebrew taps, formulae, casks, and Brewfile presence.

- Current state sources:
  - `brew tap`
  - `brew list --formula --versions`
  - `brew list --cask --versions`
  - `brew bundle check --file <Brewfile>` when a desired artifact exists.
- Desired artifact:
  - `state/macos/brew/Brewfile`
- Fact IDs:
  - `brew:tap/homebrew/cask`
  - `brew:formula/git`
  - `brew:cask/visual-studio-code`
- Capability defaults:
  - `read_only` and `auto_apply` for formula/cask install plans once dry-run and backup semantics exist.
  - `dry_run_only` until `brew bundle check`/plan output is wired into the module runner.
- Redaction:
  - Formula, cask, and tap names are usually `public`.
  - Custom tap URLs with credentials must redact credentials.
- Risk notes:
  - Casks can install apps and background services; plans should label them at least `medium` risk when they add new software.
  - Do not auto-upgrade everything as a side effect of audit or apply.

### `mas`

**Purpose:** report Mac App Store apps and optionally capture desired app IDs.

- Current state sources:
  - `mas list` when `mas` is installed.
  - Tool/account availability diagnostics when it is not installed or not signed in.
- Desired artifact:
  - `state/macos/mas.toml`
  - Optional Brewfile `mas` entries may be generated from the same desired model later.
- Fact IDs:
  - `mas:app/497799835`
- Capability defaults:
  - `read_only` during audit.
  - `dry_run_only` for install plans until sign-in/account failure modes are covered.
  - `manual` when the App Store requires GUI sign-in or purchase history action.
- Redaction:
  - App IDs and names are `personal` because they reveal installed software.
  - Never include Apple ID email addresses unless explicitly configured, and redact them by default.
- Risk notes:
  - Some apps disappear from the store or are account-bound; a missing app should be a diagnostic, not a hard failure for the whole plan.

### `apps`

**Purpose:** inventory installed `.app` bundles with bundle identifiers and source hints.

- Current state sources:
  - `/Applications/*.app`
  - `~/Applications/*.app`
  - Curated app directories only; do not recursively crawl generated app bundles in caches or vendor trees.
  - `Info.plist` keys such as bundle identifier, version, display name, and signing/team metadata when available.
- Desired artifact:
  - `state/macos/apps.toml`
- Fact IDs:
  - `apps:bundle/com.apple.Terminal`
  - `apps:bundle/com.microsoft.VSCode`
- Capability defaults:
  - `read_only` for audit.
  - `manual` or `dry_run_only` for direct-download apps until each source has a safe installer/update strategy.
  - `auto_apply` may be delegated to `brew` or `mas` when the source hint maps cleanly.
- Redaction:
  - Bundle ID and app name are `personal` unless they are first-party OS apps.
  - Local paths use `~` where applicable.
- Risk notes:
  - Direct-download app installers can be mutable, unsigned, or interactive; keep them manual until provenance is explicit.

### `defaults`

**Purpose:** report and eventually manage curated macOS preference keys.

- Current state sources:
  - `defaults read <domain> <key>` for curated domain/key pairs.
  - Avoid `defaults export` or full plist dumps by default.
- Desired artifact:
  - `state/macos/defaults.toml`
- Fact IDs:
  - `defaults:domain/com.apple.dock/key/autohide`
  - `defaults:domain/NSGlobalDomain/key/AppleShowAllExtensions`
- Capability defaults:
  - `read_only` for audit.
  - `dry_run_only` until typed value parsing, domain allowlists, rollback, and idempotency tests exist.
  - `manual` for settings that require logout, reboot, app restart, or GUI confirmation.
- Redaction:
  - Do not record values for preference keys that may contain recent files, account names, hostnames, device IDs, tokens, or browsing data.
  - Curated keys must declare sensitivity before capture.
- Risk notes:
  - Many defaults are app-private or OS-version-sensitive; unknown keys should not be captured automatically.

### `launchd`

**Purpose:** report user LaunchAgents and Homebrew services.

- Current state sources:
  - `~/Library/LaunchAgents/*.plist`
  - `launchctl print gui/<uid>` where available without elevation.
  - `brew services list --json` when Homebrew is installed.
- Desired artifact:
  - `state/macos/launchd.toml`
- Fact IDs:
  - `launchd:user/com.example.agent`
  - `launchd:brew/postgresql@16`
- Capability defaults:
  - `read_only` for audit.
  - `dry_run_only` until backup and unload/load ordering are implemented.
  - `manual` for system LaunchDaemons or agents requiring sudo.
- Redaction:
  - Program arguments and environment variables can contain secrets; redact suspicious values and avoid serializing full env blocks.
  - Normalize home paths.
- Risk notes:
  - LaunchAgents can execute arbitrary commands at login; additions/updates are at least `medium` risk and should be easy to inspect.

### `profiles`

**Purpose:** report configuration profile and MDM posture without trying to override policy.

- Current state sources:
  - System profile metadata from built-in macOS profile reporting commands when available.
  - MDM enrollment posture as coarse metadata only.
- Desired artifact:
  - `state/macos/profiles.toml` for manual checkpoints and expected profile identifiers.
- Fact IDs:
  - `profiles:identifier/com.example.profile`
- Capability defaults:
  - `read_only`, `requires_mdm`, or `manual`.
  - Never `auto_apply` in the initial architecture.
- Redaction:
  - Profile identifiers and organization names may be `personal`.
  - Payload values can include Wi-Fi, VPN, certificate, account, or security policy details; report identifiers/status unless a curated field is explicitly safe.
- Risk notes:
  - MDM state is authoritative outside dotstate. Local apply should not fight it.

### `privacy_tcc`

**Purpose:** report privacy permission posture and manual checkpoints for services such as Accessibility, Full Disk Access, Screen Recording, Automation, and Developer Tools.

- Current state sources:
  - Safe, public OS status APIs where available.
  - Manual checkpoint manifests.
  - Permission-denied observations from other modules.
- Desired artifact:
  - `state/macos/privacy.toml`
- Fact IDs:
  - `privacy_tcc:service/Accessibility/client/com.example.App`
  - `privacy_tcc:service/FullDiskAccess/client/com.apple.Terminal`
- Capability defaults:
  - `read_only`, `manual`, `requires_full_disk_access`, or `requires_mdm`.
  - Never `auto_apply` for TCC database changes.
- Redaction:
  - Never read, copy, commit, or mutate TCC databases.
  - Do not store historical permission rows or timestamps from protected databases.
  - Store only service/client/manual status facts needed for user guidance.
- Risk notes:
  - Privacy permissions are security-sensitive and intentionally user/MDM controlled. `dotstate` guides; it does not bypass.

### `subrepos`

**Purpose:** track nested git repositories by reference rather than by file contents.

- Current state sources:
  - Existing `state/subrepos.toml`.
  - Local nested git repositories discovered by `dot discover`.
  - Optional `git status`/remote metadata checks later.
- Desired artifact:
  - `state/subrepos.toml`
- Fact IDs:
  - `subrepos:path/~/.config/nvim`
- Capability defaults:
  - `read_only` during audit.
  - `auto_apply` only for clone-if-missing when the destination path is absent and the remote URL is redacted/safe.
  - `manual` when the directory already exists, is dirty, has no remote, or has credentialed remotes.
- Redaction:
  - Redact credentials in remotes.
  - Do not include untracked file names unless a status command is explicitly requested.
- Risk notes:
  - Pulling or overwriting subrepos can conflict with user-managed workflows. Start with clone-if-missing only.

### `secrets`

**Purpose:** model secret references, generated private files, and secret tooling posture without storing secret material.

- Current state sources:
  - `state/secrets/refs.toml` when present.
  - `~/.config/dotstate/secrets-env.json` metadata.
  - 1Password CLI availability/lock status when checked safely.
  - Generated cache file presence/mode/mtime from the `secrets-env` contract.
- Desired artifact:
  - `state/secrets/refs.toml`
  - Existing `secrets-env` config outside the repo for local shell cache behavior.
- Fact IDs:
  - `secrets:ref/op://vault/item/field`
  - `secrets:cache/sfr3`
- Capability defaults:
  - `read_only` for metadata audit.
  - `manual` for unlock/sign-in checkpoints.
  - `auto_apply` only for generating local private files after explicit user approval and redaction tests.
- Redaction:
  - Never print resolved values, cache contents, Secure Note JSON, Keychain items, or environment variable values.
  - `op://` references are `credential_reference` and should use item UUIDs when titles are ambiguous.
  - Cache facts may include presence, mode, variable count, and mtime, but not variable names if names are sensitive in context.
- Risk notes:
  - Secret-backed generated files are local-only and must remain outside Chezmoi capture unless they contain no secret material.

## Capture rules by surface

| Surface | Safe capture default | Notes |
| --- | --- | --- |
| `files` | Existing Chezmoi `re-add` for managed files only. | Secret scanning remains a guardrail, not permission to capture secrets. |
| `brew` | Generate/review Brewfile-compatible desired state. | Avoid auto-capturing transient pinned/outdated metadata unless requested. |
| `mas` | Capture app IDs/names when `mas` is available. | Account-bound failures become diagnostics. |
| `apps` | Capture bundle IDs and source hints only. | Direct-download URLs remain manual until curated. |
| `defaults` | Capture curated domain/key values only. | No full plist dumps. |
| `launchd` | Capture selected user agents/services only after review. | Do not capture arbitrary env/program args without redaction. |
| `profiles` | Capture expected identifiers/manual checkpoints. | No payload secrets. |
| `privacy_tcc` | Capture manual checklist state only. | No DB copies. |
| `subrepos` | Capture path, redacted remote, and branch. | Merge with existing manifest; do not track contents. |
| `secrets` | Capture references and metadata only. | Never capture values. |

## Apply order guidance

Future module apply should order low-level prerequisites before dependent state:

1. `files` non-secret baseline needed by tools.
2. `brew` formulae/taps/casks.
3. `mas` apps.
4. `apps` manual/direct-download checkpoints.
5. `subrepos` clone-if-missing.
6. `secrets` generated private files after manual unlock.
7. `launchd` agents/services.
8. `defaults` curated preferences.
9. `profiles` and `privacy_tcc` manual/MDM checkpoints.
10. `verify` all surfaces.

This order is advisory until the module runner owns dependency ordering through plan `depends_on` entries.

## Minimum macOS fixtures

Add these fixture cases before implementing the corresponding audit or apply behavior:

| Surface | Required cases |
| --- | --- |
| `brew` | no `brew`; empty install; taps/formulae/casks present; credentialed custom tap URL; Brewfile noop/create plan. |
| `mas` | no `mas`; `mas` installed with apps; not signed in/unavailable; app ID capture. |
| `apps` | `/Applications` bundle; `~/Applications` bundle; missing/invalid `Info.plist`; Homebrew cask source hint; direct-download/manual source hint. |
| `defaults` | missing domain/key; boolean/string/integer values; redacted value key; OS-version-unknown key. |
| `launchd` | user agent plist; Homebrew service; env var redaction; sudo/system daemon diagnostic. |
| `profiles` | no profiles; MDM-managed profile metadata; restricted payload redaction. |
| `privacy_tcc` | manual checkpoint; permission-denied diagnostic from another module; MDM-controlled service. |
| `subrepos` | redacted credentialed remote; existing clean repo; existing dirty repo; clone-if-missing plan. |
| `secrets` | missing `op`; locked/unavailable `op`; reference-only fact; cache metadata; sentinel secret redaction. |
| `files` | managed path normalization; unreadable file diagnostic; secret-looking managed file diagnostic. |

Fixture outputs must use the schemas from [modules.md](modules.md) and must not contain raw sentinel secrets.

## Non-goals

- No broad `~/Library` crawling as the primary macOS strategy.
- No Nix requirement for normal onboarding.
- No TCC database mutation, Keychain capture, cookie capture, or decrypted secret capture.
- No third-party executable plugin API until first-party module schemas and redaction fixtures are stable.
- No apply behavior for a surface solely because audit can observe it.
