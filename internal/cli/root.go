// Package cli provides the command-line interface for dotstate.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/discover"
	doterrors "github.com/dnery/dotstate/dot/internal/errors"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/logging"
	"github.com/dnery/dotstate/dot/internal/macos"
	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/platform"
	"github.com/dnery/dotstate/dot/internal/redact"
	"github.com/dnery/dotstate/dot/internal/runner"
	"github.com/dnery/dotstate/dot/internal/schedule"
	"github.com/dnery/dotstate/dot/internal/sync"
	"github.com/dnery/dotstate/dot/internal/ui"
)

var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type app struct {
	cfgPath string
	repoDir string
	verbose bool
	logger  *logging.Logger
	plat    *platform.Platform
}

// Execute runs the CLI application and returns an exit code.
func Execute() int {
	a := &app{}

	// Initialize platform detection
	plat, err := platform.Current()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: failed to detect platform: %v\n", err)
		return doterrors.ExitError
	}
	a.plat = plat

	root := &cobra.Command{
		Use:   "dot",
		Short: "dotstate orchestrator",
		Long:  "Cross-platform OS state orchestration for config management.",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Initialize logger based on verbose flag
			logCfg := logging.Config{
				Verbose:  a.verbose,
				LogLevel: logging.LevelInfo,
			}

			// If we can load config, use its log path
			if cfg, _, err := a.loadConfigSilent(); err == nil {
				logCfg.LogDir = cfg.LogPath()
			}

			logger, err := logging.New(logCfg)
			if err != nil {
				// Log to stderr only
				logger = logging.NewNoop()
			}
			a.logger = logger

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if a.logger != nil {
				a.logger.Close()
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.PersistentFlags().StringVar(&a.cfgPath, "config", "", "Path to dot.toml (defaults to searching upward from current dir)")
	root.PersistentFlags().StringVar(&a.repoDir, "repo-dir", "", "Repo directory override (defaults to repo.path from config)")
	root.PersistentFlags().BoolVarP(&a.verbose, "verbose", "v", false, "Enable verbose output")

	root.AddCommand(cmdVersion())
	root.AddCommand(cmdDoctor(a))
	root.AddCommand(cmdBootstrap(a))
	root.AddCommand(cmdApply(a))
	root.AddCommand(cmdCapture(a))
	root.AddCommand(cmdSync(a))
	root.AddCommand(cmdMacOS(a))
	root.AddCommand(cmdSchedule(a))
	root.AddCommand(cmdDiscover(a))
	root.AddCommand(cmdSubrepo(a))

	if err := root.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		return doterrors.Exit(err)
	}
	return doterrors.ExitOK
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("dot %s (%s) built %s\n", version, commit, date)
			fmt.Printf("  platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}
}

// loadConfigSilent loads config without logging errors.
func (a *app) loadConfigSilent() (*config.Config, string, error) {
	startDir := ""
	if a.cfgPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		startDir = cwd
	}

	cfgPath, err := config.ResolveConfigPath(a.cfgPath, startDir)
	if err != nil {
		return nil, "", err
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}

	if a.repoDir != "" {
		cfg.Repo.Path = a.repoDir
	}

	repoRoot := filepath.Dir(cfgPath)
	return cfg, repoRoot, nil
}

func (a *app) loadConfig() (*config.Config, string, error) {
	cfg, repoRoot, err := a.loadConfigSilent()
	if err != nil {
		if a.logger != nil {
			a.logger.Error("failed to load config", "error", err)
		}
		return nil, "", doterrors.NewConfigError("failed to load config", err)
	}

	if a.logger != nil {
		a.logger.Debug("config loaded",
			"path", cfg.ConfigPath(),
			"repoRoot", repoRoot,
		)
	}

	return cfg, repoRoot, nil
}

func newSyncer(cfg *config.Config, home string) *sync.Syncer {
	r := runner.New()
	g := gitx.New(cfg.Tools.Git, r)
	ch := chez.New(cfg.Tools.Chezmoi, r)
	files := modules.NewFilesModule(cfg, ch, home)
	mods := []modules.Module{files}
	mods = append(mods, macos.NewStateModules(cfg, r, home)...)
	return sync.NewWithModules(cfg, g, ch, modules.NewOrchestrator(mods...))
}

