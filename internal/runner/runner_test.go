package runner

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	r := New()
	if r == nil {
		t.Fatal("New() returned nil")
	}
	if r.Timeout != DefaultTimeout {
		t.Errorf("Timeout = %v, want %v", r.Timeout, DefaultTimeout)
	}
}

func TestNewWithTimeout(t *testing.T) {
	timeout := 10 * time.Second
	r := NewWithTimeout(timeout)
	if r == nil {
		t.Fatal("NewWithTimeout() returned nil")
	}
	if r.Timeout != timeout {
		t.Errorf("Timeout = %v, want %v", r.Timeout, timeout)
	}
}

func TestRunSuccess(t *testing.T) {
	r := New()
	ctx := context.Background()

	result, err := r.Run(ctx, "", "echo", "hello")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Code != 0 {
		t.Errorf("Code = %v, want 0", result.Code)
	}
	if result.Stdout != "hello\n" {
		t.Errorf("Stdout = %q, want %q", result.Stdout, "hello\n")
	}
}

func TestRunWithDir(t *testing.T) {
	r := New()
	ctx := context.Background()

	// Run pwd in /tmp
	result, err := r.Run(ctx, "/tmp", "pwd")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if result.Code != 0 {
		t.Errorf("Code = %v, want 0", result.Code)
	}
	// Output should contain /tmp (or its resolved path)
	if result.Stdout == "" {
		t.Error("Stdout is empty")
	}
}

func TestRunFailure(t *testing.T) {
	r := New()
	ctx := context.Background()

	result, err := r.Run(ctx, "", "false")
	if err == nil {
		t.Fatal("Run() expected error, got nil")
	}
	if result.Code != 1 {
		t.Errorf("Code = %v, want 1", result.Code)
	}

	// Check error type
	runErr, ok := err.(*RunError)
	if !ok {
		t.Fatalf("error type = %T, want *RunError", err)
	}
	if runErr.Cmd != "false" {
		t.Errorf("Cmd = %v, want %v", runErr.Cmd, "false")
	}
}

func TestRunErrorFormat(t *testing.T) {
	err := &RunError{
		Cmd:    "git",
		Args:   []string{"push"},
		Dir:    "/repo",
		Code:   128,
		Stderr: "permission denied",
	}

	got := err.Error()
	if got == "" {
		t.Error("Error() returned empty string")
	}

	// Should contain command and stderr
	if !containsString(got, "git") {
		t.Errorf("Error() should contain 'git', got: %v", got)
	}
	if !containsString(got, "permission denied") {
		t.Errorf("Error() should contain stderr, got: %v", got)
	}
}

func TestExitCode(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{
			name: "RunError",
			err:  &RunError{Code: 42},
			want: 42,
		},
		{
			name: "other error",
			err:  context.DeadlineExceeded,
			want: -1,
		},
		{
			name: "nil",
			err:  nil,
			want: -1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExitCode(tt.err)
			if got != tt.want {
				t.Errorf("ExitCode() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunContextCanceled(t *testing.T) {
	r := NewWithTimeout(0) // No timeout from runner
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := r.Run(ctx, "", "sleep", "10")
	if err == nil {
		t.Fatal("Run() expected error for canceled context")
	}
}

func TestRunnerInterface(t *testing.T) {
	// Verify ExecRunner implements Runner interface
	var _ Runner = (*ExecRunner)(nil)
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
