package discover

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/dnery/dotstate/dot/internal/platform"
)

// Scanner discovers configuration files on the filesystem.
type Scanner struct {
	opts       ScanOptions
	classifier *Classifier
	subrepo    *SubRepoDetector
}

// NewScanner creates a new scanner with the given options.
func NewScanner(opts ScanOptions) *Scanner {
	return &Scanner{
		opts:       opts,
		classifier: NewClassifier(),
		subrepo:    NewSubRepoDetector(),
	}
}

// Scan discovers configuration files starting from the configured roots.
func (s *Scanner) Scan(ctx context.Context) (*Result, error) {
	start := time.Now()
	result := &Result{
		Candidates: make(CandidateList, 0),
		SubRepos:   make([]*Candidate, 0),
	}

	// Get roots to scan
	roots := s.opts.Roots
	if len(roots) == 0 {
		roots = s.defaultRoots()
	}

	// Scan each root
	for _, root := range roots {
		if err := ctx.Err(); err != nil {
			return result, err
		}

		expanded := os.ExpandEnv(root)
		if err := s.scanRoot(ctx, expanded, result); err != nil {
			result.Errors = append(result.Errors, err)
		}
	}

	// Sort candidates
	sort.Sort(result.Candidates)

	result.ScanDuration = time.Since(start)
	return result, nil
}

// defaultRoots returns the default scan roots for the current platform.
func (s *Scanner) defaultRoots() []string {
	home := s.opts.Home
	if home == "" {
		if s.opts.Platform != nil && s.opts.Platform.Home != "" {
			home = s.opts.Platform.Home
		} else {
			home, _ = os.UserHomeDir()
		}
	}

	roots := []string{
		filepath.Join(home, ".config"),
	}

	// Add curated home dotfiles
	dotfiles := []string{
		".gitconfig",
		".gitignore_global",
		".zshrc",
		".zshenv",
		".zprofile",
		".bashrc",
		".bash_profile",
		".profile",
		".vimrc",
		".tmux.conf",
		".npmrc",
		".yarnrc",
	}
	for _, df := range dotfiles {
		path := filepath.Join(home, df)
		if _, err := os.Stat(path); err == nil {
			roots = append(roots, path)
		}
	}

	// Platform-specific roots
	plat := s.opts.Platform
	if plat == nil {
		var err error
		plat, err = platform.Current()
		if err != nil {
			return roots
		}
	}
	if plat != nil {
		switch plat.OS {
		case platform.Darwin:
			if s.opts.Deep {
				roots = append(roots,
					filepath.Join(home, "Library", "Application Support"),
					filepath.Join(home, "Library", "Preferences"),
				)
			}
			// Always check for common macOS app configs
			macosApps := []string{
				filepath.Join(home, "Library", "Application Support", "Code", "User"),
				filepath.Join(home, "Library", "Application Support", "Cursor", "User"),
				filepath.Join(home, "Library", "Application Support", "Zed"),
			}
			for _, app := range macosApps {
				if _, err := os.Stat(app); err == nil {
					roots = append(roots, app)
				}
			}

		case platform.Windows:
			appData := os.Getenv("APPDATA")
			localAppData := os.Getenv("LOCALAPPDATA")

			// Curated Windows paths
			curated := []string{
				filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
				filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
				filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
				filepath.Join(home, "Documents", "PowerShell", "Profile.ps1"),
				filepath.Join(appData, "Code", "User", "settings.json"),
				filepath.Join(appData, "Code", "User", "keybindings.json"),
			}
			for _, path := range curated {
				if _, err := os.Stat(path); err == nil {
					roots = append(roots, path)
				}
			}

			if s.opts.Deep {
				roots = append(roots, appData, localAppData)
			}

		case platform.Linux:
			// Fish config
			fishConfig := filepath.Join(home, ".config", "fish")
			if _, err := os.Stat(fishConfig); err == nil {
				roots = append(roots, fishConfig)
			}
		}
	}

	return roots
}

// scanRoot scans a single root path.
func (s *Scanner) scanRoot(ctx context.Context, root string, result *Result) error {
	info, err := os.Stat(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Skip non-existent roots
		}
		return err
	}

	// If root is a file, process it directly
	if !info.IsDir() {
		return s.processFile(ctx, root, info, result)
	}

	// Walk the directory tree
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil // Continue walking
		}

		// Check context
		if ctx.Err() != nil {
			return ctx.Err()
		}

		// Get file info
		info, err := d.Info()
		if err != nil {
			result.Errors = append(result.Errors, err)
			return nil
		}

		// Check for sub-repository
		if d.IsDir() {
			result.ScannedDirs++

			// Check if this directory should be excluded
			if s.shouldExcludeDir(path, d.Name()) {
				return filepath.SkipDir
			}

			// Check for sub-repo (has .git directory)
			if s.subrepo.IsSubRepo(path) {
				candidate, err := s.subrepo.Analyze(ctx, path, s.opts.Home)
				if err == nil && candidate != nil {
					result.SubRepos = append(result.SubRepos, candidate)
					result.Candidates = append(result.Candidates, candidate)
				}
				return filepath.SkipDir // Don't descend into sub-repos
			}

			return nil
		}

		return s.processFile(ctx, path, info, result)
	})
}

// processFile processes a single file and adds it as a candidate if appropriate.
func (s *Scanner) processFile(ctx context.Context, path string, info os.FileInfo, result *Result) error {
	result.ScannedFiles++

	// Skip if already managed
	if s.opts.ManagedPaths[path] {
		return nil
	}

	// Skip very large files
	if info.Size() > s.opts.MaxFileSize {
		// Unless it's a config-ish extension
		if !s.classifier.IsConfigExtension(path) {
			return nil
		}
	}

	// Classify the file
	candidate := s.classifier.Classify(path, info, s.opts.Home)
	if candidate.Category == CategoryIgnored {
		return nil
	}

	result.Candidates = append(result.Candidates, candidate)
	return nil
}

// shouldExcludeDir returns true if the directory should be skipped entirely.
func (s *Scanner) shouldExcludeDir(path, name string) bool {
	// Always skip these directories
	excludeNames := []string{
		// VCS
		".git",
		".svn",
		".hg",
		// Dependencies
		"node_modules",
		"vendor",
		"__pycache__",
		".venv",
		"venv",
		// Build outputs
		"build",
		"dist",
		"target",
		// Caches
		"Cache",
		"Caches",
		"GPUCache",
		"Code Cache",
		"ShaderCache",
		"CachedData",
		"CachedExtensions",
		"CachedExtensionVSIXs",
		// Logs and crashes
		"Logs",
		"logs",
		"Crashpad",
		"Crashes",
		// Temp
		"Temp",
		"tmp",
		// Games and media
		"Steam",
		"Epic Games",
		"GOG Galaxy",
		// Large app data
		"Electron",
		"blob_storage",
		"IndexedDB",
		"Local Storage",
		"Session Storage",
		"Service Worker",
		"databases",
	}

	for _, exclude := range excludeNames {
		if name == exclude {
			return true
		}
	}

	// Browser profiles (handled separately by browser modules)
	browserDirs := []string{
		"Firefox",
		"Mozilla",
		"Google",
		"Chrome",
		"Chromium",
		"Microsoft",
		"Edge",
		"BraveSoftware",
		"Arc",
		"Safari",
	}

	// Only exclude browser dirs if they're the profile root
	for _, browser := range browserDirs {
		if name == browser && containsAny(path, "Profiles", "User Data", "Application Support") {
			return true
		}
	}

	return false
}
