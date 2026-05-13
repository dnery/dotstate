package secretsenv

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestParseDotenv(t *testing.T) {
	env, err := parseDotenv(`
# comment
export FOO="bar\nbaz"
PLAIN=value
SINGLE='two words'
1BAD=skip
`)
	if err != nil {
		t.Fatal(err)
	}
	if env["FOO"] != "bar\nbaz" {
		t.Fatalf("FOO parsed incorrectly: %q", env["FOO"])
	}
	if env["PLAIN"] != "value" {
		t.Fatalf("PLAIN parsed incorrectly: %q", env["PLAIN"])
	}
	if env["SINGLE"] != "two words" {
		t.Fatalf("SINGLE parsed incorrectly: %q", env["SINGLE"])
	}
	if _, ok := env["1BAD"]; ok {
		t.Fatalf("invalid env name was accepted")
	}
}

func TestWriteCacheSetIsPrivateAndCleansTempFiles(t *testing.T) {
	dir := t.TempDir()
	values := map[string]fieldValue{
		"API_KEY": {Label: "API_KEY", Value: "abc'123", Type: "CONCEALED"},
	}
	if err := writeCacheSet(dir, "test", values); err != nil {
		t.Fatal(err)
	}
	for _, ext := range []string{".env", ".fish", ".ps1"} {
		path := filepath.Join(dir, "test"+ext)
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("%s mode = %04o, want 0600", path, info.Mode().Perm())
		}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temp cache file was left behind: %s", entry.Name())
		}
	}
}

func TestRenderPOSIXQuotes(t *testing.T) {
	got := renderPOSIX(map[string]fieldValue{
		"API_KEY": {Label: "API_KEY", Value: "a'b", Type: "CONCEALED"},
	})
	if !strings.Contains(got, "export API_KEY='a'\\''b'") {
		t.Fatalf("unexpected POSIX rendering: %s", got)
	}
}

func TestAggregateExcludeSet(t *testing.T) {
	cfg := Config{AggregateExclude: []string{"OP_SERVICE_ACCOUNT_TOKEN"}}
	if !cfg.aggregateExcludeSet()["OP_SERVICE_ACCOUNT_TOKEN"] {
		t.Fatalf("expected OP_SERVICE_ACCOUNT_TOKEN to be excluded")
	}
}

func TestScrubOPEnv(t *testing.T) {
	got := scrubOPEnv([]string{
		"OP_ACCOUNT=my.1password.com",
		"OP_SERVICE_ACCOUNT_TOKEN=secret",
		"PATH=/bin",
	})
	if strings.Join(got, "\n") != "PATH=/bin" {
		t.Fatalf("unexpected scrubbed env: %#v", got)
	}
}

func TestRefreshFailureLeavesNoTempCaches(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("shell script fake op is Unix-only")
	}
	dir := t.TempDir()
	fakeOP := filepath.Join(dir, "op")
	if err := os.WriteFile(fakeOP, []byte("#!/bin/sh\nexit 42\n"), 0o700); err != nil {
		t.Fatal(err)
	}
	cfg := &Config{
		OPBin:    fakeOP,
		CacheDir: dir,
		Scopes: []ScopeConfig{{
			Name:    "personal",
			Account: "acct",
			Vault:   "Personal",
			Item:    "local/secrets",
			Section: "secrets",
		}},
	}
	a := &app{stdout: io.Discard, stderr: io.Discard, getenv: os.Getenv}
	if err := a.refresh(cfg, []string{"--all"}); err == nil {
		t.Fatal("expected refresh to fail")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.Contains(entry.Name(), ".tmp-") {
			t.Fatalf("temp cache file was left behind: %s", entry.Name())
		}
	}
}

