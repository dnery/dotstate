# dotstate spec (draft)

This doc defines *invariants* and *guardrails* for the system, so implementation stays sane.

## Core invariants

1. Copy-based deployment only.
   - No required symlink support.
   - If a symlink is present on a platform, treat it as an implementation detail, never a requirement.

2. Secrets never land in git.
   - The repo may store *references* (1Password item/field identifiers), but never secret values.
   - Any secret-bearing outputs must live under `state/private/` (gitignored).

3. "Edit real files" is the default workflow.
   - Users edit destination files normally.
   - `dot capture` brings changes back into repo-managed state (primarily via chezmoi re-add).
   - The background schedule runs `dot sync` (capture + commit + pull + apply + push).

4. "Safe automation" over "clever automation".
   - If git diverged (conflicts likely), automation stops and notifies.
   - No unattended conflict resolution.

## Managed vs generated artifacts

- Managed artifacts:
  - Things that are safe and intended to be tracked in git.
  - Example: `~/.gitconfig`, `~/.config/zed/settings.json`, `~/.config/fish/config.fish`

- Generated artifacts:
  - Derived from secrets or machine-local probes.
  - Examples: `.npmrc` with auth token; exported browser extension backups that may contain secrets.
  - Live under `state/private/` unless explicitly safe.

## Module model (future)

Each module may implement:
- Detect() -> bool (is this module applicable on this machine)
- Capture() (export current machine state into `state/`)
- Apply() (apply desired state from repo to machine)
- Audit() (best-effort drift detection + warnings)

Modules are OS-scoped (windows/macos/linux/wsl) but can share helpers.
