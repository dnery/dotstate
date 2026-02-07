package discover

import (
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/chez"
	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/gitx"
	"github.com/dnery/dotstate/dot/internal/platform"
	"github.com/dnery/dotstate/dot/internal/sync"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestDiscoverThenCaptureReAdd(t *testing.T) {
	ctx := testutil.ContextWithCancel(t)
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)

	configContent := strings.ReplaceAll(
		testutil.MinimalDotToml(),
		`path = "~/dotstate"`,
		"path = "+strconv.Quote(repoDir),
	)
	testutil.TempDotToml(t, repoDir, configContent)

	if err := os.MkdirAll(filepath.Join(repoDir, "home"), 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	managedPath := testutil.TempFile(t, homeDir, ".zshrc", "export PATH=/usr/bin\n")
	settingsPath := testutil.TempFile(t, homeDir, "Library/Application Support/Code/User/settings.json", "{}\n")

	cfg, err := config.Load(filepath.Join(repoDir, "dot.toml"))
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	mock := testutil.NewMockRunner(t)
	mock.OnCommandSuccess(testutil.MatchCommandPrefix("chezmoi", "add"), "")
	mock.OnCommandSuccess(testutil.MatchExact("chezmoi", "re-add"), "")

	scanOpts := ScanOptions{
		Roots:         []string{homeDir},
		MaxFileSize:   DefaultMaxFileSize,
		IncludeHidden: true,
		Home:          homeDir,
		ManagedPaths:  make(map[string]bool),
	}

	discoverer := &Discoverer{
		cfg:      cfg,
		plat:     &platform.Platform{OS: platform.Darwin, Home: homeDir},
		runner:   mock,
		chezmoi:  chez.New(cfg.Tools.Chezmoi, mock),
		git:      gitx.New(cfg.Tools.Git, mock),
		scanner:  NewScanner(scanOpts),
		secrets:  NewSecretDetector(mock),
		prompter: NewPrompterWithIO(strings.NewReader(""), io.Discard, true),
	}

	if err := discoverer.Run(ctx, Options{AutoYes: true, NoCommit: true, SecretsMode: "ignore"}); err != nil {
		t.Fatalf("discover run failed: %v", err)
	}

	addCall := findCall(mock.Calls(), testutil.MatchCommandPrefix("chezmoi", "add"))
	if addCall == nil {
		t.Fatal("expected chezmoi add to be invoked")
	}
	assertArgsContain(t, addCall.Args, managedPath, settingsPath)

	if err := os.WriteFile(managedPath, []byte("export PATH=/usr/local/bin\n"), 0o644); err != nil {
		t.Fatalf("failed to update managed file: %v", err)
	}

	syncer := sync.New(cfg, gitx.New(cfg.Tools.Git, mock), chez.New(cfg.Tools.Chezmoi, mock))
	if err := syncer.Capture(ctx); err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	addIndex := findCallIndex(mock.Calls(), testutil.MatchCommandPrefix("chezmoi", "add"))
	reAddIndex := findCallIndex(mock.Calls(), testutil.MatchExact("chezmoi", "re-add"))
	if addIndex == -1 || reAddIndex == -1 {
		t.Fatalf("expected chezmoi add and re-add to be called, got calls: %v", mock.Calls())
	}
	if addIndex >= reAddIndex {
		t.Fatalf("expected chezmoi re-add after add, got add index %d and re-add index %d", addIndex, reAddIndex)
	}
}

func findCall(calls []testutil.CommandCall, matcher testutil.CommandMatcher) *testutil.CommandCall {
	for _, call := range calls {
		if matcher(call) {
			return &call
		}
	}
	return nil
}

func findCallIndex(calls []testutil.CommandCall, matcher testutil.CommandMatcher) int {
	for i, call := range calls {
		if matcher(call) {
			return i
		}
	}
	return -1
}

func assertArgsContain(t *testing.T, args []string, values ...string) {
	t.Helper()
	for _, value := range values {
		found := false
		for _, arg := range args {
			if arg == value {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected args to contain %q, got %v", value, args)
		}
	}
}