func TestExportableValuesWithSectionExcludesUnsectionedFields(t *testing.T) {
	item := opItem{Fields: []opField{
		{Label: "SCOPED", Value: "yes", Section: &opSection{Label: "sfr3"}},
		{Label: "UNSCOPED", Value: "no"},
		{Label: "OTHER", Value: "no", Section: &opSection{Label: "personal"}},
	}}

	values := exportableValues(item, "sfr3")
	if len(values) != 1 || values["SCOPED"].Value != "yes" {
		t.Fatalf("section export included wrong values: %#v", values)
	}
	if count := countExportableFieldLabels(item, "sfr3"); count != 1 {
		t.Fatalf("section count = %d, want 1", count)
	}
}

func TestConfigValidateRejectsUnsafeScopeNames(t *testing.T) {
	cfg := &Config{Scopes: []ScopeConfig{{Name: "../outside", Account: "acct", Vault: "vault", Item: "item"}}}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected unsafe scope name to be rejected")
	}

	cfg = &Config{Scopes: []ScopeConfig{{Name: aggregateName, Account: "acct", Vault: "vault", Item: "item"}}}
	if err := cfg.validate(); err == nil {
		t.Fatal("expected aggregate scope name to be reserved")
	}
}

func TestWriteCacheSetTightensExistingCacheDir(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX mode bits are not enforced on Windows")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o777); err != nil {
		t.Fatal(err)
	}
	values := map[string]fieldValue{"API_KEY": {Label: "API_KEY", Value: "secret", Type: "CONCEALED"}}
	if err := writeCacheSet(dir, "test", values); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o700 {
		t.Fatalf("cache dir mode = %04o, want 0700", info.Mode().Perm())
	}
}

func TestWriteAtomicReplacesExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.env")
	if err := os.WriteFile(path, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := writeAtomic(path, []byte("new")); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "new" {
		t.Fatalf("cache content = %q, want new", string(got))
	}
}

func TestFindSecureNoteIDPrefersIDAndRejectsAmbiguousTitles(t *testing.T) {
	items := []opListItem{
		{ID: "note-id", Title: "Shared", Category: "SECURE_NOTE"},
		{ID: "api-id", Title: "API", Category: "API_CREDENTIAL"},
	}

	id, ok, err := findSecureNoteID(items, "note-id")
	if err != nil || !ok || id != "note-id" {
		t.Fatalf("find by id = (%q, %v, %v), want note-id, true, nil", id, ok, err)
	}

	_, _, err = findSecureNoteID(items, "api-id")
	if err == nil {
		t.Fatal("expected non-secure item id to be rejected")
	}

	_, _, err = findSecureNoteID(append(items, opListItem{ID: "note-id-2", Title: "Shared", Category: "SECURE_NOTE"}), "Shared")
	if err == nil {
		t.Fatal("expected duplicate secure-note titles to be rejected")
	}
}

func TestAggregateValuesReportsOverrides(t *testing.T) {
	refreshed := map[string]map[string]fieldValue{
		"employee-secrets": {
			"API_KEY": {Label: "API_KEY", Value: "one", Type: "CONCEALED"},
		},
		"employee-project": {
			"API_KEY": {Label: "API_KEY", Value: "two", Type: "CONCEALED"},
			"OTHER":   {Label: "OTHER", Value: "value", Type: "STRING"},
		},
	}
	sources := map[string]string{
		"employee-secrets": "Employee local/secrets",
		"employee-project": "Employee local/project",
	}

	merged, conflicts := aggregateValues(refreshed, sources, []string{"employee-secrets", "employee-project"}, nil)
	if merged["API_KEY"].Value != "two" {
		t.Fatalf("last aggregate value should win, got %q", merged["API_KEY"].Value)
	}
	if len(conflicts) != 1 || conflicts[0].Name != "API_KEY" || !conflicts[0].Different {
		t.Fatalf("unexpected conflicts: %#v", conflicts)
	}
	if len(conflicts[0].Occurrences) != 2 || conflicts[0].Occurrences[1].Source != "Employee local/project" {
		t.Fatalf("unexpected conflict occurrences: %#v", conflicts[0].Occurrences)
	}
}

