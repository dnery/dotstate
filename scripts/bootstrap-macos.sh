#!/bin/sh
# One-line macOS bootstrap for dotstate Apple Silicon machines.
set -eu

OWNER_REPO="${DOTSTATE_RELEASE_REPO:-dnery/dotstate}"
VERSION="${DOTSTATE_VERSION:-latest}"
INSTALL_DIR="${DOTSTATE_INSTALL_DIR:-$HOME/.local/bin}"
REPO_URL="${DOTSTATE_REPO:-${DOTSTATE_REPO_URL:-}}"
DRY_RUN=0
SKIP_BOOTSTRAP=0

usage() {
  cat <<'EOF'
Usage: bootstrap-macos.sh [--repo URL] [--version TAG|latest] [--install-dir DIR] [--dry-run] [--skip-bootstrap]

Environment:
  DOTSTATE_RELEASE_REPO  GitHub owner/repo for release assets (default: dnery/dotstate)
  DOTSTATE_VERSION       Release tag or latest (default: latest)
  DOTSTATE_INSTALL_DIR   Install directory (default: ~/.local/bin)
  DOTSTATE_REPO          Dotstate repo URL passed to `dot bootstrap --repo`
EOF
}

safe() {
  printf '%s' "$1" | sed -E \
    -e 's#([A-Za-z][A-Za-z0-9+.-]*://)[^/@[:space:]]+@#\1<redacted:credential>@#g' \
    -e 's#DOTSTATE_TEST_SECRET_DO_NOT_PRINT#<redacted:secret>#g' \
    -e 's#(([Pp][Aa][Ss][Ss][Ww][Oo][Rr][Dd]|[Pp][Aa][Ss][Ss][Ww][Dd]|[Pp][Ww][Dd]|[Ss][Ee][Cc][Rr][Ee][Tt]|[Tt][Oo][Kk][Ee][Nn])[_-]?[A-Za-z0-9_-]*[=:])[^[:space:]]{8,}#<redacted:secret>#g'
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --repo)
      REPO_URL="${2:-}"
      shift 2
      ;;
    --version)
      VERSION="${2:-}"
      shift 2
      ;;
    --install-dir|--bin-dir)
      INSTALL_DIR="${2:-}"
      shift 2
      ;;
    --dry-run)
      DRY_RUN=1
      shift
      ;;
    --skip-bootstrap)
      SKIP_BOOTSTRAP=1
      shift
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "error: unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

if [ "$(uname -s)" != "Darwin" ]; then
  echo "error: this bootstrap script is for macOS" >&2
  exit 69
fi

ARCH="$(uname -m)"
if [ "$ARCH" != "arm64" ]; then
  echo "error: this bootstrap script currently supports Apple Silicon arm64 only (found $ARCH)" >&2
  exit 69
fi

ASSET="dot-darwin-arm64.tar.gz"
if [ "$VERSION" = "latest" ]; then
  DOWNLOAD_URL="https://github.com/${OWNER_REPO}/releases/latest/download/${ASSET}"
else
  DOWNLOAD_URL="https://github.com/${OWNER_REPO}/releases/download/${VERSION}/${ASSET}"
fi
DOT_BIN="$INSTALL_DIR/dot"

echo "dotstate macOS bootstrap"
echo "  release asset: $(safe "$DOWNLOAD_URL")"
echo "  install path:  $(safe "$DOT_BIN")"

if xcode-select -p >/dev/null 2>&1; then
  echo "  Xcode Command Line Tools: detected"
else
  echo "  Xcode Command Line Tools: manual checkpoint required"
  echo "    Run: xcode-select --install"
fi

if command -v brew >/dev/null 2>&1; then
  echo "  Homebrew: $(safe "$(command -v brew)")"
else
  echo "  Homebrew: not found"
  echo "    Install from https://brew.sh, then run: brew install git chezmoi"
fi

if command -v op >/dev/null 2>&1; then
  echo "  1Password/op checkpoint: unlock 1Password and verify with: op account list"
else
  echo "  1Password/op checkpoint: op not found; install 1Password CLI before applying secrets-backed state"
fi

if [ "$DRY_RUN" -eq 1 ]; then
  echo "dry-run: would download $(safe "$DOWNLOAD_URL")"
  echo "dry-run: would install dot to $(safe "$DOT_BIN")"
  if [ -n "$REPO_URL" ] && [ "$SKIP_BOOTSTRAP" -eq 0 ]; then
    echo "dry-run: would run $(safe "$DOT_BIN") bootstrap --repo $(safe "$REPO_URL")"
  fi
  echo "dry-run: verification commands after bootstrap: dot doctor; dot apply --dry-run; dot sync --dry-run; dot macos audit --json"
  exit 0
fi

TMPDIR="$(mktemp -d)"
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

mkdir -p "$INSTALL_DIR"
curl -fsSL "$DOWNLOAD_URL" -o "$TMPDIR/$ASSET"
tar -xzf "$TMPDIR/$ASSET" -C "$TMPDIR"
install -m 0755 "$TMPDIR/dot-darwin-arm64/dot" "$DOT_BIN"

echo "installed: $(safe "$DOT_BIN")"

if [ -n "$REPO_URL" ] && [ "$SKIP_BOOTSTRAP" -eq 0 ]; then
  "$DOT_BIN" bootstrap --repo "$REPO_URL"
else
  echo "next: run $(safe "$DOT_BIN") bootstrap --repo <repo-url>"
fi

echo "validation commands:"
echo "  dot doctor"
echo "  dot apply --dry-run"
echo "  dot sync --dry-run"
echo "  dot macos audit --json"
