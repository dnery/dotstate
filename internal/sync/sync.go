package sync

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	doterrors "github.com/dnery/dotstate/dot/internal/errors"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/modules"
)

type Syncer struct {
	Cfg     *config.Config
	Git     *gitx.Git
	Chez    *chez.Chezmoi
	Modules *modules.Orchestrator
}

type Options struct {
	NoApply bool
	NoPush  bool
	DryRun  bool
}

type RunOptions struct {
	DryRun bool
}

type SyncReport struct {
	Operations []*modules.RunReport
}

var (
	osHostname           = os.Hostname
	defaultCommitMessage = gitx.DefaultCommitMessage
)

func New(cfg *config.Config, g *gitx.Git, ch *chez.Chezmoi) *Syncer {
	home, _ := os.UserHomeDir()
	files := modules.NewFilesModule(cfg, ch, home)
	return NewWithModules(cfg, g, ch, modules.NewOrchestrator(files))
}

func NewWithModules(cfg *config.Config, g *gitx.Git, ch *chez.Chezmoi, orchestrator *modules.Orchestrator) *Syncer {
	return &Syncer{Cfg: cfg, Git: g, Chez: ch, Modules: orchestrator}
}

func (s *Syncer) Capture(ctx context.Context) error {
	_, err := s.CaptureWithOptions(ctx, RunOptions{})
	return err
}

func (s *Syncer) CaptureWithOptions(ctx context.Context, opts RunOptions) (*modules.RunReport, error) {
	return s.Modules.Run(ctx, modules.OperationCapture, modules.RunOptions{DryRun: opts.DryRun})
}

func (s *Syncer) Apply(ctx context.Context) error {
	_, err := s.ApplyWithOptions(ctx, RunOptions{})
	return err
}

func (s *Syncer) ApplyWithOptions(ctx context.Context, opts RunOptions) (*modules.RunReport, error) {
	return s.Modules.Run(ctx, modules.OperationApply, modules.RunOptions{DryRun: opts.DryRun})
}

func (s *Syncer) PlanApply(ctx context.Context) (*modules.Plan, error) {
	return s.Modules.Plan(ctx, modules.OperationApply)
}

func (s *Syncer) PlanCapture(ctx context.Context) (*modules.Plan, error) {
	return s.Modules.Plan(ctx, modules.OperationCapture)
}

func (s *Syncer) Sync(ctx context.Context, opts Options) error {
	_, err := s.SyncWithReport(ctx, opts)
	return err
}

func (s *Syncer) SyncWithReport(ctx context.Context, opts Options) (*SyncReport, error) {
	report := &SyncReport{}

	if err := s.ensureCleanBeforeSync(ctx); err != nil {
		return report, err
	}

	captureReport, err := s.CaptureWithOptions(ctx, RunOptions{DryRun: opts.DryRun})
	report.Operations = append(report.Operations, captureReport)
	if err != nil {
		return report, fmt.Errorf("capture: %w", err)
	}

	if opts.DryRun {
		if !opts.NoApply {
			applyReport, err := s.ApplyWithOptions(ctx, RunOptions{DryRun: true})
			report.Operations = append(report.Operations, applyReport)
			if err != nil {
				return report, fmt.Errorf("apply plan: %w", err)
			}
		}
		return report, nil
	}

	host, _ := osHostname()
	msg := defaultCommitMessage(host)

	committed, err := s.Git.Commit(ctx, s.Cfg.Repo.Path, msg)
	if err != nil {
		return report, fmt.Errorf("commit: %w", err)
	}
	_ = committed // kept for future UX reporting.

	// Pull/rebase before apply so we converge on the canonical remote state.
	if err := s.Git.PullRebase(ctx, s.Cfg.Repo.Path); err != nil {
		return report, s.pullError(ctx, err)
	}

	if !opts.NoApply {
		applyReport, err := s.ApplyWithOptions(ctx, RunOptions{})
		report.Operations = append(report.Operations, applyReport)
		if err != nil {
			return report, fmt.Errorf("apply: %w", err)
		}
	}

	if !opts.NoPush {
		if err := s.Git.Push(ctx, s.Cfg.Repo.Path); err != nil {
			return report, fmt.Errorf("push: %w", err)
		}
	}

	return report, nil
}

func (s *Syncer) ensureCleanBeforeSync(ctx context.Context) error {
	status, err := s.Git.PorcelainStatus(ctx, s.Cfg.Repo.Path)
	if err != nil {
		return fmt.Errorf("status: %w", err)
	}
	if strings.TrimSpace(status) == "" {
		return nil
	}
	return doterrors.NewConflictError(
		"repo has uncommitted changes before sync",
		formatStatusDetails(status)+"\nResolve, commit, or stash repo changes before running dot sync. Destination-file edits can still be captured with dot capture.",
	)
}

func (s *Syncer) pullError(ctx context.Context, err error) error {
	status, statusErr := s.Git.PorcelainStatus(ctx, s.Cfg.Repo.Path)
	if statusErr == nil && hasConflictStatus(status) {
		return doterrors.NewConflictError(
			"git pull/rebase produced conflicts",
			formatStatusDetails(status)+"\nResolve conflicts in the repo, then run git rebase --continue or abort and retry dot sync.",
		)
	}
	if statusErr == nil && strings.TrimSpace(status) != "" {
		return fmt.Errorf("pull: %w\nrepo status after failed pull:\n%s", err, formatStatusDetails(status))
	}
	return fmt.Errorf("pull: %w", err)
}

func hasConflictStatus(status string) bool {
	for _, line := range strings.Split(status, "\n") {
		if len(line) < 2 {
			continue
		}
		code := line[:2]
		if strings.Contains(code, "U") || code == "AA" || code == "DD" {
			return true
		}
	}
	return false
}

func formatStatusDetails(status string) string {
	status = strings.TrimSpace(status)
	if status == "" {
		return "repo status: clean"
	}
	lines := strings.Split(status, "\n")
	const maxLines = 20
	if len(lines) > maxLines {
		lines = append(lines[:maxLines], fmt.Sprintf("... %d more status lines omitted", len(lines)-maxLines))
	}
	return "repo status:\n  " + strings.Join(lines, "\n  ")
}