func TestExportableValuesIgnoresNoteBodyDotenv(t *testing.T) {
	item := opItem{Fields: []opField{
		{Label: "notesPlain", Value: "API_KEY='secret'\nPLAIN=value"},
		{Label: "DIRECT", Value: "field", Type: "STRING"},
	}}

	values := exportableValues(item, "")
	if _, ok := values["API_KEY"]; ok {
		t.Fatal("notesPlain dotenv should not be exported")
	}
	if values["DIRECT"].Value != "field" {
		t.Fatalf("unexpected values: %#v", values)
	}
}

func TestFieldTypeForLabelKeepsNonSecretIDsAsText(t *testing.T) {
	cases := map[string]string{
		"GOOGLE_OAUTH_CLIENT_ID":       "STRING",
		"DOCUSEAL_WEBHOOK_SECRET_NAME": "CONCEALED",
		"GOOGLE_PICKER_API_KEY":        "CONCEALED",
		"NEXT_PUBLIC_AUTH_KEYS_URL":    "STRING",
		"DB_PASS":                      "CONCEALED",
		"DB_PWD":                       "CONCEALED",
		"SSH_KEY":                      "CONCEALED",
	}
	for label, want := range cases {
		if got := fieldTypeForLabel(label); got != want {
			t.Fatalf("fieldTypeForLabel(%q) = %q, want %q", label, got, want)
		}
	}
}

func TestLocalNoteScopeNameIncludesVaultAndTitle(t *testing.T) {
	got := localNoteScopeName("Employee", "local/sfr3-signature-flow")
	if got != "employee-sfr3-signature-flow" {
		t.Fatalf("scope name = %q", got)
	}
}

func TestConfiguredLocalNoteScopesPreserveSections(t *testing.T) {
	cfg := Config{Scopes: []ScopeConfig{
		{Name: "personal", Account: "acct", Vault: "Employee", Item: "local/secrets", Section: "personal"},
		{Name: "work", Account: "acct", Vault: "Employee", Item: "local/secrets", Section: "sfr3"},
	}}
	scopes := cfg.configuredScopesForLocalNote("acct", "Employee", opListItem{ID: "item-id", Title: "local/secrets", Category: "SECURE_NOTE"})
	if len(scopes) != 2 {
		t.Fatalf("configured local note scope matches = %d, want 2", len(scopes))
	}
	if scopes[0].Section != "personal" || scopes[1].Section != "sfr3" {
		t.Fatalf("configured local note sections = %#v", scopes)
	}
}

func TestUniqueLocalNoteScopeNameAvoidsReservedAndAssignedNames(t *testing.T) {
	reserved := map[string]bool{"employee-sfr3": true}
	assigned := map[string]bool{"employee-sfr3-2": true}
	got := uniqueLocalNoteScopeName("Employee", "local/sfr3", reserved, assigned)
	if got != "employee-sfr3-3" {
		t.Fatalf("unique scope name = %q, want employee-sfr3-3", got)
	}
}

func TestStatusReturnsCacheDirReadErrors(t *testing.T) {
	cachePath := filepath.Join(t.TempDir(), "not-a-dir")
	if err := os.WriteFile(cachePath, []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{stdout: io.Discard, stderr: io.Discard, getenv: os.Getenv}
	if err := a.status(&Config{CacheDir: cachePath}); err == nil {
		t.Fatal("expected status to fail when cache dir cannot be read")
	}
}

func TestRunCommandReturnsChildExitStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("runCommand uses /bin/sh")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, aggregateName+".env"), []byte("export TEST_VALUE='ok'\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	a := &app{stdout: io.Discard, stderr: io.Discard, getenv: os.Getenv}
	err := a.runCommand(&Config{CacheDir: dir}, []string{"--", "/bin/sh", "-c", "exit 42"})
	var exitErr *commandExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("expected commandExitError, got %T %[1]v", err)
	}
	if exitErr.ExitCode() != 42 {
		t.Fatalf("exit code = %d, want 42", exitErr.ExitCode())
	}
}
