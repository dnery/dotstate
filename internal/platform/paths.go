package platform

import (
	"os"
	"path/filepath"
)

// CommonConfigPaths returns paths to common configuration locations.
// This is used by the discover command to find configuration files.
type CommonConfigPaths struct {
	// Home-relative paths (e.g., ~/.gitconfig)
	Home []string

	// XDG config paths (e.g., ~/.config/git/config)
	XDGConfig []string

	// Application-specific paths
	AppSupport []string // macOS ~/Library/Application Support
	AppData    []string // Windows %APPDATA%
	LocalData  []string // Windows %LOCALAPPDATA%
}

// ConfigLocations returns common configuration file locations for the platform.
func (p *Platform) ConfigLocations() CommonConfigPaths {
	paths := CommonConfigPaths{
		Home: []string{
			".gitconfig",
			".gitignore_global",
			".zshrc",
			".zshenv",
			".zprofile",
			".bashrc",
			".bash_profile",
			".profile",
			".inputrc",
			".vimrc",
			".tmux.conf",
			".screenrc",
			".curlrc",
			".wgetrc",
			".npmrc",
			".yarnrc",
		},
		XDGConfig: []string{
			"git/config",
			"git/ignore",
			"nvim/init.lua",
			"nvim/init.vim",
			"fish/config.fish",
			"starship.toml",
			"alacritty/alacritty.yml",
			"alacritty/alacritty.toml",
			"kitty/kitty.conf",
			"wezterm/wezterm.lua",
			"helix/config.toml",
			"zellij/config.kdl",
		},
	}

	switch p.OS {
	case Darwin:
		paths.AppSupport = []string{
			"Code/User/settings.json",
			"Code/User/keybindings.json",
			"Cursor/User/settings.json",
			"Cursor/User/keybindings.json",
			"Firefox/Profiles",
		}
	case Windows:
		paths.AppData = []string{
			"Code/User/settings.json",
			"Code/User/keybindings.json",
			"Cursor/User/settings.json",
			"Cursor/User/keybindings.json",
		}
		paths.LocalData = []string{
			"Mozilla/Firefox/Profiles",
		}
	case Linux:
		paths.XDGConfig = append(paths.XDGConfig,
			"Code/User/settings.json",
			"Code/User/keybindings.json",
			"Cursor/User/settings.json",
			"Cursor/User/keybindings.json",
		)
	}

	return paths
}

// BrowserProfiles returns paths to browser profile directories.
func (p *Platform) BrowserProfiles() map[string]string {
	profiles := make(map[string]string)

	switch p.OS {
	case Darwin:
		profiles["firefox"] = filepath.Join(p.Home, "Library", "Application Support", "Firefox", "Profiles")
		profiles["chrome"] = filepath.Join(p.Home, "Library", "Application Support", "Google", "Chrome")
		profiles["chromium"] = filepath.Join(p.Home, "Library", "Application Support", "Chromium")
		profiles["safari"] = filepath.Join(p.Home, "Library", "Safari")
		profiles["arc"] = filepath.Join(p.Home, "Library", "Application Support", "Arc")
		profiles["brave"] = filepath.Join(p.Home, "Library", "Application Support", "BraveSoftware", "Brave-Browser")
	case Linux:
		profiles["firefox"] = filepath.Join(p.Home, ".mozilla", "firefox")
		profiles["chrome"] = filepath.Join(p.ConfigDir, "google-chrome")
		profiles["chromium"] = filepath.Join(p.ConfigDir, "chromium")
		profiles["brave"] = filepath.Join(p.ConfigDir, "BraveSoftware", "Brave-Browser")
	case Windows:
		appData := p.ConfigDir                                                                      // %APPDATA%
		localAppData := getEnvOrDefault("LOCALAPPDATA", filepath.Join(p.Home, "AppData", "Local")) // %LOCALAPPDATA%
		profiles["firefox"] = filepath.Join(appData, "Mozilla", "Firefox", "Profiles")
		profiles["chrome"] = filepath.Join(localAppData, "Google", "Chrome", "User Data")
		profiles["chromium"] = filepath.Join(localAppData, "Chromium", "User Data")
		profiles["edge"] = filepath.Join(localAppData, "Microsoft", "Edge", "User Data")
		profiles["brave"] = filepath.Join(localAppData, "BraveSoftware", "Brave-Browser", "User Data")
	}

	return profiles
}

// SSHDir returns the path to the SSH configuration directory.
func (p *Platform) SSHDir() string {
	return filepath.Join(p.Home, ".ssh")
}

// GPGDir returns the path to the GPG configuration directory.
func (p *Platform) GPGDir() string {
	return filepath.Join(p.Home, ".gnupg")
}

// ShellConfigFiles returns shell configuration files for the platform.
func (p *Platform) ShellConfigFiles() []string {
	var files []string

	// Common shell configs
	common := []string{
		".profile",
		".bashrc",
		".bash_profile",
		".zshrc",
		".zshenv",
		".zprofile",
	}

	for _, f := range common {
		path := filepath.Join(p.Home, f)
		if _, err := os.Stat(path); err == nil {
			files = append(files, path)
		}
	}

	// Fish config
	fishConfig := filepath.Join(p.ConfigDir, "fish", "config.fish")
	if _, err := os.Stat(fishConfig); err == nil {
		files = append(files, fishConfig)
	}

	// PowerShell profile (Windows)
	if p.OS == Windows {
		psProfile := filepath.Join(p.Home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1")
		if _, err := os.Stat(psProfile); err == nil {
			files = append(files, psProfile)
		}
		// PowerShell 7 profile
		pwshProfile := filepath.Join(p.Home, "Documents", "PowerShell", "Profile.ps1")
		if _, err := os.Stat(pwshProfile); err == nil {
			files = append(files, pwshProfile)
		}
	}

	return files
}

// ExistsAll checks if all paths exist and returns those that do.
func ExistsAll(paths []string) []string {
	var existing []string
	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			existing = append(existing, path)
		}
	}
	return existing
}

// ResolveConfigPath attempts to find a config file in standard locations.
// Returns the first path that exists, or empty string if none found.
func (p *Platform) ResolveConfigPath(name string) string {
	candidates := []string{
		filepath.Join(p.Home, name),
		filepath.Join(p.ConfigDir, name),
	}

	if p.OS == Darwin {
		candidates = append(candidates,
			filepath.Join(p.Home, "Library", "Application Support", name),
		)
	}

	for _, path := range candidates {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}
	return ""
}