func cmdDoctor(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites and system status",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Platform info
			fmt.Println(ui.Title("System"))
			fmt.Printf("  Platform: %s/%s\n", a.plat.OS, a.plat.Arch)
			fmt.Printf("  Home: %s\n", a.plat.Home)
			if a.plat.IsWSL() {
				fmt.Println("  WSL: detected")
			}
			fmt.Println()

			// Config
			cfg, repoRoot, err := a.loadConfigSilent()
			if err != nil {
				fmt.Println(ui.Err("Config"))
				fmt.Printf("  Not found: %v\n", err)
				fmt.Println("  Tip: run from the repo root, or pass --config path/to/dot.toml")
				fmt.Println()
			} else {
				fmt.Println(ui.Title("Config"))
				fmt.Printf("  Path: %s\n", cfg.ConfigPath())
				fmt.Printf("  Repo root: %s\n", repoRoot)
				fmt.Printf("  Repo URL: %s\n", cfg.Repo.URL)
				fmt.Printf("  Branch: %s\n", cfg.Repo.Branch)
				fmt.Println()
			}

			// Tools
			fmt.Println(ui.Title("Prerequisites"))

			type tool struct {
				name        string
				bin         string
				required    bool
				installHint string
			}

			tools := []tool{
				{"git", "", true, "https://git-scm.com/downloads"},
				{"chezmoi", "", true, "https://www.chezmoi.io/install/"},
				{"op", "", false, "https://1password.com/downloads/command-line/"},
			}

			if cfg != nil {
				if cfg.Tools.Git != "" {
					tools[0].bin = cfg.Tools.Git
				}
				if cfg.Tools.Chezmoi != "" {
					tools[1].bin = cfg.Tools.Chezmoi
				}
				if cfg.Tools.OP != "" {
					tools[2].bin = cfg.Tools.OP
				}
			}

			allOk := true
			for _, t := range tools {
				bin := t.bin
				if bin == "" {
					bin = t.name
				}
				path, err := exec.LookPath(bin)
				if err != nil {
					if t.required {
						fmt.Printf("  %s: %s (MISSING)\n", ui.Err(t.name), t.installHint)
						allOk = false
					} else {
						fmt.Printf("  %s: not found (optional)\n", ui.Key(t.name))
					}
				} else {
					fmt.Printf("  %s: %s\n", ui.Key(t.name), path)
				}
			}

			fmt.Println()

			if !allOk {
				return doterrors.NewToolNotFoundError("required tool", "see above for install hints")
			}

			fmt.Println(ui.Title("Status: OK"))
			return nil
		},
	}
}

func cmdBootstrap(a *app) *cobra.Command {
	var (
		repoURL          string
		skipOPCheckpoint bool
	)

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Clone repo (if needed) and prepare this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := a.bootstrapConfig(repoURL)
			if err != nil {
				return err
			}

			printBootstrapPrerequisites(cfg, skipOPCheckpoint)

			if a.logger != nil {
				a.logger.Info("bootstrapping",
					"url", cfg.Repo.URL,
					"path", cfg.Repo.Path,
					"branch", cfg.Repo.Branch,
				)
			}

			if cfg.Repo.URL != "" {
				r := runner.New()
				g := gitx.New(cfg.Tools.Git, r)
				if err := g.EnsureCloned(context.Background(), cfg.Repo.URL, cfg.Repo.Path, cfg.Repo.Branch); err != nil {
					return doterrors.Wrap(err, "clone failed")
				}
			} else {
				fmt.Println("Repo URL is empty; skipping clone and treating repo.path as an existing local checkout.")
			}

			printBootstrapComplete(cfg)

			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Git URL of your dotstate repo (required if not running inside the repo)")
	cmd.Flags().BoolVar(&skipOPCheckpoint, "skip-op-checkpoint", false, "Do not print the 1Password/op manual checkpoint")
	return cmd
}

func (a *app) bootstrapConfig(repoURL string) (*config.Config, error) {
	cfg, _, err := a.loadConfigSilent()
	if err == nil {
		if repoURL != "" {
			cfg.Repo.URL = repoURL
		}
		return cfg, nil
	}
	if repoURL == "" {
		return nil, doterrors.NewUserError("bootstrap requires --repo, or run from a repo that contains dot.toml")
	}
	cfg = config.Default()
	cfg.Repo.URL = repoURL
	return cfg, nil
}

