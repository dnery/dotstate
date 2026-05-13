#!/usr/bin/env bash
set -euo pipefail

usage() {
    cat <<'USAGE'
Usage:
  test/e2e/discover_harness.sh [options]

Options:
  --dot-bin PATH       Path to dot binary (default: ./bin/dot)
  --out-dir PATH       Output directory (default: state/e2e-runs/<timestamp>)
  --scenario NAME      discover-fast | discover-deep | discover-interactive | capture-loop | macos-verification | all (default: all)
  --record             Record run with asciinema (session.cast)
  --upload             Upload cast after recording (opt-in only)
  --strict             Exit non-zero if any check fails
  --no-delay           Disable sleeps between steps
  --include-raw        Include raw logs in artifacts bundle
  --help               Show help
USAGE
}

DOT_BIN="./bin/dot"
OUT_DIR=""
SCENARIO="all"
RECORD=false
UPLOAD=false
STRICT=false
NO_DELAY=false
INCLUDE_RAW=false

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
        --scenario)
            SCENARIO="${2:-}"
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
        --strict)
            STRICT=true
            shift
            ;;
        --no-delay)
            NO_DELAY=true
            shift
            ;;
        --include-raw)
            INCLUDE_RAW=true
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

case "$SCENARIO" in
    discover-fast|discover-deep|discover-interactive|capture-loop|macos-verification|all) ;;
    *)
        echo "Invalid --scenario: $SCENARIO" >&2
        exit 2
        ;;
esac

timestamp_compact() { date -u +"%Y%m%dT%H%M%SZ"; }
timestamp_human() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }

HARNESS_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SENTINEL="DOTSTATE_TEST_SECRET_DO_NOT_PRINT"

if [[ -z "$OUT_DIR" ]]; then
    OUT_DIR="state/e2e-runs/$(timestamp_compact)"
fi
mkdir -p "$OUT_DIR"
OUT_DIR="$(cd "$OUT_DIR" && pwd)"

RAW_DIR="$OUT_DIR/local-raw"
ARTIFACTS_DIR="$OUT_DIR/artifacts"
CMD_DIR="$ARTIFACTS_DIR/commands"
DERIV_DIR="$ARTIFACTS_DIR/derivatives"

mkdir -p "$RAW_DIR" "$CMD_DIR" "$DERIV_DIR"

if [[ "$RECORD" == true && -z "${ASCIINEMA_REC:-}" ]]; then
    command -v asciinema >/dev/null 2>&1 || { echo "asciinema is required for --record" >&2; exit 1; }

    cast_file="$ARTIFACTS_DIR/session.cast"
    rec_args=(
        "$0"
        "--dot-bin" "$DOT_BIN"
        "--out-dir" "$OUT_DIR"
        "--scenario" "$SCENARIO"
    )

    [[ "$STRICT" == true ]] && rec_args+=("--strict")
    [[ "$NO_DELAY" == true ]] && rec_args+=("--no-delay")
    [[ "$INCLUDE_RAW" == true ]] && rec_args+=("--include-raw")

    cmd=""
    for arg in "${rec_args[@]}"; do
        cmd+=$(printf "%q " "$arg")
    done
    cmd="${cmd% }"

    echo "Recording to $cast_file"
    ASCIINEMA_REC=1 asciinema rec --stdin --overwrite --title "dotstate harness $(timestamp_human)" --command "$cmd" "$cast_file"

    if [[ "$UPLOAD" == true ]]; then
        echo "Uploading recording..."
        asciinema upload "$cast_file" | tee "$ARTIFACTS_DIR/upload.txt"
    fi

    exit 0
fi

sleep_if_enabled() {
    if [[ "$NO_DELAY" == false ]]; then
        sleep 1
    fi
}

