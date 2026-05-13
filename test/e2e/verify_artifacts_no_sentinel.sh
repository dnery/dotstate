#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 || $# -gt 2 ]]; then
    echo "Usage: $0 OUT_DIR [SENTINEL]" >&2
    exit 2
fi

OUT_DIR="$1"
SENTINEL="${2:-DOTSTATE_TEST_SECRET_DO_NOT_PRINT}"

if [[ ! -d "$OUT_DIR" ]]; then
    echo "artifact directory not found" >&2
    exit 2
fi

matches=()
while IFS= read -r -d '' file; do
    matches+=("$file")
done < <(find "$OUT_DIR" -type f ! -path "$OUT_DIR/sandbox/*" -print0 | xargs -0 grep -I -l -F "$SENTINEL" 2>/dev/null | tr '\n' '\0' || true)

if [[ "${#matches[@]}" -gt 0 ]]; then
    echo "sentinel leak detected in generated artifact file(s):" >&2
    for file in "${matches[@]}"; do
        printf '  %s\n' "$file" >&2
    done
    exit 1
fi

echo "No sentinel leaks detected in generated artifacts."
