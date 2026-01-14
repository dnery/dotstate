package cli

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/runner"
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
}

func Execute() error {
	a := &app{}
	root := &cobra.Command{
		Use:   "dot",
		Short: "dotstate orchestrator",
		Long:  "Cross-platform OS state orchestration (skeleton).",
	}

	root.PersistentFlags().StringVar(&a.cfgPath, "config", "", "Path to dot.toml (defaults to searching upward from current dir)")
	root.PersistentFlags().StringVar(&a.repoDir, "repo-dir", "", "Repo directory override (defaults to repo.path from config)")

	root.AddCommand(cmdVersion())
	root.AddCommand(cmdDoctor(a))
	root.AddCommand(cmdBootstrap(a))
	root.AddCommand(cmdApply(a))
	root.AddCommand(cmdCapture(a))
	root.AddCommand(cmdSync(a))

	return root.Execute()
}

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version info",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("%s %s (%s)\n", version, commit, date)
		},
	}
}

func (a *app) loadConfig() (*config.Config, string, error) {
	// Determine config path.
	cfgPath := a.cfgPath
	if cfgPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return nil, "", err
		}
		found, err := config.FindRepoConfig(cwd)
		if err != nil {
			return nil, "", err
		}
		cfgPath = found
	}

	cfg, err := config.Load(cfgPath)
	if err != nil {
		return nil, "", err
	}

	// Repo dir override.
	if a.repoDir != "" {
		cfg.Repo.Path = a.repoDir
	}

	// Repo root is the directory containing dot.toml (not necessarily cfg.Repo.Path).
	// In this skeleton, we assume dot.toml lives at the repo root.
	repoRoot := filepath.Dir(cfgPath)
	return cfg, repoRoot, nil
}

func newSyncer(cfg *config.Config) *sync.Syncer {
	r := runner.New()
	g := gitx.New(cfg.Tools.Git, r)
	ch := chez.New(cfg.Tools.Chezmoi, r)
	return sync.New(cfg, g, ch)
}

func cmdDoctor(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check prerequisites",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				fmt.Println(ui.Err("Config not found/loaded:"), err)
				fmt.Println("Tip: run from the repo root, or pass --config path/to/dot.toml")
				return err
			}

			fmt.Println(ui.Title("Prerequisites"))
			// Best-effort: check that binaries exist.
			bins := map[string]string{
				"git":     cfg.Tools.Git,
				"chezmoi": cfg.Tools.Chezmoi,
				"op":      cfg.Tools.OP,
			}
			for name, bin := range bins {
				if bin == "" {
					bin = name
				}
				if _, err := exec.LookPath(bin); err != nil {
					fmt.Printf("- %s: %s (missing)\n", ui.Key(name), bin)
				} else {
					fmt.Printf("- %s: %s (ok)\n", ui.Key(name), bin)
				}
			}
			return nil
		},
	}
}

func cmdBootstrap(a *app) *cobra.Command {
	var repoURL string

	cmd := &cobra.Command{
		Use:   "bootstrap",
		Short: "Clone repo (if needed) and prepare this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Bootstrap can run without an existing config in cwd.
			// If --config provided, we use it; otherwise we rely on --repo for now.
			if repoURL == "" && a.cfgPath == "" {
				return fmt.Errorf("bootstrap requires --repo, or run from a repo that contains dot.toml")
			}

			// If config can be loaded, use it; else create minimal.
			cfg := &config.Config{}
			if a.cfgPath != "" {
				loaded, _, err := a.loadConfig()
				if err != nil {
					return err
				}
				cfg = loaded
			} else {
				cfg.Repo.URL = repoURL
				home, _ := os.UserHomeDir()
				cfg.Repo.Path = filepath.Join(home, ".dotstate")
				cfg.Repo.Branch = "main"
				cfg.Chex.SourceDir = "home"
			}

			r := runner.New()
			g := gitx.New(cfg.Tools.Git, r)
			if err := g.EnsureCloned(context.Background(), cfg.Repo.URL, cfg.Repo.Path, cfg.Repo.Branch); err != nil {
				return err
			}

			fmt.Println(ui.Title("Bootstrap"))
			fmt.Println("Repo:", cfg.Repo.Path)
			fmt.Println("Next: dot apply (and later: dot schedule install)")

			return nil
		},
	}

	cmd.Flags().StringVar(&repoURL, "repo", "", "Git URL of your dotstate repo (required if not running inside the repo)")
	return cmd
}

func cmdApply(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "apply",
		Short: "Apply desired state to this machine",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}
			s := newSyncer(cfg)
			return s.Apply(context.Background())
		},
	}
}

func cmdCapture(a *app) *cobra.Command {
	return &cobra.Command{
		Use:   "capture",
		Short: "Capture local changes back into the repo",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := a.loadConfig()
			if err != nil {
				return err
			}
			s := newSyncer(cfg)
			return s.Capture(context.Background())
		},
	}
}

func cmdSync(a *app) *cobra.Command {
	var noApply bool
	var noPush bool

	syncCmd := &cobra.Command{
		Use:   "sync",
		Short: "Capture, commit, pull/rebase, apply, push",
	}

	syncCmd.PersistentFlags().BoolVar(&noApply, "no-apply", false, "Do not apply after pulling")
	syncCmd.PersistentFlags().BoolVar(&noPush, "no-push", false, "Do not push after syncing")

	run := func(cmd *cobra.Command, args []string) error {
		cfg, _, err := a.loadConfig()
		if err != nil {
			return err
		}
		s := newSyncer(cfg)
		return s.Sync(context.Background(), sync.Options{NoApply: noApply, NoPush: noPush})
	}

	syncCmd.RunE = run

	syncCmd.AddCommand(&cobra.Command{
		Use:   "now",
		Short: "Alias for dot sync",
		RunE:  run,
	})

	return syncCmd
}
