package modules

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/dnery/dotstate/dot/internal/redact"
)

// PermissionDiagnostic turns common macOS privacy/permission failures into
// stable, actionable diagnostics instead of leaking raw command output or
// crashing module reporting.
func PermissionDiagnostic(surface, id, operation string, err error) (Diagnostic, bool) {
	if err == nil {
		return Diagnostic{}, false
	}
	message := strings.ToLower(err.Error())
	kind := ""
	switch {
	case strings.Contains(message, "mdm") || strings.Contains(message, "configuration profile") || strings.Contains(message, "managed preference"):
		kind = "mdm"
	case strings.Contains(message, "system integrity protection") || strings.Contains(message, "sip") || strings.Contains(message, "read-only file system"):
		kind = "sip"
	case strings.Contains(message, "full disk access"):
		kind = "full_disk_access"
	case strings.Contains(message, "tcc") || strings.Contains(message, "not authorized") || strings.Contains(message, "automation"):
		kind = "tcc"
	case errors.Is(err, os.ErrPermission) || strings.Contains(message, "permission denied") || strings.Contains(message, "operation not permitted"):
		kind = "sudo_or_privacy"
	default:
		return Diagnostic{}, false
	}

	diag := NewDiagnostic(SeverityWarning, "macos.permission."+kind, "Permission prevented complete inspection.", surface, id)
	diag.Source = Source{Kind: "operation", Value: redact.Text(operation)}
	diag.Current = map[string]any{"error": redact.Text(err.Error())}
	diag.Sensitivity = SensitivityRestricted
	diag.Confidence = ConfidenceMedium
	diag.ManagedBy = []string{"dotstate", "manual"}
	diag.Risk = Risk{Level: RiskMedium, Reasons: []string{"permission-controlled macOS surface"}, RequiresConfirmation: true, Reversible: false}

	switch kind {
	case "full_disk_access":
		diag.Message = "Full Disk Access is required to inspect this surface completely."
		diag.Remediation = "Grant Full Disk Access to the terminal running dot, then rerun the command."
		diag.Capability = []Capability{CapabilityRequiresFullDiskAccess, CapabilityManual, CapabilityReadOnly}
	case "tcc":
		diag.Message = "macOS TCC privacy approval is required to inspect or automate this surface."
		diag.Remediation = "Approve the macOS privacy prompt in System Settings, or record the state as a manual checkpoint."
		diag.Capability = []Capability{CapabilityManual, CapabilityReadOnly}
	case "sudo_or_privacy":
		diag.Message = "Permission was denied; this may require administrator privileges or macOS privacy approval."
		diag.Remediation = "Review the path and rerun only through an explicit elevated or Full Disk Access flow if you understand the change."
		diag.Capability = []Capability{CapabilityRequiresSudo, CapabilityRequiresFullDiskAccess, CapabilityManual}
	case "sip":
		diag.Message = "System Integrity Protection prevents dotstate from inspecting or changing this surface."
		diag.Remediation = "Do not disable SIP for dotstate. Keep this state as a manual checkpoint or use supported user-level configuration."
		diag.Capability = []Capability{CapabilityUnsupported, CapabilityManual, CapabilityReadOnly}
		diag.Risk.Level = RiskCritical
		diag.Risk.Reasons = []string{"system integrity protection"}
	case "mdm":
		diag.Message = "This state appears to be managed by MDM or a configuration profile."
		diag.Remediation = "Change it in the management system or record it as a manual checkpoint; dotstate will not override policy."
		diag.Capability = []Capability{CapabilityRequiresMDM, CapabilityManual, CapabilityReadOnly}
	}
	if diag.Message == "" {
		diag.Message = fmt.Sprintf("Permission prevented %s.", operation)
	}
	return diag, true
}
