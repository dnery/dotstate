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

var bootstrapSurfaces = []string{"brew", "mas", "apps", "launchd", "defaults", "profiles", "privacy"}

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
		envelope.Summary.Warnings = 1
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
	return envelope
}

func redactHostname(host string) string {
	if host == "" {
		return "<redacted:hostname>"
	}
	return "<redacted:hostname>"
}
