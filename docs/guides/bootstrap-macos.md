# Bootstrap macOS (Apple Silicon)

This guide brings a fresh macOS machine to a usable `dotstate` baseline with explicit manual checkpoints.

Manual checkpoints you should expect:
- Xcode Command Line Tools install prompt
- Homebrew installation if it is not already present
- 1Password desktop sign-in/unlock and CLI integration

---

## Target end state

After bootstrap:

- `dot` is installed and on PATH (default: `~/.local/bin/dot`).
- The dotstate repo is cloned/prepared.
- `git`, `chezmoi`, and optionally `op` are available for normal apply/sync flows.
- `dot doctor`, `dot apply --dry-run`, and `dot macos audit --json` are reachable without internal docs.
- Optional scheduled sync is available through `dot schedule install`.

---

## One-line bootstrap

```bash
curl -fsSL https://raw.githubusercontent.com/dnery/dotstate/master/scripts/bootstrap-macos.sh \
  | sh -s -- --repo https://github.com/dnery/dotstate
```

The script is intentionally conservative:

- supports macOS Apple Silicon (`darwin/arm64`)
- downloads `dot-darwin-arm64.tar.gz` from the latest GitHub Release
- installs `dot` to `${DOTSTATE_INSTALL_DIR:-$HOME/.local/bin}`
- checks Xcode Command Line Tools with `xcode-select -p`
- checks for Homebrew and prints the official setup path if missing
- treats 1Password/op unlock as a manual checkpoint; it does not fetch or print secrets
- runs `dot bootstrap --repo <url>` when `--repo` is provided

Dry-run the script before using it:

```bash
curl -fsSL https://raw.githubusercontent.com/dnery/dotstate/master/scripts/bootstrap-macos.sh \
  | sh -s -- --dry-run --repo https://github.com/dnery/dotstate
```

Useful overrides:

```bash
DOTSTATE_VERSION=vX.Y.Z                  # release tag; default latest
DOTSTATE_INSTALL_DIR="$HOME/.local/bin"   # install destination
DOTSTATE_RELEASE_REPO=dnery/dotstate      # release owner/repo
```

---

## Manual checkpoints

### Xcode Command Line Tools

If the script reports that Xcode Command Line Tools are missing, run:

```bash
xcode-select --install
```

Then re-run bootstrap.

### Homebrew

If Homebrew is missing, install it from <https://brew.sh>, then install required tools:

```bash
brew install git chezmoi
brew install --cask 1password
brew install --cask 1password-cli
```

### 1Password/op

Sign in to 1Password desktop, enable CLI integration, unlock it, and verify:

```bash
op account list
```

This is a deliberate manual checkpoint. dotstate must not print decrypted secret values during bootstrap.

---

## Validate after bootstrap

From the cloned repo:

```bash
dot doctor
dot apply --dry-run
dot sync --dry-run
dot macos audit --json
```

`dot macos audit --json` currently emits the stable audit envelope and capability diagnostics used by bootstrap. Full macOS surface collectors are planned in the read-only audit goal.

---

## Optional scheduled sync

Install the user LaunchAgent:

```bash
dot schedule install --dry-run
dot schedule install
```

Check or remove it:

```bash
dot schedule status
dot schedule remove
```

The LaunchAgent runs `dot --config <repo>/dot.toml sync` every `[sync].interval_minutes` minutes and at load. macOS shutdown flush is not installed because user LaunchAgents cannot guarantee a safe non-destructive shutdown hook. Use `dot sync now` before shutdown/restart when you need an explicit flush.
