# Module and state contract

Status: planned implementation contract for the next macOS-first architecture slice.

This spec defines the common lifecycle, state records, safety semantics, and fixture requirements for every `dotstate` module. Platform-specific surfaces are cataloged in [macOS state surfaces](macos-state-surfaces.md).

## Design rules

1. Every module is plan-first: mutation is only allowed after facts have been normalized, diagnostics emitted, and a plan/diff produced.
2. Every module uses the same record vocabulary for facts, plans, results, diagnostics, and backups.
3. Every mutating module has an explicit backup/restore story before broad apply behavior is enabled.
4. Secret values, Keychain contents, TCC databases, and other restricted material are never captured. Modules may store references, redacted metadata, or manual checkpoints only.
5. Capabilities describe what `dot` can safely do on this machine now; they are not promises that every machine can do the same thing.

## Lifecycle

All modules implement the same lifecycle phases, even when some phases are no-ops.

| Phase | Mutates? | Contract |
| --- | --- | --- |
| `discover` | No | Find candidate desired-state artifacts or existing managers. Emit facts and diagnostics with confidence levels. |
| `audit` | No | Read current state and emit normalized facts. Missing tools or permissions are diagnostics, not panics. |
| `plan` / `diff` | No | Compare current facts to desired state and produce ordered changes with risk, capabilities, and backup requirements. |
| `backup` | Yes, local artifact only | Save the minimum reversible pre-change state for each planned mutation. Backups must be redaction-aware and local-only when sensitive. |
| `apply` | Yes | Execute only changes whose capabilities allow mutation and whose plan has not changed since backup. |
| `verify` | No | Re-read state after apply and prove whether the desired state was reached. |
| `capture` | Yes, repo artifacts | Convert reviewed current state into desired-state artifacts. Capture must never persist secret values. |
| `restore` | Yes | Revert from a backup record or explain why the backup is manual/non-reversible. |

A module may begin as `audit`-only. It must report unsupported phases using a diagnostic with capability `unsupported` rather than inventing separate behavior.

## Required common fields

The following fields are required wherever the schema below names a module subject. Records may add module-specific fields under `current`, `desired`, `metadata`, or `extensions`, but must not rename these common fields.

