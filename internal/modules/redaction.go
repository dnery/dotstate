package modules

import (
	"github.com/dnery/dotstate/dot/internal/redact"
)

// SanitizeRunReport enforces the module contract that records are redacted
// before callers render, serialize, log, or otherwise share them.
func SanitizeRunReport(report *RunReport) {
	if report == nil {
		return
	}
	SanitizePlan(report.Plan)
	for i := range report.Backups {
		sanitizeBackup(&report.Backups[i])
	}
	for i := range report.Results {
		sanitizeResult(&report.Results[i])
	}
	sanitizeDiagnostics(report.Diagnostics)
}

// SanitizePlan redacts a plan in place and promotes sensitivity on each record
// when nested values were tainted.
func SanitizePlan(plan *Plan) {
	if plan == nil {
		return
	}
	report := redact.Report{Sensitivity: redact.SensitivityPublic}
	plan.PlanID, report = mergeString(report, plan.PlanID)
	plan.CreatedAt, _ = mergeString(report, plan.CreatedAt)
	if plan.Target.Host != "" && plan.Target.Host != "<redacted:hostname>" {
		plan.Target.Host = "<redacted:hostname>"
	}
	for i := range plan.Changes {
		sanitizeChange(&plan.Changes[i])
	}
	sanitizeDiagnostics(plan.Diagnostics)
}

func sanitizeChange(change *Change) {
	report := redact.Report{Sensitivity: redact.SensitivityPublic}
	change.ChangeID, report = mergeString(report, change.ChangeID)
	change.ID, report = mergeString(report, change.ID)
	change.Source, report = sanitizeSource(report, change.Source)
	change.Current, report = sanitizeMap(report, change.Current)
	change.Desired, report = sanitizeMap(report, change.Desired)
	change.ManagedBy, report = sanitizeStringSlice(report, change.ManagedBy)
	change.Risk, report = sanitizeRisk(report, change.Risk)
	sanitizeDiagnostics(change.Diagnostics)
	change.Sensitivity = promoteSensitivity(change.Sensitivity, report)
}

func sanitizeResult(result *Result) {
	report := redact.Report{Sensitivity: redact.SensitivityPublic}
	result.RunID, report = mergeString(report, result.RunID)
	result.PlanID, report = mergeString(report, result.PlanID)
	result.ID, report = mergeString(report, result.ID)
	result.ChangeID, report = mergeString(report, result.ChangeID)
	result.Source, report = sanitizeSource(report, result.Source)
	result.Current, report = sanitizeMap(report, result.Current)
	result.Desired, report = sanitizeMap(report, result.Desired)
	result.ManagedBy, report = sanitizeStringSlice(report, result.ManagedBy)
	result.Risk, report = sanitizeRisk(report, result.Risk)
	sanitizeDiagnostics(result.Diagnostics)
	result.Sensitivity = promoteSensitivity(result.Sensitivity, report)
}

func sanitizeBackup(backup *Backup) {
	report := redact.Report{Sensitivity: redact.SensitivityPublic}
	backup.BackupID, report = mergeString(report, backup.BackupID)
	backup.ID, report = mergeString(report, backup.ID)
	backup.Source, report = sanitizeSource(report, backup.Source)
	backup.Current, report = sanitizeMap(report, backup.Current)
	backup.Desired, report = sanitizeMap(report, backup.Desired)
	backup.ManagedBy, report = sanitizeStringSlice(report, backup.ManagedBy)
	backup.Risk, report = sanitizeRisk(report, backup.Risk)
	backup.PayloadRef, report = sanitizePayloadRef(report, backup.PayloadRef)
	backup.Sensitivity = promoteSensitivity(backup.Sensitivity, report)
}

func sanitizeDiagnostics(diagnostics []Diagnostic) {
	for i := range diagnostics {
		sanitizeDiagnostic(&diagnostics[i])
	}
}

func sanitizeDiagnostic(diag *Diagnostic) {
	report := redact.Report{Sensitivity: redact.SensitivityPublic}
	diag.Code, report = mergeString(report, diag.Code)
	diag.Message, report = mergeString(report, diag.Message)
	diag.Remediation, report = mergeString(report, diag.Remediation)
	diag.ID, report = mergeString(report, diag.ID)
	diag.Source, report = sanitizeSource(report, diag.Source)
	diag.Current, report = sanitizeMap(report, diag.Current)
	diag.Desired, report = sanitizeMap(report, diag.Desired)
	diag.ManagedBy, report = sanitizeStringSlice(report, diag.ManagedBy)
	diag.Risk, report = sanitizeRisk(report, diag.Risk)
	diag.Sensitivity = promoteSensitivity(diag.Sensitivity, report)
}

func sanitizeSource(report redact.Report, source Source) (Source, redact.Report) {
	source.Kind, report = mergeString(report, source.Kind)
	source.Value, report = mergeString(report, source.Value)
	source.ObservedAt, report = mergeString(report, source.ObservedAt)
	return source, report
}

func sanitizePayloadRef(report redact.Report, ref PayloadRef) (PayloadRef, redact.Report) {
	ref.Kind, report = mergeString(report, ref.Kind)
	ref.Path, report = mergeString(report, ref.Path)
	ref.SHA256, report = mergeString(report, ref.SHA256)
	return ref, report
}

func sanitizeMap(report redact.Report, values map[string]any) (map[string]any, redact.Report) {
	if values == nil {
		return nil, report
	}
	out, valueReport := redact.Value(values)
	report = report.Merge(valueReport)
	if mapped, ok := out.(map[string]any); ok {
		return mapped, report
	}
	return map[string]any{"value": out}, report
}

func sanitizeStringSlice(report redact.Report, values []string) ([]string, redact.Report) {
	if values == nil {
		return nil, report
	}
	out := make([]string, len(values))
	for i, value := range values {
		out[i], report = mergeString(report, value)
	}
	return out, report
}

func sanitizeRisk(report redact.Report, risk Risk) (Risk, redact.Report) {
	risk.Reasons, report = sanitizeStringSlice(report, risk.Reasons)
	return risk, report
}

func mergeString(report redact.Report, value string) (string, redact.Report) {
	out, valueReport := redact.String(value)
	return out, report.Merge(valueReport)
}

func promoteSensitivity(current Sensitivity, report redact.Report) Sensitivity {
	candidate := sensitivityFromRedaction(report.Sensitivity)
	if sensitivityRank(candidate) > sensitivityRank(current) {
		return candidate
	}
	if current == "" {
		return SensitivityPublic
	}
	return current
}

func sensitivityFromRedaction(s redact.Sensitivity) Sensitivity {
	switch s {
	case redact.SensitivityLocalPath:
		return SensitivityLocalPath
	case redact.SensitivityPersonal:
		return SensitivityPersonal
	case redact.SensitivityCredentialReference:
		return SensitivityCredentialReference
	case redact.SensitivitySecret:
		return SensitivitySecret
	case redact.SensitivityRestricted:
		return SensitivityRestricted
	default:
		return SensitivityPublic
	}
}

func sensitivityRank(s Sensitivity) int {
	switch s {
	case SensitivityLocalPath:
		return 1
	case SensitivityPersonal:
		return 2
	case SensitivityCredentialReference:
		return 3
	case SensitivitySecret:
		return 4
	case SensitivityRestricted:
		return 5
	default:
		return 0
	}
}
