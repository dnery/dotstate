package modules

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"time"
)

type Module interface {
	Surface() string
	Plan(ctx context.Context, operation Operation) ([]Change, []Diagnostic, error)
	Backup(ctx context.Context, changes []Change, plan *Plan) ([]Backup, []Diagnostic, error)
	Apply(ctx context.Context, changes []Change, plan *Plan) ([]Result, []Diagnostic, error)
	Capture(ctx context.Context, changes []Change, plan *Plan) ([]Result, []Diagnostic, error)
	Verify(ctx context.Context, operation Operation, changes []Change, plan *Plan) ([]Result, []Diagnostic, error)
	Restore(ctx context.Context, backups []Backup) ([]Result, []Diagnostic, error)
}

type DiscoveryModule interface {
	Discover(ctx context.Context) ([]Fact, []Diagnostic, error)
}

type AuditModule interface {
	Audit(ctx context.Context) ([]Fact, []Diagnostic, error)
}

type RunOptions struct {
	DryRun bool
}

type RunReport struct {
	Plan        *Plan        `json:"plan"`
	Backups     []Backup     `json:"backups"`
	Results     []Result     `json:"results"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

type Orchestrator struct {
	modules []Module
	now     func() time.Time
	target  Target
}

func NewOrchestrator(mods ...Module) *Orchestrator {
	host, _ := os.Hostname()
	if host == "" {
		host = "unknown-host"
	}
	return &Orchestrator{
		modules: append([]Module(nil), mods...),
		now:     time.Now,
		target:  Target{OS: runtime.GOOS, Arch: runtime.GOARCH, Host: host},
	}
}

func (o *Orchestrator) Modules() []Module {
	mods := make([]Module, len(o.modules))
	copy(mods, o.modules)
	return mods
}

func (o *Orchestrator) Discover(ctx context.Context) ([]Fact, []Diagnostic, error) {
	var facts []Fact
	var diagnostics []Diagnostic
	for _, mod := range o.modules {
		discoverer, ok := mod.(DiscoveryModule)
		if !ok {
			continue
		}
		moduleFacts, moduleDiagnostics, err := discoverer.Discover(ctx)
		diagnostics = append(diagnostics, moduleDiagnostics...)
		if err != nil {
			return facts, diagnostics, fmt.Errorf("%s discover: %w", mod.Surface(), err)
		}
		facts = append(facts, moduleFacts...)
	}
	return facts, diagnostics, nil
}

func (o *Orchestrator) Audit(ctx context.Context) ([]Fact, []Diagnostic, error) {
	var facts []Fact
	var diagnostics []Diagnostic
	for _, mod := range o.modules {
		auditor, ok := mod.(AuditModule)
		if !ok {
			continue
		}
		moduleFacts, moduleDiagnostics, err := auditor.Audit(ctx)
		diagnostics = append(diagnostics, moduleDiagnostics...)
		if err != nil {
			return facts, diagnostics, fmt.Errorf("%s audit: %w", mod.Surface(), err)
		}
		facts = append(facts, moduleFacts...)
	}
	return facts, diagnostics, nil
}

func (o *Orchestrator) Plan(ctx context.Context, operation Operation) (*Plan, error) {
	createdAt := o.now().UTC()
	plan := &Plan{
		SchemaVersion: SchemaPlanV1,
		PlanID:        newID(createdAt, string(operation)),
		Operation:     operation,
		CreatedAt:     Timestamp(createdAt),
		Target:        o.target,
		Changes:       []Change{},
		Diagnostics:   []Diagnostic{},
	}

	for _, mod := range o.modules {
		changes, diagnostics, err := mod.Plan(ctx, operation)
		plan.Diagnostics = append(plan.Diagnostics, diagnostics...)
		if err != nil {
			return plan, fmt.Errorf("%s plan: %w", mod.Surface(), err)
		}
		plan.Changes = append(plan.Changes, changes...)
	}

	plan.Summary = Summarize(plan.Changes)
	return plan, nil
}

func (o *Orchestrator) Run(ctx context.Context, operation Operation, opts RunOptions) (*RunReport, error) {
	plan, err := o.Plan(ctx, operation)
	report := &RunReport{Plan: plan}
	if err != nil {
		return report, err
	}
	if opts.DryRun {
		return report, nil
	}

	for _, mod := range o.modules {
		changes := changesForSurface(plan.Changes, mod.Surface())
		if len(changes) == 0 {
			continue
		}
		if blocked := blockedMutations(changes); len(blocked) > 0 {
			for _, change := range blocked {
				report.Results = append(report.Results, blockedResult(plan, change, o.now()))
			}
			return report, fmt.Errorf("%s has mutation without auto_apply capability: %s", mod.Surface(), strings.Join(changeIDs(blocked), ", "))
		}

		switch operation {
		case OperationApply:
			if requiresBackup(changes) {
				backups, diagnostics, err := mod.Backup(ctx, changes, plan)
				report.Diagnostics = append(report.Diagnostics, diagnostics...)
				report.Backups = append(report.Backups, backups...)
				if err != nil {
					return report, fmt.Errorf("%s backup: %w", mod.Surface(), err)
				}
			}

			results, diagnostics, err := mod.Apply(ctx, changes, plan)
			report.Diagnostics = append(report.Diagnostics, diagnostics...)
			report.Results = append(report.Results, results...)
			if err != nil {
				return report, fmt.Errorf("%s apply: %w", mod.Surface(), err)
			}

			verifyResults, verifyDiagnostics, err := mod.Verify(ctx, operation, changes, plan)
			report.Diagnostics = append(report.Diagnostics, verifyDiagnostics...)
			report.Results = append(report.Results, verifyResults...)
			if err != nil {
				return report, fmt.Errorf("%s verify: %w", mod.Surface(), err)
			}
		case OperationCapture:
			results, diagnostics, err := mod.Capture(ctx, changes, plan)
			report.Diagnostics = append(report.Diagnostics, diagnostics...)
			report.Results = append(report.Results, results...)
			if err != nil {
				return report, fmt.Errorf("%s capture: %w", mod.Surface(), err)
			}

			verifyResults, verifyDiagnostics, err := mod.Verify(ctx, operation, changes, plan)
			report.Diagnostics = append(report.Diagnostics, verifyDiagnostics...)
			report.Results = append(report.Results, verifyResults...)
			if err != nil {
				return report, fmt.Errorf("%s verify: %w", mod.Surface(), err)
			}
		default:
			return report, fmt.Errorf("unsupported operation: %s", operation)
		}
	}

	return report, nil
}

func (o *Orchestrator) Restore(ctx context.Context, backups []Backup) (*RunReport, error) {
	report := &RunReport{Backups: append([]Backup(nil), backups...)}
	for _, mod := range o.modules {
		selected := backupsForSurface(backups, mod.Surface())
		if len(selected) == 0 {
			continue
		}
		results, diagnostics, err := mod.Restore(ctx, selected)
		report.Diagnostics = append(report.Diagnostics, diagnostics...)
		report.Results = append(report.Results, results...)
		if err != nil {
			return report, fmt.Errorf("%s restore: %w", mod.Surface(), err)
		}
	}
	return report, nil
}

func changesForSurface(changes []Change, surface string) []Change {
	selected := make([]Change, 0)
	for _, change := range changes {
		if change.Surface == surface {
			selected = append(selected, change)
		}
	}
	return selected
}

func backupsForSurface(backups []Backup, surface string) []Backup {
	selected := make([]Backup, 0)
	for _, backup := range backups {
		if backup.Surface == surface {
			selected = append(selected, backup)
		}
	}
	return selected
}

func requiresBackup(changes []Change) bool {
	for _, change := range changes {
		if change.BackupRequired && isMutation(change.Action) {
			return true
		}
	}
	return false
}

func blockedMutations(changes []Change) []Change {
	var blocked []Change
	for _, change := range changes {
		if !isMutation(change.Action) {
			continue
		}
		if hasCapability(change.Capability, CapabilityAutoApply) && !hasCapability(change.Capability, CapabilityDryRunOnly) {
			continue
		}
		blocked = append(blocked, change)
	}
	return blocked
}

func isMutation(action ChangeAction) bool {
	switch action {
	case ActionCreate, ActionUpdate, ActionDelete:
		return true
	default:
		return false
	}
}

func hasCapability(capabilities []Capability, capability Capability) bool {
	for _, candidate := range capabilities {
		if candidate == capability {
			return true
		}
	}
	return false
}

func blockedResult(plan *Plan, change Change, t time.Time) Result {
	now := Timestamp(t)
	return Result{
		SchemaVersion: SchemaResultV1,
		RunID:         newID(t, "blocked"),
		PlanID:        plan.PlanID,
		Phase:         PhaseApply,
		Surface:       change.Surface,
		ID:            change.ID,
		ChangeID:      change.ChangeID,
		Source:        change.Source,
		Current:       change.Current,
		Desired:       change.Desired,
		ManagedBy:     change.ManagedBy,
		Sensitivity:   change.Sensitivity,
		Confidence:    change.Confidence,
		Capability:    change.Capability,
		Risk:          change.Risk,
		Status:        StatusBlocked,
		StartedAt:     now,
		EndedAt:       now,
		Diagnostics: []Diagnostic{
			NewDiagnostic(SeverityError, "module.auto_apply_required", "Change is not eligible for automatic apply.", change.Surface, change.ID),
		},
	}
}

func changeIDs(changes []Change) []string {
	ids := make([]string, 0, len(changes))
	for _, change := range changes {
		ids = append(ids, change.ChangeID)
	}
	return ids
}

func newID(t time.Time, suffix string) string {
	clean := strings.NewReplacer("/", "-", ":", "-", " ", "-", "_", "-").Replace(suffix)
	return fmt.Sprintf("%s-%s", t.UTC().Format("20060102T150405Z"), clean)
}