| Field | Type | Meaning |
| --- | --- | --- |
| `surface` | string | Stable module/surface name, such as `files`, `brew`, `defaults`, or `privacy_tcc`. |
| `id` | string | Stable, deterministic identifier within the surface. Do not include volatile timestamps or machine-specific temp paths. |
| `source` | object | Where the observation or desired state came from: command, file, API, manifest, or manual checkpoint. |
| `current` | object or `null` | Current observed state. Must be redacted before serialization. |
| `desired` | object or `null` | Desired state from repo/config/user intent. Must be redacted before serialization. |
| `managed_by` | array of strings | Managers that own or influence the item, for example `dotstate`, `chezmoi`, `homebrew`, `mas`, `launchd`, `mdm`, `manual`, or `unknown`. |
| `sensitivity` | string | Highest sensitivity of the serialized record after redaction. See [sensitivity levels](#sensitivity-levels). |
| `confidence` | string | `confirmed`, `high`, `medium`, `low`, or `unknown`. |
| `capability` | array of strings | Capability labels from the standard enum. More than one may apply. |
| `risk` | object | Risk level plus short reasons and, when relevant, required confirmations. |

### Standard capability enum

Use only these capability labels in v1:

- `auto_apply`: `dot` may apply the planned change without interactive manual steps after the user accepts the plan.
- `dry_run_only`: `dot` can compute a plan but must not mutate this item yet.
- `manual`: the user must perform or confirm a step outside `dot`.
- `requires_sudo`: mutation or full observation requires administrator privileges.
- `requires_full_disk_access`: observation needs macOS Full Disk Access or equivalent privacy approval.
- `requires_mdm`: state is owned by a Mobile Device Management profile or similar policy manager.
- `read_only`: `dot` may report the item but not apply or capture desired state.
- `unsupported`: the item or phase is out of scope for this platform/version.

### Sensitivity levels

| Level | Meaning | Serialization rule |
| --- | --- | --- |
| `public` | Safe to share in normal reports. | Serialize as-is. |
| `local_path` | Reveals local usernames, hostnames, or private directory names. | Prefer `~`, repo-relative paths, or redacted path segments. |
| `personal` | Reveals installed apps, account names, organization names, or preference choices. | Serialize only when useful; avoid unnecessary detail. |
| `credential_reference` | Points to a secret without containing the secret value, for example an `op://` reference. | Store the reference only when intentional; never resolve it in reports. |
| `secret` | Contains a token, password, private key, cookie, credential, or decrypted secret. | Never serialize; replace with a redaction marker. |
| `restricted` | Protected OS databases or policy-controlled state, such as TCC internals or Keychain contents. | Store metadata/manual checkpoint only. |

### Risk object

```json
{
  "level": "low",
  "reasons": [],
  "requires_confirmation": false,
  "reversible": true
}
```

Allowed `level` values are `low`, `medium`, `high`, and `critical`. Use `critical` for changes that can lock the user out, destroy data, weaken security, or write to protected system policy surfaces.

## Versioned schemas

All records include `schema_version`. A breaking field rename, semantic change, or enum change increments the record version. Additive module-specific metadata may remain in the same version.

### Fact v1

Facts represent current observed state or a report-only/manual checkpoint.

```json
{
  "schema_version": "dotstate.fact.v1",
  "surface": "brew",
  "id": "brew:formula/git",
  "source": {
    "kind": "command",
    "value": "brew list --formula --versions",
    "observed_at": "2026-05-13T00:00:00Z"
  },
  "current": {
    "name": "git",
    "version": "2.x",
    "installed": true
  },
  "desired": null,
  "managed_by": ["homebrew"],
  "sensitivity": "public",
  "confidence": "confirmed",
  "capability": ["read_only", "auto_apply"],
  "risk": {
    "level": "low",
    "reasons": [],
    "requires_confirmation": false,
    "reversible": true
  },
  "diagnostics": []
}
```

Fact IDs must be stable:

- `files:path/~/.zshrc`
- `brew:tap/homebrew/cask`
- `brew:formula/git`
- `brew:cask/visual-studio-code`
- `mas:app/497799835`
- `apps:bundle/com.apple.Terminal`
- `defaults:domain/com.apple.dock/key/autohide`
- `launchd:user/com.example.agent`
- `profiles:identifier/com.example.profile`
- `privacy_tcc:service/Accessibility/client/com.example.App`
- `subrepos:path/~/.config/nvim`
- `secrets:ref/op://vault/item/field`

### Plan v1

Plans are the only input accepted by mutating phases.

```json
{
  "schema_version": "dotstate.plan.v1",
  "plan_id": "20260513T000000Z-abc123",
  "created_at": "2026-05-13T00:00:00Z",
  "target": {
    "os": "darwin",
    "arch": "arm64",
    "host": "<redacted:hostname>"
  },
  "summary": {
    "create": 1,
    "update": 0,
    "delete": 0,
    "noop": 3,
    "manual": 1,
    "blocked": 0
  },
  "changes": [
    {
      "change_id": "brew:formula/git:create",
      "surface": "brew",
      "id": "brew:formula/git",
      "action": "create",
      "source": {"kind": "manifest", "value": "state/macos/brew/Brewfile"},
      "current": {"installed": false},
      "desired": {"installed": true, "name": "git"},
      "managed_by": ["dotstate", "homebrew"],
      "sensitivity": "public",
      "confidence": "high",
      "capability": ["auto_apply"],
      "risk": {"level": "low", "reasons": [], "requires_confirmation": false, "reversible": true},
      "backup_required": false,
      "depends_on": [],
      "diagnostics": []
    }
  ],
  "diagnostics": []
}
```

Allowed change `action` values are `create`, `update`, `delete`, `noop`, `report`, `manual`, and `blocked`.

### Result v1

Results report what actually happened during `backup`, `apply`, `verify`, `capture`, or `restore`.

```json
{
  "schema_version": "dotstate.result.v1",
  "run_id": "20260513T000010Z-def456",
  "plan_id": "20260513T000000Z-abc123",
  "phase": "apply",
  "surface": "brew",
  "id": "brew:formula/git",
  "change_id": "brew:formula/git:create",
  "source": {"kind": "plan", "value": "20260513T000000Z-abc123"},
  "current": {"installed": true},
  "desired": {"installed": true, "name": "git"},
  "managed_by": ["dotstate", "homebrew"],
  "sensitivity": "public",
  "confidence": "confirmed",
  "capability": ["auto_apply"],
  "risk": {"level": "low", "reasons": [], "requires_confirmation": false, "reversible": true},
  "status": "applied",
  "started_at": "2026-05-13T00:00:10Z",
  "ended_at": "2026-05-13T00:00:12Z",
  "diagnostics": []
}
```

Allowed result `status` values are `applied`, `verified`, `captured`, `restored`, `noop`, `skipped`, `manual`, `blocked`, and `failed`.

### Diagnostic v1

Diagnostics are structured messages for users and tests. Do not encode state solely in free text.

```json
{
  "schema_version": "dotstate.diagnostic.v1",
  "severity": "warning",
  "code": "macos.full_disk_access.required",
  "message": "Full Disk Access is required to inspect this surface completely.",
  "remediation": "Grant Full Disk Access to the terminal running dot, then rerun the audit.",
  "surface": "privacy_tcc",
  "id": "privacy_tcc:service/Accessibility/client/com.example.App",
  "source": {"kind": "system", "value": "macOS privacy controls"},
  "current": null,
  "desired": null,
  "managed_by": ["manual"],
  "sensitivity": "restricted",
  "confidence": "medium",
  "capability": ["requires_full_disk_access", "manual"],
  "risk": {"level": "medium", "reasons": ["protected privacy surface"], "requires_confirmation": true, "reversible": false}
}
```

Allowed severities are `info`, `warning`, and `error`. Diagnostic codes are dotted, stable identifiers suitable for tests.

### Backup v1

Backups describe reversible pre-change state. Sensitive backup payloads stay local and are referenced by path/checksum rather than printed.

```json
{
  "schema_version": "dotstate.backup.v1",
  "backup_id": "20260513T000005Z-files-zshrc",
  "created_at": "2026-05-13T00:00:05Z",
  "surface": "files",
  "id": "files:path/~/.zshrc",
  "source": {"kind": "path", "value": "~/.zshrc"},
  "current": {"exists": true, "mode": "0644", "sha256": "<sha256>"},
  "desired": null,
  "managed_by": ["dotstate", "chezmoi"],
  "sensitivity": "local_path",
  "confidence": "confirmed",
  "capability": ["auto_apply"],
  "risk": {"level": "low", "reasons": [], "requires_confirmation": false, "reversible": true},
  "payload_ref": {
    "kind": "local_file",
    "path": "state/backups/20260513T000005Z/files/zshrc",
    "sha256": "<sha256>"
  },
  "restore": {
    "supported": true,
    "requires_confirmation": true
  }
}
```

Backup payloads under `state/backups/` must be gitignored. If a module cannot make a useful backup, its plan change must say `backup_required: false` and explain why in diagnostics.

## Audit JSON envelope

`dot macos audit --json` and future platform audits should wrap facts in a stable envelope:

```json
{
  "schema_version": "dotstate.audit.v1",
  "generated_at": "2026-05-13T00:00:00Z",
  "target": {"os": "darwin", "arch": "arm64", "host": "<redacted:hostname>"},
  "facts": [],
  "diagnostics": [],
  "summary": {
    "facts": 0,
    "warnings": 0,
    "errors": 0,
    "redactions": 0
  }
}
```

The envelope is successful when unsupported or unavailable surfaces are represented as diagnostics and the command can continue safely.

## Redaction rules

1. Redaction happens before any record is logged, rendered, serialized, tested, or returned from a module boundary.
2. Strings matching known secret patterns are replaced with `<redacted:secret>` and taint the record as `secret` unless the record stores only a reference.
3. Credentialed URLs must preserve scheme/host/path while replacing credentials, for example `https://<redacted:credential>@github.com/user/repo.git`.
4. Local absolute paths under the home directory should use `~`. Paths outside home that reveal user or organization names should be shortened when the exact path is not required.
5. `op://` references are `credential_reference`, not `secret`, as long as they do not include resolved values.
6. Keychain values, decrypted 1Password values, cookies, private keys, OAuth tokens, and TCC database rows are never serialized.
7. Redacted values should preserve enough type information for users to act: `<redacted:token>`, `<redacted:private-key>`, `<redacted:hostname>`, `<redacted:path-segment>`.
8. Sensitivity is monotonic within a record: once a child value is sensitive, the parent record reports the highest applicable sensitivity after redaction.
9. Diagnostics must not quote raw command output until it has been redacted.
10. Golden tests must fail if fixture sentinel values such as `DOTSTATE_TEST_SECRET_DO_NOT_PRINT` appear anywhere in stdout, stderr, JSON, logs, plans, results, diagnostics, or backups.

## UX wording contract

Use consistent verbs in human output:

- `Would create`, `Would update`, `Would remove`, `No change`, `Manual step`, and `Blocked` for plan/diff output.
- `Applied`, `Skipped`, `Verified`, `Failed`, `Restored`, and `Needs manual action` for results.
- Capability explanations:
  - `requires_sudo`: "Requires administrator privileges; rerun with an explicit elevated flow when you understand the change."
  - `requires_full_disk_access`: "Requires Full Disk Access for complete inspection."
  - `requires_mdm`: "Managed by MDM; dotstate can report it but cannot change it."
  - `dry_run_only`: "Planning is available, but apply is intentionally disabled for this surface."
  - `manual`: "Record this as a manual checkpoint or perform it outside dotstate."

A module must not introduce new safety wording for the same capability without updating this spec.

## Desired-state artifact conventions

The macOS-first module artifacts should live under stable paths unless a later migration intentionally changes them:

```text
state/macos/brew/Brewfile
state/macos/mas.toml
state/macos/apps.toml
state/macos/defaults.toml
state/macos/launchd.toml
state/macos/profiles.toml
state/macos/privacy.toml
state/secrets/refs.toml
state/subrepos.toml
home/                         # Chezmoi source for files module
```

Artifacts may be absent. An absent artifact means "no desired state declared"; it is not an implicit delete request.

## Contract fixtures before implementation

Every new module must add fixtures before broad implementation. Use this layout:

```text
test/fixtures/modules/v1/<surface>/<case>/
  README.md
  input/
  desired/
  fact.golden.json
  plan.golden.json
  result.golden.json          # when apply/capture/restore exists
  diagnostics.golden.json
  redaction.assert_absent.txt
```

Minimum fixture cases for each surface:

1. `audit-empty`: tool absent or no state; emits diagnostics if needed and no fake facts.
2. `audit-present`: normal current state emits stable fact IDs.
3. `plan-noop`: current equals desired and produces only `noop` changes.
4. `plan-create-or-update`: desired differs and risk/capability are explicit.
5. `unavailable-tool`: missing command emits `*.unavailable` diagnostic and exits safely.
6. `permission-denied`: protected path/API emits a capability diagnostic, not a crash.
7. `redaction`: sentinel secrets and credentialed URLs do not appear in any output.
8. `manual-or-mdm`: policy/manual state is reported without auto-apply.
9. `unsupported-platform`: non-target OS emits `unsupported` where applicable.
10. `backup-restore`: required for any surface that mutates files or system state.

A fixture is complete only when a reviewer can run the future module against the fixture and compare stable JSON without relying on the developer's machine.

## Implementation gates

Before enabling `auto_apply` for a module:

- Facts, plans, diagnostics, and backups use this schema.
- The module has redaction fixtures with sentinel secret values.
- A dry-run plan exists and is reviewed in tests.
- Backup/restore is implemented or the plan proves the change is reversible without a backup.
- Apply is idempotent: applying the same plan twice produces no meaningful diff.
- Permission failures are actionable diagnostics.
- Human output and JSON output are both covered by tests.