redact_stream() {
    sed -E \
        -e 's/DOTSTATE_TEST_SECRET_DO_NOT_PRINT/[REDACTED_SENTINEL]/g' \
        -e 's/gh[pors]_[A-Za-z0-9_]+/[REDACTED_GITHUB_TOKEN]/g' \
        -e 's/glpat-[A-Za-z0-9_-]+/[REDACTED_GITLAB_TOKEN]/g' \
        -e 's/AKIA[0-9A-Z]{16}/[REDACTED_AWS_ACCESS_KEY]/g' \
        -e 's/ASIA[0-9A-Z]{16}/[REDACTED_AWS_ACCESS_KEY]/g' \
        -e 's/xox[baprs]-[A-Za-z0-9-]+/[REDACTED_SLACK_TOKEN]/g' \
        -e 's/sk_live_[A-Za-z0-9]+/[REDACTED_STRIPE_KEY]/g' \
        -e 's/op:\/\/[A-Za-z0-9._\/-]+/[REDACTED_OP_REFERENCE]/g' \
        -e 's/-----BEGIN [A-Z ]*PRIVATE KEY-----/[REDACTED_PRIVATE_KEY_HEADER]/g' \
        -e 's/-----END [A-Z ]*PRIVATE KEY-----/[REDACTED_PRIVATE_KEY_FOOTER]/g'
}

RAW_RUN_LOG="$RAW_DIR/run.log"
SUMMARY_MD="$OUT_DIR/summary.md"
SUMMARY_JSON="$OUT_DIR/summary.json"
TIMELINE_MD="$OUT_DIR/timeline.md"
ENV_TXT="$OUT_DIR/environment.txt"

exec > >(tee -a "$RAW_RUN_LOG") 2>&1

echo "== dotstate discover harness =="
echo "UTC: $(timestamp_human)"
echo "Scenario: $SCENARIO"
echo "Output: $OUT_DIR"

DOT_BIN_ABS="$(cd "$(dirname "$DOT_BIN")" && pwd)/$(basename "$DOT_BIN")"
if [[ ! -x "$DOT_BIN_ABS" ]]; then
    echo "dot binary not executable: $DOT_BIN_ABS" >&2
    echo "Run: make build-local" >&2
    exit 1
fi

SANDBOX_DIR="$OUT_DIR/sandbox"
FAKE_HOME="$SANDBOX_DIR/home"
REPO_DIR="$SANDBOX_DIR/repo"
CHEZMOI_DEFAULT="$FAKE_HOME/.local/share/chezmoi"

rm -rf "$SANDBOX_DIR"
mkdir -p "$FAKE_HOME/.config/app" "$FAKE_HOME/.ssh" "$REPO_DIR/home" "$REPO_DIR/state"

cat > "$FAKE_HOME/.gitconfig" <<'GITCONFIG'
[user]
    name = Harness User
    email = harness@example.com
GITCONFIG

cat > "$FAKE_HOME/.zshrc" <<'ZSHRC'
export EDITOR=vim
alias ll='ls -la'
ZSHRC

cat > "$FAKE_HOME/.bashrc" <<'BASHRC'
export EDITOR=vim
alias gs='git status'
BASHRC

cat > "$FAKE_HOME/.config/app/settings.json" <<'SETTINGS'
{"theme":"dark","fontSize":14}
SETTINGS

cat > "$FAKE_HOME/.config/app/secret.env" <<'SECRETENV'
API_TOKEN=DOTSTATE_TEST_SECRET_DO_NOT_PRINT
SECRETENV

cat > "$FAKE_HOME/.ssh/id_rsa" <<'SSHKEY'
-----BEGIN OPENSSH PRIVATE KEY-----
FAKE-KEY-MATERIAL
-----END OPENSSH PRIVATE KEY-----
SSHKEY

cat > "$FAKE_HOME/.ssh/config" <<'SSHCONFIG'
Host github.com
    HostName github.com
    User git
SSHCONFIG

git -C "$REPO_DIR" init --initial-branch=main >/dev/null

git -C "$REPO_DIR" config user.name "Harness User"
git -C "$REPO_DIR" config user.email "harness@example.com"
git -C "$REPO_DIR" config commit.gpgsign false

