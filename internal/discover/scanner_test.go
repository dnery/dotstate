package discover

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/dnery/dotstate/dot/internal/platform"
)

func TestDefaultRootsUsesInjectedPlatformFast(t *testing.T) {
	home := t.TempDir()
	codeUser := filepath.Join(home, "Library", "Application Support", "Code", "User")
	if err := ensureDir(codeUser); err != nil {
		t.Fatalf("create Code User path: %v", err)
	}

	scanner := NewScanner(ScanOptions{
		Home:     home,
		Platform: &platform.Platform{OS: platform.Darwin, Home: home},
	})

	roots := scanner.defaultRoots()

	if !containsRoot(roots, codeUser) {
		t.Fatalf("expected roots to include %q, got %v", codeUser, roots)
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

func containsRoot(roots []string, target string) bool {
	for _, root := range roots {
		if root == target {
			return true
		}
	}
	return false
}

func ensureDir(path string) error {
	return os.MkdirAll(path, 0o755)
}
