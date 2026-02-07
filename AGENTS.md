# Dotstate Agent Contract

Global behavior (interaction mode, safety, external CLI harness) comes from `/Users/danilo.nery/.codex/AGENTS.md`.
This file contains dotstate-specific constraints only.

## Repo Hard Constraints

- Prioritize correctness/reproducibility over speed.
- Verify behavior end-to-end before expanding scope.
- Keep gap reporting explicit: implemented vs planned vs broken.

## Session Baseline

Run at session start:

1. `git status --short --branch`
2. `git log --oneline --decorate -n 12`
3. `go test ./...`
4. `go run ./cmd/dot doctor`
5. `go run ./cmd/dot discover --report` (can surface sensitive matches)

## Validation Bundles

- Fast smoke: `make test-e2e-fast`
- Full strict: `./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario all --strict`

Use `state/e2e-runs/<timestamp>/` outputs (`summary.md`, `summary.json`, `timeline.md`, `environment.txt`) as verification artifacts.

## Working Notes

- Keep dated stocktake notes in `docs/stocktake-YYYY-MM-DD.md`.
- Append commands run, results, and unresolved risks.