func printBootstrapComplete(cfg *config.Config) {
	fmt.Println(ui.Title("Bootstrap complete"))
	fmt.Printf("  Repo: %s\n", redact.Text(cfg.Repo.Path))
	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Println("  1. cd", redact.Text(cfg.Repo.Path))
	fmt.Println("  2. dot doctor")
	fmt.Println("  3. dot apply --dry-run")
	fmt.Println("  4. dot sync --dry-run")
	fmt.Println("  5. dot macos audit --json")
	fmt.Println("  6. dot schedule install")
}

func printBootstrapPrerequisites(cfg *config.Config, skipOPCheckpoint bool) {
	fmt.Println(ui.Title("Bootstrap checks"))
	if runtime.GOOS == "darwin" {
		if err := exec.Command("xcode-select", "-p").Run(); err != nil {
			fmt.Println("  Xcode Command Line Tools: manual checkpoint required")
			fmt.Println("    Run: xcode-select --install")
		} else {
			fmt.Println("  Xcode Command Line Tools: detected")
		}
		if path, err := exec.LookPath("brew"); err != nil {
			fmt.Println("  Homebrew: not found")
			fmt.Println("    Install from https://brew.sh, then run: brew install git chezmoi")
		} else {
			fmt.Printf("  Homebrew: %s\n", redact.Text(path))
		}
	} else {
		fmt.Printf("  Platform: %s/%s (macOS bootstrap checks skipped)\n", runtime.GOOS, runtime.GOARCH)
	}

	if skipOPCheckpoint {
		fmt.Println("  1Password/op checkpoint: skipped by flag")
	} else if path, err := exec.LookPath(firstNonEmpty(cfg.Tools.OP, "op")); err != nil {
		fmt.Println("  1Password/op checkpoint: op not found")
		fmt.Println("    Install 1Password CLI, sign in to the desktop app, and enable CLI integration before applying secrets-backed state.")
	} else {
		fmt.Printf("  1Password/op checkpoint: %s\n", redact.Text(path))
		fmt.Println("    Unlock 1Password and verify with: op account list")
	}
	fmt.Println()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func cmdApply(a *app) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply desired state to this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}

			if a.logger != nil {
				a.logger.Info("applying configuration", "source", cfg.SourcePath())
			}

			s := newSyncer(cfg, a.plat.Home)
			report, err := s.ApplyWithOptions(context.Background(), sync.RunOptions{DryRun: dryRun})
			if err != nil {
				return doterrors.Wrap(err, "apply failed")
			}
			if dryRun {
				printRunReport("Apply plan", report)
				return nil
			}

			fmt.Println(ui.Title("Apply complete"))
			printRunReport("Apply result", report)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the module plan without applying changes")
	return cmd
}

func cmdCapture(a *app) *cobra.Command {
	var dryRun bool

	cmd := &cobra.Command{
		Use:   "capture",
		Short: "Capture local changes back into the repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}

			if a.logger != nil {
				a.logger.Info("capturing changes", "source", cfg.SourcePath())
			}

			s := newSyncer(cfg, a.plat.Home)
			report, err := s.CaptureWithOptions(context.Background(), sync.RunOptions{DryRun: dryRun})
			if err != nil {
				return doterrors.Wrap(err, "capture failed")
			}
			if dryRun {
				printRunReport("Capture plan", report)
				return nil
			}

			fmt.Println(ui.Title("Capture complete"))
			printRunReport("Capture result", report)
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show the module plan without capturing changes")
	return cmd
}

func cmdSync(a *app) *cobra.Command {
	var noApply bool
	var noPush bool
	var dryRun bool

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Capture, commit, pull/rebase, apply, push",
	}

	syncCmd.PersistentFlags().BoolVar(&noApply, "no-apply", false, "Do not apply after pulling")
	syncCmd.PersistentFlags().BoolVar(&noPush, "no-push", false, "Do not push after syncing")
	syncCmd.PersistentFlags().BoolVar(&dryRun, "dry-run", false, "Show module plans without capture, git, apply, or push mutations")

	run := func(cmd *cobra.Command, args []string) error {
		cfg, _, err := a.loadConfig()
		if err != nil {
			return err
		}

		if a.logger != nil {
			a.logger.Info("syncing",
				"noApply", noApply,
				"noPush", noPush,
			)
		}

		s := newSyncer(cfg, a.plat.Home)
		report, err := s.SyncWithReport(context.Background(), sync.Options{NoApply: noApply, NoPush: noPush, DryRun: dryRun})
		if err != nil {
			return doterrors.Wrap(err, "sync failed")
		}
		if dryRun {
			printSyncReport("Sync plan", report)
			return nil
		}

		fmt.Println(ui.Title("Sync complete"))
		printSyncReport("Sync result", report)
		return nil
	}

	syncCmd.RunE = run

	syncCmd.AddCommand(&cobra.Command{
		Use:   "now",
		Short: "Alias for dot sync",
		RunE:  run,
	})

	return syncCmd
}

