# dotstate

`dotstate` is a safety-focused developer-state orchestrator. It helps you move a
Mac from "fresh install" to "usable workstation" and keeps day-to-day dotfile and
machine-state changes reviewable.

It does **not** try to replace every native tool. Instead, it coordinates proven
backends and macOS APIs behind one plan-first workflow:

- **Files and dotfiles:** Chezmoi-compatible copy semantics from `home/`.
- **Packages and apps:** Homebrew Bundle, `mas` when available, and app bundle IDs.
- **macOS settings:** curated `defaults` keys, LaunchAgents, profiles/MDM posture,
  privacy/TCC checkpoints, and subrepo references.
- **Secrets:** references and metadata only. Secret values, Keychain contents, and
  TCC databases are never captured or printed.

## Current status

Primary implementation target: **macOS Tahoe on Apple Silicon**.
Windows support remains a design target, but the richest flows today are macOS-first.

Implemented today:

| Area | Status |
| --- | --- |
| Fresh macOS bootstrap | GitHub Release binary + `scripts/bootstrap-macos.sh` |
| Files/dotfiles | Chezmoi-backed `apply`, `capture`, and `sync` |
| Read-only macOS audit | `dot macos audit --json` emits `dotstate.audit.v1` facts and diagnostics |
| Curated discovery | `dot discover` defaults to high-signal file candidates; broad scans are opt-in |
| Non-file capture | `dot capture` writes reviewable artifacts under `state/macos/` and `state/secrets/` |
| Scheduled sync | macOS user LaunchAgent via `dot schedule install|status|remove` |
| Subrepos | `dot subrepo status` and clone-if-missing apply for `state/subrepos.toml` |

Still intentionally conservative:

- Homebrew/MAS/apps/defaults system mutation is mostly **manual/dry-run-only** until
  each surface has rollback, idempotency, and permission-denied coverage.
- macOS privacy/TCC and Keychain are **reference/checkpoint-only**; dotstate does
  not bypass platform security.
- Broad filesystem crawling is **not** the default onboarding strategy.

## Mental model

Think in five verbs:

1. **audit** — read current machine state without mutating it.
2. **discover** — find high-signal unmanaged config files for review.
3. **capture** — write reviewed current state into repo artifacts.
4. **apply** — apply repo state back to the machine when the module is safe to mutate.
5. **sync** — capture, commit, pull/rebase, apply, and push with dirty-repo guards.

Most commands are plan-first. If you are unsure, run `--dry-run` first.

Plain-language glossary: audit — inspect without mutation; discover — shortlist unmanaged files; capture — write reviewed state into the repo; apply — make the machine match safe repo state; sync — run the guarded capture/commit/pull/apply/push loop.

### What mutates what?

| Command | Mutates machine? | Mutates repo? | Notes |
| --- | --- | --- | --- |
| `dot doctor` | No | No | Checks config and prerequisites. |
| `dot macos audit --json` | No | No | Safe read-only facts; partial state is reported as diagnostics. |
| `dot discover --report` | No | No | Redacted report only. |
| `dot discover` | No system mutation | Yes, after selection | Adds selected files to Chezmoi source. |
| `dot capture --dry-run` | No | No | Shows module capture plan. |
| `dot capture` | No system mutation | Yes | Updates `home/`, `state/macos/`, `state/secrets/` artifacts. |
| `dot apply --dry-run` | No | No | Shows module apply plan. |
| `dot apply` | Yes, for safe modules | Local backup artifacts | Files apply through Chezmoi; subrepos can clone if missing. |
| `dot sync --dry-run` | No | No | Shows capture/apply plans without git or apply mutations. |
| `dot sync` | Yes, after capture/commit/pull | Yes | Refuses to start if repo is already dirty. |
| `dot schedule install` | Writes LaunchAgent | No | Use `--dry-run` first. |

## First 15 minutes on a fresh Mac

### 1. Bootstrap the binary and repo

