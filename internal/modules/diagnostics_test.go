package modules

import (
	"errors"
	"strings"
	"testing"
)

func TestPermissionDiagnosticClassifiesMacOSRemedies(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantCode   string
		wantCap    Capability
		wantPhrase string
	}{
		{
			name:       "full disk access",
			err:        errors.New("Full Disk Access required for ~/Library/Mail"),
			wantCode:   "macos.permission.full_disk_access",
			wantCap:    CapabilityRequiresFullDiskAccess,
			wantPhrase: "Full Disk Access",
		},
		{
			name:       "tcc prompt",
			err:        errors.New("not authorized to send Apple events"),
			wantCode:   "macos.permission.tcc",
			wantCap:    CapabilityManual,
			wantPhrase: "privacy prompt",
		},
		{
			name:       "sudo or privacy",
			err:        errors.New("permission denied"),
			wantCode:   "macos.permission.sudo_or_privacy",
			wantCap:    CapabilityRequiresSudo,
			wantPhrase: "administrator",
		},
		{
			name:       "sip",
			err:        errors.New("operation blocked by System Integrity Protection"),
			wantCode:   "macos.permission.sip",
			wantCap:    CapabilityUnsupported,
			wantPhrase: "SIP",
		},
		{
			name:       "mdm",
			err:        errors.New("configuration profile managed preference"),
			wantCode:   "macos.permission.mdm",
			wantCap:    CapabilityRequiresMDM,
			wantPhrase: "MDM",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			diag, ok := PermissionDiagnostic("files", "files:path/~/.secret", "audit", tt.err)
			if !ok {
				t.Fatal("expected diagnostic")
			}
			if diag.Code != tt.wantCode {
				t.Fatalf("code = %q, want %q", diag.Code, tt.wantCode)
			}
			if !hasCapability(diag.Capability, tt.wantCap) {
				t.Fatalf("capabilities = %#v, want %s", diag.Capability, tt.wantCap)
			}
			if !strings.Contains(diag.Remediation+diag.Message, tt.wantPhrase) {
				t.Fatalf("diagnostic does not explain %q: %#v", tt.wantPhrase, diag)
			}
			if diag.Sensitivity != SensitivityRestricted {
				t.Fatalf("sensitivity = %q, want restricted", diag.Sensitivity)
			}
		})
	}
}

func TestPermissionDiagnosticRedactsRawError(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	diag, ok := PermissionDiagnostic("files", "files:path/~/.env", "audit", errors.New("permission denied: "+sentinel))
	if !ok {
		t.Fatal("expected diagnostic")
	}
	if strings.Contains(diag.Current["error"].(string), sentinel) {
		t.Fatalf("diagnostic leaked sentinel: %#v", diag.Current)
	}
}
