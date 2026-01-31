// Package testutil provides testing utilities and mocks for dotstate.
package testutil

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TempDir creates a temporary directory for testing and returns a cleanup function.
// The directory is automatically cleaned up when the test finishes.
func TempDir(t *testing.T) string {
	t.Helper()
	dir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.RemoveAll(dir); err != nil {
			t.Logf("warning: failed to clean up temp dir: %v", err)
		}
	})
	return dir
}

// TempFile creates a temporary file with the given content and returns its path.
// The file is automatically cleaned up when the test finishes.
func TempFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("failed to create parent dirs: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	return path
}

// TempDotToml creates a dot.toml file in the given directory with the provided content.
func TempDotToml(t *testing.T, dir, content string) string {
	t.Helper()
	return TempFile(t, dir, "dot.toml", content)
}

// MinimalDotToml returns a minimal valid dot.toml configuration.
func MinimalDotToml() string {
	return `[repo]
url = "https://github.com/test/dotstate"
path = "~/dotstate"
branch = "main"

[sync]
interval_minutes = 30
enable_idle = true
enable_shutdown = true

[tools]
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"

[wsl]
enable = false
distro_name = ""
flake_ref = ""
`
}

// SetEnv sets an environment variable for the duration of the test.
// The original value is restored when the test finishes.
func SetEnv(t *testing.T, key, value string) {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Setenv(key, value); err != nil {
		t.Fatalf("failed to set env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, original)
		} else {
			os.Unsetenv(key)
		}
	})
}

// UnsetEnv unsets an environment variable for the duration of the test.
// The original value is restored when the test finishes.
func UnsetEnv(t *testing.T, key string) {
	t.Helper()
	original, existed := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatalf("failed to unset env %s: %v", key, err)
	}
	t.Cleanup(func() {
		if existed {
			os.Setenv(key, original)
		}
	})
}

// Chdir changes the working directory for the duration of the test.
// The original directory is restored when the test finishes.
func Chdir(t *testing.T, dir string) {
	t.Helper()
	original, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current dir: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("failed to change dir: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(original); err != nil {
			t.Logf("warning: failed to restore dir: %v", err)
		}
	})
}

// ContextWithCancel returns a context that is cancelled when the test finishes.
func ContextWithCancel(t *testing.T) context.Context {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return ctx
}

// AssertFileExists asserts that a file exists at the given path.
func AssertFileExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("expected file to exist: %s", path)
	}
}

// AssertFileNotExists asserts that a file does not exist at the given path.
func AssertFileNotExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err == nil {
		t.Errorf("expected file to not exist: %s", path)
	}
}

// AssertFileContent asserts that a file contains the expected content.
func AssertFileContent(t *testing.T, path, expected string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Errorf("failed to read file %s: %v", path, err)
		return
	}
	if string(content) != expected {
		t.Errorf("file content mismatch:\n  got:  %q\n  want: %q", string(content), expected)
	}
}
