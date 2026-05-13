package macos

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestBootstrapAuditEnvelopeGolden(t *testing.T) {
	caseDir := macOSFixtureDir(t, "macos", "audit-empty")
	audit := NewBootstrapAudit("darwin", "arm64", "fixture-host.local", time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC))

	compareMacOSGoldenJSON(t, filepath.Join(caseDir, "audit.golden.json"), audit)
	assertMacOSFixtureSentinelsAbsent(t, caseDir)
}

func macOSFixtureDir(t *testing.T, parts ...string) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
	elems := append([]string{repoRoot, "test", "fixtures", "modules", "v1"}, parts...)
	return filepath.Join(elems...)
}

func compareMacOSGoldenJSON(t *testing.T, path string, got any) {
	t.Helper()
	actual, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("marshal actual JSON: %v", err)
	}
	actual = append(actual, '\n')
	if os.Getenv("DOTSTATE_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, actual, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if !bytes.Equal(want, actual) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\nactual:\n%s", path, want, actual)
	}
}

func assertMacOSFixtureSentinelsAbsent(t *testing.T, dir string) {
	t.Helper()
	assertPath := filepath.Join(dir, "redaction.assert_absent.txt")
	data, err := os.ReadFile(assertPath)
	if err != nil {
		t.Fatalf("read %s: %v", assertPath, err)
	}
	var sentinels []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		sentinels = append(sentinels, line)
	}
	if len(sentinels) == 0 {
		t.Fatalf("%s did not declare any sentinel values", assertPath)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read fixture dir: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("read fixture json: %v", err)
		}
		for _, sentinel := range sentinels {
			if strings.Contains(string(content), sentinel) {
				t.Fatalf("fixture %s leaked sentinel %q", entry.Name(), sentinel)
			}
		}
	}
}
