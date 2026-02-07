# Non-Trivial Configuration Tracking

This document covers configuration items that require special handling beyond simple file copying. These items are documented here for future implementation consideration.

---

## Firefox Profile Tracking

### Challenge

Firefox profiles contain a mix of:
1. **User preferences** (`prefs.js`, `user.js`) - Trackable
2. **Extension settings** - Partially trackable
3. **Browser state** (history, bookmarks, sessions) - Not suitable for tracking
4. **Large binary data** (cache, databases) - Excluded

### Profile Location

| OS | Path |
|----|------|
| macOS | `~/Library/Application Support/Firefox/Profiles/*.default-release/` |
| Windows | `%APPDATA%\Mozilla\Firefox\Profiles\*.default-release\` |
| Linux | `~/.mozilla/firefox/*.default-release/` |

### What to Track

**Trackable files:**
- `user.js` - Custom user preferences (best practice: create this file manually)
- `chrome/userChrome.css` - Custom UI styling
- `chrome/userContent.css` - Custom page styling
- `search.json.mozlz4` - Search engine configuration
- `handlers.json` - File handler associations
- `permissions.sqlite` - Site permissions (consider privacy implications)

**Do NOT track:**
- `prefs.js` - Auto-modified by Firefox, causes merge conflicts
- `places.sqlite` - Browsing history (privacy, size)
- `cookies.sqlite` - Cookies (privacy)
- `key*.db`, `logins.json` - Credentials (security)
- `*.lz4` files - Compressed cache (size)
- `storage/` - Extension local storage (varies per machine)

### Recommended Approach

1. **Create `user.js` manually**: This file is read on startup and overrides `prefs.js`. Put your preferences here.

2. **Use Firefox Sync**: Let Mozilla handle bookmarks, history, tabs, and passwords.

3. **Export extension list**: Use about:support or an extension like "Extension List Exporter" to save a list that can be reinstalled.

4. **Track only customizations**: Track `user.js` and `chrome/` directory if you use userChrome customizations.

### Future Implementation

```toml
# Possible configuration in dot.toml
[firefox]
# Track user.js if it exists (must be created manually)
track_user_js = true

# Track userChrome customizations
track_userchrome = true

# Export extension list on sync (read-only)
export_extensions = true
```

---

## Sub-Repository Tracking

### Challenge

Some configuration directories are their own git repositories (e.g., `~/.config/nvim` cloned from `user/nvim-config`). These should not be tracked as files, but as references.

### Current Implementation

The discover command detects sub-repositories and extracts:
- Repository path relative to home
- Remote URL (if available)
- Current branch

Sub-repos are tracked in `state/subrepos.toml`:

```toml
[[subrepo]]
path = "~/.config/nvim"
url = "https://github.com/user/nvim-config"
branch = "main"

[[subrepo]]
path = "~/.emacs.d"
url = "https://github.com/user/emacs-config"
```

### Handling During Apply

**Option A: Clone if missing, skip if present**
- Simplest approach
- User manages updates manually
- No conflicts with local changes

**Option B: Clone if missing, pull on sync**
- More automated
- Risk of conflicts with local changes
- Could use `--autostash`

**Option C: Clone if missing, status check on sync**
- Warn if sub-repo has uncommitted changes or is behind remote
- User decides how to proceed

### Recommended Approach

Start with **Option A** (clone if missing, skip if present). The user already manages these repos manually, so we shouldn't override that workflow.

Later, add a `dot subrepo status` command to check all sub-repos for uncommitted changes or out-of-date status.

---

## macOS Defaults Tracking

### Challenge

Many macOS settings are stored in the `defaults` system (plists) rather than dotfiles. These are accessed via:
```bash
defaults read com.apple.dock
defaults write com.apple.dock autohide -bool true
```

### Approach Options

**Option A: Defaults script**
- Maintain a `defaults-macos.sh` script with all desired settings
- Run during bootstrap
- Pros: Simple, explicit
- Cons: Not bidirectional (can't capture changes)

**Option B: Plist diffing**
- Export current defaults: `defaults export com.apple.dock -`
- Compare against tracked version
- Highlight differences
- Pros: Can detect drift
- Cons: Plists contain cache/transient data

**Option C: Curated settings list**
- Define a list of specific preferences to track
- Export only those during capture
- Apply only those during apply
- Pros: Clean, focused
- Cons: Manual curation required

### Recommended Approach

Start with **Option A** (defaults script). Create `targets/macos/defaults.sh` with curated settings. Idempotent (safe to run multiple times).

Later, add `dot defaults diff` to compare current settings against the script.

---

## Windows Registry Tracking

### Challenge

Windows stores many settings in the registry. Registry changes need:
- Elevation for HKLM (system) keys
- Care with user vs. system scope
- Export/import tooling

### Approach Options

**Option A: Reg files**
- Export: `reg export HKEY_CURRENT_USER\...\key export.reg`
- Import: `reg import export.reg`
- Pros: Native Windows tooling
- Cons: Machine-specific paths, security concerns

**Option B: PowerShell scripts**
- Set values explicitly with `Set-ItemProperty`
- Pros: Idempotent, scriptable
- Cons: More verbose

**Option C: Curated key list**
- Define keys to track in a manifest
- Export on capture, apply on import
- Pros: Focused, avoids junk
- Cons: Manual curation

### Recommended Approach

Start with **Option B** (PowerShell scripts). Create `targets/windows/registry.ps1` with idempotent settings. Use `Set-ItemProperty -Path ... -Name ... -Value ...` pattern.

---

## Browser Extension Settings

### Challenge

Browser extensions store settings in:
- Browser sync (if extension supports it)
- Extension local storage (SQLite/LevelDB)
- Extension-specific export/import features

### Per-Extension Considerations

| Extension | Sync Support | Export Support | Recommendation |
|-----------|--------------|----------------|----------------|
| uBlock Origin | Yes (cloud sync) | Yes (settings backup) | Use built-in export/import |
| Bitwarden | Yes (account) | N/A | Account-based, no local tracking |
| 1Password | Yes (account) | N/A | Account-based, no local tracking |
| Vimium | No | Yes (options export) | Track exported settings file |
| Dark Reader | Yes (Firefox Sync) | Yes (export) | Use Firefox Sync or export |

### Recommended Approach

1. **Rely on browser sync** for extensions that support it
2. **Export settings manually** for extensions with export features
3. **Store exports** in `state/extensions/` (tracked but manually updated)
4. **Document extension list** for bootstrapping new machines

---

## Summary: Implementation Priority

| Feature | Complexity | Value | Priority |
|---------|------------|-------|----------|
| Sub-repos (clone if missing) | Low | High | P1 |
| macOS defaults script | Low | Medium | P2 |
| Windows registry script | Low | Medium | P2 |
| Firefox user.js tracking | Low | Medium | P2 |
| Extension settings export | Medium | Low | P3 |
| Bidirectional defaults sync | High | Low | P4 |

---

## Open Questions

1. **Firefox profile detection**: Should we auto-detect the default profile or require explicit configuration?

2. **Sub-repo conflicts**: How to handle when a sub-repo has local changes during `dot apply`?

3. **macOS defaults scope**: Which defaults domains are worth tracking? Need curation.

4. **Windows elevation**: How to handle registry keys that require admin rights?

5. **Extension tracking granularity**: Track all extensions or curated list?