func printSyncReport(title string, report *sync.SyncReport) {
	fmt.Println(ui.Title(title))
	if report == nil || len(report.Operations) == 0 {
		fmt.Println("  No module operations recorded.")
		return
	}
	for _, operation := range report.Operations {
		printRunReport("", operation)
	}
}

func printRunReport(title string, report *modules.RunReport) {
	if title != "" {
		fmt.Println(ui.Title(title))
	}
	if report == nil || report.Plan == nil {
		fmt.Println("  No module plan recorded.")
		return
	}
	plan := report.Plan
	fmt.Printf("  Operation: %s\n", redact.Text(string(plan.Operation)))
	fmt.Printf("  Plan: %s\n", redact.Text(plan.PlanID))
	fmt.Printf("  Summary: create=%d update=%d delete=%d noop=%d manual=%d blocked=%d\n",
		plan.Summary.Create,
		plan.Summary.Update,
		plan.Summary.Delete,
		plan.Summary.Noop,
		plan.Summary.Manual,
		plan.Summary.Blocked,
	)
	for _, change := range plan.Changes {
		fmt.Printf("  - %s %s", humanAction(change.Action), redact.Text(change.ID))
		if len(change.Capability) > 0 {
			fmt.Printf(" [%s]", joinCapabilities(change.Capability))
		}
		if change.BackupRequired {
			fmt.Print(" backup-required")
		}
		fmt.Println()
		for _, diag := range change.Diagnostics {
			fmt.Printf("      %s: %s\n", redact.Text(diag.Code), redact.Text(diag.Message))
		}
	}
	for _, diag := range plan.Diagnostics {
		fmt.Printf("  diagnostic %s: %s\n", redact.Text(diag.Code), redact.Text(diag.Message))
	}
	if len(report.Backups) > 0 {
		fmt.Printf("  Backups: %d\n", len(report.Backups))
	}
	if len(report.Results) > 0 {
		fmt.Println("  Results:")
		for _, result := range report.Results {
			fmt.Printf("    - %s %s %s\n", redact.Text(string(result.Phase)), redact.Text(result.ID), redact.Text(string(result.Status)))
		}
	}
	for _, diag := range report.Diagnostics {
		fmt.Printf("  diagnostic %s: %s\n", redact.Text(diag.Code), redact.Text(diag.Message))
	}
}

func humanAction(action modules.ChangeAction) string {
	switch action {
	case modules.ActionCreate:
		return "Would create"
	case modules.ActionUpdate:
		return "Would update"
	case modules.ActionDelete:
		return "Would remove"
	case modules.ActionNoop, modules.ActionReport:
		return "No change"
	case modules.ActionManual:
		return "Manual step"
	case modules.ActionBlocked:
		return "Blocked"
	default:
		return string(action)
	}
}

func joinCapabilities(capabilities []modules.Capability) string {
	parts := make([]string, 0, len(capabilities))
	for _, capability := range capabilities {
		parts = append(parts, string(capability))
	}
	return strings.Join(parts, ",")
}

