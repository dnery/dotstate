package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

type Config struct {
	Repo  RepoConfig  `toml:"repo"`
	Sync  SyncConfig  `toml:"sync"`
	Tools ToolsConfig `toml:"tools"`
	Chex  ChexConfig  `toml:"chex"`
	WSL   WSLConfig   `toml:"wsl"`
}

type RepoConfig struct {
	URL    string `toml:"url"`
	Path   string `toml:"path"`
	Branch string `toml:"branch"`
}

type SyncConfig struct {
	IntervalMinutes int  `toml:"interval_minutes"`
	EnableIdle      bool `toml:"enable_idle"`
	EnableShutdown  bool `toml:"enable_shutdown"`
}

type ToolsConfig struct {
	Git     string `toml:"git"`
	Chezmoi string `toml:"chezmoi"`
	OP      string `toml:"op"`
}

type ChexConfig struct {
	SourceDir string `toml:"source_dir"`
}

type WSLConfig struct {
	Enable     bool   `toml:"enable"`
	DistroName string `toml:"distro_name"`
	FlakeRef   string `toml:"flake_ref"`
}

func ExpandUser(p string) (string, error) {
	if p == "" {
		return "", nil
	}
	if strings.HasPrefix(p, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		if p == "~" {
			return home, nil
		}
		if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, "~\\") {
			return filepath.Join(home, p[2:]), nil
		}
	}
	return p, nil
}

func Load(path string) (*Config, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg Config
	if err := toml.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	cfg.Repo.Path, err = ExpandUser(cfg.Repo.Path)
	if err != nil {
		return nil, fmt.Errorf("expand repo.path: %w", err)
	}
	return &cfg, nil
}

func FindRepoConfig(startDir string) (string, error) {
	// Walk upward looking for dot.toml
	dir := startDir
	for {
		cand := filepath.Join(dir, "dot.toml")
		if _, err := os.Stat(cand); err == nil {
			return cand, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("could not find dot.toml starting from %s", startDir)
		}
		dir = parent
	}
}
