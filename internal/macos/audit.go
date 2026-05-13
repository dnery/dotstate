// Package macos provides macOS-specific state reporting helpers.
package macos

import (
	"time"

	"github.com/dnery/dotstate/dot/internal/modules"
)

// AuditEnvelope is the stable dotstate.audit.v1 JSON wrapper emitted by
// `dot macos audit --json`.
type AuditEnvelope struct {
	SchemaVersion string               `json:"schema_version"`
	GeneratedAt   string               `json:"generated_at"`
	Target        modules.Target       `json:"target"`
	Facts         []modules.Fact       `json:"facts"`
	Diagnostics   []modules.Diagnostic `json:"diagnostics"`
	Summary       AuditSummary         `json:"summary"`
}

// AuditSummary gives callers a cheap way to inspect audit completeness.
type AuditSummary struct {
	Facts      int `json:"facts"`
	Warnings   int `json:"warnings"`
	Errors     int `json:"errors"`
	Redactions int `json:"redactions"`
}

var bootstrapSurfaces = []string{"brew", "mas", "apps", "launchd", "defaults", "profiles", "privacy_tcc", "secrets"}

// NewBootstrapAudit returns a non-mutating audit envelope that bootstrap can
// rely on today. It intentionally reports unsupported or pending surfaces as
// diagnostics so the command is safe to run on clean machines before the full
// Goal 1 macOS collectors exist.
func NewBootstrapAudit(goos, arch, host string, generatedAt time.Time) AuditEnvelope {
	envelope := AuditEnvelope{
		SchemaVersion: modules.SchemaAuditV1,
		GeneratedAt:   modules.Timestamp(generatedAt),
		Target: modules.Target{
			OS:   goos,
			Arch: arch,
			Host: redactHostname(host),
		},
		Facts:       []modules.Fact{},
		Diagnostics: []modules.Diagnostic{},
		Summary: AuditSummary{
			Redactions: 1,
		},
	}

	if goos != "darwin" {
		diag := modules.NewDiagnostic(
			modules.SeverityWarning,
			"macos_audit_unsupported_platform",
			"macOS audit is only available on darwin; no elevated checks were attempted.",
			"macos",
			"macos:platform",
		)
		diag.Capability = []modules.Capability{modules.CapabilityUnsupported}
		diag.Remediation = "Run this command on macOS, or use platform-specific audit commands when they are implemented."
		envelope.Diagnostics = append(envelope.Diagnostics, diag)
		envelope.updateSummary()
		return envelope
	}

	for _, surface := range bootstrapSurfaces {
		diag := modules.NewDiagnostic(
			modules.SeverityInfo,
			"macos_audit_surface_pending",
			"Read-only macOS audit collection for this surface is pending; bootstrap can continue without elevated permissions.",
			surface,
			surface+":audit",
		)
		diag.Capability = []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityDryRunOnly}
		diag.Remediation = "Use dot doctor, dot apply --dry-run, and dot sync --dry-run for current bootstrap validation; future audit modules will add facts here."
		envelope.Diagnostics = append(envelope.Diagnostics, diag)
	}
	envelope.Diagnostics = append(envelope.Diagnostics, privacySafetyDiagnostics()...)
	envelope.updateSummary()
	return envelope
}

func privacySafetyDiagnostics() []modules.Diagnostic {
	fullDisk := modules.NewDiagnostic(
		modules.SeverityWarning,
		"macos.full_disk_access.manual_checkpoint",
		"Some macOS state can be hidden by privacy controls; dotstate reports the gap instead of requesting Full Disk Access automatically.",
		"privacy_tcc",
		"privacy_tcc:full_disk_access",
	)
	fullDisk.Capability = []modules.Capability{modules.CapabilityRequiresFullDiskAccess, modules.CapabilityManual, modules.CapabilityReadOnly}
	fullDisk.Sensitivity = modules.SensitivityRestricted
	fullDisk.Remediation = "Grant Full Disk Access to the terminal running dot only if you want complete inspection, then rerun dot macos audit --json."
	fullDisk.Risk = modules.Risk{Level: modules.RiskMedium, Reasons: []string{"protected privacy surface"}, RequiresConfirmation: true, Reversible: false}

	tcc := modules.NewDiagnostic(
		modules.SeverityInfo,
		"macos.tcc.reference_only",
		"TCC/privacy databases are never read, copied, committed, or mutated; audit output records manual checkpoints only.",
		"privacy_tcc",
		"privacy_tcc:database",
	)
	tcc.Capability = []modules.Capability{modules.CapabilityManual, modules.CapabilityReadOnly}
	tcc.Sensitivity = modules.SensitivityRestricted
	tcc.Remediation = "Use System Settings or MDM for privacy permissions. dotstate will not bypass TCC."
	tcc.Risk = modules.Risk{Level: modules.RiskMedium, Reasons: []string{"restricted OS privacy database"}, RequiresConfirmation: true, Reversible: false}

	keychain := modules.NewDiagnostic(
		modules.SeverityInfo,
		"macos.keychain.reference_only",
		"Keychain contents are never read or serialized; store 1Password/op references or manual checkpoints instead of secret values.",
		"secrets",
		"secrets:keychain",
	)
	keychain.Capability = []modules.Capability{modules.CapabilityManual, modules.CapabilityReadOnly}
	keychain.Sensitivity = modules.SensitivityRestricted
	keychain.Remediation = "Keep decrypted values out of dotstate output. Use op:// references or secrets-env cache metadata only."
	keychain.Risk = modules.Risk{Level: modules.RiskHigh, Reasons: []string{"decrypted Keychain values are secret material"}, RequiresConfirmation: true, Reversible: false}

	mdm := modules.NewDiagnostic(
		modules.SeverityInfo,
		"macos.mdm.report_only",
		"MDM-managed profiles and policy-owned settings are report-only; dotstate cannot safely change them.",
		"profiles",
		"profiles:mdm",
	)
	mdm.Capability = []modules.Capability{modules.CapabilityRequiresMDM, modules.CapabilityReadOnly, modules.CapabilityManual}
	mdm.Sensitivity = modules.SensitivityPersonal
	mdm.Remediation = "Change MDM-managed state in the management system or record it as a manual checkpoint."

	sip := modules.NewDiagnostic(
		modules.SeverityInfo,
		"macos.sip.protected_surface",
		"SIP-protected system surfaces are reported as unsupported/manual rather than inspected or mutated with elevated hooks.",
		"macos",
		"macos:sip",
	)
	sip.Capability = []modules.Capability{modules.CapabilityUnsupported, modules.CapabilityManual, modules.CapabilityReadOnly}
	sip.Sensitivity = modules.SensitivityRestricted
	sip.Remediation = "Do not disable SIP for dotstate. Prefer supported user-level state or explicit manual checkpoints."
	sip.Risk = modules.Risk{Level: modules.RiskCritical, Reasons: []string{"system integrity protection"}, RequiresConfirmation: true, Reversible: false}

	return []modules.Diagnostic{fullDisk, tcc, keychain, mdm, sip}
}

func (a *AuditEnvelope) updateSummary() {
	a.Summary.Facts = len(a.Facts)
	a.Summary.Warnings = 0
	a.Summary.Errors = 0
	for _, diag := range a.Diagnostics {
		switch diag.Severity {
		case modules.SeverityWarning:
			a.Summary.Warnings++
		case modules.SeverityError:
			a.Summary.Errors++
		}
	}
}

func redactHostname(host string) string {
	if host == "" {
		return "<redacted:hostname>"
	}
	return "<redacted:hostname>"
}
