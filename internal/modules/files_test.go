package modules

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
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
