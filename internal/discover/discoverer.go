package discover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/platform"
	"github.com/dnery/dotstate/dot/internal/redact"
	"github.com/dnery/dotstate/dot/internal/runner"
	toml "github.com/pelletier/go-toml/v2"
)

// Discoverer orchestrates the discovery process.
type Discoverer struct {
	cfg      *config.Config
	plat     *platform.Platform
	runner   runner.Runner
	chezmoi  *chez.Chezmoi
	git      *gitx.Git
	scanner  *Scanner
	secrets  *SecretDetector
	prompter *Prompter
}

// Options configures the discovery process.
type Options struct {
	// AutoYes auto-accepts recommended files without prompting.
	AutoYes bool

	// DryRun shows what would be added without making changes.
	DryRun bool

	// NoCommit skips the commit step.
	NoCommit bool

	// Deep enables deep scanning of additional directories.
	Deep bool

	// ReportOnly prints a report without any prompts.
	ReportOnly bool

	// SecretsMode controls how secrets are handled: "error", "warning", "ignore".
	SecretsMode string

	// MaxFileSize is the maximum file size to consider.
	MaxFileSize int64

	// Roots overrides the default scan roots.
	Roots []string

	// Platform overrides the detected platform for discovery.
	Platform *platform.Platform
}

const (
	SecretsModeError   = "error"
	SecretsModeWarning = "warning"
	SecretsModeIgnore  = "ignore"
)

// DefaultOptions returns default discovery options.
func DefaultOptions() Options {
	return Options{
		SecretsMode: SecretsModeError,
		MaxFileSize: DefaultMaxFileSize,
	}
}

func normalizeOptions(opts Options) (Options, error) {
	defaults := DefaultOptions()

	if strings.TrimSpace(opts.SecretsMode) == "" {
		opts.SecretsMode = defaults.SecretsMode
	}
	opts.SecretsMode = strings.ToLower(strings.TrimSpace(opts.SecretsMode))

	switch opts.SecretsMode {
	case SecretsModeError, SecretsModeWarning, SecretsModeIgnore:
		// valid
	default:
		return Options{}, fmt.Errorf("invalid secrets mode %q (expected: error, warning, ignore)", opts.SecretsMode)
	}

	if opts.MaxFileSize <= 0 {
		opts.MaxFileSize = defaults.MaxFileSize
	}

	return opts, nil
}

func normalizeManagedPaths(paths []string, home string) map[string]bool {
	managed := make(map[string]bool, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(path)
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
			path = filepath.Join(home, path[2:])
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(home, path)
		}
		managed[filepath.Clean(path)] = true
	}
	return managed
}

func readDiscoverRegistries(cfg *config.Config, home string) ([]string, []string) {
	if cfg == nil {
		return nil, nil
	}
	registryDir := filepath.Join(cfg.StatePath(), "discover")
	curated := expandDiscoverRoots(readRegistryLines(filepath.Join(registryDir, "curated-roots.txt")), home)
	ignored := readRegistryLines(filepath.Join(registryDir, "ignore.txt"))
	return curated, ignored
}

func readRegistryLines(path string) []string {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var lines []string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return lines
}

