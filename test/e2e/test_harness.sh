#!/bin/bash
# dotstate end-to-end test harness
# This script creates a sandbox environment and tests dotstate functionality
#
# USAGE: ./test_harness.sh [path-to-dot-binary]

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test directories
export TEST_BASE="/tmp/dotstate-e2e-test-$$"
export FAKE_HOME="${TEST_BASE}/fake_home"
export DOTSTATE_REPO="${TEST_BASE}/dotstate_repo"
export DOT_BIN="${1:-/home/user/dotstate/bin/dot}"

# Bug tracking
BUGS_FOUND=()

log_bug() {
    BUGS_FOUND+=("$1")
    echo -e "${RED}[BUG] $1${NC}"
}

# Ensure we have the binary
if [ ! -x "$DOT_BIN" ]; then
    echo -e "${RED}Error: dot binary not found at $DOT_BIN${NC}"
    exit 1
fi

echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}dotstate E2E Test Harness${NC}"
echo -e "${CYAN}========================================${NC}"
echo ""
echo -e "${YELLOW}Test base directory: ${TEST_BASE}${NC}"
echo -e "${YELLOW}Fake home: ${FAKE_HOME}${NC}"
echo -e "${YELLOW}Dotstate repo: ${DOTSTATE_REPO}${NC}"
echo -e "${YELLOW}Binary: ${DOT_BIN}${NC}"
echo ""

# Cleanup function
cleanup() {
    echo ""
    echo -e "${YELLOW}Cleaning up test environment...${NC}"
    rm -rf "$TEST_BASE"
    echo -e "${GREEN}Cleanup complete${NC}"
}

# Set trap for cleanup on exit (comment out for debugging)
trap cleanup EXIT

# Step 1: Create test directories
echo -e "${CYAN}[Step 1] Creating test environment...${NC}"
mkdir -p "$FAKE_HOME"
mkdir -p "$DOTSTATE_REPO/home"
mkdir -p "$DOTSTATE_REPO/state"

# Step 2: Create mock home environment with config files
echo -e "${CYAN}[Step 2] Creating mock home environment with config files...${NC}"

# Create .config directory structure (common on Linux/macOS)
mkdir -p "$FAKE_HOME/.config/nvim"
mkdir -p "$FAKE_HOME/.config/alacritty"
mkdir -p "$FAKE_HOME/.config/starship"
mkdir -p "$FAKE_HOME/.ssh"

# Create various config files that should be discovered
cat > "$FAKE_HOME/.gitconfig" << 'EOF'
[user]
    name = Test User
    email = test@example.com
[core]
    editor = vim
[alias]
    st = status
    co = checkout
EOF

cat > "$FAKE_HOME/.config/nvim/init.lua" << 'EOF'
-- Neovim configuration
vim.opt.number = true
vim.opt.relativenumber = true
vim.opt.expandtab = true
vim.opt.shiftwidth = 4
vim.opt.tabstop = 4
EOF

cat > "$FAKE_HOME/.config/alacritty/alacritty.toml" << 'EOF'
[window]
opacity = 0.95
decorations = "full"

[font]
size = 12.0

[font.normal]
family = "JetBrains Mono"
EOF

cat > "$FAKE_HOME/.config/starship/starship.toml" << 'EOF'
format = "$all"

[character]
success_symbol = "[➜](bold green)"
error_symbol = "[✗](bold red)"

[git_branch]
symbol = "🌱 "
EOF

cat > "$FAKE_HOME/.zshrc" << 'EOF'
# Zsh configuration
export EDITOR=vim
export PATH="$HOME/.local/bin:$PATH"

# Aliases
alias ll='ls -la'
alias gs='git status'
EOF

cat > "$FAKE_HOME/.bashrc" << 'EOF'
# Bash configuration
export EDITOR=vim
export PATH="$HOME/.local/bin:$PATH"

# Aliases
alias ll='ls -la'
alias gs='git status'
EOF

# Create a file that should NOT be tracked (ssh private key pattern)
cat > "$FAKE_HOME/.ssh/id_rsa" << 'EOF'
-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAABlwAAAAdzc2gtcn
NhAAAAAwEAAQAAAYEA... (fake key for testing)
-----END OPENSSH PRIVATE KEY-----
EOF

# Create ssh config (should be trackable)
cat > "$FAKE_HOME/.ssh/config" << 'EOF'
Host github.com
    HostName github.com
    User git
    IdentityFile ~/.ssh/id_ed25519