func cmdMacOS(a *app) *cobra.Command {
	macosCmd := &cobra.Command{
		Use:   "macos",
		Short: "macOS-specific state commands",
	}

	var jsonOut bool
	auditCmd := &cobra.Command{
		Use:   "audit",
		Short: "Audit macOS state without mutating it",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !jsonOut {
				return doterrors.NewUserError("dot macos audit currently requires --json")
			}
			host, _ := os.Hostname()
			r := runner.New()
			opts := macos.AuditOptions{
				GOOS:        runtime.GOOS,
				Arch:        runtime.GOARCH,
				Host:        host,
				GeneratedAt: time.Now(),
				Runner:      r,
				HomeDir:     a.plat.Home,
			}
			if cfg, _, err := a.loadConfigSilent(); err == nil {
				opts.RepoRoot = cfg.RepoRoot()
				opts.BrewfilePath = filepath.Join(cfg.StatePath(), "macos", "brew", "Brewfile")
				opts.ExtraModules = []modules.Module{
					modules.NewFilesModule(cfg, chez.New(cfg.Tools.Chezmoi, r), a.plat.Home),
				}
			}
			envelope := macos.NewAudit(cmd.Context(), opts)
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(envelope)
		},
	}
	auditCmd.Flags().BoolVar(&jsonOut, "json", false, "Emit dotstate.audit.v1 JSON")

	macosCmd.AddCommand(auditCmd)
	return macosCmd
}

func cmdSchedule(a *app) *cobra.Command {
	scheduleCmd := &cobra.Command{
		Use:   "schedule",
		Short: "Manage the dotstate macOS user LaunchAgent",
	}

	var (
		dotBin   string
		interval int
		noLoad   bool
		dryRun   bool
	)
	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install and load the dotstate sync LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}
			if dotBin == "" {
				dotBin, err = os.Executable()
				if err != nil {
					return doterrors.Wrap(err, "resolve dot executable")
				}
			}
			opts, err := schedule.OptionsFromConfig(cfg, dotBin)
			if err != nil {
				return doterrors.Wrap(err, "build schedule options")
			}
			if interval > 0 {
				opts.IntervalMinutes = interval
			}
			opts.NoLoad = noLoad
			if dryRun {
				printScheduleStatus("Schedule install plan", &schedule.Status{
					Label:           schedule.Label,
					Path:            schedule.LaunchAgentPath(a.plat.Home),
					Installed:       false,
					Loaded:          false,
					IntervalMinutes: opts.IntervalMinutes,
					ProgramArgs:     []string{opts.DotBin, "--config", opts.ConfigPath, "sync"},
					Message:         "Dry run only: would write the LaunchAgent plist and load it with launchctl unless --no-load is set. No shutdown hook would be installed.",
				})
				return nil
			}
			mgr := schedule.NewManager(a.plat.Home, runner.New())
			status, err := mgr.Install(context.Background(), opts)
			if err != nil {
				return wrapScheduleError(err)
			}
			printScheduleStatus("Schedule installed", status)
			return nil
		},
	}
	installCmd.Flags().StringVar(&dotBin, "dot-bin", "", "Path to dot binary for launchd (defaults to current executable)")
	installCmd.Flags().IntVar(&interval, "interval", 0, "Sync interval in minutes (defaults to sync.interval_minutes)")
	installCmd.Flags().BoolVar(&noLoad, "no-load", false, "Write the LaunchAgent without loading it into launchd")
	installCmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the LaunchAgent plan without writing or loading it")

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show dotstate sync LaunchAgent status",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := schedule.NewManager(a.plat.Home, runner.New())
			status, err := mgr.Inspect(context.Background())
			if err != nil {
				return wrapScheduleError(err)
			}
			printScheduleStatus("Schedule status", status)
			return nil
		},
	}

	removeCmd := &cobra.Command{
		Use:   "remove",
		Short: "Unload and remove the dotstate sync LaunchAgent",
		RunE: func(cmd *cobra.Command, args []string) error {
			mgr := schedule.NewManager(a.plat.Home, runner.New())
			status, err := mgr.Remove(context.Background())
			if err != nil {
				return wrapScheduleError(err)
			}
			printScheduleStatus("Schedule removed", status)
			return nil
		},
	}

	scheduleCmd.AddCommand(installCmd, statusCmd, removeCmd)
	return scheduleCmd
}

func wrapScheduleError(err error) error {
	if errors.Is(err, schedule.ErrUnsupported) {
		return doterrors.WithCode(err, doterrors.ExitUnavailable)
	}
	return err
}

