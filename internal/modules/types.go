// Package modules defines the shared module lifecycle and schema records.
package modules

import "time"

const (
	SchemaFactV1       = "dotstate.fact.v1"
	SchemaPlanV1       = "dotstate.plan.v1"
	SchemaResultV1     = "dotstate.result.v1"
	SchemaDiagnosticV1 = "dotstate.diagnostic.v1"
	SchemaBackupV1     = "dotstate.backup.v1"
	SchemaAuditV1      = "dotstate.audit.v1"
)

type Phase string

const (
	PhaseDiscover Phase = "discover"
	PhaseAudit    Phase = "audit"
	PhasePlan     Phase = "plan"
	PhaseDiff     Phase = "diff"
	PhaseBackup   Phase = "backup"
	PhaseApply    Phase = "apply"
	PhaseVerify   Phase = "verify"
	PhaseCapture  Phase = "capture"
	PhaseRestore  Phase = "restore"
)

type Operation string

const (
	OperationApply   Operation = "apply"
	OperationCapture Operation = "capture"
	OperationRestore Operation = "restore"
)

type Capability string

const (
	CapabilityAutoApply              Capability = "auto_apply"
	CapabilityDryRunOnly             Capability = "dry_run_only"
	CapabilityManual                 Capability = "manual"
	CapabilityRequiresSudo           Capability = "requires_sudo"
	CapabilityRequiresFullDiskAccess Capability = "requires_full_disk_access"
	CapabilityRequiresMDM            Capability = "requires_mdm"
	CapabilityReadOnly               Capability = "read_only"
	CapabilityUnsupported            Capability = "unsupported"
)

type Sensitivity string

const (
	SensitivityPublic              Sensitivity = "public"
	SensitivityLocalPath           Sensitivity = "local_path"
	SensitivityPersonal            Sensitivity = "personal"
	SensitivityCredentialReference Sensitivity = "credential_reference"
	SensitivitySecret              Sensitivity = "secret"
	SensitivityRestricted          Sensitivity = "restricted"
)

type Confidence string

const (
	ConfidenceConfirmed Confidence = "confirmed"
	ConfidenceHigh      Confidence = "high"
	ConfidenceMedium    Confidence = "medium"
	ConfidenceLow       Confidence = "low"
	ConfidenceUnknown   Confidence = "unknown"
)

type RiskLevel string

const (
	RiskLow      RiskLevel = "low"
	RiskMedium   RiskLevel = "medium"
	RiskHigh     RiskLevel = "high"
	RiskCritical RiskLevel = "critical"
)

type ChangeAction string

const (
	ActionCreate  ChangeAction = "create"
	ActionUpdate  ChangeAction = "update"
	ActionDelete  ChangeAction = "delete"
	ActionNoop    ChangeAction = "noop"
	ActionReport  ChangeAction = "report"
	ActionManual  ChangeAction = "manual"
	ActionBlocked ChangeAction = "blocked"
)

type ResultStatus string

const (
	StatusApplied  ResultStatus = "applied"
	StatusVerified ResultStatus = "verified"
	StatusCaptured ResultStatus = "captured"
	StatusRestored ResultStatus = "restored"
	StatusNoop     ResultStatus = "noop"
	StatusSkipped  ResultStatus = "skipped"
	StatusManual   ResultStatus = "manual"
	StatusBlocked  ResultStatus = "blocked"
	StatusFailed   ResultStatus = "failed"
)

type DiagnosticSeverity string

const (
	SeverityInfo    DiagnosticSeverity = "info"
	SeverityWarning DiagnosticSeverity = "warning"
	SeverityError   DiagnosticSeverity = "error"
)

type Source struct {
	Kind       string `json:"kind"`
	Value      string `json:"value"`
	ObservedAt string `json:"observed_at,omitempty"`
}

type Risk struct {
	Level                RiskLevel `json:"level"`
	Reasons              []string  `json:"reasons"`
	RequiresConfirmation bool      `json:"requires_confirmation"`
	Reversible           bool      `json:"reversible"`
}

type Diagnostic struct {
	SchemaVersion string             `json:"schema_version"`
	Severity      DiagnosticSeverity `json:"severity"`
	Code          string             `json:"code"`
	Message       string             `json:"message"`
	Remediation   string             `json:"remediation,omitempty"`
	Surface       string             `json:"surface"`
	ID            string             `json:"id"`
	Source        Source             `json:"source"`
	Current       map[string]any     `json:"current"`
	Desired       map[string]any     `json:"desired"`
	ManagedBy     []string           `json:"managed_by"`
	Sensitivity   Sensitivity        `json:"sensitivity"`
	Confidence    Confidence         `json:"confidence"`
	Capability    []Capability       `json:"capability"`
	Risk          Risk               `json:"risk"`
}

