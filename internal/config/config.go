// Package config handles loading and validation of dotstate configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// ConfigFileName is the name of the dotstate configuration file.
const ConfigFileName = "dot.toml"

// Config is the root configuration structure.
type Config struct {
	Repo  RepoConfig  `toml:"repo"`
	Sync  SyncConfig  `toml:"sync"`
	Tools ToolsConfig `toml:"tools"`
	Chex  ChexConfig  `toml:"chex"`
	WSL   WSLConfig   `toml:"wsl"`

	// Runtime fields (not persisted)
	configPath string // Path to the config file
	repoRoot   string // Directory containing the config file
}

// RepoConfig configures the repository settings.
type RepoConfig struct {
	URL    string `toml:"url"`
	Path   string `toml:"path"`
	Branch string `toml:"branch"`
}

// SyncConfig configures sync behavior.
type SyncConfig struct {
	IntervalMinutes int  `toml:"interval_minutes"`
	EnableIdle      bool `toml:"enable_idle"`
	EnableShutdown  bool `toml:"enable_shutdown"`
}

// ToolsConfig configures external tool paths.
type ToolsConfig struct {
	Git     string `toml:"git"`
	Chezmoi string `toml:"chezmoi"`
	OP      string `toml:"op"`
}

// ChexConfig configures chezmoi settings.
type ChexConfig struct {
	SourceDir string `toml:"source_dir"`
}

// WSLConfig configures WSL integration.
type WSLConfig struct {
	Enable     bool   `toml:"enable"`
	DistroName string `toml:"distro_name"`
	FlakeRef   string `toml:"flake_ref"`
}

// Default values.
const (
	DefaultBranch          = "main"
	DefaultSyncInterval    = 30
	DefaultSourceDir       = "home"
	DefaultEnableIdle      = true
	DefaultEnableShutdown  = true
)

// Environment variable names.
const (
	EnvRepoURL    = "DOTSTATE_REPO_URL"
	EnvRepoPath   = "DOTSTATE_REPO_PATH"
	EnvRepoBranch = "DOTSTATE_REPO_BRANCH"
	EnvVerbose    = "DOTSTATE_VERBOSE"
	EnvLogLevel   = "DOTSTATE_LOG_LEVEL"
)

// Load loads configuration from a file path.
func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config file: %w", err)
	}

	var cfg Config
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config file: %w", err)
	}

	// Set runtime fields
	cfg.configPath = path
	cfg.repoRoot = filepath.Dir(path)

	// Apply defaults
	cfg.applyDefaults()

	// Apply environment overrides
	cfg.applyEnvOverrides()

	// Expand paths
	if err := cfg.expandPaths(); err != nil {
		return nil, err
	}

	// Validate
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("validate config: %w", err)
	}

	return &cfg, nil
}

// applyDefaults sets default values for unset fields.
func (c *Config) applyDefaults() {
	if c.Repo.Branch == "" {
		c.Repo.Branch = DefaultBranch
	}
	if c.Sync.IntervalMinutes == 0 {
		c.Sync.IntervalMinutes = DefaultSyncInterval
	}
	if c.Chex.SourceDir == "" {
		c.Chex.SourceDir = DefaultSourceDir
	}
	// Note: EnableIdle and EnableShutdown default to false (zero value)
	// so we can't distinguish "not set" from "set to false"
	// The toml file should explicitly set these
}

// applyEnvOverrides applies environment variable overrides.
func (c *Config) applyEnvOverrides() {
	if url := os.Getenv(EnvRepoURL); url != "" {
		c.Repo.URL = url
	}
	if path := os.Getenv(EnvRepoPath); path != "" {
		c.Repo.Path = path
	}
	if branch := os.Getenv(EnvRepoBranch); branch != "" {
		c.Repo.Branch = branch
	}
}

// expandPaths expands ~ and environment variables in path fields.
func (c *Config) expandPaths() error {
	var err error

	c.Repo.Path, err = ExpandPath(c.Repo.Path)
	if err != nil {
		return fmt.Errorf("expand repo.path: %w", err)
	}

	c.Tools.Git, err = ExpandPath(c.Tools.Git)
	if err != nil {
		return fmt.Errorf("expand tools.git: %w", err)
	}

	c.Tools.Chezmoi, err = ExpandPath(c.Tools.Chezmoi)
	if err != nil {
		return fmt.Errorf("expand tools.chezmoi: %w", err)
	}

	c.Tools.OP, err = ExpandPath(c.Tools.OP)
	if err != nil {
		return fmt.Errorf("expand tools.op: %w", err)
	}

	return nil
}