cat > "$REPO_DIR/dot.toml" <<DOT
[repo]
url = "file://$REPO_DIR"
path = "$REPO_DIR"
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
DOT

git -C "$REPO_DIR" add dot.toml home state
git -C "$REPO_DIR" commit -m "harness: initialize sandbox repo" >/dev/null

cat > "$TIMELINE_MD" <<'TIMELINE'
# Timeline

| Start UTC | End UTC | Command | Exit |
|---|---|---|---|
TIMELINE

cat > "$ENV_TXT" <<ENV
utc=$(timestamp_human)
os=$(uname -s)
arch=$(uname -m)
go=$(go version 2>/dev/null || echo "missing")
git=$(git --version 2>/dev/null || echo "missing")
chezmoi=$(chezmoi --version 2>/dev/null || echo "missing")
asciinema=$(asciinema --version 2>/dev/null || echo "missing")
ffmpeg=$(ffmpeg -version 2>/dev/null | head -n1 || echo "missing")
dot=$($DOT_BIN_ABS version 2>/dev/null || echo "dot version unavailable")
ENV

declare -a CHECK_NAME CHECK_STATUS CHECK_NOTE CHECK_ISSUE

record_check() {
    CHECK_NAME+=("$1")
    CHECK_STATUS+=("$2")
    CHECK_NOTE+=("$3")
    CHECK_ISSUE+=("$4")
}

run_cmd() {
    local id="$1"
    shift
    local cmd="$*"

    local raw_file="$RAW_DIR/${id}.txt"
    local redacted_file="$CMD_DIR/${id}.txt"

    local start end rc
    start="$(timestamp_human)"

    set +e
    bash -lc "$cmd" >"$raw_file" 2>&1
    rc=$?
    set -e

    end="$(timestamp_human)"
    printf '| %s | %s | `%s` | %s |\n' "$start" "$end" "$id" "$rc" >> "$TIMELINE_MD"

    redact_stream < "$raw_file" > "$redacted_file"

    if [[ "$INCLUDE_RAW" == true ]]; then
        mkdir -p "$ARTIFACTS_DIR/raw"
        cp "$raw_file" "$ARTIFACTS_DIR/raw/${id}.txt"
    fi

    return "$rc"
}

summarize_report_counts() {
    local report_file="$1"
    local cat="$2"
    local count
    count="$(awk -v cat="$cat" '$0 ~ "=== "cat" " { gsub("=== "cat" ","",$0); gsub(" ===","",$0); gsub("[()]","",$0); split($0,a," "); print a[1]; exit }' "$report_file")"
    if [[ -z "$count" ]]; then
        echo 0
    else
        echo "$count"
    fi
}

run_env_prefix="cd \"$REPO_DIR\" && HOME=\"$FAKE_HOME\" XDG_CONFIG_HOME=\"$FAKE_HOME/.config\" \"$DOT_BIN_ABS\""

run_discover_fast() {
    if run_cmd "doctor" "$run_env_prefix doctor --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\""; then
        record_check "doctor exits cleanly" "PASS" "doctor command succeeded" ""
    else
        record_check "doctor exits cleanly" "FAIL" "doctor command failed" ""
    fi

    if run_cmd "discover_report_fast" "$run_env_prefix discover --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" --report"; then
        record_check "discover --report exits cleanly" "PASS" "discover report command succeeded" ""
    else
        record_check "discover --report exits cleanly" "FAIL" "discover report command failed" ""
    fi

    if rg -q "~/.gitconfig|~/.zshrc|~/.bashrc" "$CMD_DIR/discover_report_fast.txt"; then
        record_check "shell dotfiles appear in report" "PASS" "report includes shell dotfiles" "#6"
    else
        record_check "shell dotfiles appear in report" "FAIL" "report missing shell dotfiles" "#6"
    fi
}

run_discover_deep() {
    if run_cmd "discover_report_deep" "$run_env_prefix discover --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" --report --deep"; then
        record_check "discover --deep report exits cleanly" "PASS" "deep report command succeeded" ""
    else
        record_check "discover --deep report exits cleanly" "FAIL" "deep report command failed" ""
    fi
}

