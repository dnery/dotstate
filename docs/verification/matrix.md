# Verification Matrix

This matrix tracks reproducible validation for priority targets.

## Target Platforms

- macOS Tahoe (Apple Silicon, M4 Max)
- Windows 11 (Zen4)

## Latest Runs

| Date (UTC) | Platform | Scope | Result | Evidence |
|---|---|---|---|---|
| 2026-02-07 | macOS (darwin/arm64, M4 Max) | `go test ./...`, `make docs-check`, `./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario all --no-delay --strict`, `make test-e2e` | pass | `state/e2e-runs/20260207T184438Z/summary.md`, `state/e2e-runs/20260207T184530Z/summary.md` |
| 2026-02-07 | Windows 11 (Zen4) | pending native smoke | pending | to be captured by `discover_harness_windows.ps1` |

## Required Commands

### macOS

```bash
go test ./...
make docs-check
make test-e2e
```

### Windows

```powershell
go test ./...
pwsh -File .\test\e2e\discover_harness_windows.ps1 -DotBin .\bin\dot.exe -Scenario all
```

## Notes

- Use `--strict` when validating candidate release readiness.
- Use `--record` for an asciinema cast; `--upload` remains opt-in.
