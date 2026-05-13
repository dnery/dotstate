package modules

import (
	"context"
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

type stubModule struct {
	surface string
	changes []Change
	applied bool
}

func (m *stubModule) Surface() string { return m.surface }
func (m *stubModule) Plan(context.Context, Operation) ([]Change, []Diagnostic, error) {
	return m.changes, nil, nil
}
func (m *stubModule) Backup(context.Context, []Change, *Plan) ([]Backup, []Diagnostic, error) {
	return nil, nil, nil
}
func (m *stubModule) Apply(context.Context, []Change, *Plan) ([]Result, []Diagnostic, error) {
	m.applied = true
	return nil, nil, nil
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
