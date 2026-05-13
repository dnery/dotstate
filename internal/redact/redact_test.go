package redact

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestStringRedactsSentinelAndPromotesSensitivity(t *testing.T) {
	got, report := String("token=DOTSTATE_TEST_SECRET_DO_NOT_PRINT")
	if strings.Contains(got, "DOTSTATE_TEST_SECRET_DO_NOT_PRINT") {
		t.Fatalf("String leaked sentinel: %q", got)
	}
	if !strings.Contains(got, "<redacted:secret>") {
		t.Fatalf("String = %q, want redaction marker", got)
	}
	if report.Redactions == 0 || report.Sensitivity != SensitivitySecret {
		t.Fatalf("report = %#v, want secret redaction", report)
	}
}

func TestStringRedactsCredentialedURLButKeepsHostPath(t *testing.T) {
	got, report := String("remote=https://user:DOTSTATE_TEST_SECRET_DO_NOT_PRINT@github.com/dnery/dotstate.git")
	if strings.Contains(got, "DOTSTATE_TEST_SECRET_DO_NOT_PRINT") || strings.Contains(got, "user:") {
		t.Fatalf("String leaked credentials: %q", got)
	}
	if !strings.Contains(got, "https://<redacted:credential>@github.com/dnery/dotstate.git") {
		t.Fatalf("String = %q, want sanitized remote", got)
	}
	if report.Sensitivity != SensitivitySecret {
		t.Fatalf("sensitivity = %v, want secret", report.Sensitivity)
	}
}

func TestStringTreatsOpReferenceAsCredentialReference(t *testing.T) {
	got, report := String("source=op://Employee/local-env/API_TOKEN")
	if got != "source=op://Employee/local-env/API_TOKEN" {
		t.Fatalf("op reference changed: %q", got)
	}
	if report.Sensitivity != SensitivityCredentialReference || report.Redactions != 0 {
		t.Fatalf("report = %#v, want credential reference without redaction", report)
	}
}

func TestValueRedactsNestedStringsAndMapKeys(t *testing.T) {
	value := map[string]any{
		"safe": "ok",
		"DOTSTATE_TEST_SECRET_DO_NOT_PRINT": []string{
			"password=DOTSTATE_TEST_SECRET_DO_NOT_PRINT",
		},
	}
	got, report := Value(value)
	encoded, err := json.Marshal(got)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	if strings.Contains(string(encoded), "DOTSTATE_TEST_SECRET_DO_NOT_PRINT") {
		t.Fatalf("Value leaked sentinel: %s", encoded)
	}
	if report.Sensitivity != SensitivitySecret || report.Redactions < 2 {
		t.Fatalf("report = %#v, want nested secret redactions", report)
	}
}

func TestStringMarksRestrictedHints(t *testing.T) {
	_, report := String("/Library/Application Support/com.apple.TCC/TCC.db")
	if report.Sensitivity != SensitivityRestricted {
		t.Fatalf("sensitivity = %v, want restricted", report.Sensitivity)
	}
}
