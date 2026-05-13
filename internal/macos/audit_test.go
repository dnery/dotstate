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
