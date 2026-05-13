# E2E Harnesses

This directory contains reviewer-oriented harnesses that generate reproducible bundles for async inspection.

## Harnesses

- `test/e2e/discover_harness.sh` (Unix/macOS/Linux)
- `test/e2e/discover_harness_windows.ps1` (Windows/PowerShell)

Both harnesses emit the same bundle schema:

- `summary.md` (human report)
- `summary.json` (machine-readable report)
- `timeline.md` (UTC command timeline)
- `environment.txt` (tool and platform context)
- `artifacts/commands/*.txt` (redacted outputs)
- optional `artifacts/session.cast` when recording is enabled

Raw logs are kept under `local-raw/` and are excluded from review bundle by default unless explicitly requested.

## Scenarios

- `discover-fast`
- `discover-deep`
- `discover-interactive`
- `capture-loop`
- `macos-verification` — sandbox-home apply/capture/idempotency, LaunchAgent install/status/remove on macOS, bootstrap dry-run, and sentinel leak verification.
- `all`

## Common Commands

```bash
# local build + all scenarios
make test-e2e

# specific scenario
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario discover-fast

# macOS verification bundle
make test-e2e-verify

# strict mode
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario all --strict

# record cast (local only)
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario all --record

# record and upload explicitly
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --scenario all --record --upload
```

```powershell
# windows parity harness
pwsh -File .\test\e2e\discover_harness_windows.ps1 -DotBin .\bin\dot.exe -Scenario all
```