run_discover_interactive() {
    if run_cmd "discover_interactive" "printf 'n\ny\n' | $run_env_prefix discover --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" --no-commit"; then
        if rg -q "Commands:|Selected:" "$CMD_DIR/discover_interactive.txt"; then
            record_check "interactive flow transcript captured" "PASS" "interactive prompts captured" "#8"
        else
            record_check "interactive flow transcript captured" "FAIL" "interactive prompts missing" "#8"
        fi
    else
        record_check "interactive flow transcript captured" "FAIL" "interactive command failed" "#8"
    fi
}

run_capture_loop() {
    local discover_ok=false

    if run_cmd "discover_yes_warning" "$run_env_prefix discover --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" --yes --no-commit --secrets warning"; then
        discover_ok=true
        record_check "discover --yes --secrets warning exits cleanly" "PASS" "discover add completed" "#9"
    else
        record_check "discover --yes --secrets warning exits cleanly" "FAIL" "discover add failed" "#9"
    fi

    local repo_home_files=0
    local chez_default_files=0
    repo_home_files="$(find "$REPO_DIR/home" -type f | wc -l | tr -d ' ')"
    if [[ -d "$CHEZMOI_DEFAULT" ]]; then
        chez_default_files="$(find "$CHEZMOI_DEFAULT" -type f | wc -l | tr -d ' ')"
    fi

    if [[ "$discover_ok" == true && "$repo_home_files" -gt 0 ]]; then
        record_check "discover adds into repo home/" "PASS" "repo home file count: $repo_home_files" "#5"
    else
        record_check "discover adds into repo home/" "FAIL" "repo home file count: $repo_home_files" "#5"
    fi

    if [[ "$discover_ok" == true && "$chez_default_files" -eq 0 ]]; then
        record_check "discover avoids default chezmoi source" "PASS" "default chezmoi file count: $chez_default_files" "#5"
    else
        record_check "discover avoids default chezmoi source" "FAIL" "default chezmoi file count: $chez_default_files" "#5"
    fi

    local symlink_count=0
    local symlink_roots=("$REPO_DIR/home")
    if [[ -d "$CHEZMOI_DEFAULT" ]]; then
        symlink_roots+=("$CHEZMOI_DEFAULT")
    fi
    symlink_count="$(find "${symlink_roots[@]}" -type l 2>/dev/null | wc -l | tr -d ' ')"
    if [[ "$symlink_count" -eq 0 ]]; then
        record_check "copy semantics (no symlinks)" "PASS" "symlink count: 0" ""
    else
        record_check "copy semantics (no symlinks)" "FAIL" "symlink count: $symlink_count" ""
    fi

    echo "# modified by harness at $(timestamp_human)" >> "$FAKE_HOME/.gitconfig"

    if run_cmd "capture" "$run_env_prefix capture --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\""; then
        record_check "capture exits cleanly" "PASS" "capture command succeeded" ""
    else
        record_check "capture exits cleanly" "FAIL" "capture command failed" ""
    fi

    if [[ -n "$(git -C "$REPO_DIR" status --porcelain)" ]]; then
        record_check "capture creates repo diff after edit" "PASS" "repo has changes after capture" ""
    else
        record_check "capture creates repo diff after edit" "FAIL" "repo clean after capture" ""
    fi
}

