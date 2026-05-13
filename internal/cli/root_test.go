package cli

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/schedule"
)

func TestBootstrapOutputRedactsSentinelValues(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	cfg := config.Default()
	cfg.Repo.Path = "/tmp/" + sentinel + "/dotstate"

	out := captureStdout(t, func() { printBootstrapComplete(cfg) })
	assertNoSentinel(t, out, sentinel)
	if !strings.Contains(out, "<redacted:secret>") || !strings.Contains(out, "dot macos audit --json") {
		t.Fatalf("bootstrap output missing redaction marker or next steps:\n%s", out)
	}
}

func TestBootstrapScriptDryRunRedactsSentinelValues(t *testing.T) {
	if runtime.GOOS != "darwin" || runtime.GOARCH != "arm64" {
		t.Skip("bootstrap script is Apple Silicon macOS only")
	}
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	script := filepath.Join("..", "..", "scripts", "bootstrap-macos.sh")
	cmd := exec.Command("sh", script,
		"--dry-run",
		"--repo", "https://user:"+sentinel+"@github.com/dnery/dotstate.git",
		"--install-dir", filepath.Join(t.TempDir(), sentinel),
	)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("bootstrap script dry-run error = %v\n%s", err, out)
	}
	output := string(out)
	assertNoSentinel(t, output, sentinel)
	if !strings.Contains(output, "https://<redacted:credential>@github.com/dnery/dotstate.git") || !strings.Contains(output, "dot macos audit --json") {
		t.Fatalf("bootstrap script output missing redaction marker or validation commands:\n%s", output)
	}
}

func TestSchedulePlanOutputRedactsSentinelValues(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	status := &schedule.Status{
		Label:           schedule.Label,
		Path:            "/tmp/" + sentinel + "/agent.plist",
		Installed:       false,
		Loaded:          false,
		IntervalMinutes: 30,
		ProgramArgs:     []string{"/bin/dot", "--config", "/tmp/" + sentinel + "/dot.toml", "sync"},
		Message:         "would not print " + sentinel,
	}

	out := captureStdout(t, func() { printScheduleStatus("Schedule install plan", status) })
	assertNoSentinel(t, out, sentinel)
	if !strings.Contains(out, "<redacted:secret>") || !strings.Contains(out, "Command:") {
		t.Fatalf("schedule output missing redaction marker or command:\n%s", out)
	}
}

func TestRunReportOutputRedactsSentinelValues(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	report := &modules.RunReport{
		Plan: &modules.Plan{
			SchemaVersion: modules.SchemaPlanV1,
			PlanID:        "plan-" + sentinel,
			Operation:     modules.OperationApply,
			Changes: []modules.Change{
				{
					ID:         "files:path/" + sentinel,
					Action:     modules.ActionUpdate,
					Capability: []modules.Capability{modules.CapabilityDryRunOnly},
					Diagnostics: []modules.Diagnostic{
						modules.NewDiagnostic(modules.SeverityWarning, "files.secret", "found "+sentinel, "files", "files:path/~/.env"),
					},
				},
			},
			Diagnostics: []modules.Diagnostic{
				modules.NewDiagnostic(modules.SeverityWarning, "plan.secret", "plan "+sentinel, "files", "files:path/~/.env"),
			},
		},
		Results: []modules.Result{{Phase: modules.PhaseApply, ID: "files:path/" + sentinel, Status: modules.StatusFailed}},
		Diagnostics: []modules.Diagnostic{
			modules.NewDiagnostic(modules.SeverityError, "result.secret", "result "+sentinel, "files", "files:path/~/.env"),
		},
	}

	out := captureStdout(t, func() { printRunReport("Apply plan", report) })
	assertNoSentinel(t, out, sentinel)
	if !strings.Contains(out, "<redacted:secret>") || !strings.Contains(out, "Apply plan") {
		t.Fatalf("run report output missing redaction marker or title:\n%s", out)
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("Pipe: %v", err)
	}
	os.Stdout = w
	fn()
	if err := w.Close(); err != nil {
		t.Fatalf("Close writer: %v", err)
	}
	os.Stdout = old
	defer r.Close()
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, r); err != nil {
		t.Fatalf("Copy stdout: %v", err)
	}
	return buf.String()
}

func assertNoSentinel(t *testing.T, out, sentinel string) {
	t.Helper()
	if strings.Contains(out, sentinel) {
		t.Fatalf("output leaked sentinel:\n%s", out)
	}
}
