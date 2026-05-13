package modules

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/runner"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestFilesModuleApplyDryRunPlansWithoutMutating(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "diff"),
		"--- a/.zshrc\n+++ b/.zshrc\n",
	)

	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	orch := NewOrchestrator(files)
	report, err := orch.Run(ctx, OperationApply, RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if report.Plan == nil {
		t.Fatal("expected plan")
	}
	if report.Plan.SchemaVersion != SchemaPlanV1 {
		t.Fatalf("schema version = %q, want %q", report.Plan.SchemaVersion, SchemaPlanV1)
	}
	if report.Plan.Summary.Update != 1 || report.Plan.Summary.Noop != 0 {
		t.Fatalf("unexpected summary: %#v", report.Plan.Summary)
	}
	if len(report.Plan.Changes) != 1 || !report.Plan.Changes[0].BackupRequired {
		t.Fatalf("expected one backup-required change, got %#v", report.Plan.Changes)
	}
	mock.AssertNotCalled(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "apply"))
	mock.AssertNotCalled(testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "managed"))
}

func TestFilesModuleApplyBacksUpAppliesAndVerifies(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)
	testutil.TempFile(t, homeDir, ".zshrc", "old\n")

	r := &queuedRunner{t: t}
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "diff"}, "--- old\n+++ new\n", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "managed"}, ".zshrc\n", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "apply"}, "", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "diff"}, "", "", nil)

	files := NewFilesModule(cfg, chez.New("chezmoi", r), homeDir)
	orch := NewOrchestrator(files)
	report, err := orch.Run(ctx, OperationApply, RunOptions{})
	if err != nil {
		t.Fatalf("Run apply error = %v", err)
	}
	if len(report.Backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(report.Backups))
	}
	if len(report.Results) != 2 {
		t.Fatalf("result count = %d, want apply+verify", len(report.Results))
	}
	if report.Results[0].Status != StatusApplied || report.Results[1].Status != StatusVerified {
		t.Fatalf("unexpected result statuses: %#v", report.Results)
	}
	if r.remaining() != 0 {
		t.Fatalf("not all expected commands were consumed: %d", r.remaining())
	}
}

func TestFilesModulePlanNoopWhenDiffEmpty(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "diff"),
		"",
	)

	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	orch := NewOrchestrator(files)
	report, err := orch.Run(ctx, OperationApply, RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	if report.Plan.Summary.Noop != 1 || report.Plan.Summary.Update != 0 {
		t.Fatalf("unexpected summary: %#v", report.Plan.Summary)
	}
	if report.Plan.Changes[0].BackupRequired {
		t.Fatalf("noop plan should not require backup: %#v", report.Plan.Changes[0])
	}
}

func TestFilesModuleBackupCopiesManagedFiles(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)
	managedPath := testutil.TempFile(t, homeDir, ".zshrc", "export PATH=/usr/bin\n")

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "managed"),
		".zshrc\n.missing\n",
	)

	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	change := files.baseChange(OperationApply)
	change.Action = ActionUpdate
	change.BackupRequired = true
	plan := &Plan{SchemaVersion: SchemaPlanV1, PlanID: "test-plan", Operation: OperationApply}

	backups, diagnostics, err := files.Backup(ctx, []Change{change}, plan)
	if err != nil {
		t.Fatalf("Backup error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if len(backups) != 2 {
		t.Fatalf("backup count = %d, want 2", len(backups))
	}

	var fileBackup *Backup
	for i := range backups {
		if backups[i].Source.Value == "~/.zshrc" {
			fileBackup = &backups[i]
		}
	}
	if fileBackup == nil {
		t.Fatalf("missing backup for %s: %#v", managedPath, backups)
	}
	if fileBackup.SchemaVersion != SchemaBackupV1 {
		t.Fatalf("backup schema = %q", fileBackup.SchemaVersion)
	}
	if fileBackup.PayloadRef.Kind != "local_file" || fileBackup.PayloadRef.Path == "" {
		t.Fatalf("missing payload ref: %#v", fileBackup.PayloadRef)
	}
	content, err := os.ReadFile(fileBackup.PayloadRef.Path)
	if err != nil {
		t.Fatalf("failed to read backup payload: %v", err)
	}
	if string(content) != "export PATH=/usr/bin\n" {
		t.Fatalf("backup payload = %q", string(content))
	}
	if fileBackup.PayloadRef.SHA256 == "" {
		t.Fatal("expected backup sha")
	}

	var missingBackup *Backup
	for i := range backups {
		if backups[i].Source.Value == "~/.missing" {
			missingBackup = &backups[i]
		}
	}
	if missingBackup == nil {
		t.Fatalf("missing backup record for absent file: %#v", backups)
	}
	if exists, _ := missingBackup.Current["exists"].(bool); exists {
		t.Fatalf("missing file backup should record exists=false: %#v", missingBackup.Current)
	}

	if err := os.WriteFile(managedPath, []byte("changed\n"), 0o644); err != nil {
		t.Fatalf("failed to change managed file: %v", err)
	}
	results, diagnostics, err := files.Restore(ctx, []Backup{*fileBackup})
	if err != nil {
		t.Fatalf("Restore error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected restore diagnostics: %#v", diagnostics)
	}
	if len(results) != 1 || results[0].Status != StatusRestored {
		t.Fatalf("unexpected restore results: %#v", results)
	}
	testutil.AssertFileContent(t, managedPath, "export PATH=/usr/bin\n")
}

