// Package runner provides command execution abstractions.
package runner

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// CmdResult holds the result of a command execution.
type CmdResult struct {
	Stdout string
	Stderr string
	Code   int
}

// Runner defines the interface for executing external commands.
// This interface enables testing by allowing mock implementations.
type Runner interface {
	// Run executes a command and returns its result.
	// The dir parameter specifies the working directory (empty means current dir).
	// Returns a CmdResult with stdout/stderr/code, and an error if the command failed.
	Run(ctx context.Context, dir, name string, args ...string) (*CmdResult, error)
}

// DefaultTimeout is the default timeout for command execution.
const DefaultTimeout = 5 * time.Minute

// ExecRunner is the production implementation of Runner.
// It executes real commands using os/exec.
type ExecRunner struct {
	// Timeout is the maximum duration for command execution.
	// Zero means no timeout.
	Timeout time.Duration
}

// New creates a new ExecRunner with the default timeout.
func New() *ExecRunner {
	return &ExecRunner{Timeout: DefaultTimeout}
}

// NewWithTimeout creates a new ExecRunner with a custom timeout.
func NewWithTimeout(timeout time.Duration) *ExecRunner {
	return &ExecRunner{Timeout: timeout}
}

// Run executes a command and returns its result.
func (r *ExecRunner) Run(ctx context.Context, dir, name string, args ...string) (*CmdResult, error) {
	if r.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.Timeout)
		defer cancel()
	}

	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}

	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()

	res := &CmdResult{
		Stdout: outBuf.String(),
		Stderr: errBuf.String(),
		Code:   0,
	}

	if err == nil {
		return res, nil
	}

	// Extract exit code if available
	if ee, ok := err.(*exec.ExitError); ok {
		res.Code = ee.ExitCode()
	} else {
		res.Code = -1
	}

	// Build a helpful error message
	msg := strings.TrimSpace(res.Stderr)
	if msg == "" {
		msg = err.Error()
	}

	return res, &RunError{
		Cmd:    name,
		Args:   args,
		Dir:    dir,
		Code:   res.Code,
		Stderr: res.Stderr,
		Err:    err,
	}
}

// RunError provides detailed information about a command failure.
type RunError struct {
	Cmd    string
	Args   []string
	Dir    string
	Code   int
	Stderr string
	Err    error
}

func (e *RunError) Error() string {
	stderr := strings.TrimSpace(e.Stderr)
	if stderr != "" {
		return fmt.Sprintf("%s %v failed (exit %d): %s", e.Cmd, e.Args, e.Code, stderr)
	}
	return fmt.Sprintf("%s %v failed (exit %d): %v", e.Cmd, e.Args, e.Code, e.Err)
}

func (e *RunError) Unwrap() error {
	return e.Err
}

// ExitCode returns the exit code from a RunError, or -1 if not a RunError.
func ExitCode(err error) int {
	if re, ok := err.(*RunError); ok {
		return re.Code
	}
	return -1
}

// Compile-time check that ExecRunner implements Runner.
var _ Runner = (*ExecRunner)(nil)
