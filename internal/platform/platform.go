// Package platform provides OS-specific abstractions for dotstate.
//
// This package detects the current platform and provides consistent
// path handling across Windows, macOS, and Linux.
package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// OS represents a target operating system.
type OS string

// Supported operating systems.
const (
	Darwin  OS = "darwin"
	Linux   OS = "linux"
	Windows OS = "windows"
)

// Platform holds information about the current platform.
type Platform struct {
	// OS is the operating system.
	OS OS

	// Arch is the architecture (amd64, arm64, etc.).
	Arch string

	// Home is the user's home directory.
	Home string

	// ConfigDir is the user configuration directory.
	// - macOS: ~/Library/Application Support
	// - Linux: ~/.config (XDG_CONFIG_HOME)
	// - Windows: %APPDATA%
	ConfigDir string

	// DataDir is the user data directory.
	// - macOS: ~/Library/Application Support
	// - Linux: ~/.local/share (XDG_DATA_HOME)
	// - Windows: %LOCALAPPDATA%
	DataDir string

	// CacheDir is the user cache directory.
	// - macOS: ~/Library/Caches
	// - Linux: ~/.cache (XDG_CACHE_HOME)
	// - Windows: %LOCALAPPDATA%\cache
	CacheDir string

	// StateDir is the user state directory (for logs, etc.).
	// - macOS: ~/Library/Application Support
	// - Linux: ~/.local/state (XDG_STATE_HOME)
	// - Windows: %LOCALAPPDATA%
	StateDir string
}

// Current returns the current platform.
func Current() (*Platform, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	p := &Platform{
		OS:   OS(runtime.GOOS),
		Arch: runtime.GOARCH,
		Home: home,
	}

	switch p.OS {
	case Darwin:
		p.ConfigDir = filepath.Join(home, "Library", "Application Support")
		p.DataDir = filepath.Join(home, "Library", "Application Support")
		p.CacheDir = filepath.Join(home, "Library", "Caches")
		p.StateDir = filepath.Join(home, "Library", "Application Support")
	case Linux:
		p.ConfigDir = getEnvOrDefault("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
		p.DataDir = getEnvOrDefault("XDG_DATA_HOME", filepath.Join(home, ".local", "share"))
		p.CacheDir = getEnvOrDefault("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
		p.StateDir = getEnvOrDefault("XDG_STATE_HOME", filepath.Join(home, ".local", "state"))
	case Windows:
		p.ConfigDir = getEnvOrDefault("APPDATA", filepath.Join(home, "AppData", "Roaming"))
		localAppData := getEnvOrDefault("LOCALAPPDATA", filepath.Join(home, "AppData", "Local"))
		p.DataDir = localAppData
		p.CacheDir = filepath.Join(localAppData, "cache")
		p.StateDir = localAppData
	default:
		// Fall back to Linux-like behavior
		p.ConfigDir = filepath.Join(home, ".config")
		p.DataDir = filepath.Join(home, ".local", "share")
		p.CacheDir = filepath.Join(home, ".cache")
		p.StateDir = filepath.Join(home, ".local", "state")
	}

	return p, nil
}

// DotstatePaths returns paths specific to dotstate.
type DotstatePaths struct {
	// ConfigDir is where dotstate configuration lives.
	ConfigDir string

	// DataDir is where dotstate data lives.
	DataDir string

	// CacheDir is where dotstate cache lives.
	CacheDir string

	// LogDir is where dotstate logs are written.
	LogDir string
}

// Paths returns dotstate-specific paths for the platform.
func (p *Platform) Paths() DotstatePaths {
	return DotstatePaths{
		ConfigDir: filepath.Join(p.ConfigDir, "dotstate"),
		DataDir:   filepath.Join(p.DataDir, "dotstate"),
		CacheDir:  filepath.Join(p.CacheDir, "dotstate"),
		LogDir:    filepath.Join(p.StateDir, "dotstate", "logs"),
	}
}

// IsDarwin returns true if running on macOS.
func (p *Platform) IsDarwin() bool {
	return p.OS == Darwin
}

// IsLinux returns true if running on Linux.
func (p *Platform) IsLinux() bool {
	return p.OS == Linux
}

// IsWindows returns true if running on Windows.
func (p *Platform) IsWindows() bool {
	return p.OS == Windows
}

// IsWSL returns true if running in Windows Subsystem for Linux.
func (p *Platform) IsWSL() bool {
	if p.OS != Linux {
		return false
	}
	// Check for WSL markers
	if _, err := os.Stat("/proc/sys/fs/binfmt_misc/WSLInterop"); err == nil {
		return true
	}
	// Check kernel version for WSL indicator
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(data)), "microsoft")
}

// ExpandPath expands ~ and environment variables in a path.
func (p *Platform) ExpandPath(path string) string {
	if path == "" {
		return ""
	}

	// Expand ~
	if strings.HasPrefix(path, "~") {
		if path == "~" {
			return p.Home
		}
		if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, "~\\") {
			path = filepath.Join(p.Home, path[2:])
		}
	}

	// Expand environment variables
	path = os.ExpandEnv(path)

	// Clean the path
	return filepath.Clean(path)
}

// NormalizePath converts a path to the native format for the current OS.
func (p *Platform) NormalizePath(path string) string {
	// Convert forward slashes to backslashes on Windows
	if p.OS == Windows {
		path = strings.ReplaceAll(path, "/", "\\")
	} else {
		path = strings.ReplaceAll(path, "\\", "/")
	}
	return filepath.Clean(path)
}

// ToSlash converts a path to use forward slashes (for portability).
func ToSlash(path string) string {
	return filepath.ToSlash(path)
}

// FromSlash converts a slash-separated path to the native OS format.
func FromSlash(path string) string {
	return filepath.FromSlash(path)
}

// getEnvOrDefault returns the environment variable value or a default.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// Hostname returns the current hostname.
func Hostname() string {
	host, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return host
}

// Username returns the current username.
func Username() string {
	// Try common environment variables
	for _, key := range []string{"USER", "USERNAME", "LOGNAME"} {
		if user := os.Getenv(key); user != "" {
			return user
		}
	}
	return "unknown"
}
