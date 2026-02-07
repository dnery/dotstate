#!/usr/bin/env bash
# Discover workflow harness with optional asciinema capture.
set -euo pipefail

usage() {
    cat <<'EOF'
Usage:
  test/e2e/discover_harness.sh [options]

Options:
  --dot-bin PATH         Path to dot binary (default: ./bin/dot)
  --out-dir PATH         Output directory (default: state/e2e-runs/<timestamp>)
  --record               Record the run with asciinema (writes session.cast)
  --upload               Upload recording after --record
  --interactive-demo     Run scripted interactive discover flow
  --no-delay             Disable step delays
  --strict               Exit non-zero if any check fails
  --help                 Show this help

Examples:
  test/e2e/discover_harness.sh --dot-bin ./bin/dot
  test/e2e/discover_harness.sh --record --upload --dot-bin ./bin/dot
EOF
}

DOT_BIN="./bin/dot"
RECORD=false
UPLOAD=false
INTERACTIVE_DEMO=false
NO_DELAY=false
STRICT=false
OUT_DIR=""

while [[ $# -gt 0 ]]; do
    case "$1" in
        --dot-bin)
            DOT_BIN="${2:-}"
            shift 2
            ;;
        --out-dir)
            OUT_DIR="${2:-}"
            shift 2
            ;;
        --record)
            RECORD=true
            shift
            ;;
        --upload)
            UPLOAD=true
            shift
            ;;
        --interactive-demo)
            INTERACTIVE_DEMO=true
            shift
            ;;
        --no-delay)
            NO_DELAY=true
            shift
            ;;
        --strict)
            STRICT=true
            shift
            ;;
        --help|-h)
            usage
            exit 0
            ;;
        *)
            echo "Unknown option: $1" >&2
            usage
            exit 2
            ;;
    esac
done

timestamp_utc() {
    date -u +"%Y%m%dT%H%M%SZ"
}

if [[ -z "$OUT_DIR" ]]; then
    OUT_DIR="state/e2e-runs/$(timestamp_utc)"
fi

mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

if [[ "$RECORD" == true && -z "${ASCIINEMA_REC:-}" ]]; then
    if ! command -v asciinema >/dev/null 2>&1; then
        echo "asciinema is required for --record" >&2
        exit 1
    fi

    cast_file="$OUT_DIR/session.cast"
    rec_cmd=(
        "$0"
        "--dot-bin" "$DOT_BIN"
        "--out-dir" "$OUT_DIR"
    )
    if [[ "$NO_DELAY" == true ]]; then
        rec_cmd+=("--no-delay")
    fi
    if [[ "$INTERACTIVE_DEMO" == true ]]; then
        rec_cmd+=("--interactive-demo")
    fi
    if [[ "$STRICT" == true ]]; then
        rec_cmd+=("--strict")
    fi

    quoted_cmd=""
    for part in "${rec_cmd[@]}"; do
        quoted_cmd+=$(printf "%q " "$part")
    done
    quoted_cmd="${quoted_cmd% }"

    echo "Recording to $cast_file"
    ASCIINEMA_REC=1 asciinema rec \
        --stdin \
        --overwrite \
        --title "dotstate discover harness $(date -u +%F)" \
        --command "$quoted_cmd" \
        "$cast_file"

    if [[ "$UPLOAD" == true ]]; then
        echo "Uploading recording..."
        asciinema upload "$cast_file" | tee "$OUT_DIR/upload.txt"
    fi
    exit 0
fi

run_log="$OUT_DIR/run.log"
summary_md="$OUT_DIR/summary.md"
summary_json="$OUT_DIR/summary.json"
doctor_out="$OUT_DIR/doctor.txt"
report_out="$OUT_DIR/discover-report.txt"
discover_yes_out="$OUT_DIR/discover-yes.txt"
capture_out="$OUT_DIR/capture.txt"
interactive_out="$OUT_DIR/discover-interactive.txt"

sandbox="$OUT_DIR/sandbox"
fake_home="$sandbox/home"
repo_dir="$sandbox/repo"
chezmoi_default="$fake_home/.local/share/chezmoi"

mkdir -p "$sandbox"
touch "$run_log"
exec > >(tee -a "$run_log") 2>&1

sleep_if_enabled() {
    if [[ "$NO_DELAY" == false ]]; then
        sleep 1
    fi
}

step() {
    echo
    echo "==> $1"
}

declare -a CHECK_NAMES CHECK_STATUS CHECK_NOTES
record_check() {
    local name="$1"
    local status="$2"
    local note="$3"
    CHECK_NAMES+=("$name")
    CHECK_STATUS+=("$status")
    CHECK_NOTES+=("$note")
}