run_macos_verification() {
    cat > "$FAKE_HOME/.harness_apply" <<'OLDSTATE'
old managed content
OLDSTATE
    cat > "$REPO_DIR/home/dot_harness_apply" <<'NEWSTATE'
new managed content
NEWSTATE
    git -C "$REPO_DIR" add home/dot_harness_apply
    git -C "$REPO_DIR" commit -m "harness: add desired managed file" >/dev/null

    if run_cmd "apply_dry_run_diff" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" apply --dry-run"; then
        if rg -q "update=1|backup-required" "$CMD_DIR/apply_dry_run_diff.txt"; then
            record_check "apply dry-run reports diff and backup" "PASS" "apply plan showed update and backup requirement" ""
        else
            record_check "apply dry-run reports diff and backup" "FAIL" "apply plan missed update or backup requirement" ""
        fi
    else
        record_check "apply dry-run reports diff and backup" "FAIL" "apply dry-run command failed" ""
    fi

    if run_cmd "apply_update" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" apply"; then
        if [[ "$(cat "$FAKE_HOME/.harness_apply")" == "new managed content" ]]; then
            record_check "apply updates sandbox home" "PASS" "managed file matched desired content" ""
        else
            record_check "apply updates sandbox home" "FAIL" "managed file did not match desired content" ""
        fi
    else
        record_check "apply updates sandbox home" "FAIL" "apply command failed" ""
    fi

    if find "$REPO_DIR/state/backups" -type f -name ".harness_apply" -print -quit | rg -q ".harness_apply"; then
        record_check "apply creates restorable backup payload" "PASS" "backup payload for managed file exists" ""
    else
        record_check "apply creates restorable backup payload" "FAIL" "backup payload missing" ""
    fi

    if run_cmd "apply_idempotent_dry_run" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" apply --dry-run"; then
        if rg -q "noop=1|No change" "$CMD_DIR/apply_idempotent_dry_run.txt"; then
            record_check "apply twice is no-op" "PASS" "second apply plan was no-op" ""
        else
            record_check "apply twice is no-op" "FAIL" "second apply plan was not no-op" ""
        fi
    else
        record_check "apply twice is no-op" "FAIL" "second apply dry-run failed" ""
    fi

    cat > "$FAKE_HOME/.harness_apply" <<'CAPTUREDSTATE'
captured managed content
CAPTUREDSTATE

    if run_cmd "capture_dry_run" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" capture --dry-run"; then
        record_check "capture dry-run exits cleanly" "PASS" "capture plan rendered without mutation" ""
    else
        record_check "capture dry-run exits cleanly" "FAIL" "capture dry-run command failed" ""
    fi

    if run_cmd "capture_update" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" capture"; then
        if rg -q "captured managed content" "$REPO_DIR/home/dot_harness_apply"; then
            record_check "capture updates desired artifact" "PASS" "source artifact reflects sandbox-home edit" ""
        else
            record_check "capture updates desired artifact" "FAIL" "source artifact did not reflect sandbox-home edit" ""
        fi
    else
        record_check "capture updates desired artifact" "FAIL" "capture command failed" ""
    fi

    if [[ "$(uname -s)" == "Darwin" ]]; then
        if run_cmd "schedule_install_dry_run" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" schedule install --dry-run --dot-bin \"$DOT_BIN_ABS\" --interval 5"; then
            record_check "schedule install dry-run exits cleanly" "PASS" "LaunchAgent plan rendered without writing" ""
        else
            record_check "schedule install dry-run exits cleanly" "FAIL" "schedule install dry-run failed" ""
        fi

        if run_cmd "schedule_install_no_load" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" schedule install --no-load --dot-bin \"$DOT_BIN_ABS\" --interval 5"; then
            record_check "schedule install sandbox no-load exits cleanly" "PASS" "LaunchAgent written in sandbox home" ""
        else
            record_check "schedule install sandbox no-load exits cleanly" "FAIL" "schedule install --no-load failed" ""
        fi

        if run_cmd "schedule_status" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" schedule status"; then
            record_check "schedule status exits cleanly" "PASS" "status inspected sandbox LaunchAgent" ""
        else
            record_check "schedule status exits cleanly" "FAIL" "schedule status failed" ""
        fi

        if run_cmd "schedule_remove" "$run_env_prefix --config \"$REPO_DIR/dot.toml\" --repo-dir \"$REPO_DIR\" schedule remove"; then
            record_check "schedule remove exits cleanly" "PASS" "sandbox LaunchAgent removed" ""
        else
            record_check "schedule remove exits cleanly" "FAIL" "schedule remove failed" ""
        fi

        if [[ "$(uname -m)" == "arm64" ]]; then
            if run_cmd "bootstrap_script_dry_run" "\"$HARNESS_ROOT/scripts/bootstrap-macos.sh\" --dry-run --repo \"https://user:$SENTINEL@github.com/dnery/dotstate.git\" --install-dir \"$SANDBOX_DIR/install-$SENTINEL\""; then
                record_check "release bootstrap script dry-run exits cleanly" "PASS" "bootstrap script rendered validation steps" ""
            else
                record_check "release bootstrap script dry-run exits cleanly" "FAIL" "bootstrap script dry-run failed" ""
            fi
        else
            record_check "release bootstrap script dry-run exits cleanly" "PASS" "skipped on non-arm64 macOS runner" ""
        fi
    else
        record_check "schedule install/status/remove macOS checks" "PASS" "skipped on non-macOS runner" ""
        record_check "release bootstrap script dry-run exits cleanly" "PASS" "skipped on non-macOS runner" ""
    fi

    if "$HARNESS_ROOT/test/e2e/verify_artifacts_no_sentinel.sh" "$OUT_DIR" "$SENTINEL" > "$CMD_DIR/sentinel_verifier.txt" 2>&1; then
        record_check "generated artifacts contain no sentinel leaks" "PASS" "sentinel verifier passed" ""
    else
        record_check "generated artifacts contain no sentinel leaks" "FAIL" "sentinel verifier failed" ""
    fi
}

