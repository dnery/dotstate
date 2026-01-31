package gitx

import (
	"context"
	"testing"

	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name    string
		bin     string
		wantBin string
	}{
		{
			name:    "default binary",
			bin:     "",
			wantBin: "git",
		},
		{
			name:    "custom binary",
			bin:     "/usr/local/bin/git",
			wantBin: "/usr/local/bin/git",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			g := New(tt.bin, nil)
			if g.Bin != tt.wantBin {
				t.Errorf("Bin = %v, want %v", g.Bin, tt.wantBin)
			}
			if g.R == nil {
				t.Error("Runner is nil")
			}
		})
	}
}

func TestPorcelainStatus(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "status", "--porcelain"),
		"M  modified.txt\n?? untracked.txt\n",
	)

	g := New("git", mock)
	ctx := context.Background()

	status, err := g.PorcelainStatus(ctx, "/repo")
	if err != nil {
		t.Fatalf("PorcelainStatus() error = %v", err)
	}

	expected := "M  modified.txt\n?? untracked.txt"
	if status != expected {
		t.Errorf("PorcelainStatus() = %q, want %q", status, expected)
	}

	mock.AssertCalled(testutil.MatchCommandPrefix("git", "status"))
}

func TestHasChanges(t *testing.T) {
	tests := []struct {
		name       string
		stdout     string
		wantResult bool
	}{
		{
			name:       "no changes",
			stdout:     "",
			wantResult: false,
		},
		{
			name:       "has changes",
			stdout:     "M  file.txt\n",
			wantResult: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockRunner(t)
			mock.OnCommandSuccess(
				testutil.MatchCommandPrefix("git", "status", "--porcelain"),
				tt.stdout,
			)

			g := New("git", mock)
			ctx := context.Background()

			got, err := g.HasChanges(ctx, "/repo")
			if err != nil {
				t.Fatalf("HasChanges() error = %v", err)
			}
			if got != tt.wantResult {
				t.Errorf("HasChanges() = %v, want %v", got, tt.wantResult)
			}
		})
	}
}

func TestAddAll(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("git", "add", "-A"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	err := g.AddAll(ctx, "/repo")
	if err != nil {
		t.Fatalf("AddAll() error = %v", err)
	}

	mock.AssertCalled(testutil.MatchExact("git", "add", "-A"))
}

func TestAdd(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "add"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	err := g.Add(ctx, "/repo", "file1.txt", "file2.txt")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if len(calls[0].Args) != 3 {
		t.Errorf("expected 3 args, got %d: %v", len(calls[0].Args), calls[0].Args)
	}
}

func TestAddEmpty(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	g := New("git", mock)
	ctx := context.Background()

	// Adding no files should do nothing
	err := g.Add(ctx, "/repo")
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	mock.AssertCallCount(0)
}

func TestCommit(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	// Status shows changes
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "status", "--porcelain"),
		"M  file.txt\n",
	)
	// Add succeeds
	mock.OnCommandSuccess(
		testutil.MatchExact("git", "add", "-A"),
		"",
	)
	// Commit succeeds
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "commit", "-m"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	committed, err := g.Commit(ctx, "/repo", "test commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if !committed {
		t.Error("Commit() returned false, expected true")
	}

	mock.AssertCalled(testutil.MatchCommandPrefix("git", "commit"))
}

func TestCommitNoChanges(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	// Status shows no changes
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "status", "--porcelain"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	committed, err := g.Commit(ctx, "/repo", "test commit")
	if err != nil {
		t.Fatalf("Commit() error = %v", err)
	}
	if committed {
		t.Error("Commit() returned true, expected false (no changes)")
	}

	// Should not have called commit
	mock.AssertNotCalled(testutil.MatchCommandPrefix("git", "commit"))
}

func TestPullRebase(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("git", "pull", "--rebase", "--autostash"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	err := g.PullRebase(ctx, "/repo")
	if err != nil {
		t.Fatalf("PullRebase() error = %v", err)
	}

	mock.AssertCalled(testutil.MatchExact("git", "pull", "--rebase", "--autostash"))
}

func TestPush(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("git", "push"),
		"",
	)

	g := New("git", mock)
	ctx := context.Background()

	err := g.Push(ctx, "/repo")
	if err != nil {
		t.Fatalf("Push() error = %v", err)
	}

	mock.AssertCalled(testutil.MatchExact("git", "push"))
}

func TestCurrentBranch(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "rev-parse", "--abbrev-ref", "HEAD"),
		"main\n",
	)

	g := New("git", mock)
	ctx := context.Background()

	branch, err := g.CurrentBranch(ctx, "/repo")
	if err != nil {
		t.Fatalf("CurrentBranch() error = %v", err)
	}
	if branch != "main" {
		t.Errorf("CurrentBranch() = %q, want %q", branch, "main")
	}
}

func TestRemoteURL(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("git", "remote", "get-url", "origin"),
		"https://github.com/test/repo.git\n",
	)

	g := New("git", mock)
	ctx := context.Background()

	url, err := g.RemoteURL(ctx, "/repo")
	if err != nil {
		t.Fatalf("RemoteURL() error = %v", err)
	}
	if url != "https://github.com/test/repo.git" {
		t.Errorf("RemoteURL() = %q, want %q", url, "https://github.com/test/repo.git")
	}
}

func TestDefaultCommitMessage(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		wantHas  string
	}{
		{
			name:     "with hostname",
			hostname: "my-machine",
			wantHas:  "my-machine",
		},
		{
			name:     "empty hostname",
			hostname: "",
			wantHas:  "unknown-host",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := DefaultCommitMessage(tt.hostname)
			if !containsString(msg, tt.wantHas) {
				t.Errorf("DefaultCommitMessage() = %q, should contain %q", msg, tt.wantHas)
			}
			if !containsString(msg, "dot sync") {
				t.Errorf("DefaultCommitMessage() = %q, should contain 'dot sync'", msg)
			}
			// Should contain a timestamp (RFC3339 format has 'T')
			if !containsString(msg, "T") {
				t.Errorf("DefaultCommitMessage() = %q, should contain timestamp", msg)
			}
		})
	}
}

func TestDefaultCommitMessageTimestamp(t *testing.T) {
	msg := DefaultCommitMessage("test")

	// The message should contain a timestamp
	if msg == "" {
		t.Error("DefaultCommitMessage() returned empty string")
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
