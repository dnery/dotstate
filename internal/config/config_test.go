package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("could not get home dir: %v", err)
	}

	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:  "empty string",
			input: "",
			want:  "",
		},
		{
			name:  "tilde only",
			input: "~",
			want:  home,
		},
		{
			name:  "tilde with slash",
			input: "~/foo/bar",
			want:  filepath.Join(home, "foo", "bar"),
		},
		{
			name:  "no tilde",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "relative path",
			input: "relative/path",
			want:  "relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExpandPath(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ExpandPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ExpandPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandPathWithEnvVar(t *testing.T) {
	// Set a test env var
	os.Setenv("TEST_DOT_VAR", "test_value")
	defer os.Unsetenv("TEST_DOT_VAR")

	got, err := ExpandPath("$TEST_DOT_VAR/subpath")
	if err != nil {
		t.Fatalf("ExpandPath() error = %v", err)
	}
	want := "test_value/subpath"
	if got != want {
		t.Errorf("ExpandPath() = %v, want %v", got, want)
	}
}

func TestFindRepoConfig(t *testing.T) {
	// Create a temp directory structure
	tmpDir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create nested directories
	subDir := filepath.Join(tmpDir, "a", "b", "c")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("failed to create nested dirs: %v", err)
	}

	// Create dot.toml in tmpDir
	configPath := filepath.Join(tmpDir, ConfigFileName)
	if err := os.WriteFile(configPath, []byte("[repo]\npath = \"test\""), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	tests := []struct {
		name      string
		startDir  string
		want      string
		wantErr   bool
		errType   error
	}{
		{
			name:     "find from same dir",
			startDir: tmpDir,
			want:     configPath,
		},
		{
			name:     "find from nested dir",
			startDir: subDir,
			want:     configPath,
		},
		{
			name:     "find from middle dir",
			startDir: filepath.Join(tmpDir, "a"),
			want:     configPath,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindRepoConfig(tt.startDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindRepoConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FindRepoConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindRepoConfigNotFound(t *testing.T) {
	// Create a temp directory without dot.toml
	tmpDir, err := os.MkdirTemp("", "dotstate-test-notoml-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	_, err = FindRepoConfig(tmpDir)
	if err == nil {
		t.Error("FindRepoConfig() expected error, got nil")
	}

	// Check it's the right error type
	var notFound *NotFoundError
	if !As(err, &notFound) {
		t.Errorf("FindRepoConfig() error type = %T, want *NotFoundError", err)
	}
}

func TestLoad(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `[repo]
url = "https://github.com/test/dotstate"
path = "` + tmpDir + `/repo"
branch = "main"

[sync]
interval_minutes = 15
enable_idle = true
enable_shutdown = false

[tools]
git = ""
chezmoi = ""
op = ""

[chex]
source_dir = "home"

[wsl]
enable = false
`

	configPath := filepath.Join(tmpDir, "dot.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check loaded values
	if cfg.Repo.URL != "https://github.com/test/dotstate" {
		t.Errorf("Repo.URL = %v, want %v", cfg.Repo.URL, "https://github.com/test/dotstate")
	}
	if cfg.Sync.IntervalMinutes != 15 {
		t.Errorf("Sync.IntervalMinutes = %v, want %v", cfg.Sync.IntervalMinutes, 15)
	}
	if !cfg.Sync.EnableIdle {
		t.Error("Sync.EnableIdle = false, want true")
	}
	if cfg.Sync.EnableShutdown {
		t.Error("Sync.EnableShutdown = true, want false")
	}

	// Check computed paths
	if cfg.RepoRoot() != tmpDir {
		t.Errorf("RepoRoot() = %v, want %v", cfg.RepoRoot(), tmpDir)
	}
}

func TestLoadWithDefaults(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Minimal config
	configContent := `[repo]
path = "` + tmpDir + `/repo"
`

	configPath := filepath.Join(tmpDir, "dot.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check defaults were applied
	if cfg.Repo.Branch != DefaultBranch {
		t.Errorf("Repo.Branch = %v, want default %v", cfg.Repo.Branch, DefaultBranch)
	}
	if cfg.Sync.IntervalMinutes != DefaultSyncInterval {
		t.Errorf("Sync.IntervalMinutes = %v, want default %v", cfg.Sync.IntervalMinutes, DefaultSyncInterval)
	}
	if cfg.Chex.SourceDir != DefaultSourceDir {
		t.Errorf("Chex.SourceDir = %v, want default %v", cfg.Chex.SourceDir, DefaultSourceDir)
	}
}

func TestLoadWithEnvOverride(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	configContent := `[repo]
url = "https://github.com/original/url"
path = "` + tmpDir + `/repo"
branch = "original"
`

	configPath := filepath.Join(tmpDir, "dot.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Set env override
	os.Setenv(EnvRepoBranch, "env-override-branch")
	defer os.Unsetenv(EnvRepoBranch)

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Check env override was applied
	if cfg.Repo.Branch != "env-override-branch" {
		t.Errorf("Repo.Branch = %v, want env override %v", cfg.Repo.Branch, "env-override-branch")
	}
}

func TestValidationError(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dotstate-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Config missing required repo.path
	configContent := `[repo]
url = "test"
`

	configPath := filepath.Join(tmpDir, "dot.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o644); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	_, err = Load(configPath)
	if err == nil {
		t.Fatal("Load() expected validation error, got nil")
	}

	// Error message should mention the validation issue
	if !contains(err.Error(), "repo.path") {
		t.Errorf("error should mention repo.path, got: %v", err)
	}
}

func TestDefault(t *testing.T) {
	cfg := Default()

	if cfg.Repo.Branch != DefaultBranch {
		t.Errorf("Branch = %v, want %v", cfg.Repo.Branch, DefaultBranch)
	}
	if cfg.Sync.IntervalMinutes != DefaultSyncInterval {
		t.Errorf("IntervalMinutes = %v, want %v", cfg.Sync.IntervalMinutes, DefaultSyncInterval)
	}
	if cfg.Chex.SourceDir != DefaultSourceDir {
		t.Errorf("SourceDir = %v, want %v", cfg.Chex.SourceDir, DefaultSourceDir)
	}
}

// As is a simple type assertion for tests
func As(err error, target interface{}) bool {
	switch t := target.(type) {
	case **NotFoundError:
		if e, ok := err.(*NotFoundError); ok {
			*t = e
			return true
		}
	case **ValidationError:
		if e, ok := err.(*ValidationError); ok {
			*t = e
			return true
		}
	}
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsAt(s, substr, 0))
}

func containsAt(s, substr string, start int) bool {
	for i := start; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
