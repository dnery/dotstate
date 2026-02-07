package secretsenv

import (
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