```sh
curl -fsSL https://raw.githubusercontent.com/dnery/dotstate/master/scripts/bootstrap-macos.sh \
  | sh -s -- --repo https://github.com/dnery/dotstate
```

If you want to see what would happen first:

```sh
curl -fsSL https://raw.githubusercontent.com/dnery/dotstate/master/scripts/bootstrap-macos.sh \
  | sh -s -- --dry-run --repo https://github.com/dnery/dotstate
```

The bootstrap script is deliberately conservative. It checks Xcode Command Line
Tools, points you to Homebrew when needed, installs the release asset
`dot-darwin-arm64.tar.gz`, and treats 1Password/op unlock as a manual checkpoint.
It does not fetch or print decrypted secrets.

### Manual checkpoints

Expect a few human-in-the-loop steps on a brand-new Mac:

- Xcode Command Line Tools may need `xcode-select --install`.
- Homebrew may need to be installed from <https://brew.sh> before package/app facts are useful.
- 1Password desktop and `op` may need sign-in, CLI integration, and unlock before secret-backed workflows can run.
- macOS privacy prompts may hide some state until you grant the terminal app permission; dotstate reports those gaps instead of requesting access automatically.

### 2. Validate the environment

From the cloned repo:

```sh
dot doctor
dot apply --dry-run
dot sync --dry-run
dot macos audit --json
```

`dot macos audit --json` may report missing optional tools such as `mas` or
privacy/MDM limitations. Those are diagnostics, not necessarily failures.

### 3. Review before mutating

When the dry-run output looks reasonable:

```sh
dot apply
```

This applies safe managed state. Today that mainly means Chezmoi-managed files and
clone-if-missing subrepos. Non-file package/app/defaults state is captured as
reviewable artifacts first and remains manual to apply unless the module says it
has `auto_apply` capability.

## Onboard an already-customized Mac

Use this flow when the machine already has dotfiles, app settings, Homebrew
packages, editor config, and other local state.

### 1. Get a normalized read-only inventory

```sh
dot macos audit --json > /tmp/dotstate-audit.json
```

The audit envelope includes facts for:

- files already managed by Chezmoi
- Homebrew taps/formulae/casks and Brewfile presence
- `mas` app inventory when `mas` is installed and signed in
- installed `.app` bundles with bundle IDs/source hints
- user LaunchAgents and Homebrew services
- curated macOS `defaults` keys
- profile/MDM posture and privacy/TCC manual checkpoints
- subrepo manifest presence
- Keychain/secret reference-only policy

Treat audit output as local machine data. It is redacted, but it can still reveal
installed software and local paths.

### 2. Discover unmanaged config files

```sh
dot discover --report
```

Default discovery is curated and should produce a short list. It avoids caches,
language-server payloads, vendor trees, browser databases, source maps, generated
`.app` bundles, and broad `~/Library` crawling.

If you need a special scan, make it explicit:

```sh
# Scan specific roots only
dot discover --report --roots ~/.config/nvim,~/Library/Application\ Support/Zed

# Deep scan broader roots with the normal filters
dot discover --report --deep

# Change the candidate size cutoff
dot discover --report --max-file-size 524288
```

You can maintain local curation files:

```text
state/discover/curated-roots.txt  # add high-signal roots to fast discovery
state/discover/ignore.txt         # ignore glob or substring patterns
```

### 3. Capture reviewed state into the repo

Preview first:

```sh
dot capture --dry-run
```

Then capture:

```sh
dot capture
```

Capture can update:

```text
home/                         # Chezmoi source for managed files
state/macos/brew/Brewfile     # Homebrew desired state
state/macos/mas.toml          # Mac App Store app IDs/names
state/macos/apps.toml         # App bundle IDs/source hints
state/macos/defaults.toml     # Curated defaults values only
state/secrets/generated.toml  # Secret-generation policy/metadata only
```

The secret artifact is intentionally metadata-only. It records reference/policy
information such as `secret_values_serialized = false`; it must not contain secret
values.

### 4. Sync only from a clean repo

```sh
git status --short
dot sync --dry-run
dot sync
```

