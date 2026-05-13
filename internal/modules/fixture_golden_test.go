package modules

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

var fixtureTime = time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC)

func TestFilesFixtureAuditPresentGolden(t *testing.T) {
	ctx := context.Background()
	caseDir := moduleFixtureDir(t, "files", "audit-present")
	repoDir := testutil.TempDir(t)
	homeDir := filepath.Join(caseDir, "input", "home")
	cfg := loadModuleTestConfig(t, repoDir)

	managedPath := filepath.Join(homeDir, ".zshrc")
	if err := os.Chmod(managedPath, 0o644); err != nil {
		t.Fatalf("chmod fixture file: %v", err)
	}

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "managed"),
		"~/.zshrc\n",
	)

	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	facts, diagnostics, err := files.Audit(ctx)
	if err != nil {
		t.Fatalf("Audit error = %v", err)
	}
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %#v", diagnostics)
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "fact.golden.json"), facts)
	assertFixtureSentinelsAbsent(t, caseDir)
}

func TestFilesFixturePlanNoopGolden(t *testing.T) {
	caseDir := moduleFixtureDir(t, "files", "plan-noop")
	orch := fixtureFilesOrchestrator(t, "")

	plan, err := orch.Plan(context.Background(), OperationApply)
	if err != nil {
		t.Fatalf("Plan error = %v", err)
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "plan.golden.json"), plan)
	assertFixtureSentinelsAbsent(t, caseDir)
}

func TestFilesFixturePlanCreateOrUpdateGolden(t *testing.T) {
	caseDir := moduleFixtureDir(t, "files", "plan-create-or-update")
	orch := fixtureFilesOrchestrator(t, "--- old\n+++ new\n")

	plan, err := orch.Plan(context.Background(), OperationApply)
	if err != nil {
		t.Fatalf("Plan error = %v", err)
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "plan.golden.json"), plan)
	assertFixtureSentinelsAbsent(t, caseDir)
}

func TestFilesFixtureRedactionGolden(t *testing.T) {
	caseDir := moduleFixtureDir(t, "files", "redaction")
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	plan := &Plan{
		SchemaVersion: SchemaPlanV1,
		PlanID:        "plan-" + sentinel,
		Operation:     OperationApply,
		CreatedAt:     Timestamp(fixtureTime),
		Target:        Target{OS: "darwin", Arch: "arm64", Host: "fixture-host.local"},
		Summary:       PlanSummary{Update: 1},
		Changes: []Change{
			{
				ChangeID:    "files:path/~/.env:update:" + sentinel,
				Surface:     "files",
				ID:          "files:path/~/.env",
				Action:      ActionUpdate,
				Source:      Source{Kind: "manifest", Value: "https://user:" + sentinel + "@github.com/dnery/dotstate.git"},
				Current:     map[string]any{"line": "API_TOKEN=" + sentinel},
				Desired:     map[string]any{"reference": "op://Employee/local/API_TOKEN"},
				ManagedBy:   []string{"dotstate", "chezmoi"},
				Sensitivity: SensitivityLocalPath,
				Confidence:  ConfidenceHigh,
				Capability:  []Capability{CapabilityAutoApply},
				Risk:        Risk{Level: RiskMedium, Reasons: []string{"contains " + sentinel}, Reversible: true},
			},
		},
	}
	SanitizePlan(plan)
	if plan.Changes[0].Sensitivity != SensitivitySecret {
		t.Fatalf("sanitized change sensitivity = %q, want secret", plan.Changes[0].Sensitivity)
	}
	if got := plan.Changes[0].Desired["reference"]; got != "op://Employee/local/API_TOKEN" {
		t.Fatalf("op reference changed during redaction: %#v", got)
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "plan.golden.json"), plan)
	assertFixtureSentinelsAbsent(t, caseDir)
}

func TestFilesFixtureResultRecordsGolden(t *testing.T) {
	caseDir := moduleFixtureDir(t, "files", "result-records")
	orch := fixtureFilesOrchestrator(t, "")
	for _, mod := range orch.Modules() {
		if files, ok := mod.(*FilesModule); ok {
			files.now = func() time.Time { return fixtureTime.Add(time.Second) }
		}
	}

	report, err := orch.Run(context.Background(), OperationApply, RunOptions{})
	if err != nil {
		t.Fatalf("Run error = %v", err)
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "result.golden.json"), report.Results)
	assertFixtureSentinelsAbsent(t, caseDir)
}

func TestFilesFixturePermissionDeniedGolden(t *testing.T) {
	caseDir := moduleFixtureDir(t, "files", "permission-denied")
	diag, ok := PermissionDiagnostic("files", "files:path/~/Library/Mail", "stat managed path", errors.New("Full Disk Access required"))
	if !ok {
		t.Fatal("expected permission diagnostic")
	}

	compareGoldenJSON(t, filepath.Join(caseDir, "diagnostics.golden.json"), []Diagnostic{diag})
	assertFixtureSentinelsAbsent(t, caseDir)
}

func fixtureFilesOrchestrator(t *testing.T, diff string) *Orchestrator {
	t.Helper()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadModuleTestConfig(t, repoDir)
	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(
		testutil.MatchCommandPrefix("chezmoi", "--source", filepath.Join(repoDir, "home"), "diff"),
		diff,
	)
	files := NewFilesModule(cfg, chez.New("chezmoi", mock), homeDir)
	files.now = func() time.Time { return fixtureTime }
	orch := NewOrchestrator(files)
	orch.now = func() time.Time { return fixtureTime }
	orch.target = Target{OS: "darwin", Arch: "arm64", Host: "fixture-host.local"}
	return orch
}

func moduleFixtureDir(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	elems := append([]string{repoRoot, "test", "fixtures", "modules", "v1"}, parts...)
	return filepath.Join(elems...)
}

func compareGoldenJSON(t *testing.T, path string, got any) {
	t.Helper()
	actual, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal actual JSON: %v", err)
	}
	actual = append(actual, '\n')
	if os.Getenv("DOTSTATE_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(want, actual) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\nactual:\n%s", path, want, actual)
	}
}

func assertFixtureSentinelsAbsent(t *testing.T, dir string) {
	t.Helper()
	assertPath := filepath.Join(dir, "redaction.assert_absent.txt")
	data, err := os.ReadFile(assertPath)
	if err != nil {
		t.Fatalf("read %s: %v", assertPath, err)
	}
	var sentinels []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sentinels = append(sentinels, line)
	}
	if len(sentinels) == 0 {
		t.Fatalf("%s did not declare any sentinel values", assertPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read fixture json: %v", err)
		}
		for _, sentinel := range sentinels {
			if strings.Contains(string(content), sentinel) {
				t.Fatalf("fixture %s leaked sentinel %q", entry.Name(), sentinel)
			}
		}
	}
}
