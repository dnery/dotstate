# Stocktake Notes - 2026-02-07

## Checkpoint 1 - Repo and docs baseline

Commands run:
- `git status --short --branch`
- `git log --oneline --decorate -n 12`
- doc scans across `README.md`, `DOTSTATE-TECHNICAL-DESIGN.md`, and `docs/*.md`

Findings:
- Branch is clean: `master` tracking `origin/master`.
- Latest merged work is PRs #11, #12, #13 (subrepo manifest write, platform-injected discovery roots, discover->capture integration test).
- Docs still frame implementation as Phase 0-2 complete; Phase 3+ planned.
- `docs/discover.md` still contains unchecked checklist items around add behavior and secret rejection path.

## Checkpoint 2 - GitHub context ingestion (`gh`)

Commands run:
- `gh repo view dnery/dotstate --json ...`
- `gh issue list --repo dnery/dotstate --state open --limit 50 --json ...`
- `gh pr list --repo dnery/dotstate --state open --limit 50 --json ...`
- `gh pr list --repo dnery/dotstate --state all --limit 30 --search 'sort:updated-desc' --json ...`
- `gh api graphql ... discussions(...)`

Findings:
- Open issues: #5, #6, #7, #8, #9, #10 (all opened 2026-02-02).
- Open PR: #4 (docs restructuring + e2e harness); not merged.
- No repository discussions returned from GraphQL query.
- Could not read GitHub Projects/tasks: token lacks `read:project` scope.

## Checkpoint 3 - Verification runs

Commands run:
- `go test ./...`
- `go run ./cmd/dot version`
- `go run ./cmd/dot doctor`

Results:
- `go test ./...`: pass across all current test packages.
- `dot doctor`: passes on this macOS host with `git`, `chezmoi`, and `op` detected.

## Checkpoint 4 - Targeted behavior checks (discover)

Commands run:
- `go run ./cmd/dot discover --report` (real host)
- sandbox run with temp HOME + `.zshrc` only
- sandbox run with temp HOME + `.config/app/settings.json`

Verified behaviors:
- Shell dotfiles are still not surfaced in report output on this host.
- With only `.zshrc` in sandbox, discover reports no candidates.
- With JSON config file in sandbox, discover adds via chezmoi, but file lands in default source (`$HOME/.local/share/chezmoi/...`) instead of repo `home/`.

Implications:
- Issue #5 is still reproducible.
- Issue #6 is still reproducible (root cause appears tied to zero default `MaxFileSize` path through CLI options, not just classifier scoring).
- Issue #7 appears fixed in code and tests.
- Issue #9 appears still open by code inspection (`warning` mode not propagated to chezmoi add).

## Platform verification coverage snapshot

macOS Tahoe / M4 Max:
- Current host verifies compile, tests, doctor, and runtime discover behavior.
- Additional targeted tests still needed for `discover --yes` add-to-repo path and `--secrets` modes.

Windows 11 / Zen4:
- No native runtime execution in this session.
- Coverage currently limited to cross-platform Go tests and Windows-path discovery logic/unit tests.
- Needs a Windows-host smoke run (`doctor`, `discover --report`, `discover --yes --no-commit`) before calling support stable.

## Checkpoint 5 - Harness rework from open PR #4 direction

Commands/files:
- Added `test/e2e/discover_harness.sh` (sandboxed e2e routine with artifacts and optional asciinema recording/upload).
- Added `test/e2e/README.md`.
- Added Make targets: `test-e2e`, `test-e2e-record`.
- Added `state/e2e-runs/` to `.gitignore`.

Intent:
- Keep the useful harness portion from PR #4 while avoiding a broad docs migration merge.
- Produce inspectable output bundles (`summary.md`, `summary.json`, logs, optional `.cast`) for asynchronous review when away from the machine.
