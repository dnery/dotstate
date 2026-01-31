package chez

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
			wantBin: "chezmoi",
		},
		{
			name:    "custom binary",
			bin:     "/opt/bin/chezmoi",
			wantBin: "/opt/bin/chezmoi",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := New(tt.bin, nil)
			if c.Bin != tt.wantBin {
				t.Errorf("Bin = %v, want %v", c.Bin, tt.wantBin)
			}
			if c.R == nil {
				t.Error("Runner is nil")
			}
		})
	}
}

func TestReAdd(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("chezmoi", "re-add"),
		"",
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	err := c.ReAdd(ctx, "/repo")
	if err != nil {
		t.Fatalf("ReAdd() error = %v", err)
	}

	mock.AssertCalled(testutil.MatchExact("chezmoi", "re-add"))
}

func TestApply(t *testing.T) {
	tests := []struct {
		name      string
		sourceDir string
		wantArgs  []string
	}{
		{
			name:      "with source dir",
			sourceDir: "home",
			wantArgs:  []string{"--source", "/repo/home", "apply"},
		},
		{
			name:      "without source dir",
			sourceDir: "",
			wantArgs:  []string{"apply"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := testutil.NewMockRunner(t)
			mock.SetFallback("", "", 0)

			c := New("chezmoi", mock)
			ctx := context.Background()

			err := c.Apply(ctx, "/repo", tt.sourceDir)
			if err != nil {
				t.Fatalf("Apply() error = %v", err)
			}

			calls := mock.Calls()
			if len(calls) != 1 {
				t.Fatalf("expected 1 call, got %d", len(calls))
			}

			// Verify the arguments
			call := calls[0]
			if len(call.Args) != len(tt.wantArgs) {
				t.Errorf("args count = %d, want %d", len(call.Args), len(tt.wantArgs))
			}
		})
	}
}

func TestApplyError(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandFailure(
		testutil.MatchCommandPrefix("chezmoi"),
		"apply failed",
		1,
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	err := c.Apply(ctx, "/repo", "home")
	if err == nil {
		t.Fatal("Apply() expected error, got nil")
	}

	// Error should wrap the chezmoi error
	if !containsString(err.Error(), "chezmoi apply failed") {
		t.Errorf("error should mention chezmoi apply failed: %v", err)
	}
}

func TestAdd(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "add"),
		"",
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	err := c.Add(ctx, "/repo", []string{"~/.gitconfig", "~/.zshrc"}, false)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	// Should have add + 2 files = 3 args
	if len(calls[0].Args) != 3 {
		t.Errorf("args = %v, want 3 args", calls[0].Args)
	}
}

func TestAddWithSecretsError(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.SetFallback("", "", 0)

	c := New("chezmoi", mock)
	ctx := context.Background()

	err := c.Add(ctx, "/repo", []string{"~/.gitconfig"}, true)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	calls := mock.Calls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}

	// Should include --secrets=error flag
	call := calls[0]
	foundFlag := false
	for _, arg := range call.Args {
		if arg == "--secrets=error" {
			foundFlag = true
			break
		}
	}
	if !foundFlag {
		t.Errorf("Add() with secretsError should include --secrets=error flag, got: %v", call.Args)
	}
}

func TestAddEmpty(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	c := New("chezmoi", mock)
	ctx := context.Background()

	// Adding no files should do nothing
	err := c.Add(ctx, "/repo", []string{}, false)
	if err != nil {
		t.Fatalf("Add() error = %v", err)
	}

	mock.AssertCallCount(0)
}

func TestManaged(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("chezmoi", "managed"),
		".gitconfig\n.zshrc\n.config/nvim/init.lua\n",
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	files, err := c.Managed(ctx, "/repo")
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	expected := []string{".gitconfig", ".zshrc", ".config/nvim/init.lua"}
	if len(files) != len(expected) {
		t.Errorf("Managed() returned %d files, want %d", len(files), len(expected))
	}

	for i, f := range files {
		if f != expected[i] {
			t.Errorf("files[%d] = %q, want %q", i, f, expected[i])
		}
	}
}

func TestManagedEmpty(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("chezmoi", "managed"),
		"",
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	files, err := c.Managed(ctx, "/repo")
	if err != nil {
		t.Fatalf("Managed() error = %v", err)
	}

	if len(files) != 0 {
		t.Errorf("Managed() returned %d files, want 0", len(files))
	}
}

func TestVersion(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchExact("chezmoi", "--version"),
		"chezmoi version v2.47.0\n",
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	version, err := c.Version(ctx)
	if err != nil {
		t.Fatalf("Version() error = %v", err)
	}

	expected := "chezmoi version v2.47.0"
	if version != expected {
		t.Errorf("Version() = %q, want %q", version, expected)
	}
}

func TestDiff(t *testing.T) {
	mock := testutil.NewMockRunner(t)
	diffOutput := "--- a/.gitconfig\n+++ b/.gitconfig\n@@ -1 +1 @@\n-old\n+new\n"
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi"),
		diffOutput,
	)

	c := New("chezmoi", mock)
	ctx := context.Background()

	diff, err := c.Diff(ctx, "/repo", "home")
	if err != nil {
		t.Fatalf("Diff() error = %v", err)
	}

	if diff != diffOutput {
		t.Errorf("Diff() = %q, want %q", diff, diffOutput)
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