case "$SCENARIO" in
    discover-fast)
        run_discover_fast
        ;;
    discover-deep)
        run_discover_deep
        ;;
    discover-interactive)
        run_discover_interactive
        ;;
    capture-loop)
        run_capture_loop
        ;;
    macos-verification)
        run_macos_verification
        ;;
    all)
        run_discover_fast
        sleep_if_enabled
        run_discover_deep
        sleep_if_enabled
        run_discover_interactive
        sleep_if_enabled
        run_capture_loop
        sleep_if_enabled
        run_macos_verification
        ;;
esac

DERIV_INDEX="$DERIV_DIR/index.md"
{
    echo "# Derivative Hooks"
    echo
    if [[ -f "$ARTIFACTS_DIR/session.cast" ]]; then
        echo "Session cast: present"
        if command -v asciinema >/dev/null 2>&1; then
            if asciinema convert --overwrite --output-format txt "$ARTIFACTS_DIR/session.cast" "$DERIV_DIR/session.txt" >/dev/null 2>&1; then
                echo "- Generated text transcript: artifacts/derivatives/session.txt"
            else
                echo "- Text transcript conversion failed"
            fi
        fi
    else
        echo "Session cast: not present (run with --record)"
    fi

    command -v agg >/dev/null 2>&1 && echo "- agg available for gif conversion" || echo "- agg not installed"
    command -v vhs >/dev/null 2>&1 && echo "- vhs available for rendering" || echo "- vhs not installed"
    command -v ffmpeg >/dev/null 2>&1 && echo "- ffmpeg available for post-processing" || echo "- ffmpeg not installed"
} > "$DERIV_INDEX"

report_fast="$CMD_DIR/discover_report_fast.txt"
report_deep="$CMD_DIR/discover_report_deep.txt"
recommended_fast="0"
maybe_fast="0"
risky_fast="0"
recommended_deep="0"

if [[ -f "$report_fast" ]]; then
    recommended_fast="$(summarize_report_counts "$report_fast" "Recommended" || echo 0)"
    maybe_fast="$(summarize_report_counts "$report_fast" "Maybe" || echo 0)"
    risky_fast="$(summarize_report_counts "$report_fast" "Risky" || echo 0)"
fi
if [[ -f "$report_deep" ]]; then
    recommended_deep="$(summarize_report_counts "$report_deep" "Recommended" || echo 0)"
fi