func TestFilesModuleBackupTaintsSecretPayloadWithoutSerializingIt(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	testutil.TempFile(t, homeDir, ".env", "API_TOKEN="+sentinel+"\n")

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "managed"),
		".env\n",
	)

	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	change := files.baseChange(OperationApply)
	change.Action = ActionUpdate
	change.BackupRequired = true
	plan := &Plan{SchemaVersion: SchemaPlanV1, PlanID: "test-plan", Operation: OperationApply}

	backups, diagnostics, err := files.Backup(ctx, []Change{change}, plan)
	if err != nil {
		t.Fatalf("Backup error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}
	if len(backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(backups))
	}
	if backups[0].Sensitivity != SensitivitySecret {
		t.Fatalf("backup sensitivity = %q, want secret", backups[0].Sensitivity)
	}
	if redacted, _ := backups[0].Current["content_redacted"].(bool); !redacted {
		t.Fatalf("backup did not record redacted secret content: %#v", backups[0].Current)
	}
	encoded, err := json.Marshal(backups)
	if err != nil {
		t.Fatalf("Marshal backups: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("backup record leaked sentinel: %s", encoded)
	}
	content, err := os.ReadFile(backups[0].PayloadRef.Path)
	if err != nil {
		t.Fatalf("read backup payload: %v", err)
	}
	if !strings.Contains(string(content), sentinel) {
		t.Fatalf("backup payload should remain restorable local content")
	}
}

func TestFilesModuleInterruptedApplyRestoresSecretTaintedBackup(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	managedPath := testutil.TempFile(t, homeDir, ".env", "API_TOKEN="+sentinel+"\n")

	r := &queuedRunner{t: t}
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "diff"}, "--- old\n+++ new\n", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "managed"}, ".env\n", "", nil)
	r.Expect("chezmoi", []string{"--source", filepath.Join(repoDir, "home"), "apply"}, "", "interrupted", errors.New("interrupted apply"))

	files := NewFilesModule(cfg, chez.New("chezmoi", r), homeDir)
	orch := NewOrchestrator(files)
	report, err := orch.Run(ctx, OperationApply, RunOptions{})
	if err == nil || !strings.Contains(err.Error(), "interrupted apply") {
		t.Fatalf("Run error = %v, want interrupted apply", err)
	}
	if len(report.Backups) != 1 {
		t.Fatalf("backup count = %d, want 1", len(report.Backups))
	}
	backup := report.Backups[0]
	if backup.Sensitivity != SensitivitySecret {
		t.Fatalf("backup sensitivity = %q, want secret", backup.Sensitivity)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("interrupted apply report leaked sentinel: %s", encoded)
	}
	payload, err := os.ReadFile(backup.PayloadRef.Path)
	if err != nil {
		t.Fatalf("read backup payload: %v", err)
	}
	if !strings.Contains(string(payload), sentinel) {
		t.Fatalf("backup payload should keep local restorable secret content")
	}

	if err := os.WriteFile(managedPath, []byte("PARTIAL=1\n"), 0o644); err != nil {
		t.Fatalf("write partial state: %v", err)
	}
	restoreReport, err := orch.Restore(ctx, report.Backups)
	if err != nil {
		t.Fatalf("Restore error = %v", err)
	}
	if len(restoreReport.Results) != 1 || restoreReport.Results[0].Status != StatusRestored {
		t.Fatalf("unexpected restore report: %#v", restoreReport)
	}
	testutil.AssertFileContent(t, managedPath, "API_TOKEN="+sentinel+"\n")
	restoredEncoded, err := json.Marshal(restoreReport)
	if err != nil {
		t.Fatalf("Marshal restore report: %v", err)
	}
	if strings.Contains(string(restoredEncoded), sentinel) {
		t.Fatalf("restore report leaked sentinel: %s", restoredEncoded)
	}
	if r.remaining() != 0 {
		t.Fatalf("not all expected commands were consumed: %d", r.remaining())
	}
}

func loadModuleTestConfig(t *testing.T, repoDir string) *config.Config {
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
