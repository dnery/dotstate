package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSecretDetector_ScanFile(t *testing.T) {
	// Create temp files with test content
	tmpDir, err := os.MkdirTemp("", "secret-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name         string
		content      string
		wantFindings int
		wantPatterns []string
	}{
		{
			name:         "aws access key",
			content:      "AWS_ACCESS_KEY_ID=AKIAIOSFODNN7EXAMPLE",
			wantFindings: 1,
			wantPatterns: []string{"aws-access-key"},
		},
		{
			name:         "github token",
			content:      "GITHUB_TOKEN=ghp_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx",
			wantFindings: 2, // github-token and token-assignment both match
			wantPatterns: []string{"github-token"},
		},
		{
			name:         "private key header",
			content:      "-----BEGIN RSA PRIVATE KEY-----",
			wantFindings: 1,
			wantPatterns: []string{"rsa-private-key"},
		},
		{
			name:         "openssh private key",
			content:      "-----BEGIN OPENSSH PRIVATE KEY-----",
			wantFindings: 1,
			wantPatterns: []string{"openssh-private-key"},
		},
		{
			name:         "password in config",
			content:      "password = \"supersecret123\"",
			wantFindings: 1,
			wantPatterns: []string{"password-assignment"},
		},
		{
			name:         "api token assignment",
			content:      "api_token: some-very-long-token-value-here",
			wantFindings: 1,
			wantPatterns: []string{"token-assignment"},
		},
		{
			name:         "postgres connection string",
			content:      "DATABASE_URL=postgres://user:password@localhost:5432/db",
			wantFindings: 1,
			wantPatterns: []string{"postgres-uri"},
		},
		{
			name:         "jwt token",
			content:      "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0NTY3ODkwIiwibmFtZSI6IkpvaG4gRG9lIiwiaWF0IjoxNTE2MjM5MDIyfQ.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
			wantFindings: 1,
			wantPatterns: []string{"jwt-token"},
		},
		{
			name:         "no secrets",
			content:      "# This is a normal config file\nname = \"test\"\nport = 8080",
			wantFindings: 0,
		},
		{
			name:         "multiple secrets",
			content:      "password = \"secret123\"\napi_token = \"long-token-value-here\"",
			wantFindings: 2,
		},
		{
			name:         "slack token",
			content:      "SLACK_TOKEN=xoxa-0000000000-0000000000000-test", // App token format for testing
			wantFindings: 2, // slack-token and token-assignment both match
			wantPatterns: []string{"slack-token"},
		},
		{
			name:         "age secret key",
			content:      "AGE-SECRET-KEY-1ABCDEFGHIJKLMNOPQRSTUVWXYZABCDEFGHIJKLMNOPQRSTUVWXYZ123456",
			wantFindings: 1,
			wantPatterns: []string{"age-secret-key"},
		},
	}

	d := NewSecretDetector(nil)
	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temp file with content
			path := filepath.Join(tmpDir, tt.name+".txt")
			if err := os.WriteFile(path, []byte(tt.content), 0o644); err != nil {
				t.Fatalf("failed to write file: %v", err)
			}

			findings, err := d.ScanFile(ctx, path)
			if err != nil {
				t.Fatalf("ScanFile() error = %v", err)
			}

			if len(findings) != tt.wantFindings {
				t.Errorf("ScanFile() found %d secrets, want %d", len(findings), tt.wantFindings)
				for _, f := range findings {
					t.Logf("  found: %s (%s)", f.PatternID, f.Match)
				}
			}

			// Check for expected patterns
			if len(tt.wantPatterns) > 0 {
				foundPatterns := make(map[string]bool)
				for _, f := range findings {
					foundPatterns[f.PatternID] = true
				}
				for _, p := range tt.wantPatterns {
					if !foundPatterns[p] {
						t.Errorf("ScanFile() missing expected pattern %q", p)
					}
				}
			}
		})
	}
}

func TestSecretDetector_ScanFiles(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secret-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	file1 := filepath.Join(tmpDir, "clean.txt")
	file2 := filepath.Join(tmpDir, "secret.txt")
	file3 := filepath.Join(tmpDir, "also_clean.txt")

	os.WriteFile(file1, []byte("name = test"), 0o644)
	os.WriteFile(file2, []byte("password = secretvalue"), 0o644)
	os.WriteFile(file3, []byte("port = 8080"), 0o644)

	d := NewSecretDetector(nil)
	ctx := context.Background()

	results, err := d.ScanFiles(ctx, []string{file1, file2, file3})
	if err != nil {
		t.Fatalf("ScanFiles() error = %v", err)
	}

	// Only file2 should have findings
	if len(results) != 1 {
		t.Errorf("ScanFiles() returned %d files with findings, want 1", len(results))
	}

	if _, ok := results[file2]; !ok {
		t.Error("ScanFiles() should have findings for secret.txt")
	}
}

func TestSecretDetector_ConfidenceLevel(t *testing.T) {
	d := NewSecretDetector(nil)

	highConfidence := []string{
		"rsa-private-key",
		"openssh-private-key",
		"aws-access-key",
		"github-token",
		"age-secret-key",
	}

	lowConfidence := []string{
		"password-assignment",
		"secret-assignment",
		"token-assignment",
	}

	for _, pattern := range highConfidence {
		level := d.confidenceLevel(pattern)
		if level != "high" {
			t.Errorf("confidenceLevel(%q) = %q, want %q", pattern, level, "high")
		}
	}

	for _, pattern := range lowConfidence {
		level := d.confidenceLevel(pattern)
		if level != "low" {
			t.Errorf("confidenceLevel(%q) = %q, want %q", pattern, level, "low")
		}
	}

	// Medium for unknown patterns
	level := d.confidenceLevel("some-other-pattern")
	if level != "medium" {
		t.Errorf("confidenceLevel(unknown) = %q, want %q", level, "medium")
	}
}

func TestSecretDetector_UpdateCandidates(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "secret-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file with secrets
	secretFile := filepath.Join(tmpDir, "config.txt")
	os.WriteFile(secretFile, []byte("password = supersecret"), 0o644)

	// Create a clean file
	cleanFile := filepath.Join(tmpDir, "clean.txt")
	os.WriteFile(cleanFile, []byte("name = test"), 0o644)

	candidates := CandidateList{
		{Path: secretFile, Category: CategoryRecommended, Reasons: []string{"test"}},
		{Path: cleanFile, Category: CategoryRecommended, Reasons: []string{"test"}},
	}

	d := NewSecretDetector(nil)
	ctx := context.Background()

	if err := d.UpdateCandidates(ctx, candidates); err != nil {
		t.Fatalf("UpdateCandidates() error = %v", err)
	}

	// Secret file should be downgraded to Risky
	if candidates[0].Category != CategoryRisky {
		t.Errorf("secret file category = %v, want %v", candidates[0].Category, CategoryRisky)
	}
	if len(candidates[0].SecretWarnings) == 0 {
		t.Error("secret file should have warnings")
	}

	// Clean file should stay Recommended
	if candidates[1].Category != CategoryRecommended {
		t.Errorf("clean file category = %v, want %v", candidates[1].Category, CategoryRecommended)
	}
	if len(candidates[1].SecretWarnings) != 0 {
		t.Error("clean file should not have warnings")
	}
}
