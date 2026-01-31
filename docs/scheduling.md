# Scheduling: automated sync cadence

Goal: make “configs just propagate” without you thinking about it.

**Policy**
- Run a full `dot sync` every **30 minutes**
- Also run when the machine is **idle**
- On **shutdown/restart**, run a **fast flush** (best effort)
- Always provide a manual override: `dot sync now`

Why the special shutdown behavior?
- Operating systems (especially Windows) may terminate tasks during shutdown.
- A full pull/apply/push can be cut off, leaving half-finished state.
- A “flush” should prioritize **not losing local edits** over cross-machine convergence.

---

## Windows (Task Scheduler)

### Tasks to create

Create three user-level tasks (not admin) that run in your user context:

1) `dotstate - sync (interval)`
   - Trigger: At log on + repeat every 30 minutes
   - Action: `dot.exe sync`
   - Conditions: allow start on battery (optional), stop if running too long (e.g., 5 min)

2) `dotstate - sync (idle)`
   - Trigger: On idle
   - Action: `dot.exe sync`
   - Notes:
     - Task Scheduler has an “idle trigger” and also “idle conditions”.
       Prefer the idle trigger to actually *start* a run once the machine goes idle.

3) `dotstate - capture (shutdown)`
   - Trigger: On event (System log), user-initiated shutdown/restart events
   - Action: `dot.exe capture`
   - Why: Keep this fast. The next interval/idle sync will commit/push.

### Privileges
Run scheduled tasks without elevation.
- User config sync should not require admin.
- Admin-required system modules should be applied manually (or via an explicit elevated task you opt into later).

---

## macOS (launchd user agent)

Create a LaunchAgent:

- StartInterval: 1800 seconds (30 min)
- RunAtLoad: true
- ProgramArguments: `/path/to/dot sync`

Idle integration on macOS is non-trivial without additional tooling; treat it as a later enhancement.

Shutdown flush on macOS is also not guaranteed. Prefer:
- more frequent interval syncs
- and manual `dot sync now` when needed

---

## Linux (systemd user timer)

Create a systemd user service + timer:

- Timer:
  - OnBootSec=2m
  - OnUnitActiveSec=30m
  - AccuracySec=1m
- Service runs: `dot sync`

Idle integration can be added later via:
- systemd inhibitors
- desktop environment hooks
- or a lightweight idle detector

---

## Manual command

Always available:

```bash
dot sync now
```

Use this when you made a change you want on another machine immediately.
