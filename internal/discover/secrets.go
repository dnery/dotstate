package discover

import (
	"bufio"
	"context"
	"os"
	"regexp"
	"strconv"

	"github.com/dnery/dotstate/dot/internal/runner"
)

// SecretDetector scans files for potential secrets before adding them.
//
// It uses a combination of:
// 1. Built-in regex patterns (similar to gitleaks)
// 2. External gitleaks binary if available
// 3. Chezmoi's --secrets=error flag during add
type SecretDetector struct {
	patterns []*secretPattern
	runner   runner.Runner
}

// secretPattern defines a pattern to match potential secrets.
type secretPattern struct {
	Name    string
	Regex   *regexp.Regexp
	Entropy float64 // Minimum entropy threshold (0 = disabled)
}

// SecretFinding represents a potential secret found in a file.
type SecretFinding struct {
	File       string
	Line       int
	Match      string
	PatternID  string
	Confidence string // "high", "medium", "low"
}

// NewSecretDetector creates a new secret detector.
func NewSecretDetector(r runner.Runner) *SecretDetector {
	if r == nil {
		r = runner.New()
	}

	return &SecretDetector{
		patterns: defaultSecretPatterns(),
		runner:   r,
	}
}

// defaultSecretPatterns returns the built-in secret detection patterns.
// These are similar to gitleaks patterns but simplified for quick scanning.
func defaultSecretPatterns() []*secretPattern {
	patterns := []struct {
		name  string
		regex string
	}{
		// API Keys and Tokens
		{"aws-access-key", `AKIA[0-9A-Z]{16}`},
		{"aws-secret-key", `(?i)aws[_\-]?secret[_\-]?access[_\-]?key\s*[:=]\s*[A-Za-z0-9/+=]{40}`},
		{"github-token", `gh[pousr]_[A-Za-z0-9_]{36,}`},
		{"github-oauth", `gho_[A-Za-z0-9]{36}`},
		{"gitlab-token", `glpat-[A-Za-z0-9\-_]{20,}`},
		{"slack-token", `xox[baprs]-[0-9]{10,13}-[0-9]{10,13}[a-zA-Z0-9-]*`},
		{"slack-webhook", `https://hooks\.slack\.com/services/T[A-Z0-9]+/B[A-Z0-9]+/[A-Za-z0-9]+`},
		{"discord-webhook", `https://discord(app)?\.com/api/webhooks/[0-9]+/[A-Za-z0-9_-]+`},
		{"stripe-key", `sk_live_[A-Za-z0-9]{24,}`},
		{"stripe-restricted", `rk_live_[A-Za-z0-9]{24,}`},
		{"twilio-key", `SK[a-f0-9]{32}`},
		{"sendgrid-key", `SG\.[A-Za-z0-9_-]{22}\.[A-Za-z0-9_-]{43}`},
		{"npm-token", `npm_[A-Za-z0-9]{36}`},
		{"pypi-token", `pypi-AgEIcHlwaS5vcmc[A-Za-z0-9_-]{50,}`},
		{"heroku-key", `[hH]eroku.*[0-9A-F]{8}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{4}-[0-9A-F]{12}`},
		{"google-api-key", `AIza[0-9A-Za-z_-]{35}`},
		{"google-oauth", `[0-9]+-[0-9A-Za-z_]{32}\.apps\.googleusercontent\.com`},
		{"firebase-key", `AAAA[A-Za-z0-9_-]{7}:[A-Za-z0-9_-]{140}`},
		{"azure-key", `(?i)azure[_\-]?[a-z]*[_\-]?key\s*[:=]\s*[A-Za-z0-9+/=]{43,}`},
		{"digitalocean-token", `dop_v1_[a-f0-9]{64}`},
		{"1password-token", `ops_[A-Za-z0-9_-]{40,}`},

		// Private Keys
		{"rsa-private-key", `-----BEGIN RSA PRIVATE KEY-----`},
		{"openssh-private-key", `-----BEGIN OPENSSH PRIVATE KEY-----`},
		{"dsa-private-key", `-----BEGIN DSA PRIVATE KEY-----`},
		{"ec-private-key", `-----BEGIN EC PRIVATE KEY-----`},
		{"pgp-private-key", `-----BEGIN PGP PRIVATE KEY BLOCK-----`},
		{"age-secret-key", `AGE-SECRET-KEY-[A-Z0-9]{59}`},

		// Passwords and credentials in config
		{"password-assignment", `(?i)(password|passwd|pwd)\s*[:=]\s*['"]*[^\s'"]{8,}['"]*`},
		{"secret-assignment", `(?i)secret\s*[:=]\s*['"]*[^\s'"]{8,}['"]*`},
		{"token-assignment", `(?i)(api[_\-]?)?token\s*[:=]\s*['"]*[^\s'"]{16,}['"]*`},
		{"auth-assignment", `(?i)(auth|credential)[_\-]?(key|token|secret)?\s*[:=]\s*['"]*[^\s'"]{8,}['"]*`},

		// Connection strings
		{"postgres-uri", `postgres(?:ql)?://[^:]+:[^@]+@[^/]+`},
		{"mysql-uri", `mysql://[^:]+:[^@]+@[^/]+`},
		{"mongodb-uri", `mongodb(\+srv)?://[^:]+:[^@]+@[^/]+`},
		{"redis-uri", `redis://:[^@]+@[^/]+`},

		// JWT tokens (they often contain sensitive claims)
		{"jwt-token", `eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`},
	}

	result := make([]*secretPattern, 0, len(patterns))
	for _, p := range patterns {
		re, err := regexp.Compile(p.regex)
		if err != nil {
			continue // Skip invalid patterns
		}
		result = append(result, &secretPattern{
			Name:  p.name,
			Regex: re,
		})
	}

	return result
}

