package sync

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/runner"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestSyncRefusesDirtyRepoBeforeCapture(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	cfg := loadSyncTestConfig(t, repoDir)
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(testutil.MatchExact("git", "status", "--porcelain"), " M unrelated.txt\n")

	s := New(cfg, gitx.New("git", mock), chez.New("chezmoi", mock))
	err := s.Sync(ctx, Options{})
	if err == nil {
		t.Fatal("expected dirty repo error")
	}
	if !strings.Contains(err.Error(), "repo has uncommitted changes before sync") {
		t.Fatalf("unexpected error: %v", err)
	}
	mock.AssertNotCalled(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "re-add"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "commit"))
}

func TestSyncDryRunPlansWithoutMutating(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	cfg := loadSyncTestConfig(t, repoDir)
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(testutil.MatchExact("git", "status", "--porcelain"), "")
	mock.OnCommandSuccess(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "diff"), "--- a/.zshrc\n+++ b/.zshrc\n")

	s := New(cfg, gitx.New("git", mock), chez.New("chezmoi", mock))
	report, err := s.SyncWithReport(ctx, Options{DryRun: true})
	if err != nil {
		t.Fatalf("SyncWithReport dry-run error = %v", err)
	}
	if len(report.Operations) != 2 {
		t.Fatalf("operation reports = %d, want 2", len(report.Operations))
	}
	if report.Operations[0].Plan.Operation != "capture" || report.Operations[1].Plan.Operation != "apply" {
		t.Fatalf("unexpected operations: %#v", report.Operations)
	}
	mock.AssertNotCalled(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "re-add"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "apply"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "add"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "commit"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "pull"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "push"))
}

func TestSyncReportsPullRebaseConflicts(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	cfg := loadSyncTestConfig(t, repoDir)
	r := &queuedRunner{t: t}
	r.Expect("git", []string{"status", "--porcelain"}, "", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "re-add"}, "", "", nil)
	r.Expect("git", []string{"status", "--porcelain"}, " M home/.zshrc\n", "", nil)
	r.Expect("git", []string{"add", "-A"}, "", "", nil)
	r.Expect("git", []string{"commit", "-m", "dot sync from test-host at 2026-05-13T00:00:00Z"}, "", "", nil)
	r.Expect("git", []string{"pull", "--rebase", "--autostash"}, "", "conflict", fmt.Errorf("pull failed"))
	r.Expect("git", []string{"status", "--porcelain"}, "UU home/.zshrc\n", "", nil)

	oldHostname := osHostname
	osHostname = func() (string, error) { return "test-host", nil }
	t.Cleanup(func() { osHostname = oldHostname })
	oldDefaultCommitMessage := defaultCommitMessage
	defaultCommitMessage = func(host string) string { return "dot sync from test-host at 2026-05-13T00:00:00Z" }
	t.Cleanup(func() { defaultCommitMessage = oldDefaultCommitMessage })

	s := New(cfg, gitx.New("git", r), chez.New("chezmoi", r))
	err := s.Sync(ctx, Options{NoPush: true})
	if err == nil {
		t.Fatal("expected pull conflict error")
	}
	if !strings.Contains(err.Error(), "git pull/rebase produced conflicts") || !strings.Contains(err.Error(), "UU home/.zshrc") {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.remaining() != 0 {
		t.Fatalf("not all expected commands were consumed: %d", r.remaining())
	}
}

func loadSyncTestConfig(t *testing.T, repoDir string) *config.Config {
	t.Helper()
	content := strings.ReplaceAll(
		testutil.MinimalDotToml(),
		`path = "~/dotstate"`,
		"path = "+strconv.Quote(repoDir),
	)
	testutil.TempDotToml(t, repoDir, content)
	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}
	cfg, err := config.Load(filepath.Join(repoDir, "dot.toml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}
	return cfg
}

type queuedRunner struct {
	t         *testing.T
	responses []queuedResponse
}

type queuedResponse struct {
	name   string
	args   []string
	stdout string
	stderr string
	err    error
}

func (r *queuedRunner) Expect(name string, args []string, stdout, stderr string, err error) {
	r.responses = append(r.responses, queuedResponse{name: name, args: args, stdout: stdout, stderr: stderr, err: err})
}

func (r *queuedRunner) Run(ctx context.Context, dir, name string, args ...string) (*runner.CmdResult, error) {
	r.t.Helper()
	if len(r.responses) == 0 {
		r.t.Fatalf("unexpected command: %s %v", name, args)
	}
	resp := r.responses[0]
	r.responses = r.responses[1:]
	if resp.name != name || !sameStrings(resp.args, args) {
		r.t.Fatalf("command = %s %v, want %s %v", name, args, resp.name, resp.args)
	}
	return &runner.CmdResult{Stdout: resp.stdout, Stderr: resp.stderr}, resp.err
}

func (r *queuedRunner) remaining() int { return len(r.responses) }

func sameStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
