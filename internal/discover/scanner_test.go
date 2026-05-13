package discover

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/platform"
)

func TestDefaultRootsUsesInjectedPlatformFast(t *testing.T) {
	home := t.TempDir()
	codeSettings := filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json")
	if err := ensureDir(filepath.Dir(codeSettings)); err != nil {
		t.Fatalf("create Code User path: %v", err)
	}
	if err := os.WriteFile(codeSettings, []byte("{}"), 0o644); err != nil {
		t.Fatalf("write Code settings: %v", err)
	}

	scanner := NewScanner(ScanOptions{
		Home:     home,
		Platform: &platform.Platform{OS: platform.Darwin, Home: home},
	})

	roots := scanner.defaultRoots()

	if !containsRoot(roots, codeSettings) {
		t.Fatalf("expected roots to include %q, got %v", codeSettings, roots)
	}
	if containsRoot(roots, filepath.Dir(codeSettings)) {
		t.Fatalf("fast roots should not include broad Code/User directory: %v", roots)
	}
}

func TestDefaultRootsAvoidsBroadConfigUntilDeep(t *testing.T) {
	home := t.TempDir()
	if err := ensureDir(filepath.Join(home, ".config", "noisy", "node_modules")); err != nil {
		t.Fatalf("create noisy config: %v", err)
	}
	nvimInit := filepath.Join(home, ".config", "nvim", "init.lua")
	if err := ensureDir(filepath.Dir(nvimInit)); err != nil {
		t.Fatalf("create curated config: %v", err)
	}
	if err := os.WriteFile(nvimInit, []byte("vim.opt.number = true\n"), 0o644); err != nil {
		t.Fatalf("write nvim init: %v", err)
	}

	scanner := NewScanner(ScanOptions{
		Home:     home,
		Platform: &platform.Platform{OS: platform.Darwin, Home: home},
	})

	roots := scanner.defaultRoots()
	if containsRoot(roots, filepath.Join(home, ".config")) {
		t.Fatalf("fast roots should not include broad ~/.config: %v", roots)
	}
	if !containsRoot(roots, nvimInit) {
		t.Fatalf("fast roots should include curated ~/.config/nvim/init.lua: %v", roots)
	}
}

func TestDefaultRootsUsesInjectedPlatformDeep(t *testing.T) {
	home := t.TempDir()

	scanner := NewScanner(ScanOptions{
		Deep:     true,
		Home:     home,
		Platform: &platform.Platform{OS: platform.Darwin, Home: home},
	})

	roots := scanner.defaultRoots()

	appSupport := filepath.Join(home, "Library", "Application Support")
	preferences := filepath.Join(home, "Library", "Preferences")

	if !containsRoot(roots, appSupport) {
		t.Fatalf("expected roots to include %q, got %v", appSupport, roots)
	}
	if !containsRoot(roots, preferences) {
		t.Fatalf("expected roots to include %q, got %v", preferences, roots)
	}
}

func TestScanIncludesDotfilesWhenMaxFileSizeUnset(t *testing.T) {
	home := t.TempDir()
	if err := os.WriteFile(filepath.Join(home, ".zshrc"), []byte("export EDITOR=vim\n"), 0o644); err != nil {
		t.Fatalf("write .zshrc: %v", err)
	}

	scanner := NewScanner(ScanOptions{
		Home:     home,
		Platform: &platform.Platform{OS: platform.Darwin, Home: home},
		// MaxFileSize intentionally unset (0) to verify it does not exclude everything.
		ManagedPaths: make(map[string]bool),
	})

	result, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("scan: %v", err)
	}

	found := false
	for _, candidate := range result.Candidates {
		if candidate.RelPath == "~/.zshrc" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected ~/.zshrc candidate with MaxFileSize=0, got %d candidates", len(result.Candidates))
	}
}

func TestScanAppliesNoiseAndUserIgnoreFilters(t *testing.T) {
	home := t.TempDir()
	root := filepath.Join(home, "Library", "Application Support", "NoisyApp")
	files := map[string]string{
		"settings.json":               "{}",
		"Cache/generated.json":        "{}",
		"bundle.js.map":               "{}",
		"Browser/History.sqlite":      "sqlite",
		"Generated.app/Contents/info": "ignored",
		"ignored.toml":                "ignored = true",
	}
	for rel, content := range files {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(root, rel)), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", rel, err)
		}
		if err := os.WriteFile(filepath.Join(root, rel), []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", rel, err)
		}
	}

	scanner := NewScanner(ScanOptions{
		Home:           home,
		Roots:          []string{root},
		ManagedPaths:   make(map[string]bool),
		IgnorePatterns: []string{"ignored.toml"},
	})
	result, err := scanner.Scan(context.Background())
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}
	var rels []string
	for _, candidate := range result.Candidates {
		rels = append(rels, candidate.RelPath)
	}
	if !containsString(rels, "~/Library/Application Support/NoisyApp/settings.json") {
		t.Fatalf("expected settings candidate, got %v", rels)
	}
	for _, unwanted := range []string{"bundle.js.map", "History.sqlite", "generated.json", "ignored.toml"} {
		for _, rel := range rels {
			if strings.Contains(rel, unwanted) {
				t.Fatalf("unexpected noisy candidate %s in %v", unwanted, rels)
			}
		}
	}
	if result.Ignored["user ignore registry"] == 0 || result.Ignored["generated/cache/browser file"] == 0 || result.Ignored["cache/vendor/browser/generated directory"] == 0 {
		t.Fatalf("ignored summary missing expected reasons: %#v", result.Ignored)
	}
}

func containsRoot(roots []string, target string) bool {
	for _, root := range roots {
		if root == target {
			return true
		}
	}
	return false
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