`dot sync` refuses to start when the repo already has uncommitted changes. That is
intentional: it prevents unrelated work from being swept into an automatic sync
commit.

## Daily workflow

Common loop:

```sh
# See what changed locally
git status --short

# Pull real-file edits back into repo artifacts
dot capture --dry-run
dot capture

# Review and commit if appropriate
git diff
git status --short

# Apply repo state back to this machine
dot apply --dry-run
dot apply

# Or do the guarded all-in-one flow
dot sync --dry-run
dot sync
```

For scheduled sync on macOS:

```sh
dot schedule install --dry-run
dot schedule install
dot schedule status
```

This creates a user LaunchAgent at:

```text
~/Library/LaunchAgents/com.dnery.dotstate.sync.plist
```

There is no shutdown hook. Use `dot sync now` before reboot/shutdown when you want
an explicit flush.

## Subrepos

Nested git repositories are tracked by reference instead of by copying their
contents. Declare them in `state/subrepos.toml`:

```toml
[[subrepo]]
path = "~/.config/nvim"
url = "https://github.com/example/nvim.git"
branch = "main"
```

Then inspect or apply:

```sh
dot subrepo status
dot apply --dry-run
dot apply
```

Apply can clone a missing subrepo. If the destination already exists and is not a
git repository, dotstate leaves it manual.

## Secrets and privacy guarantees

Non-negotiable rules:

- Secret values never land in git.
- Keychain contents are never read or serialized.
- TCC/privacy databases are never read, copied, committed, or mutated.
- 1Password/op references such as `op://...` may be stored intentionally, but
  decrypted values must stay out of reports and artifacts.
- Sensitive backup payloads stay local and should not be shared.

Reports are redacted, but still treat them as local diagnostic artifacts. In
particular, avoid pasting full `dot discover --report` or `dot macos audit --json`
output into public places without review.

## Repository layout

```text
.
├── cmd/                      # CLI entrypoints: dot and secrets-env
├── internal/                 # core packages and module implementations
├── home/                     # Chezmoi source state for managed files
├── state/                    # desired artifacts, logs, captures, local runtime state
├── docs/                     # canonical documentation
├── scripts/                  # bootstrap scripts
└── test/                     # docs/e2e harnesses and fixtures
```

Important state paths:

```text
state/macos/                  # portable macOS desired-state artifacts
state/secrets/                # secret references/policy metadata, not values
state/private/                # local-only generated/private files; gitignored
state/e2e-runs/               # local verification bundles; gitignored
```

## Develop dotstate itself

Prerequisites:

- Go 1.23+
- git
- Chezmoi
- Homebrew for macOS flows
- Optional: `mas`, 1Password desktop + `op`, gitleaks, golangci-lint, goimports

Build and run locally:

```sh
make build-local
./bin/dot doctor
./bin/dot macos audit --json
./bin/dot discover --report
```

Validation commands:

```sh
# Fast baseline used frequently during development
go test ./...
make docs-check
git diff --check

# Build local binaries
make build-local

# End-to-end harnesses
make test-e2e-fast
make test-e2e-verify
make test-e2e
```

Golden fixture updates should be explicit and reviewed:

```sh
DOTSTATE_UPDATE_GOLDEN=1 go test ./internal/modules ./internal/macos
```

## Documentation map

Start here when you need more detail:

- [Documentation Index](docs/README.md)
- [Bootstrap macOS Guide](docs/guides/bootstrap-macos.md)
- [CLI Reference](docs/reference/cli.md)
- [Configuration Reference](docs/reference/configuration.md)
- [Module and State Contract](docs/specs/modules.md)
- [macOS State Surfaces](docs/specs/macos-state-surfaces.md)
- [Discover Spec](docs/specs/discover.md)
- [Scheduling Spec](docs/specs/scheduling.md)
- [Secrets Spec](docs/specs/secrets.md)
- [Verification Matrix](docs/verification/matrix.md)

Legacy docs remain as compatibility pointers for one release cycle.
