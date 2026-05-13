package discover

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
		Ignored:    make(map[string]int),
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

	roots := make([]string, 0)
	addIfExists := func(path string) {
		if strings.TrimSpace(path) == "" {
			return
		}
		if _, err := os.Stat(path); err == nil {
			roots = append(roots, path)
		}
	}

	// Add curated home dotfiles. Broad home/.config scans are deep-only so the
	// default macOS report stays actionable instead of listing vendor caches.
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
		addIfExists(filepath.Join(home, df))
	}

	curatedXDG := []string{
		"git/config",
		"fish/config.fish",
		"fish/conf.d",
		"fish/functions",
		"nvim/init.lua",
		"nvim/init.vim",
		"nvim/lua",
		"nvim/lazy-lock.json",
		"starship.toml",
		"wezterm/wezterm.lua",
		"ghostty/config",
		"kitty/kitty.conf",
		"alacritty/alacritty.toml",
		"aerospace/aerospace.toml",
		"karabiner/karabiner.json",
		"skhd/skhdrc",
		"yabai/yabairc",
	}
	for _, rel := range curatedXDG {
		addIfExists(filepath.Join(home, ".config", rel))
	}

	for _, root := range s.opts.CuratedRoots {
		addIfExists(root)
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
			// Always check for common macOS app configs that are known to be
			// user-authored and relatively small. App inventory/defaults/launchd
			// are now handled by `dot macos audit --json`.
			macosApps := []string{
				filepath.Join(home, "Library", "Application Support", "Code", "User", "settings.json"),
				filepath.Join(home, "Library", "Application Support", "Code", "User", "keybindings.json"),
				filepath.Join(home, "Library", "Application Support", "Code", "User", "snippets"),
				filepath.Join(home, "Library", "Application Support", "Cursor", "User", "settings.json"),
				filepath.Join(home, "Library", "Application Support", "Cursor", "User", "keybindings.json"),
				filepath.Join(home, "Library", "Application Support", "Cursor", "User", "snippets"),
				filepath.Join(home, "Library", "Application Support", "Zed", "settings.json"),
				filepath.Join(home, "Library", "Application Support", "Zed", "keymap.json"),
				filepath.Join(home, "Library", "Application Support", "Zed", "tasks.json"),
			}
			for _, app := range macosApps {
				addIfExists(app)
			}

			if s.opts.Deep {
				roots = append(roots,
					filepath.Join(home, ".config"),
					filepath.Join(home, "Library", "Application Support"),
					filepath.Join(home, "Library", "Preferences"),
				)
			}

		case platform.Windows:
			appData := os.Getenv("APPDATA")
			localAppData := os.Getenv("LOCALAPPDATA")

			curated := []string{
				filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminal_8wekyb3d8bbwe", "LocalState", "settings.json"),
				filepath.Join(localAppData, "Packages", "Microsoft.WindowsTerminalPreview_8wekyb3d8bbwe", "LocalState", "settings.json"),
				filepath.Join(home, "Documents", "PowerShell", "Microsoft.PowerShell_profile.ps1"),
				filepath.Join(home, "Documents", "PowerShell", "Profile.ps1"),
				filepath.Join(appData, "Code", "User", "settings.json"),
				filepath.Join(appData, "Code", "User", "keybindings.json"),
			}
			for _, path := range curated {
				addIfExists(path)
			}

			if s.opts.Deep {
				roots = append(roots, appData, localAppData)
			}

		case platform.Linux:
			fishConfig := filepath.Join(home, ".config", "fish")
			addIfExists(fishConfig)
			if s.opts.Deep {
				roots = append(roots, filepath.Join(home, ".config"))
			}
		}
	}

	return uniqueStrings(roots)
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

			if s.matchesIgnore(path) {
				result.recordIgnored("user ignore registry")
				return filepath.SkipDir
			}

			// Check if this directory should be excluded
			if s.shouldExcludeDir(path, d.Name()) {
				result.recordIgnored("cache/vendor/browser/generated directory")
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

	// Only regular files are safe to classify and scan. Special files such as
	// FIFOs can block indefinitely if later opened for secret scanning.
	if !info.Mode().IsRegular() {
		return nil
	}

	// Skip if already managed
	if s.opts.ManagedPaths[path] {
		result.recordIgnored("already managed")
		return nil
	}

	if s.matchesIgnore(path) {
		result.recordIgnored("user ignore registry")
		return nil
	}

	if s.shouldExcludeFile(path, filepath.Base(path)) {
		result.recordIgnored("generated/cache/browser file")
		return nil
	}

	// Skip very large files when a max is configured.
	if s.opts.MaxFileSize > 0 && info.Size() > s.opts.MaxFileSize {
		// Unless it's a config-ish extension
		if !s.classifier.IsConfigExtension(path) {
			result.recordIgnored("over max file size")
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
	if strings.HasSuffix(name, ".app") {
		return true
	}

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
		"DerivedData",
		"Archives",
		"Products",
		"SourcePackages",
		".next",
		".nuxt",
		".turbo",
		".parcel-cache",
		".pytest_cache",
		".mypy_cache",
		".ruff_cache",
		// Caches
		"Cache",
		"Caches",
		"GPUCache",
		"Code Cache",
		"ShaderCache",
		"CachedData",
		"CachedExtensions",
		"CachedExtensionVSIXs",
		"CacheStorage",
		"workspaceStorage",
		"LanguageServer",
		"Language Servers",
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
		"Network",
		"DawnCache",
		"GrShaderCache",
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

func (s *Scanner) shouldExcludeFile(path, name string) bool {
	lowerName := strings.ToLower(name)
	if strings.HasSuffix(lowerName, ".map") || strings.HasSuffix(lowerName, ".app") {
		return true
	}
	if matchesAny(lowerName, "history", "cookies", "login data", "web data") && containsAny(path, "Chrome", "Chromium", "BraveSoftware", "Edge", "Safari", "Firefox") {
		return true
	}
	switch filepath.Ext(lowerName) {
	case ".sqlite", ".sqlite3", ".db", ".ldb", ".log":
		if containsAny(path, "Application Support", "User Data", "Profiles", "IndexedDB", "Local Storage", "Session Storage") {
			return true
		}
	}
	return false
}

func (s *Scanner) matchesIgnore(path string) bool {
	if len(s.opts.IgnorePatterns) == 0 {
		return false
	}
	rel := relPath(path, s.opts.Home)
	base := filepath.Base(path)
	for _, pattern := range s.opts.IgnorePatterns {
		if pathMatchesPattern(pattern, path, rel, base) {
			return true
		}
	}
	return false
}

func pathMatchesPattern(pattern, path, rel, base string) bool {
	pattern = strings.TrimSpace(pattern)
	if pattern == "" {
		return false
	}
	pattern = filepath.Clean(os.ExpandEnv(pattern))
	candidates := []string{path, rel, strings.TrimPrefix(rel, "~/"), base}
	for _, candidate := range candidates {
		if matched, err := filepath.Match(pattern, candidate); err == nil && matched {
			return true
		}
		if strings.Contains(candidate, pattern) {
			return true
		}
	}
	return false
}

func (r *Result) recordIgnored(reason string) {
	if r.Ignored == nil {
		r.Ignored = make(map[string]int)
	}
	r.Ignored[reason]++
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]bool, len(values))
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