abs_dot_bin="$(cd "$(dirname "$DOT_BIN")" && pwd)/$(basename "$DOT_BIN")"
if [[ ! -x "$abs_dot_bin" ]]; then
    echo "dot binary not executable at $abs_dot_bin" >&2
    echo "Run: make build-local" >&2
    exit 1
fi

step "Preparing sandbox"
rm -rf "$sandbox"
mkdir -p "$fake_home/.config/app" "$fake_home/.ssh" "$repo_dir/home" "$repo_dir/state"
sleep_if_enabled

step "Creating sample files"
cat > "$fake_home/.gitconfig" <<'EOF'
[user]
    name = Harness User
    email = harness@example.com
EOF

cat > "$fake_home/.zshrc" <<'EOF'
export EDITOR=vim
alias ll='ls -la'
EOF

cat > "$fake_home/.bashrc" <<'EOF'
export EDITOR=vim
alias gs='git status'
EOF

cat > "$fake_home/.config/app/settings.json" <<'EOF'
{"theme":"dark","fontSize":14}
EOF

cat > "$fake_home/.ssh/id_rsa" <<'EOF'
-----BEGIN OPENSSH PRIVATE KEY-----
FAKE-KEY-MATERIAL
-----END OPENSSH PRIVATE KEY-----
EOF
sleep_if_enabled

step "Initializing temporary repo"
git -C "$repo_dir" init --initial-branch=main
git -C "$repo_dir" config user.name "Harness User"
git -C "$repo_dir" config user.email "harness@example.com"
git -C "$repo_dir" config commit.gpgsign false

cat > "$repo_dir/dot.toml" <<EOF
[repo]
url = "file://$repo_dir"
path = "$repo_dir"
branch = "main"

[sync]
interval_minutes = 30
enable_idle = true
enable_shutdown = true

[tools]
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"

[wsl]
enable = false
distro_name = ""
flake_ref = ""
EOF

git -C "$repo_dir" add dot.toml home state
git -C "$repo_dir" commit -m "harness: initialize sandbox repo"
sleep_if_enabled

run_env() {
    HOME="$fake_home" XDG_CONFIG_HOME="$fake_home/.config" "$@"
}

step "Running doctor"
if run_env "$abs_dot_bin" doctor --config "$repo_dir/dot.toml" --repo-dir "$repo_dir" > "$doctor_out" 2>&1; then
    record_check "doctor exits cleanly" "PASS" "doctor output in doctor.txt"
else
    record_check "doctor exits cleanly" "FAIL" "doctor failed; inspect doctor.txt"
fi

step "Running discover --report"
if run_env "$abs_dot_bin" discover --config "$repo_dir/dot.toml" --repo-dir "$repo_dir" --report > "$report_out" 2>&1; then
    record_check "discover --report exits cleanly" "PASS" "report in discover-report.txt"
else
    record_check "discover --report exits cleanly" "FAIL" "discover report failed"
fi

if rg -q "~/.gitconfig|~/.zshrc|~/.bashrc" "$report_out"; then
    record_check "shell dotfiles appear in report" "PASS" "dotfiles were listed"
else
    record_check "shell dotfiles appear in report" "FAIL" "dotfiles missing from report"
fi

step "Running discover --yes --no-commit"
discover_yes_ok=false
if run_env "$abs_dot_bin" discover --config "$repo_dir/dot.toml" --repo-dir "$repo_dir" --yes --no-commit > "$discover_yes_out" 2>&1; then
    discover_yes_ok=true
    record_check "discover --yes exits cleanly" "PASS" "discover-yes.txt captured"
else
    record_check "discover --yes exits cleanly" "FAIL" "discover --yes failed"
fi

repo_home_files="$(find "$repo_dir/home" -type f | wc -l | tr -d ' ')"
chez_default_files="0"
if [[ -d "$chezmoi_default" ]]; then
    chez_default_files="$(find "$chezmoi_default" -type f | wc -l | tr -d ' ')"
fi

if [[ "$discover_yes_ok" == false ]]; then
    record_check "discover adds into repo home/" "SKIP" "discover --yes failed before add checks"
    record_check "discover avoids default chezmoi source" "SKIP" "discover --yes failed before add checks"
elif [[ "$repo_home_files" -gt 0 ]]; then
    record_check "discover adds into repo home/" "PASS" "repo home file count: $repo_home_files"
else
    record_check "discover adds into repo home/" "FAIL" "repo home file count: $repo_home_files"