Host *
    AddKeysToAgent yes
EOF

# Create some files that should be ignored (caches, logs)
mkdir -p "$FAKE_HOME/.cache/some_app"
echo "cache data" > "$FAKE_HOME/.cache/some_app/cache.db"

echo -e "${GREEN}Created $(find "$FAKE_HOME" -type f | wc -l) files in fake home${NC}"
echo ""
echo "Files created:"
find "$FAKE_HOME" -type f | sort | while read f; do
    size=$(stat -c%s "$f" 2>/dev/null || stat -f%z "$f" 2>/dev/null)
    echo "  $f ($size bytes)"
done
echo ""

# Step 3: Initialize dotstate repo
echo -e "${CYAN}[Step 3] Initializing dotstate repository...${NC}"

# Initialize git repo
cd "$DOTSTATE_REPO"
git init --initial-branch=main
git config user.email "test@example.com"
git config user.name "Test User"
# Disable commit signing for test environment
git config commit.gpgsign false
git config tag.gpgsign false

# Create minimal dot.toml
cat > "$DOTSTATE_REPO/dot.toml" << EOF
[repo]
url = "file://${DOTSTATE_REPO}"
path = "${DOTSTATE_REPO}"
branch = "main"

[sync]
interval_minutes = 30

[tools]
git = ""
chezmoi = ""

[chex]
source_dir = "home"
EOF

# Create .chezmoiroot for chezmoi
echo "home" > "$DOTSTATE_REPO/.chezmoiroot"

# Create initial commit
git add .
git commit -m "Initial dotstate repo setup"

echo -e "${GREEN}Repository initialized${NC}"
echo ""

# Step 4: Initialize chezmoi with our fake home as target
echo -e "${CYAN}[Step 4] Setting up environment...${NC}"
export HOME="$FAKE_HOME"
export XDG_CONFIG_HOME="$FAKE_HOME/.config"

# Note: chezmoi init sets up its OWN source dir, not necessarily the dotstate repo
# This is a potential mismatch we need to test

echo "Environment:"
echo "  HOME=$HOME"
echo "  XDG_CONFIG_HOME=$XDG_CONFIG_HOME"
echo ""

# Step 5: Test dot doctor
echo -e "${CYAN}[Step 5] Running 'dot doctor'...${NC}"
cd "$DOTSTATE_REPO"
"$DOT_BIN" doctor || true
echo ""

# Step 6: Test dot discover --report
echo -e "${CYAN}[Step 6] Running 'dot discover --report' to see what would be found...${NC}"
"$DOT_BIN" discover --report || true
echo ""

# Step 7: Run dot discover with --yes to auto-accept
echo -e "${CYAN}[Step 7] Running 'dot discover --yes --no-commit' to add recommended files...${NC}"
"$DOT_BIN" discover --yes --no-commit 2>&1 || true
echo ""

# Step 8: Check what was added to the repo
echo -e "${CYAN}[Step 8] Checking files added to dotstate repo (home/ directory)...${NC}"
echo "Files in home/:"
if [ -d "$DOTSTATE_REPO/home" ]; then
    file_count=$(find "$DOTSTATE_REPO/home" -type f 2>/dev/null | wc -l)
    if [ "$file_count" -eq 0 ]; then
        echo -e "${RED}  (empty - no files were added!)${NC}"
        log_bug "dot discover says files were added but home/ directory is empty"
    else
        find "$DOTSTATE_REPO/home" -type f 2>/dev/null | head -20
    fi
else
    echo -e "${RED}  home/ directory does not exist${NC}"
fi
echo ""

# Step 9: Check if chezmoi added files to ITS default location instead
echo -e "${CYAN}[Step 9] Checking chezmoi's default source location...${NC}"
CHEZMOI_SOURCE="$FAKE_HOME/.local/share/chezmoi"
if [ -d "$CHEZMOI_SOURCE" ]; then
    chezmoi_file_count=$(find "$CHEZMOI_SOURCE" -type f 2>/dev/null | wc -l)
    if [ "$chezmoi_file_count" -gt 0 ]; then
        echo -e "${YELLOW}Found $chezmoi_file_count files in chezmoi's default source: $CHEZMOI_SOURCE${NC}"
        find "$CHEZMOI_SOURCE" -type f 2>/dev/null | head -10
        log_bug "chezmoi add uses its default source (~/.local/share/chezmoi) instead of repo's home/"
    fi