type Fact struct {
	SchemaVersion string         `json:"schema_version"`
	Surface       string         `json:"surface"`
	ID            string         `json:"id"`
	Source        Source         `json:"source"`
	Current       map[string]any `json:"current"`
	Desired       map[string]any `json:"desired"`
	ManagedBy     []string       `json:"managed_by"`
	Sensitivity   Sensitivity    `json:"sensitivity"`
	Confidence    Confidence     `json:"confidence"`
	Capability    []Capability   `json:"capability"`
	Risk          Risk           `json:"risk"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}

type Change struct {
	ChangeID       string         `json:"change_id"`
	Surface        string         `json:"surface"`
	ID             string         `json:"id"`
	Action         ChangeAction   `json:"action"`
	Source         Source         `json:"source"`
	Current        map[string]any `json:"current"`
	Desired        map[string]any `json:"desired"`
	ManagedBy      []string       `json:"managed_by"`
	Sensitivity    Sensitivity    `json:"sensitivity"`
	Confidence     Confidence     `json:"confidence"`
	Capability     []Capability   `json:"capability"`
	Risk           Risk           `json:"risk"`
	BackupRequired bool           `json:"backup_required"`
	DependsOn      []string       `json:"depends_on"`
	Diagnostics    []Diagnostic   `json:"diagnostics"`
}

type Plan struct {
	SchemaVersion string       `json:"schema_version"`
	PlanID        string       `json:"plan_id"`
	Operation     Operation    `json:"operation"`
	CreatedAt     string       `json:"created_at"`
	Target        Target       `json:"target"`
	Summary       PlanSummary  `json:"summary"`
	Changes       []Change     `json:"changes"`
	Diagnostics   []Diagnostic `json:"diagnostics"`
}

type Target struct {
	OS   string `json:"os"`
	Arch string `json:"arch"`
	Host string `json:"host"`
}

type PlanSummary struct {
	Create  int `json:"create"`
	Update  int `json:"update"`
	Delete  int `json:"delete"`
	Noop    int `json:"noop"`
	Manual  int `json:"manual"`
	Blocked int `json:"blocked"`
}

type Result struct {
	SchemaVersion string         `json:"schema_version"`
	RunID         string         `json:"run_id"`
	PlanID        string         `json:"plan_id"`
	Phase         Phase          `json:"phase"`
	Surface       string         `json:"surface"`
	ID            string         `json:"id"`
	ChangeID      string         `json:"change_id"`
	Source        Source         `json:"source"`
	Current       map[string]any `json:"current"`
	Desired       map[string]any `json:"desired"`
	ManagedBy     []string       `json:"managed_by"`
	Sensitivity   Sensitivity    `json:"sensitivity"`
	Confidence    Confidence     `json:"confidence"`
	Capability    []Capability   `json:"capability"`
	Risk          Risk           `json:"risk"`
	Status        ResultStatus   `json:"status"`
	StartedAt     string         `json:"started_at"`
	EndedAt       string         `json:"ended_at"`
	Diagnostics   []Diagnostic   `json:"diagnostics"`
}

type Backup struct {
	SchemaVersion string         `json:"schema_version"`
	BackupID      string         `json:"backup_id"`
	CreatedAt     string         `json:"created_at"`
	Surface       string         `json:"surface"`
	ID            string         `json:"id"`
	Source        Source         `json:"source"`
	Current       map[string]any `json:"current"`
	Desired       map[string]any `json:"desired"`
	ManagedBy     []string       `json:"managed_by"`
	Sensitivity   Sensitivity    `json:"sensitivity"`
	Confidence    Confidence     `json:"confidence"`
	Capability    []Capability   `json:"capability"`
	Risk          Risk           `json:"risk"`
	PayloadRef    PayloadRef     `json:"payload_ref"`
	Restore       RestoreInfo    `json:"restore"`
}

type PayloadRef struct {
	Kind   string `json:"kind,omitempty"`
	Path   string `json:"path,omitempty"`
	SHA256 string `json:"sha256,omitempty"`
}

type RestoreInfo struct {
	Supported            bool `json:"supported"`
	RequiresConfirmation bool `json:"requires_confirmation"`
}

func LowRisk(reversible bool) Risk {
	return Risk{Level: RiskLow, Reasons: []string{}, RequiresConfirmation: false, Reversible: reversible}
}

func NewDiagnostic(severity DiagnosticSeverity, code, message, surface, id string) Diagnostic {
	return Diagnostic{
		SchemaVersion: SchemaDiagnosticV1,
		Severity:      severity,
		Code:          code,
		Message:       message,
		Surface:       surface,
		ID:            id,
		Source:        Source{Kind: "module", Value: surface},
		Current:       nil,
		Desired:       nil,
		ManagedBy:     []string{"dotstate"},
		Sensitivity:   SensitivityPublic,
		Confidence:    ConfidenceConfirmed,
		Capability:    []Capability{CapabilityReadOnly},
		Risk:          LowRisk(true),
	}
}

func Summarize(changes []Change) PlanSummary {
	var summary PlanSummary
	for _, change := range changes {
		switch change.Action {
		case ActionCreate:
			summary.Create++
		case ActionUpdate:
			summary.Update++
		case ActionDelete:
			summary.Delete++
		case ActionNoop, ActionReport:
			summary.Noop++
		case ActionManual:
			summary.Manual++
		case ActionBlocked:
			summary.Blocked++
		}
	}
	return summary
}

func Timestamp(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}
