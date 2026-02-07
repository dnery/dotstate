# Agent Working Notes for `dotstate`

This file is a lightweight local bootstrap for agents working in this repo.

## Current Goal

Verify that implemented functionality actually works end-to-end before expanding scope.

Primary support targets right now:
- macOS Tahoe on Apple Silicon (M4 Max)
- Windows 11 on Zen4

## Working Priorities

1. Correctness and reproducibility over speed.
2. Tests and command verification over assumption.
3. Clear gap reporting (implemented vs planned vs broken).

## Stocktake Checklist

Run this sequence at the start of a session:

1. Local state:
   - `git status --short --branch`
   - `git log --oneline --decorate -n 12`
2. Docs ingestion:
   - `README.md`
   - `DOTSTATE-TECHNICAL-DESIGN.md`
   - `docs/*.md`
3. GitHub context (`gh`):
   - `gh issue list --repo dnery/dotstate --state open --limit 50`
   - `gh pr list --repo dnery/dotstate --state open --limit 50`
   - `gh pr list --repo dnery/dotstate --state all --limit 30 --search 'sort:updated-desc'`
4. Verification baseline:
   - `go test ./...`
   - `go run ./cmd/dot doctor`
   - `go run ./cmd/dot discover --report` (careful: output may include sensitive matches)

## Note-Taking Convention

- Keep a dated stocktake note in `docs/stocktake-YYYY-MM-DD.md`.
- Append checkpoints with:
  - what was checked,
  - commands run,
  - result,
  - unresolved risks.

## Known Validation Focus Areas (as of 2026-02-07)

- Discover add path must use repo `home/` source dir, not default chezmoi source.
- Discover defaults must include sensible `MaxFileSize` and keep shell dotfiles discoverable.
- `--secrets` behavior should be consistent with declared modes.
- Sub-repo manifest persistence should keep passing tests (`state/subrepos.toml` write path).
