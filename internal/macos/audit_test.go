package macos

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dnery/dotstate/dot/internal/modules"
)

func TestNewBootstrapAuditRedactsHostAndEmitsStableEnvelope(t *testing.T) {
	audit := NewBootstrapAudit("darwin", "arm64", "Dans-MacBook-Pro.local", time.Date(2026, 5, 13, 12, 0, 0, 0, time.UTC))
	if audit.SchemaVersion != modules.SchemaAuditV1 {
		t.Fatalf("schema = %q, want %q", audit.SchemaVersion, modules.SchemaAuditV1)
	}
	if audit.Target.Host != "<redacted:hostname>" {
		t.Fatalf("host = %q, want redacted", audit.Target.Host)
	}
	if audit.GeneratedAt != "2026-05-13T12:00:00Z" {
		t.Fatalf("generated_at = %q", audit.GeneratedAt)
	}
	if len(audit.Diagnostics) == 0 {
		t.Fatalf("expected bootstrap capability diagnostics")
	}
	encoded, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "Dans-MacBook") {
		t.Fatalf("encoded audit leaked host: %s", encoded)
	}
}

func TestNewBootstrapAuditUnsupportedPlatformIsDiagnosticNotFailure(t *testing.T) {
	audit := NewBootstrapAudit("linux", "amd64", "host", time.Unix(0, 0))
	if audit.Summary.Warnings != 1 || audit.Summary.Errors != 0 {
		t.Fatalf("summary = %#v, want one warning and no errors", audit.Summary)
	}
	if len(audit.Diagnostics) != 1 {
		t.Fatalf("diagnostics = %d, want 1", len(audit.Diagnostics))
	}
	diag := audit.Diagnostics[0]
	if diag.Code != "macos_audit_unsupported_platform" {
		t.Fatalf("diag code = %q", diag.Code)
	}
	if len(diag.Capability) != 1 || diag.Capability[0] != modules.CapabilityUnsupported {
		t.Fatalf("capability = %#v, want unsupported", diag.Capability)
	}
}

func TestNewBootstrapAuditIncludesPrivacyAndKeychainGuardrails(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	audit := NewBootstrapAudit("darwin", "arm64", sentinel, time.Unix(0, 0))
	encoded, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("audit leaked sentinel: %s", encoded)
	}

	codes := map[string]modules.Diagnostic{}
	for _, diag := range audit.Diagnostics {
		codes[diag.Code] = diag
	}
	for _, want := range []string{
		"macos.full_disk_access.manual_checkpoint",
		"macos.tcc.reference_only",
		"macos.keychain.reference_only",
		"macos.mdm.report_only",
		"macos.sip.protected_surface",
	} {
		if _, ok := codes[want]; !ok {
			t.Fatalf("missing diagnostic %s in %#v", want, codes)
		}
	}
	if keychain := codes["macos.keychain.reference_only"]; keychain.Sensitivity != modules.SensitivityRestricted || !strings.Contains(keychain.Message, "never read") {
		t.Fatalf("keychain diagnostic is not reference-only/restricted: %#v", keychain)
	}
	if fullDisk := codes["macos.full_disk_access.manual_checkpoint"]; !hasCapability(fullDisk.Capability, modules.CapabilityRequiresFullDiskAccess) {
		t.Fatalf("full disk diagnostic capabilities = %#v", fullDisk.Capability)
	}
}

func hasCapability(capabilities []modules.Capability, want modules.Capability) bool {
	for _, capability := range capabilities {
		if capability == want {
			return true
		}
	}
	return false
}