func expandDiscoverRoots(paths []string, home string) []string {
	roots := make([]string, 0, len(paths))
	for _, path := range paths {
		path = strings.TrimSpace(os.ExpandEnv(path))
		if path == "" {
			continue
		}
		if strings.HasPrefix(path, "~/") || strings.HasPrefix(path, `~\`) {
			path = filepath.Join(home, path[2:])
		} else if !filepath.IsAbs(path) {
			path = filepath.Join(home, path)
		}
		roots = append(roots, filepath.Clean(path))
	}
	return roots
}

func (d *Discoverer) addTypedModuleGuidance(result *Result) {
	if d == nil || d.plat == nil || d.plat.OS != platform.Darwin || result == nil {
		return
	}
	diag := modules.NewDiagnostic(
		modules.SeverityInfo,
		"discover.macos.typed_modules_preferred",
		"macOS apps, Homebrew, mas, launchd, defaults, profiles, privacy/TCC, subrepos, and secrets are better handled through typed module facts than broad ~/Library crawling.",
		"discover",
		"discover:macos:typed_modules",
	)
	diag.Remediation = "Run dot macos audit --json for normalized non-file facts. Use dot discover --deep or --roots only when you intentionally want filesystem candidates."
	diag.Capability = []modules.Capability{modules.CapabilityReadOnly}
	result.Diagnostics = append(result.Diagnostics, diag)
}

// NewDiscoverer creates a new discoverer.
func NewDiscoverer(cfg *config.Config, opts Options) (*Discoverer, error) {
	opts, err := normalizeOptions(opts)
	if err != nil {
		return nil, err
	}

	plat := opts.Platform
	if plat == nil {
		var err error
		plat, err = platform.Current()
		if err != nil {
			return nil, fmt.Errorf("detect platform: %w", err)
		}
	}

	r := runner.New()

	curatedRoots, ignorePatterns := readDiscoverRegistries(cfg, plat.Home)
	scanOpts := ScanOptions{
		Deep:           opts.Deep,
		MaxFileSize:    opts.MaxFileSize,
		IncludeHidden:  true,
		Home:           plat.Home,
		ManagedPaths:   make(map[string]bool),
		Roots:          expandDiscoverRoots(opts.Roots, plat.Home),
		CuratedRoots:   curatedRoots,
		IgnorePatterns: ignorePatterns,
		Platform:       plat,
	}

	// Get managed paths from chezmoi to exclude
	ch := chez.New(cfg.Tools.Chezmoi, r)
	managed, err := ch.Managed(context.Background(), cfg.RepoRoot(), cfg.Chex.SourceDir)
	if err == nil {
		scanOpts.ManagedPaths = normalizeManagedPaths(managed, plat.Home)
	}

	return &Discoverer{
		cfg:      cfg,
		plat:     plat,
		runner:   r,
		chezmoi:  ch,
		git:      gitx.New(cfg.Tools.Git, r),
		scanner:  NewScanner(scanOpts),
		secrets:  NewSecretDetector(r),
		prompter: NewPrompter(opts.AutoYes),
	}, nil
}

// Run executes the discovery process.
func (d *Discoverer) Run(ctx context.Context, opts Options) error {
	opts, err := normalizeOptions(opts)
	if err != nil {
		return err
	}

	// Scan for candidates
	result, err := d.scanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	d.addTypedModuleGuidance(result)

	// Run secret detection on candidates
	if opts.SecretsMode != SecretsModeIgnore {
		if err := d.secrets.UpdateCandidates(ctx, result.Candidates); err != nil {
			return fmt.Errorf("secret scan failed: %w", err)
		}
		if opts.ReportOnly {
			if diag := d.secrets.GitleaksUnavailableDiagnostic(ctx); diag != nil {
				result.Diagnostics = append(result.Diagnostics, *diag)
			}
		}
	}

	// Report-only mode
	if opts.ReportOnly {
		d.prompter.PrintReport(result)
		return nil
	}

	// Select candidates
	selected, err := d.prompter.SelectCandidates(ctx, result)
	if err != nil {
		return fmt.Errorf("selection failed: %w", err)
	}

	if len(selected) == 0 {
		fmt.Println("No files selected.")
		return nil
	}

	// Confirm addition
	if !d.prompter.ConfirmAdd(selected) {
		fmt.Println("Cancelled.")
		return nil
	}

	// Dry run mode
	if opts.DryRun {
		fmt.Printf("Would add %d files (dry run).\n", len(selected))
		for _, c := range selected {
			fmt.Printf("  %s\n", redact.Text(c.RelPath))
		}
		return nil
	}

	repoAlreadyDirty := false
	if !opts.NoCommit {
		dirty, err := d.git.HasChanges(ctx, d.cfg.RepoRoot())
		if err != nil {
			return fmt.Errorf("check repo status before discover commit: %w", err)
		}
		repoAlreadyDirty = dirty
	}

	// Add files
	if err := d.addCandidates(ctx, selected, opts); err != nil {
		return err
	}

	// Commit if enabled
	if !opts.NoCommit {
		if repoAlreadyDirty {
			fmt.Println("Skipping automatic commit because the repo had pre-existing changes.")
		} else if d.prompter.ConfirmCommit() {
			if err := d.commit(ctx); err != nil {
				return fmt.Errorf("commit failed: %w", err)
			}
		}
	}

	// Print next steps
	fmt.Println("\nNext steps:")
	fmt.Println("  dot apply    - Apply the repository state to this machine")
	fmt.Println("  dot sync now - Sync changes to other machines")

	return nil
}

// addCandidates adds the selected candidates to the repository.
func (d *Discoverer) addCandidates(ctx context.Context, candidates []*Candidate, opts Options) error {
	// Separate files from sub-repos
	var files []string
	var subRepos []*Candidate

	for _, c := range candidates {
		if c.IsSubRepo {
			subRepos = append(subRepos, c)
		} else {
			files = append(files, c.Path)
		}
	}

	// Add files with chezmoi
	if len(files) > 0 {
		if err := d.chezmoi.Add(ctx, d.cfg.RepoRoot(), d.cfg.Chex.SourceDir, files, opts.SecretsMode); err != nil {
			return fmt.Errorf("chezmoi add failed: %w", err)
		}
		fmt.Printf("Added %d files.\n", len(files))
	}

	// Handle sub-repos
	if len(subRepos) > 0 {
		if err := d.handleSubRepos(ctx, subRepos); err != nil {
			return err
		}
	}

	return nil
}

// handleSubRepos writes sub-repository references to a manifest file.
func (d *Discoverer) handleSubRepos(ctx context.Context, subRepos []*Candidate) error {
	if len(subRepos) == 0 {
		return nil
	}

	if err := ctx.Err(); err != nil {
		return err
	}

	fmt.Printf("\nFound %d sub-repositories:\n", len(subRepos))
	var discovered []SubRepoManifest
	for _, r := range subRepos {
		url := r.SubRepoURL
		redacted := false
		if url != "" {
			url, redacted = sanitizeGitRemoteURL(url)
			r.SubRepoURL = url
		}

		if url == "" {
			fmt.Printf("  SKIP: %s (local only - will be skipped)\n", redact.Text(r.RelPath))
			continue
		}

		if redacted {
			fmt.Printf("  OK: %s -> %s (credentials redacted)\n", redact.Text(r.RelPath), redact.Text(url))
		} else {
			fmt.Printf("  OK: %s -> %s\n", redact.Text(r.RelPath), redact.Text(url))
		}
		discovered = append(discovered, SubRepoManifest{
			Path:   r.RelPath,
			URL:    url,
			Branch: r.SubRepoBranch,
		})
	}

	if len(discovered) == 0 {
		fmt.Println("No sub-repositories with remotes to track.")
		return nil
	}

	manifestPath := filepath.Join(d.cfg.StatePath(), "subrepos.toml")
	manifest, err := mergeSubRepoManifest(manifestPath, discovered)
	if err != nil {
		return err
	}

	fmt.Printf("\nSub-repository manifest (%d repos):\n", len(manifest.SubRepos))
	for _, m := range manifest.SubRepos {
		fmt.Printf("  [[subrepo]]\n")
		fmt.Printf("  path = %q\n", redact.Text(m.Path))
		fmt.Printf("  url = %q\n", redact.Text(m.URL))
		if m.Branch != "" {
			fmt.Printf("  branch = %q\n", redact.Text(m.Branch))
		}
		fmt.Println()
	}

	data, err := toml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal sub-repo manifest: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write sub-repo manifest: %w", err)
	}

	fmt.Printf("Note: Sub-repo manifest saved to %s\n", redact.Text(manifestPath))
	fmt.Println("      During 'dot apply', these repos will be cloned/updated.")

	return nil
}

func mergeSubRepoManifest(path string, discovered []SubRepoManifest) (SubReposManifest, error) {
	byPath := map[string]SubRepoManifest{}

	existing, err := readSubRepoManifest(path)
	if err != nil {
		return SubReposManifest{}, err
	}
	for _, entry := range existing.SubRepos {
		entry.URL, _ = sanitizeGitRemoteURL(entry.URL)
		byPath[entry.Path] = entry
	}

	for _, entry := range discovered {
		entry.URL, _ = sanitizeGitRemoteURL(entry.URL)
		if previous, ok := byPath[entry.Path]; ok && entry.Description == "" {
			entry.Description = previous.Description
		}
		byPath[entry.Path] = entry
	}

	merged := make([]SubRepoManifest, 0, len(byPath))
	for _, entry := range byPath {
		merged = append(merged, entry)
	}
	sort.Slice(merged, func(i, j int) bool {
		return merged[i].Path < merged[j].Path
	})

	return SubReposManifest{SubRepos: merged}, nil
}

func readSubRepoManifest(path string) (SubReposManifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return SubReposManifest{}, nil
		}
		return SubReposManifest{}, fmt.Errorf("read existing sub-repo manifest: %w", err)
	}
	if strings.TrimSpace(string(b)) == "" {
		return SubReposManifest{}, nil
	}

	var manifest SubReposManifest
	if err := toml.Unmarshal(b, &manifest); err != nil {
		return SubReposManifest{}, fmt.Errorf("parse existing sub-repo manifest: %w", err)
	}
	return manifest, nil
}

// commit commits the added files.
func (d *Discoverer) commit(ctx context.Context) error {
	hostname := platform.Hostname()
	message := gitx.DefaultCommitMessage(hostname)
	message = "discover: " + message

	committed, err := d.git.Commit(ctx, d.cfg.RepoRoot(), message)
	if err != nil {
		return err
	}

	if committed {
		fmt.Println("Changes committed.")
	} else {
		fmt.Println("No changes to commit.")
	}

	return nil
}