known_issue_rows=""
for i in "${!CHECK_NAME[@]}"; do
    if [[ "${CHECK_STATUS[$i]}" == "FAIL" && -n "${CHECK_ISSUE[$i]}" ]]; then
        known_issue_rows+="| ${CHECK_NAME[$i]} | ${CHECK_ISSUE[$i]} | ${CHECK_NOTE[$i]} |\n"
    fi
done

{
    echo "# Discover Harness Summary"
    echo
    echo "- UTC: $(timestamp_human)"
    echo "- Scenario: \`$SCENARIO\`"
    echo "- Dot binary: \`$DOT_BIN_ABS\`"
    echo "- Output dir: \`$OUT_DIR\`"
    echo
    echo "## Pass/Fail Matrix"
    echo
    echo "| Check | Status | Note |"
    echo "|---|---|---|"
    for i in "${!CHECK_NAME[@]}"; do
        echo "| ${CHECK_NAME[$i]} | ${CHECK_STATUS[$i]} | ${CHECK_NOTE[$i]} |"
    done
    echo
    echo "## What Changed"
    echo
    echo "- Fast report Recommended: $recommended_fast"
    echo "- Fast report Maybe: $maybe_fast"
    echo "- Fast report Risky: $risky_fast"
    echo "- Deep report Recommended: $recommended_deep"
    echo "- Repo home tracked files: $(find "$REPO_DIR/home" -type f | wc -l | tr -d ' ')"
    echo "- Default chezmoi tracked files: $(find "$CHEZMOI_DEFAULT" -type f 2>/dev/null | wc -l | tr -d ' ')"
    echo "- Repo diff summary:"
    git -C "$REPO_DIR" status --short | sed 's/^/  - /'
    echo
    echo "## Known Issue Detection"
    echo
    if [[ -n "$known_issue_rows" ]]; then
        echo "| Failing Check | Related Issue | Evidence |"
        echo "|---|---|---|"
        printf "%b" "$known_issue_rows"
    else
        echo "No known-issue signatures detected in failing checks."
    fi
    echo
    echo "## Artifacts"
    echo
    echo "- \`$ENV_TXT\`"
    echo "- \`$TIMELINE_MD\`"
    echo "- \`$CMD_DIR\` (redacted command outputs)"
    echo "- \`$DERIV_INDEX\`"
    if [[ -f "$ARTIFACTS_DIR/session.cast" ]]; then
        echo "- \`$ARTIFACTS_DIR/session.cast\`"
    fi
    if [[ "$INCLUDE_RAW" == true ]]; then
        echo "- \`$ARTIFACTS_DIR/raw\`"
    else
        echo "- raw logs kept local under \`$RAW_DIR\` (not included in review bundle by default)"
    fi
} > "$SUMMARY_MD"

{
    echo "{"
    echo "  \"generated_at_utc\": \"$(timestamp_human)\"," 
    echo "  \"scenario\": \"$SCENARIO\"," 
    echo "  \"output_dir\": \"$OUT_DIR\"," 
    echo "  \"checks\": ["
    for i in "${!CHECK_NAME[@]}"; do
        comma=","
        if [[ "$i" -eq "$((${#CHECK_NAME[@]} - 1))" ]]; then
            comma=""
        fi
        printf '    {"name":"%s","status":"%s","note":"%s","issue":"%s"}%s\n' \
            "${CHECK_NAME[$i]}" "${CHECK_STATUS[$i]}" "${CHECK_NOTE[$i]}" "${CHECK_ISSUE[$i]}" "$comma"
    done
    echo "  ]"
    echo "}"
} > "$SUMMARY_JSON"

redact_stream < "$SUMMARY_MD" > "$ARTIFACTS_DIR/summary.redacted.md"

cat "$SUMMARY_MD"

fail_count=0
for s in "${CHECK_STATUS[@]}"; do
    [[ "$s" == "FAIL" ]] && fail_count=$((fail_count + 1))
done

echo
echo "Completed with $fail_count failing checks."
echo "Summary: $SUMMARY_MD"

if [[ "$STRICT" == true && "$fail_count" -gt 0 ]]; then
    exit 1
fi
