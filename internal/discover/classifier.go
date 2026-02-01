package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// Classifier categorizes discovered files.
type Classifier struct {
	// Configuration extension patterns
	configExtensions map[string]bool

	// Known config file basenames
	knownConfigs map[string]int // basename -> score boost

	// Risky path patterns
	riskyPatterns []string

	// Risky basenames
	riskyNames []string
}

// NewClassifier creates a new classifier with default rules.
func NewClassifier() *Classifier {
	return &Classifier{
		configExtensions: map[string]bool{
			".json":       true,
			".toml":       true,
			".yaml":       true,
			".yml":        true,
			".ini":        true,
			".conf":       true,
			".config":     true,
			".cfg":        true,
			".plist":      true,
			".lua":        true,
			".vim":        true,
			".el":         true,
			".kdl":        true,
			".ron":        true,
			".properties": true,
			".xml":        true,
		},
		knownConfigs: map[string]int{
			// Git
			".gitconfig":        100,
			".gitignore_global": 80,
			".gitignore":        60,
			"config":            50, // Can be many things
			// Shell
			".zshrc":        100,
			".zshenv":       90,
			".zprofile":     80,
			".bashrc":       100,
			".bash_profile": 90,
			".profile":      80,
			".inputrc":      70,
			// Editors
			".vimrc":     100,
			".gvimrc":    80,
			"init.vim":   100,
			"init.lua":   100,
			".emacs":     100,
			"init.el":    100,
			".nanorc":    80,
			".editorconfig": 90,
			// Terminal
			".tmux.conf":    100,
			".screenrc":     80,
			"alacritty.yml": 100,
			"alacritty.toml": 100,
			"kitty.conf":    100,
			"wezterm.lua":   100,
			"starship.toml": 100,
			// Package managers
			".npmrc":      80,
			".yarnrc":     80,
			".yarnrc.yml": 80,
			".cargo":      70,
			"Cargo.toml":  60, // Usually project-specific
			// Tools
			".curlrc":  70,
			".wgetrc":  70,
			".ripgreprc": 70,
			".rgignore": 70,
			".fdignore": 70,
			".ignore":   60,
			// App-specific
			"settings.json":    100,
			"keybindings.json": 100,
			"keymap.json":      90,
			"themes.json":      80,
			"tasks.json":       70,
			"launch.json":      70,
			"config.fish":      100,
			"config.kdl":       90,
			"config.toml":      80,
			"config.yaml":      80,
			"config.yml":       80,
			"config.json":      80,
		},
		riskyPatterns: []string{
			"id_rsa",
			"id_ed25519",
			"id_ecdsa",
			"id_dsa",
			".pem",
			".p12",
			".pfx",
			".key",
			".crt",
			"token",
			"secret",
			"password",
			"credential",
			"kubeconfig",
			"auth",
			"api_key",
			"apikey",
			"private",
			"oauth",
		},
		riskyNames: []string{
			".netrc",
			".npmrc", // Can contain auth tokens
			".pypirc",
			".gem/credentials",
			".docker/config.json",
			".kube/config",
			"credentials",
			"credentials.json",
			"service-account.json",
			".env",
			".env.local",
			".env.production",
		},
	}
}

// Classify categorizes a file and returns a Candidate.
func (c *Classifier) Classify(path string, info os.FileInfo, home string) *Candidate {
	candidate := &Candidate{
		Path:    path,
		RelPath: relPath(path, home),
		Size:    info.Size(),
		IsDir:   info.IsDir(),
		ModTime: info.ModTime(),
		Reasons: make([]string, 0),
	}

	// Start with base score
	score := 0
	var reasons []string

	name := baseName(path)
	ext := fileExt(path)

	// Check if it's risky first
	if c.isRisky(path, name) {
		candidate.Category = CategoryRisky
		candidate.Reasons = append(candidate.Reasons, "potentially contains secrets")
		return candidate
	}

	// Check for known config names (highest priority)
	if boost, ok := c.knownConfigs[name]; ok {
		score += boost
		reasons = append(reasons, "known config file")
	} else if boost, ok := c.knownConfigs[filepath.Base(path)]; ok {
		score += boost
		reasons = append(reasons, "known config file")
	}

	// Config extension boost
	if c.configExtensions[ext] {
		score += 30
		reasons = append(reasons, "config extension")
	}

	// Location-based scoring
	if containsAny(path, ".config", "config") {
		score += 20
		reasons = append(reasons, "in config directory")
	}

	if containsAny(path, "Application Support", "AppData") {
		score += 10
		reasons = append(reasons, "in app data directory")
	}

	// Size penalty for large files
	if info.Size() > 100*1024 { // > 100KB
		score -= 10
		reasons = append(reasons, "large file")
	}
	if info.Size() > 1024*1024 { // > 1MB
		score -= 20
	}

	// Dotfile in home directory
	if strings.HasPrefix(name, ".") && filepath.Dir(path) == home {
		score += 40
		reasons = append(reasons, "home dotfile")
	}

	// Categorize based on final score
	candidate.Score = score
	candidate.Reasons = reasons

	switch {
	case score >= 70:
		candidate.Category = CategoryRecommended
	case score >= 30:
		candidate.Category = CategoryMaybe
	case score > 0:
		candidate.Category = CategoryMaybe
	default:
		candidate.Category = CategoryIgnored
	}

	return candidate
}

// isRisky checks if a file is likely to contain secrets.
func (c *Classifier) isRisky(path, name string) bool {
	pathLower := strings.ToLower(path)
	nameLower := strings.ToLower(name)

	// Check risky patterns in path
	for _, pattern := range c.riskyPatterns {
		if strings.Contains(pathLower, pattern) {
			return true
		}
	}

	// Check risky names
	for _, risky := range c.riskyNames {
		if nameLower == strings.ToLower(risky) {
			return true
		}
		// Also check if path ends with the risky name
		if strings.HasSuffix(pathLower, strings.ToLower(risky)) {
			return true
		}
	}

	// SSH directory files (except known safe ones)
	if containsAny(path, ".ssh") {
		// config and known_hosts are safe
		if !matchesAny(name, "config", "known_hosts", "known_hosts.old", "authorized_keys") {
			return true
		}
	}

	// GPG directory
	if containsAny(path, ".gnupg", "gnupg") {
		// pubring is safe, but most other files are not
		if !containsAny(name, "pubring", "trustdb") {
			return true
		}
	}

	return false
}

// IsConfigExtension returns true if the path has a config-like extension.
func (c *Classifier) IsConfigExtension(path string) bool {
	return c.configExtensions[fileExt(path)]
}

// IsSafeSSHFile returns true if the SSH file is safe to track.
func (c *Classifier) IsSafeSSHFile(name string) bool {
	safeFiles := []string{
		"config",
		"known_hosts",
		"known_hosts.old",
		"authorized_keys",
		"authorized_keys2",
	}
	return matchesAny(name, safeFiles...)
}

// ScoreBoost returns the score boost for a known config file.
func (c *Classifier) ScoreBoost(name string) int {
	if boost, ok := c.knownConfigs[strings.ToLower(name)]; ok {
		return boost
	}
	return 0
}
