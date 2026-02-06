package discover

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/platform"
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
}

// DefaultOptions returns default discovery options.
func DefaultOptions() Options {
	return Options{
		SecretsMode: "error",
		MaxFileSize: DefaultMaxFileSize,
	}
}

// NewDiscoverer creates a new discoverer.
func NewDiscoverer(cfg *config.Config, opts Options) (*Discoverer, error) {
	plat, err := platform.Current()
	if err != nil {
		return nil, fmt.Errorf("detect platform: %w", err)
	}

	r := runner.New()

	scanOpts := ScanOptions{
		Deep:          opts.Deep,
		MaxFileSize:   opts.MaxFileSize,
		IncludeHidden: true,
		Home:          plat.Home,
		ManagedPaths:  make(map[string]bool),
		Roots:         opts.Roots,
	}

	// Get managed paths from chezmoi to exclude
	ch := chez.New(cfg.Tools.Chezmoi, r)
	managed, err := ch.Managed(context.Background(), cfg.RepoRoot())
	if err == nil {
		for _, path := range managed {
			scanOpts.ManagedPaths[path] = true
		}
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
	// Scan for candidates
	result, err := d.scanner.Scan(ctx)
	if err != nil {
		return fmt.Errorf("scan failed: %w", err)
	}

	// Run secret detection on candidates
	if opts.SecretsMode != "ignore" {
		if err := d.secrets.UpdateCandidates(ctx, result.Candidates); err != nil {
			return fmt.Errorf("secret scan failed: %w", err)
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
			fmt.Printf("  %s\n", c.RelPath)
		}
		return nil
	}

	// Add files
	if err := d.addCandidates(ctx, selected, opts); err != nil {
		return err
	}

	// Commit if enabled
	if !opts.NoCommit {
		if d.prompter.ConfirmCommit() {
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
		secretsError := opts.SecretsMode == "error"
		if err := d.chezmoi.Add(ctx, d.cfg.RepoRoot(), files, secretsError); err != nil {
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

	fmt.Printf("\nFound %d sub-repositories:\n", len(subRepos))
	for _, r := range subRepos {
		url := r.SubRepoURL
		if url == "" {
			url = "(local only - will be skipped)"
			fmt.Printf("  SKIP: %s %s\n", r.RelPath, url)
		} else {
			fmt.Printf("  OK: %s -> %s\n", r.RelPath, url)
		}
	}

	// Create manifest for sub-repos with remotes
	var manifests []SubRepoManifest
	for _, r := range subRepos {
		if r.SubRepoURL != "" {
			manifests = append(manifests, SubRepoManifest{
				Path:   r.RelPath,
				URL:    r.SubRepoURL,
				Branch: r.SubRepoBranch,
			})
		}
	}

	if len(manifests) == 0 {
		fmt.Println("No sub-repositories with remotes to track.")
		return nil
	}

	fmt.Printf("\nSub-repository manifest (%d repos):\n", len(manifests))
	for _, m := range manifests {
		fmt.Printf("  [[subrepo]]\n")
		fmt.Printf("  path = %q\n", m.Path)
		fmt.Printf("  url = %q\n", m.URL)
		if m.Branch != "" {
			fmt.Printf("  branch = %q\n", m.Branch)
		}
		fmt.Println()
	}

	manifest := SubReposManifest{SubRepos: manifests}
	data, err := toml.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("marshal sub-repo manifest: %w", err)
	}

	manifestPath := filepath.Join(d.cfg.StatePath(), "subrepos.toml")
	if err := os.MkdirAll(filepath.Dir(manifestPath), 0o755); err != nil {
		return fmt.Errorf("create state directory: %w", err)
	}
	if err := os.WriteFile(manifestPath, data, 0o644); err != nil {
		return fmt.Errorf("write sub-repo manifest: %w", err)
	}

	fmt.Printf("Note: Sub-repo manifest saved to %s\n", manifestPath)
	fmt.Println("      During 'dot apply', these repos will be cloned/updated.")

	return nil
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
