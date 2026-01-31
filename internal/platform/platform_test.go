package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCurrent(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	if p.OS != OS(runtime.GOOS) {
		t.Errorf("OS = %v, want %v", p.OS, runtime.GOOS)
	}
	if p.Arch != runtime.GOARCH {
		t.Errorf("Arch = %v, want %v", p.Arch, runtime.GOARCH)
	}
	if p.Home == "" {
		t.Error("Home is empty")
	}
	if p.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}
	if p.DataDir == "" {
		t.Error("DataDir is empty")
	}
	if p.CacheDir == "" {
		t.Error("CacheDir is empty")
	}
	if p.StateDir == "" {
		t.Error("StateDir is empty")
	}
}

func TestPlatformMethods(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	// Test OS detection methods
	osCount := 0
	if p.IsDarwin() {
		osCount++
	}
	if p.IsLinux() {
		osCount++
	}
	if p.IsWindows() {
		osCount++
	}

	// Exactly one should be true
	if osCount != 1 {
		t.Errorf("Expected exactly one OS method to return true, got %d", osCount)
	}
}

func TestExpandPath(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "empty",
			input: "",
			want:  "",
		},
		{
			name:  "tilde only",
			input: "~",
			want:  p.Home,
		},
		{
			name:  "tilde with path",
			input: "~/foo/bar",
			want:  filepath.Join(p.Home, "foo", "bar"),
		},
		{
			name:  "absolute path",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := p.ExpandPath(tt.input)
			if got != tt.want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPathWithEnv(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	os.Setenv("TEST_PLATFORM_VAR", "test_value")
	defer os.Unsetenv("TEST_PLATFORM_VAR")

	got := p.ExpandPath("$TEST_PLATFORM_VAR/subpath")
	want := "test_value/subpath"
	if got != want {
		t.Errorf("ExpandPath() = %q, want %q", got, want)
	}
}

func TestPaths(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	paths := p.Paths()

	// All paths should be non-empty
	if paths.ConfigDir == "" {
		t.Error("ConfigDir is empty")
	}
	if paths.DataDir == "" {
		t.Error("DataDir is empty")
	}
	if paths.CacheDir == "" {
		t.Error("CacheDir is empty")
	}
	if paths.LogDir == "" {
		t.Error("LogDir is empty")
	}

	// All paths should contain "dotstate"
	if !containsString(paths.ConfigDir, "dotstate") {
		t.Errorf("ConfigDir should contain 'dotstate': %v", paths.ConfigDir)
	}
}

func TestHostname(t *testing.T) {
	host := Hostname()
	if host == "" {
		t.Error("Hostname() returned empty string")
	}
}

func TestUsername(t *testing.T) {
	user := Username()
	if user == "" {
		t.Error("Username() returned empty string")
	}
}

func TestToSlash(t *testing.T) {
	// Note: ToSlash uses filepath.ToSlash which only converts backslashes
	// to forward slashes on Windows. On Unix, backslash is a valid
	// filename character and is preserved.
	tests := []struct {
		input string
		want  string
	}{
		{"foo/bar", "foo/bar"},
		{"foo/bar/baz", "foo/bar/baz"},
	}

	for _, tt := range tests {
		got := ToSlash(tt.input)
		if got != tt.want {
			t.Errorf("ToSlash(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestBrowserProfiles(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	profiles := p.BrowserProfiles()

	// Should have firefox at minimum
	if _, ok := profiles["firefox"]; !ok {
		t.Error("BrowserProfiles() missing 'firefox'")
	}

	// All paths should be non-empty
	for name, path := range profiles {
		if path == "" {
			t.Errorf("BrowserProfiles()[%q] is empty", name)
		}
	}
}

func TestSSHDir(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	sshDir := p.SSHDir()
	if sshDir == "" {
		t.Error("SSHDir() returned empty string")
	}
	if !containsString(sshDir, ".ssh") {
		t.Errorf("SSHDir() should contain '.ssh': %v", sshDir)
	}
}

func TestConfigLocations(t *testing.T) {
	p, err := Current()
	if err != nil {
		t.Fatalf("Current() error = %v", err)
	}

	locs := p.ConfigLocations()

	// Should have some home-relative paths
	if len(locs.Home) == 0 {
		t.Error("ConfigLocations().Home is empty")
	}

	// Should include common files
	found := false
	for _, path := range locs.Home {
		if path == ".gitconfig" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ConfigLocations().Home should include '.gitconfig'")
	}
}

func TestExistsAll(t *testing.T) {
	// Create a temp directory with some files
	tmpDir, err := os.MkdirTemp("", "platform-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create some files
	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0o644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	nonExistingFile := filepath.Join(tmpDir, "nonexistent.txt")

	tests := []struct {
		name  string
		paths []string
		want  int
	}{
		{
			name:  "all exist",
			paths: []string{existingFile},
			want:  1,
		},
		{
			name:  "none exist",
			paths: []string{nonExistingFile},
			want:  0,
		},
		{
			name:  "mixed",
			paths: []string{existingFile, nonExistingFile},
			want:  1,
		},
		{
			name:  "empty",
			paths: []string{},
			want:  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExistsAll(tt.paths)
			if len(got) != tt.want {
				t.Errorf("ExistsAll() returned %d paths, want %d", len(got), tt.want)
			}
		})
	}
}

func containsString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
