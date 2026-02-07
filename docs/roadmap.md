# Roadmap

## Implementation Status

- Phase 0: repo hygiene and docs - complete.
- Phase 1: core plumbing (runner/config/logging/platform/errors) - complete.
- Phase 2: discover baseline - complete.
- Phase 3+: discover UX, capture/apply/sync hardening, scheduling, platform modules - in progress/planned.

## Current Workstream (2026-02-07)

1. Documentation consolidation to canonical `docs/` hierarchy.
2. Deep e2e harness and visualization bundles for async review.
3. Fixes for discover defects tracked by issues #5, #6, #9, #10, with explicit validation for #7 and #8.
4. Verification matrix for macOS Tahoe (M4 Max) and Windows 11 (Zen4).

## Active Risk List

- Discover add path can write to default chezmoi source if source dir is not enforced.
- Discover option defaulting can unintentionally suppress shell dotfiles.
- Secret mode handling must match declared CLI contract.
- Documentation drift is possible without a docs consistency check.