func printScheduleStatus(title string, status *schedule.Status) {
	fmt.Println(ui.Title(title))
	if status == nil {
		fmt.Println("  No schedule status available.")
		return
	}
	fmt.Printf("  Label: %s\n", redact.Text(status.Label))
	fmt.Printf("  Path: %s\n", redact.Text(status.Path))
	fmt.Printf("  Installed: %t\n", status.Installed)
	fmt.Printf("  Loaded: %t\n", status.Loaded)
	if status.IntervalMinutes > 0 {
		fmt.Printf("  Interval: %d minutes\n", status.IntervalMinutes)
	}
	if len(status.ProgramArgs) > 0 {
		fmt.Printf("  Command: %s\n", strings.Join(redactStrings(status.ProgramArgs), " "))
	}
	if status.Message != "" {
		fmt.Printf("  Note: %s\n", redact.Text(status.Message))
	}
}

func redactStrings(values []string) []string {
	out := make([]string, len(values))
	for i, value := range values {
		out[i] = redact.Text(value)
	}
	return out
}

func cmdSubrepo(a *app) *cobra.Command {
	subrepoCmd := &cobra.Command{
		Use:   "subrepo",
		Short: "Inspect sub-repositories tracked by dotstate",
	}
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Report state/subrepos.toml clone status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}
			statuses, err := macos.SubrepoStatuses(cfg, a.plat.Home)
			if err != nil {
				return err
			}
			fmt.Println(ui.Title("Subrepo status"))
			if len(statuses) == 0 {
				fmt.Println("  No subrepos declared in state/subrepos.toml")
				return nil
			}
			for _, status := range statuses {
				url := redact.Text(status.URL)
				if url == "" {
					url = "(no remote)"
				}
				branch := status.Branch
				if branch == "" {
					branch = "default"
				}
				fmt.Printf("  %s: %s [%s] %s\n", redact.Text(status.Path), status.Status, branch, url)
			}
			return nil
		},
	}
	subrepoCmd.AddCommand(statusCmd)
	return subrepoCmd
}

func cmdDiscover(a *app) *cobra.Command {
	var (
		autoYes     bool
		dryRun      bool
		noCommit    bool
		deep        bool
		reportOnly  bool
		secretsMode string
		roots       []string
		maxFileSize int64
	)

	cmd := &cobra.Command{
		Use:   "discover",
		Short: "Discover and add configuration files",
		Long: `Discover configuration files on this machine that are not yet tracked.

This command scans common configuration locations, classifies files by
likelihood of being useful, detects potential secrets, and lets you
interactively select which files to add to the repository.

Examples:
  dot discover              # Interactive discovery
  dot discover --yes        # Auto-accept recommended files
  dot discover --report     # Show what would be discovered (no changes)
  dot discover --deep       # Scan additional directories
`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}

			opts := discover.DefaultOptions()
			opts.AutoYes = autoYes
			opts.DryRun = dryRun
			opts.NoCommit = noCommit
			opts.Deep = deep
			opts.ReportOnly = reportOnly
			opts.SecretsMode = secretsMode
			opts.Roots = roots
			opts.MaxFileSize = maxFileSize

			if a.logger != nil {
				a.logger.Info("starting discovery",
					"deep", deep,
					"autoYes", autoYes,
					"dryRun", dryRun,
					"roots", roots,
					"maxFileSize", maxFileSize,
				)
			}

			disc, err := discover.NewDiscoverer(cfg, opts)
			if err != nil {
				return doterrors.Wrap(err, "init discover")
			}

			if err := disc.Run(context.Background(), opts); err != nil {
				return doterrors.Wrap(err, "discover failed")
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&autoYes, "yes", "y", false, "Auto-accept recommended files without prompting")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be added without making changes")
	cmd.Flags().BoolVar(&noCommit, "no-commit", false, "Skip the commit step")
	cmd.Flags().BoolVar(&deep, "deep", false, "Scan additional directories (AppData, Library)")
	cmd.Flags().BoolVar(&reportOnly, "report", false, "Print report only (no prompts, no changes)")
	cmd.Flags().StringVar(&secretsMode, "secrets", discover.SecretsModeError, "How to handle secrets: error, warning, ignore")
	cmd.Flags().StringSliceVar(&roots, "roots", nil, "Override discovery roots (comma-separated or repeated; advanced)")
	cmd.Flags().Int64Var(&maxFileSize, "max-file-size", discover.DefaultMaxFileSize, "Maximum candidate file size in bytes")

	return cmd
}