// Validate checks that the configuration is valid.
func (c *Config) Validate() error {
	var errs []string

	// Repo URL can be empty for local-only mode, but if set, validate it
	// (basic validation - just check it's not whitespace)
	if c.Repo.URL != "" && strings.TrimSpace(c.Repo.URL) == "" {
		errs = append(errs, "repo.url cannot be empty whitespace")
	}

	// Repo path must be set
	if c.Repo.Path == "" {
		errs = append(errs, "repo.path is required")
	}

	// Sync interval must be positive
	if c.Sync.IntervalMinutes < 0 {
		errs = append(errs, "sync.interval_minutes must be non-negative")
	}

	// Source dir must be set
	if c.Chex.SourceDir == "" {
		errs = append(errs, "chex.source_dir is required")
	}

	// WSL validation
	if c.WSL.Enable {
		if c.WSL.DistroName == "" {
			errs = append(errs, "wsl.distro_name is required when wsl.enable is true")
		}
	}

	if len(errs) > 0 {
		return &ValidationError{Errors: errs}
	}
	return nil
}

// ValidationError represents configuration validation errors.
type ValidationError struct {
	Errors []string
}

func (e *ValidationError) Error() string {
	if len(e.Errors) == 1 {
		return fmt.Sprintf("config validation error: %s", e.Errors[0])
	}
	return fmt.Sprintf("config validation errors:\n  - %s", strings.Join(e.Errors, "\n  - "))
}

// ConfigPath returns the path to the config file.
func (c *Config) ConfigPath() string {
	return c.configPath
}

// RepoRoot returns the directory containing the config file.
func (c *Config) RepoRoot() string {
	return c.repoRoot
}

// SourcePath returns the full path to the chezmoi source directory.
func (c *Config) SourcePath() string {
	return filepath.Join(c.repoRoot, c.Chex.SourceDir)
}

// StatePath returns the full path to the state directory.
func (c *Config) StatePath() string {
	return filepath.Join(c.repoRoot, "state")
}

// PrivatePath returns the full path to the private state directory.
func (c *Config) PrivatePath() string {
	return filepath.Join(c.repoRoot, "state", "private")
}

// LogPath returns the full path to the log directory.
func (c *Config) LogPath() string {
	return filepath.Join(c.repoRoot, "state", "logs")
}

// FindRepoConfig searches for dot.toml starting from startDir and walking upward.
func FindRepoConfig(startDir string) (string, error) {
	dir := startDir
	for {
		candidate := filepath.Join(dir, ConfigFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", &NotFoundError{StartDir: startDir}
		}
		dir = parent
	}
}

// NotFoundError indicates that the config file was not found.
type NotFoundError struct {
	StartDir string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("could not find %s starting from %s", ConfigFileName, e.StartDir)
}

// ExpandPath expands ~ and environment variables in a path.
func ExpandPath(p string) (string, error) {
	if p == "" {
		return "", nil
	}

	// Expand ~
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("get home directory: %w", err)
		}
		if p == "~" {
			return home, nil
		}
		if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
			p = filepath.Join(home, p[2:])
		}
	}

	// Expand environment variables
	p = os.ExpandEnv(p)

	return p, nil
}

// ExpandUser is an alias for ExpandPath for backward compatibility.
func ExpandUser(p string) (string, error) {
	return ExpandPath(p)
}

// Default returns a Config with all default values.
func Default() *Config {
	home, _ := os.UserHomeDir()
	return &Config{
		Repo: RepoConfig{
			Path:   filepath.Join(home, ".dotstate"),
			Branch: DefaultBranch,
		},
		Sync: SyncConfig{
			IntervalMinutes: DefaultSyncInterval,
			EnableIdle:      DefaultEnableIdle,
			EnableShutdown:  DefaultEnableShutdown,
		},
		Chex: ChexConfig{
			SourceDir: DefaultSourceDir,
		},
	}
}
