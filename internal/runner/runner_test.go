package runner

import (
	"errors"
	"strings"
	"testing"
)

func TestRunErrorRedactsSecretsFromArgsAndStderr(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	err := &RunError{
		Cmd:    "tool",
		Args:   []string{"--token", sentinel},
		Code:   1,
		Stderr: "failed with password=" + sentinel,
		Err:    errors.New("raw " + sentinel),
	}
	got := err.Error()
	if strings.Contains(got, sentinel) {
		t.Fatalf("RunError leaked sentinel: %q", got)
	}
	if !strings.Contains(got, "<redacted:secret>") {
		t.Fatalf("RunError = %q, want redaction marker", got)
	}
}

func TestRunErrorRedactsWrappedErrorWhenStderrEmpty(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	err := &RunError{Cmd: "tool", Args: []string{"--password=" + sentinel}, Code: -1, Err: errors.New("dial " + sentinel)}
	got := err.Error()
	if strings.Contains(got, sentinel) {
		t.Fatalf("RunError leaked sentinel: %q", got)
	}
}