// ScanFile scans a file for potential secrets.
func (d *SecretDetector) ScanFile(ctx context.Context, path string) ([]SecretFinding, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var findings []SecretFinding
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()

		// Check against all patterns
		for _, pattern := range d.patterns {
			if ctx.Err() != nil {
				return findings, ctx.Err()
			}

			if match := pattern.Regex.FindString(line); match != "" {
				// Truncate long matches for display
				displayMatch := match
				if len(displayMatch) > 40 {
					displayMatch = displayMatch[:20] + "..." + displayMatch[len(displayMatch)-10:]
				}

				findings = append(findings, SecretFinding{
					File:       path,
					Line:       lineNum,
					Match:      displayMatch,
					PatternID:  pattern.Name,
					Confidence: d.confidenceLevel(pattern.Name),
				})
			}
		}
	}

	return findings, scanner.Err()
}

// ScanFiles scans multiple files for secrets.
func (d *SecretDetector) ScanFiles(ctx context.Context, paths []string) (map[string][]SecretFinding, error) {
	results := make(map[string][]SecretFinding)

	for _, path := range paths {
		if ctx.Err() != nil {
			return results, ctx.Err()
		}

		findings, err := d.ScanFile(ctx, path)
		if err != nil {
			// Log error but continue scanning other files
			continue
		}

		if len(findings) > 0 {
			results[path] = findings
		}
	}

	return results, nil
}

// HasGitleaks returns true if gitleaks is available.
func (d *SecretDetector) HasGitleaks(ctx context.Context) bool {
	_, err := d.runner.Run(ctx, "", "gitleaks", "version")
	return err == nil
}

// ScanWithGitleaks uses the gitleaks binary for more comprehensive scanning.
func (d *SecretDetector) ScanWithGitleaks(ctx context.Context, paths []string) ([]SecretFinding, error) {
	// This is a placeholder for gitleaks integration
	// In production, we would:
	// 1. Write paths to a temp file
	// 2. Run gitleaks with --source pointing to the paths
	// 3. Parse the JSON output
	// For now, fall back to built-in scanning
	var allFindings []SecretFinding
	for _, path := range paths {
		findings, err := d.ScanFile(ctx, path)
		if err != nil {
			continue
		}
		allFindings = append(allFindings, findings...)
	}
	return allFindings, nil
}

// confidenceLevel returns the confidence level for a pattern.
func (d *SecretDetector) confidenceLevel(patternID string) string {
	highConfidence := map[string]bool{
		"rsa-private-key":     true,
		"openssh-private-key": true,
		"dsa-private-key":     true,
		"ec-private-key":      true,
		"pgp-private-key":     true,
		"age-secret-key":      true,
		"aws-access-key":      true,
		"github-token":        true,
		"github-oauth":        true,
		"gitlab-token":        true,
		"slack-token":         true,
		"stripe-key":          true,
		"npm-token":           true,
		"1password-token":     true,
	}

	if highConfidence[patternID] {
		return "high"
	}

	lowConfidence := map[string]bool{
		"password-assignment": true,
		"secret-assignment":   true,
		"token-assignment":    true,
		"auth-assignment":     true,
	}

	if lowConfidence[patternID] {
		return "low"
	}

	return "medium"
}

// UpdateCandidates updates candidates with secret scan findings.
func (d *SecretDetector) UpdateCandidates(ctx context.Context, candidates CandidateList) error {
	for _, c := range candidates {
		if c.IsDir || c.IsSubRepo {
			continue
		}

		findings, err := d.ScanFile(ctx, c.Path)
		if err != nil {
			continue
		}

		if len(findings) > 0 {
			// Downgrade to Risky if secrets found
			if c.Category == CategoryRecommended || c.Category == CategoryMaybe {
				c.Category = CategoryRisky
			}

			for _, f := range findings {
				c.SecretWarnings = append(c.SecretWarnings,
					f.PatternID+": "+f.Match+" (line "+strconv.Itoa(f.Line)+")")
			}
			c.Reasons = append(c.Reasons, "potential secrets detected")
		}
	}

	return nil
}

