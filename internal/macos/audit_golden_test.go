package macos

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestBootstrapAuditEnvelopeGolden(t *testing.T) {
	caseDir := macOSFixtureDir(t, "macos", "audit-empty")
	audit := NewBootstrapAudit("darwin", "arm64", "fixture-host.local", time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC))

	compareMacOSGoldenJSON(t, filepath.Join(caseDir, "audit.golden.json"), audit)
	assertMacOSFixtureSentinelsAbsent(t, caseDir)
}

func TestReadOnlyMacOSAuditGolden(t *testing.T) {
	caseDir := macOSFixtureDir(t, "macos", "audit-readonly")
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	brewfile := testutil.TempFile(t, repoDir, "state/macos/brew/Brewfile", "tap \"homebrew/cask\"\nbrew \"git\"\n")
	appInfo := testutil.TempFile(t, homeDir, "Applications/Test App.app/Contents/Info.plist", "{}")
	agentPlist := testutil.TempFile(t, homeDir, "Library/LaunchAgents/com.example.agent.plist", "{}")

	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("brew", "tap"), "homebrew/cask\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--formula", "--versions"), "git 2.51.0\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--cask", "--versions"), "visual-studio-code 1.99.0\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "bundle", "check", "--file", brewfile), "The Brewfile's dependencies are satisfied.\n")
	r.OnCommandSuccess(testutil.MatchExact("mas", "list"), "497799835 Xcode (15.4)\n")
	r.OnCommandSuccess(testutil.MatchExact("plutil", "-convert", "json", "-o", "-", appInfo), `{"CFBundleIdentifier":"com.example.TestApp","CFBundleDisplayName":"Test App","CFBundleShortVersionString":"1.2.3"}`)
	r.OnCommandSuccess(testutil.MatchExact("plutil", "-convert", "json", "-o", "-", agentPlist), `{"Label":"com.example.agent"}`)
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "print", "gui/501/com.example.agent"), "service state = running\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "services", "list", "--json"), `[{"name":"postgresql@16","status":"started","user":"fixture-user","file":"`+homeDir+`/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist"}]`)
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "NSGlobalDomain", "AppleShowAllExtensions"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.finder", "AppleShowAllFiles"), "0\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.dock", "autohide"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("profiles", "status", "-type", "enrollment"), "Enrolled via DEP: No\nMDM enrollment: Yes\n")

	audit := NewAudit(context.Background(), AuditOptions{
		GOOS:           "darwin",
		Arch:           "arm64",
		Host:           "fixture-host.local",
		GeneratedAt:    time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
		Runner:         r,
		RepoRoot:       repoDir,
		HomeDir:        homeDir,
		UID:            "501",
		BrewfilePath:   brewfile,
		AppDirs:        []string{filepath.Join(homeDir, "Applications")},
		LaunchAgentDir: filepath.Join(homeDir, "Library", "LaunchAgents"),
	})

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
		if entry.IsDir() || entry.Name() == "README.md" || entry.Name() == "redaction.assert_absent.txt" {
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
