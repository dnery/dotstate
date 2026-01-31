package testutil

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/dnery/dotstate/dot/internal/runner"
)

// CommandCall records a command that was executed.
type CommandCall struct {
	Dir  string
	Name string
	Args []string
}

// String returns a human-readable representation of the command.
func (c CommandCall) String() string {
	if c.Dir != "" {
		return fmt.Sprintf("%s %s (in %s)", c.Name, strings.Join(c.Args, " "), c.Dir)
	}
	return fmt.Sprintf("%s %s", c.Name, strings.Join(c.Args, " "))
}

// CommandMatcher determines if a CommandCall matches a pattern.
type CommandMatcher func(CommandCall) bool

// MatchCommand returns a matcher for the given command name.
func MatchCommand(name string) CommandMatcher {
	return func(c CommandCall) bool {
		return c.Name == name
	}
}

// MatchCommandPrefix returns a matcher for commands starting with the given name and args.
func MatchCommandPrefix(name string, args ...string) CommandMatcher {
	return func(c CommandCall) bool {
		if c.Name != name {
			return false
		}
		if len(c.Args) < len(args) {
			return false
		}
		for i, arg := range args {
			if c.Args[i] != arg {
				return false
			}
		}
		return true
	}
}

// MatchExact returns a matcher for exact command with name and args.
func MatchExact(name string, args ...string) CommandMatcher {
	return func(c CommandCall) bool {
		if c.Name != name {
			return false
		}
		if len(c.Args) != len(args) {
			return false
		}
		for i, arg := range args {
			if c.Args[i] != arg {
				return false
			}
		}
		return true
	}
}

// MatchAny returns a matcher that matches any command.
func MatchAny() CommandMatcher {
	return func(c CommandCall) bool {
		return true
	}
}

// MockResponse defines what a mock runner should return for a matching command.
type MockResponse struct {
	Matcher CommandMatcher
	Result  runner.CmdResult
	Err     error
}

// MockRunner is a test double for the runner.Runner interface.
type MockRunner struct {
	mu        sync.Mutex
	responses []MockResponse
	calls     []CommandCall
	fallback  *runner.CmdResult
	t         *testing.T
}

// NewMockRunner creates a new MockRunner for testing.
func NewMockRunner(t *testing.T) *MockRunner {
	return &MockRunner{
		t:         t,
		responses: make([]MockResponse, 0),
		calls:     make([]CommandCall, 0),
	}
}

// OnCommand registers a response for commands matching the given pattern.
// Later registrations take precedence over earlier ones.
func (m *MockRunner) OnCommand(matcher CommandMatcher, stdout, stderr string, code int, err error) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses = append(m.responses, MockResponse{
		Matcher: matcher,
		Result:  runner.CmdResult{Stdout: stdout, Stderr: stderr, Code: code},
		Err:     err,
	})
	return m
}

// OnCommandSuccess is a convenience method for registering a successful command.
func (m *MockRunner) OnCommandSuccess(matcher CommandMatcher, stdout string) *MockRunner {
	return m.OnCommand(matcher, stdout, "", 0, nil)
}

// OnCommandFailure is a convenience method for registering a failed command.
func (m *MockRunner) OnCommandFailure(matcher CommandMatcher, stderr string, code int) *MockRunner {
	return m.OnCommand(matcher, "", stderr, code, fmt.Errorf("command failed: %s", stderr))
}

// SetFallback sets a default response for any unmatched commands.
func (m *MockRunner) SetFallback(stdout, stderr string, code int) *MockRunner {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.fallback = &runner.CmdResult{Stdout: stdout, Stderr: stderr, Code: code}
	return m
}

// Run implements the runner.Runner interface for testing.
func (m *MockRunner) Run(ctx context.Context, dir, name string, args ...string) (*runner.CmdResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	call := CommandCall{Dir: dir, Name: name, Args: args}
	m.calls = append(m.calls, call)

	// Search responses in reverse order (later registrations take precedence)
	for i := len(m.responses) - 1; i >= 0; i-- {
		resp := m.responses[i]
		if resp.Matcher(call) {
			result := &runner.CmdResult{
				Stdout: resp.Result.Stdout,
				Stderr: resp.Result.Stderr,
				Code:   resp.Result.Code,
			}
			return result, resp.Err
		}
	}

	// Use fallback if available
	if m.fallback != nil {
		return &runner.CmdResult{
			Stdout: m.fallback.Stdout,
			Stderr: m.fallback.Stderr,
			Code:   m.fallback.Code,
		}, nil
	}

	// No match found - fail the test with helpful info
	m.t.Errorf("MockRunner: unexpected command: %s", call.String())
	return &runner.CmdResult{}, fmt.Errorf("unexpected command: %s", call.String())
}

// Calls returns all commands that were executed.
func (m *MockRunner) Calls() []CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]CommandCall, len(m.calls))
	copy(result, m.calls)
	return result
}

// CallCount returns the number of commands executed.
func (m *MockRunner) CallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// LastCall returns the most recent command call, or nil if none.
func (m *MockRunner) LastCall() *CommandCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.calls) == 0 {
		return nil
	}
	call := m.calls[len(m.calls)-1]
	return &call
}

// Reset clears all recorded calls (but keeps responses).
func (m *MockRunner) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = make([]CommandCall, 0)
}

// AssertCalled asserts that a command matching the pattern was called.
func (m *MockRunner) AssertCalled(matcher CommandMatcher) {
	m.t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.calls {
		if matcher(call) {
			return
		}
	}
	m.t.Errorf("MockRunner: expected command was not called")
}

// AssertNotCalled asserts that no command matching the pattern was called.
func (m *MockRunner) AssertNotCalled(matcher CommandMatcher) {
	m.t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, call := range m.calls {
		if matcher(call) {
			m.t.Errorf("MockRunner: unexpected command was called: %s", call.String())
			return
		}
	}
}

// AssertCallCount asserts that the given number of commands were called.
func (m *MockRunner) AssertCallCount(expected int) {
	m.t.Helper()
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.calls) != expected {
		m.t.Errorf("MockRunner: expected %d calls, got %d", expected, len(m.calls))
	}
}

// Compile-time check that MockRunner implements runner.Runner.
var _ runner.Runner = (*MockRunner)(nil)
