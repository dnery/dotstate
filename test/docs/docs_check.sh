#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT_DIR"

fail() {
    echo "[FAIL] $1" >&2
    exit 1
}

required_files=(
    "docs/README.md"
    "docs/architecture.md"
    "docs/roadmap.md"
    "docs/reference/cli.md"
    "docs/reference/configuration.md"
    "docs/specs/discover.md"
    "docs/specs/sync.md"
    "docs/specs/scheduling.md"
    "docs/specs/secrets.md"
    "docs/specs/advanced-tracking.md"
    "docs/guides/bootstrap-macos.md"
    "docs/guides/bootstrap-windows.md"
    "docs/guides/bootstrap-linux.md"
    "docs/guides/wsl-nixos.md"
)

legacy_pointer_files=(
    "DOTSTATE-TECHNICAL-DESIGN.md"
    "docs/bootstrap-macos.md"
    "docs/bootstrap-windows.md"
    "docs/bootstrap-linux.md"
    "docs/wsl-nixos.md"
    "docs/discover.md"
    "docs/sync.md"
    "docs/scheduling.md"
    "docs/secrets-1password.md"
    "docs/non-trivial-tracking.md"
    "docs/spec.md"
)

echo "Checking canonical docs files..."
for file in "${required_files[@]}"; do
    [[ -f "$file" ]] || fail "Missing required canonical file: $file"
done

echo "Checking compatibility pointers..."
for file in "${legacy_pointer_files[@]}"; do
    [[ -f "$file" ]] || fail "Missing legacy pointer file: $file"
    rg -qi "compatibility pointer|deprecated" "$file" || fail "Legacy pointer missing deprecation language: $file"
done

echo "Checking docs index references..."
rg -q "docs/reference/cli\.md|reference/cli\.md" docs/README.md || fail "docs/README.md missing CLI reference link"
rg -q "docs/specs/discover\.md|specs/discover\.md" docs/README.md || fail "docs/README.md missing discover spec link"
rg -q "docs/README\.md" README.md || fail "README.md missing canonical docs link"

is_ignored_link() {
    local target="$1"
    [[ "$target" == "" ]] && return 0
    [[ "$target" =~ ^https?:// ]] && return 0
    [[ "$target" =~ ^mailto: ]] && return 0
    [[ "$target" =~ ^# ]] && return 0
    return 1
}

echo "Checking local markdown links..."
mapfile -t markdown_files < <(find README.md docs test/e2e -type f -name '*.md' | sort)

for md in "${markdown_files[@]}"; do
    while IFS= read -r link; do
        target="$(echo "$link" | sed -E 's/.*\]\(([^)]+)\).*/\1/')"
        target="${target%%#*}"

        if is_ignored_link "$target"; then
            continue
        fi

        base_dir="$(dirname "$md")"
        resolved="$base_dir/$target"
        if [[ ! -e "$resolved" ]]; then
            fail "Broken local link in $md -> $target"
        fi
    done < <(rg -No '\[[^]]+\]\(([^)]+)\)' "$md" || true)
done

echo "[PASS] docs-check"
