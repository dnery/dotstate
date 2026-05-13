package modules

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestOrchestratorBlocksMutationsWithoutAutoApply(t *testing.T) {
	ctx := context.Background()
	mod := &stubModule{
		surface: "unsafe",
		changes: []Change{
			{
				ChangeID:       "unsafe:item:update",
				Surface:        "unsafe",
				ID:             "unsafe:item",
				Action:         ActionUpdate,
				Source:         Source{Kind: "fixture", Value: "test"},
				ManagedBy:      []string{"dotstate"},
				Sensitivity:    SensitivityPublic,
				Confidence:     ConfidenceHigh,
				Capability:     []Capability{CapabilityDryRunOnly},
				Risk:           LowRisk(true),
				BackupRequired: false,
			},
		},
	}
	orch := NewOrchestrator(mod)

	report, err := orch.Run(ctx, OperationApply, RunOptions{})
	if err == nil {
		t.Fatal("expected unsafe mutation to be blocked")
	}
	if !strings.Contains(err.Error(), "without auto_apply") {
		t.Fatalf("unexpected error: %v", err)
	}
	if mod.applied {
		t.Fatal("module apply should not run")
	}
	if report == nil || len(report.Results) != 1 || report.Results[0].Status != StatusBlocked {
		t.Fatalf("expected blocked result, got %#v", report)
	}
}

func TestOrchestratorDryRunAllowsDryRunOnlyPlan(t *testing.T) {
	ctx := context.Background()
	mod := &stubModule{
		surface: "unsafe",
		changes: []Change{
			{
				ChangeID:   "unsafe:item:update",
				Surface:    "unsafe",
				ID:         "unsafe:item",
				Action:     ActionUpdate,
				Source:     Source{Kind: "fixture", Value: "test"},
				ManagedBy:  []string{"dotstate"},
				Capability: []Capability{CapabilityDryRunOnly},
				Risk:       LowRisk(true),
			},
		},
	}
	orch := NewOrchestrator(mod)

	report, err := orch.Run(ctx, OperationApply, RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("dry-run should not block: %v", err)
	}
	if mod.applied {
		t.Fatal("module apply should not run during dry-run")
	}
	if report.Plan.Summary.Update != 1 {
		t.Fatalf("unexpected summary: %#v", report.Plan.Summary)
	}
}

func TestOrchestratorSanitizesPlanRecordsBeforeReturn(t *testing.T) {
	ctx := context.Background()
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	mod := &stubModule{
		surface: "unsafe",
		changes: []Change{
			{
				ChangeID:   "unsafe:item:" + sentinel,
				Surface:    "unsafe",
				ID:         "unsafe:item/" + sentinel,
				Action:     ActionUpdate,
				Source:     Source{Kind: "manifest", Value: "https://user:" + sentinel + "@github.com/dnery/dotstate.git"},
				Current:    map[string]any{"raw": "password=" + sentinel},
				Desired:    map[string]any{"reference": "op://Employee/local/API_TOKEN"},
				ManagedBy:  []string{"dotstate"},
				Capability: []Capability{CapabilityDryRunOnly},
				Risk:       Risk{Level: RiskHigh, Reasons: []string{"contains " + sentinel}},
				Diagnostics: []Diagnostic{
					NewDiagnostic(SeverityWarning, "unsafe.secret", "raw secret "+sentinel, "unsafe", "unsafe:item"),
				},
			},
		},
	}
	orch := NewOrchestrator(mod)

	report, err := orch.Run(ctx, OperationApply, RunOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Run dry-run error = %v", err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("sanitized plan leaked sentinel: %s", encoded)
	}
	change := report.Plan.Changes[0]
	if report.Plan.Target.Host != "<redacted:hostname>" {
		t.Fatalf("target host = %q, want redacted", report.Plan.Target.Host)
	}
	if change.Sensitivity != SensitivitySecret {
		t.Fatalf("change sensitivity = %q, want secret", change.Sensitivity)
	}
	if !strings.Contains(change.Source.Value, "https://<redacted:credential>@github.com/dnery/dotstate.git") {
		t.Fatalf("source value not sanitized: %q", change.Source.Value)
	}
	if got := change.Desired["reference"]; got != "op://Employee/local/API_TOKEN" {
		t.Fatalf("op reference changed: %#v", got)
	}
}

func TestOrchestratorSanitizesBackupsAndResults(t *testing.T) {
	ctx := context.Background()
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	change := Change{
		ChangeID:       "safe:item:update",
		Surface:        "safe",
		ID:             "safe:item",
		Action:         ActionUpdate,
		Source:         Source{Kind: "fixture", Value: "test"},
		ManagedBy:      []string{"dotstate"},
		Sensitivity:    SensitivityPublic,
		Confidence:     ConfidenceHigh,
		Capability:     []Capability{CapabilityAutoApply},
		Risk:           LowRisk(true),
		BackupRequired: true,
	}
	mod := &stubModule{
		surface: "safe",
		changes: []Change{change},
		backups: []Backup{
			{
				SchemaVersion: SchemaBackupV1,
				BackupID:      "backup-" + sentinel,
				Surface:       "safe",
				ID:            "safe:item",
				Source:        Source{Kind: "path", Value: "/tmp/" + sentinel},
				Current:       map[string]any{"payload": sentinel},
				Sensitivity:   SensitivityPublic,
				Capability:    []Capability{CapabilityAutoApply},
				Risk:          LowRisk(true),
			},
		},
		applyResults: []Result{
			{
				SchemaVersion: SchemaResultV1,
				Surface:       "safe",
				ID:            "safe:item",
				Current:       map[string]any{"stderr": "token=" + sentinel},
				Sensitivity:   SensitivityPublic,
				Status:        StatusApplied,
			},
		},
	}
	orch := NewOrchestrator(mod)

	report, err := orch.Run(ctx, OperationApply, RunOptions{})
	if err != nil {
		t.Fatalf("Run apply error = %v", err)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("run report leaked sentinel: %s", encoded)
	}
	if len(report.Backups) != 1 || report.Backups[0].Sensitivity != SensitivitySecret {
		t.Fatalf("backup sensitivity not tainted: %#v", report.Backups)
	}
	if len(report.Results) != 1 || report.Results[0].Sensitivity != SensitivitySecret {
		t.Fatalf("result sensitivity not tainted: %#v", report.Results)
	}
}

type stubModule struct {
	surface      string
	changes      []Change
	backups      []Backup
	applyResults []Result
	applied      bool
}

func (m *stubModule) Surface() string { return m.surface }
func (m *stubModule) Plan(context.Context, Operation) ([]Change, []Diagnostic, error) {
	return m.changes, nil, nil
}
func (m *stubModule) Backup(context.Context, []Change, *Plan) ([]Backup, []Diagnostic, error) {
	return m.backups, nil, nil
}
func (m *stubModule) Apply(context.Context, []Change, *Plan) ([]Result, []Diagnostic, error) {
	m.applied = true
	return m.applyResults, nil, nil
}
func (m *stubModule) Capture(context.Context, []Change, *Plan) ([]Result, []Diagnostic, error) {
	return nil, nil, nil
}
func (m *stubModule) Verify(context.Context, Operation, []Change, *Plan) ([]Result, []Diagnostic, error) {
	return nil, nil, nil
}
func (m *stubModule) Restore(context.Context, []Backup) ([]Result, []Diagnostic, error) {
	return nil, nil, nil
}
