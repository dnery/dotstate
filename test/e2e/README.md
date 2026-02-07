# E2E Harness

This directory contains reproducible command-line harnesses for validating discover/capture behavior in an isolated sandbox.

## Discover Harness

Script:
- `test/e2e/discover_harness.sh`

What it does:
1. Creates a temporary fake home and temp repo under `state/e2e-runs/<timestamp>/sandbox/`.
2. Seeds representative files (`.gitconfig`, shell dotfiles, `.config` JSON, risky SSH key path).
3. Runs `dot doctor`, `dot discover --report`, `dot discover --yes --no-commit`, and `dot capture`.
4. Produces machine-readable and human-readable results.

Artifacts per run:
- `state/e2e-runs/<timestamp>/summary.md`
- `state/e2e-runs/<timestamp>/summary.json`
- `state/e2e-runs/<timestamp>/run.log`
- command-specific outputs (`doctor.txt`, `discover-report.txt`, etc.)
- optional `session.cast` when recording is enabled

## Common Commands

```bash
# Build local binary
make build-local

# Run harness
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --interactive-demo

# Record with asciinema
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --record --interactive-demo

# Record and upload
./test/e2e/discover_harness.sh --dot-bin ./bin/dot --record --upload --interactive-demo
```

You can also use:
- `make test-e2e`
- `make test-e2e-record`