else
    echo "Chezmoi default source does not exist (this is expected if properly configured)"
fi
echo ""

# Step 10: Verify files are copies, not symlinks
echo -e "${CYAN}[Step 10] Verifying copy semantics (not symlinks)...${NC}"
SYMLINK_COUNT=0
FILE_COUNT=0

# Check both possible locations
for check_dir in "$DOTSTATE_REPO/home" "$CHEZMOI_SOURCE"; do
    if [ -d "$check_dir" ]; then
        for f in $(find "$check_dir" -type f 2>/dev/null); do
            FILE_COUNT=$((FILE_COUNT + 1))
            if [ -L "$f" ]; then
                echo -e "${RED}SYMLINK FOUND: $f${NC}"
                SYMLINK_COUNT=$((SYMLINK_COUNT + 1))
            fi
        done
    fi
done

if [ $FILE_COUNT -eq 0 ]; then
    echo -e "${YELLOW}No files found in either location to check${NC}"
elif [ $SYMLINK_COUNT -eq 0 ]; then
    echo -e "${GREEN}✓ All $FILE_COUNT files are copies (no symlinks)${NC}"
else
    echo -e "${RED}✗ Found $SYMLINK_COUNT symlinks out of $FILE_COUNT files${NC}"
    log_bug "Found symlinks instead of copies"
fi
echo ""

# Step 11: Commit whatever was added
echo -e "${CYAN}[Step 11] Committing any added files...${NC}"
cd "$DOTSTATE_REPO"
git add -A
if git diff --staged --quiet; then
    echo "No changes to commit"
else
    git status --short
    git commit -m "Add discovered config files" || echo "Commit failed"
fi
echo ""

# Step 12: Make a change to a tracked file (destination)
echo -e "${CYAN}[Step 12] Modifying a tracked file at destination...${NC}"
if [ -f "$FAKE_HOME/.gitconfig" ]; then
    echo "" >> "$FAKE_HOME/.gitconfig"
    echo "# Added by test harness at $(date)" >> "$FAKE_HOME/.gitconfig"
    echo "Modified: $FAKE_HOME/.gitconfig"
else
    echo -e "${YELLOW}No .gitconfig found to modify${NC}"
fi
echo ""

# Step 13: Test if the system can detect the change
echo -e "${CYAN}[Step 13] Testing change detection with 'dot capture'...${NC}"
"$DOT_BIN" capture 2>&1 || true
echo ""

# Step 14: Check if the change was captured
echo -e "${CYAN}[Step 14] Checking if change was captured in repo...${NC}"
cd "$DOTSTATE_REPO"
if git diff --quiet && git diff --staged --quiet; then
    echo -e "${YELLOW}No changes detected in repo (git diff is clean)${NC}"
    # Check if the file even exists in the repo
    if [ ! -f "$DOTSTATE_REPO/home/dot_gitconfig" ] && [ ! -f "$DOTSTATE_REPO/home/.gitconfig" ]; then
        log_bug "Change to .gitconfig not captured - file was never added to repo"
    else
        log_bug "Change to .gitconfig not captured despite file being in repo"
    fi
else
    echo -e "${GREEN}✓ Changes detected in repo:${NC}"
    git diff --stat
    git diff --staged --stat
fi
echo ""

# Summary
echo -e "${CYAN}========================================${NC}"
echo -e "${CYAN}Test Summary${NC}"
echo -e "${CYAN}========================================${NC}"
echo -e "Fake home files:           $(find "$FAKE_HOME" -type f 2>/dev/null | wc -l)"
echo -e "Repo home/ files:          $(find "$DOTSTATE_REPO/home" -type f 2>/dev/null | wc -l)"
echo -e "Chezmoi default files:     $(find "$CHEZMOI_SOURCE" -type f 2>/dev/null | wc -l)"
echo -e "Symlinks found:            $SYMLINK_COUNT"
echo ""

if [ ${#BUGS_FOUND[@]} -gt 0 ]; then
    echo -e "${RED}========================================${NC}"
    echo -e "${RED}BUGS FOUND: ${#BUGS_FOUND[@]}${NC}"
    echo -e "${RED}========================================${NC}"
    for bug in "${BUGS_FOUND[@]}"; do
        echo -e "${RED}  - $bug${NC}"
    done
else
    echo -e "${GREEN}No bugs found!${NC}"
fi
echo ""

echo -e "${GREEN}Test harness complete!${NC}"
