# `dot sync` spec

`dot sync` is the core convergence operation. It is designed around the “edit real files” workflow:

- You edit real config files (destination files) normally.
- `dot sync` captures those edits back into the repo and propagates them across machines.

`dot sync` is intentionally conservative about conflicts: it will not attempt clever auto-resolution.

---

## What `dot sync` does (transaction)

A full `dot sync` runs these phases, in order:

1. **Capture**
   - Pulls changes from destination files back into the repo’s Chezmoi source state through the module orchestrator.
   - Current files-module implementation: `chezmoi --source <repo>/<chex.source_dir> re-add`.
   - `--dry-run` emits the shared module plan and does not run `re-add`.

2. **Commit**
   - If the repo has changes, create a local commit.
   - Default commit message: `dot sync from <hostname> at <RFC3339 timestamp>`

3. **Pull/Rebase**
   - Fetch remote changes and rebase local commits on top.
   - Default behavior uses `git pull --rebase --autostash` to reduce friction.

4. **Apply**
   - Apply desired state from repo to destination files through the module orchestrator.
   - Current files-module implementation: plan with `chezmoi diff`, backup managed destination files before writes, apply with `chezmoi apply`, then verify with a second diff.

5. **Push**
   - Push local commits to the configured remote branch.

This order is deliberate:
- Capturing first makes local edits explicit commits before the pull.
- Pulling before applying reduces “apply thrash” across machines.

---

## Flags

- `dot sync --no-apply`
  - Runs capture + commit + pull/rebase + push, but does not apply.
  - Useful when you only want to publish local edits without altering the current machine.

- `dot sync --no-push`
  - Runs capture + commit + pull/rebase + apply, but does not push.
  - Useful when you want to converge locally first or you’re offline.

- `dot sync --dry-run`
  - Refuses a dirty repo the same way normal sync does.
  - Emits capture/apply module plans without running `chezmoi re-add`, committing, pulling, applying, or pushing.

- `dot sync now`
  - Alias for `dot sync`.
  - Intended for “I need this on the other machine right now”.

---

## Failure modes and expected behavior

### Git conflicts
If pull/rebase results in conflicts:
- `dot sync` must stop and report:
  - which files are conflicted
  - how to resolve (open repo, resolve, `git rebase --continue`)
- It must not attempt automated conflict resolution.

### Dirty repo state
If the repo already has staged/untracked changes unrelated to dot operations:
- `dot sync` refuses before capture and prints the porcelain status plus recovery guidance.
- Use `dot capture` when you only want to update managed artifacts, or commit/stash unrelated repo work before syncing.
- Later versions may optionally stash/restore with careful scoping.

### Tool missing
If `git` or `chezmoi` are missing:
- `dot doctor` should catch it.
- `dot sync` should fail fast with a clear error.

---

## Relationship to scheduling

Scheduled sync should be safe and non-invasive:

- Interval runs (every 30 minutes) are full `dot sync`.
- Idle-triggered runs are full `dot sync`.
- Shutdown-triggered runs should be **fast** (see `docs/specs/scheduling.md`):
  - generally `dot capture` only (no pull/apply/push), because Windows may kill tasks during shutdown.

---

## Scope boundaries (important)

`dot sync` only manages artifacts that are:
- in the repo’s Chezmoi source state, and
- intended to be “managed” on this OS/host (via `.chezmoiignore` rules)

Secret-bearing generated files should not be part of the managed set.
Use include-file patterns (e.g. `.gitconfig.private`) or generate secret outputs under `state/private/`.