fi

if [[ "$discover_yes_ok" == false ]]; then
    :
elif [[ "$chez_default_files" -eq 0 ]]; then
    record_check "discover avoids default chezmoi source" "PASS" "default chezmoi file count: 0"
else
    record_check "discover avoids default chezmoi source" "FAIL" "default chezmoi file count: $chez_default_files"
fi

symlink_count="0"
symlink_scan_roots=("$repo_dir/home")
if [[ -d "$chezmoi_default" ]]; then
    symlink_scan_roots+=("$chezmoi_default")
fi
symlink_count="$(find "${symlink_scan_roots[@]}" -type l 2>/dev/null | wc -l | tr -d ' ')"
if [[ "$symlink_count" -eq 0 ]]; then
    record_check "copy semantics (no symlinks)" "PASS" "symlink count: 0"
else
    record_check "copy semantics (no symlinks)" "FAIL" "symlink count: $symlink_count"
fi

step "Modifying destination and running capture"
echo "# modified by harness at $(date -u +%FT%TZ)" >> "$fake_home/.gitconfig"
if run_env "$abs_dot_bin" capture --config "$repo_dir/dot.toml" --repo-dir "$repo_dir" > "$capture_out" 2>&1; then
    record_check "capture exits cleanly" "PASS" "capture output in capture.txt"
else
    record_check "capture exits cleanly" "FAIL" "capture command failed"
fi

if [[ -n "$(git -C "$repo_dir" status --porcelain)" ]]; then
    record_check "capture creates repo diff after edit" "PASS" "repo has changes after capture"
else
    record_check "capture creates repo diff after edit" "FAIL" "repo stayed clean after capture"
fi

if [[ "$INTERACTIVE_DEMO" == true ]]; then
    step "Running scripted interactive discover demo"
    printf '\n\n' | run_env "$abs_dot_bin" discover --config "$repo_dir/dot.toml" --repo-dir "$repo_dir" --no-commit > "$interactive_out" 2>&1 || true
    if rg -q "Selected:|Commands:|Add .* files" "$interactive_out"; then
        record_check "scripted interactive flow captured" "PASS" "interactive transcript in discover-interactive.txt"
    else
        record_check "scripted interactive flow captured" "FAIL" "interactive transcript did not contain expected prompts"
    fi
fi

step "Writing summary"
{
    echo "# Discover Harness Summary"
    echo
    echo "- UTC time: $(date -u +%FT%TZ)"
    echo "- Dot binary: \`$abs_dot_bin\`"
    echo "- Output dir: \`$OUT_DIR\`"
    echo
    echo "## Checks"
    echo
    echo "| Check | Status | Note |"
    echo "|---|---|---|"
    for i in "${!CHECK_NAMES[@]}"; do
        echo "| ${CHECK_NAMES[$i]} | ${CHECK_STATUS[$i]} | ${CHECK_NOTES[$i]} |"
    done
    echo
    echo "## Artifacts"
    echo
    echo "- \`$run_log\`"
    echo "- \`$doctor_out\`"
    echo "- \`$report_out\`"
    echo "- \`$discover_yes_out\`"
    echo "- \`$capture_out\`"
    if [[ "$INTERACTIVE_DEMO" == true ]]; then
        echo "- \`$interactive_out\`"
    fi
    if [[ -f "$OUT_DIR/session.cast" ]]; then
        echo "- \`$OUT_DIR/session.cast\`"
    fi
} > "$summary_md"

{
    echo "{"
    echo "  \"generated_at_utc\": \"$(date -u +%FT%TZ)\","
    echo "  \"output_dir\": \"$OUT_DIR\","
    echo "  \"checks\": ["
    for i in "${!CHECK_NAMES[@]}"; do
        comma=","
        if [[ "$i" -eq "$((${#CHECK_NAMES[@]} - 1))" ]]; then
            comma=""
        fi
        printf '    {"name":"%s","status":"%s","note":"%s"}%s\n' \
            "${CHECK_NAMES[$i]}" "${CHECK_STATUS[$i]}" "${CHECK_NOTES[$i]}" "$comma"
    done
    echo "  ]"
    echo "}"
} > "$summary_json"

cat "$summary_md"

fail_count=0
for status in "${CHECK_STATUS[@]}"; do
    if [[ "$status" == "FAIL" ]]; then
        fail_count=$((fail_count + 1))
    fi
done

echo
echo "Completed with $fail_count failing checks."
echo "Summary: $summary_md"

if [[ "$STRICT" == true && "$fail_count" -gt 0 ]]; then
    exit 1
fi
