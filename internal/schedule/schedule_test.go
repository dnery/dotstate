package schedule

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestRenderLaunchAgentUsesSafeIntervalSyncOnly(t *testing.T) {
	opts := InstallOptions{
		DotBin:          "/usr/local/bin/dot",
		ConfigPath:      "/Users/test/Projects/dotstate/dot.toml",
		RepoRoot:        "/Users/test/Projects/dotstate",
		LogDir:          "/Users/test/Projects/dotstate/state/logs",
		IntervalMinutes: 30,
	}

	plist := RenderLaunchAgent(opts)
	for _, want := range []string{
		"<string>/usr/local/bin/dot</string>",
		"<string>--config</string>",
		"<string>/Users/test/Projects/dotstate/dot.toml</string>",
		"<string>sync</string>",
		"<key>StartInterval</key>",
		"<integer>1800</integer>",
		"<key>RunAtLoad</key>",
		"<true/>",
		"DOTSTATE_SCHEDULED",
	} {
		if !strings.Contains(plist, want) {
			t.Fatalf("plist missing %q:\n%s", want, plist)
		}
	}
	for _, notWant := range []string{"sudo", "Shutdown", "StartCalendarInterval"} {
		if strings.Contains(plist, notWant) {
			t.Fatalf("plist contains unsafe/unexpected %q:\n%s", notWant, plist)
		}
	}
}

func TestInstallWritesLaunchAgentAndLoadsIt(t *testing.T) {
	ctx := context.Background()
	home := testutil.TempDir(t)
	logs := filepath.Join(home, "repo", "state", "logs")
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "bootout", "gui/501", LaunchAgentPath(home)), "")
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "bootstrap", "gui/501", LaunchAgentPath(home)), "")
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "enable", "gui/501/"+Label), "")

	m := &Manager{Home: home, OS: "darwin", UID: "501", Runner: r}
	status, err := m.Install(ctx, InstallOptions{
		DotBin:          "/bin/dot",
		ConfigPath:      filepath.Join(home, "repo", "dot.toml"),
		RepoRoot:        filepath.Join(home, "repo"),
		LogDir:          logs,
		IntervalMinutes: 15,
	})
	if err != nil {
		t.Fatalf("Install error = %v", err)
	}
	if !status.Installed || !status.Loaded {
		t.Fatalf("status = %#v, want installed and loaded", status)
	}
	testutil.AssertFileExists(t, LaunchAgentPath(home))
	content, err := os.ReadFile(LaunchAgentPath(home))
	if err != nil {
		t.Fatalf("read LaunchAgent: %v", err)
	}
	if !strings.Contains(string(content), "<integer>900</integer>") {
		t.Fatalf("LaunchAgent did not use requested interval:\n%s", content)
	}
	r.AssertCalled(testutil.MatchExact("launchctl", "bootstrap", "gui/501", LaunchAgentPath(home)))
	r.AssertCalled(testutil.MatchExact("launchctl", "enable", "gui/501/"+Label))
}

func TestInspectReportsMissingWithoutLaunchctl(t *testing.T) {
	m := &Manager{Home: testutil.TempDir(t), OS: "darwin", UID: "501", Runner: testutil.NewMockRunner(t)}
	status, err := m.Inspect(context.Background())
	if err != nil {
		t.Fatalf("Inspect error = %v", err)
	}
	if status.Installed || status.Loaded {
		t.Fatalf("status = %#v, want not installed and not loaded", status)
	}
}

func TestRemoveDeletesLaunchAgentBestEffort(t *testing.T) {
	home := testutil.TempDir(t)
	testutil.TempFile(t, filepath.Dir(LaunchAgentPath(home)), filepath.Base(LaunchAgentPath(home)), RenderLaunchAgent(InstallOptions{
		DotBin:          "/bin/dot",
		ConfigPath:      filepath.Join(home, "repo", "dot.toml"),
		RepoRoot:        filepath.Join(home, "repo"),
		LogDir:          filepath.Join(home, "repo", "state", "logs"),
		IntervalMinutes: 30,
	}))
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "bootout", "gui/501", LaunchAgentPath(home)), "")
	m := &Manager{Home: home, OS: "darwin", UID: "501", Runner: r}

	status, err := m.Remove(context.Background())
	if err != nil {
		t.Fatalf("Remove error = %v", err)
	}
	if status.Installed || status.Loaded {
		t.Fatalf("status = %#v, want removed", status)
	}
	testutil.AssertFileNotExists(t, LaunchAgentPath(home))
}

func TestInstallRefusesNonDotstateLaunchAgent(t *testing.T) {
	home := testutil.TempDir(t)
	testutil.TempFile(t, filepath.Dir(LaunchAgentPath(home)), filepath.Base(LaunchAgentPath(home)), "<plist><dict><key>Label</key><string>com.example.other</string></dict></plist>")
	m := &Manager{Home: home, OS: "darwin", UID: "501", Runner: testutil.NewMockRunner(t)}

	_, err := m.Install(context.Background(), InstallOptions{
		DotBin:          "/bin/dot",
		ConfigPath:      filepath.Join(home, "repo", "dot.toml"),
		RepoRoot:        filepath.Join(home, "repo"),
		LogDir:          filepath.Join(home, "repo", "state", "logs"),
		IntervalMinutes: 30,
		NoLoad:          true,
	})
	if err == nil || !strings.Contains(err.Error(), "refusing to overwrite non-dotstate LaunchAgent") {
		t.Fatalf("Install error = %v, want non-dotstate refusal", err)
	}
}

func TestRemoveRefusesNonDotstateLaunchAgent(t *testing.T) {
	home := testutil.TempDir(t)
	testutil.TempFile(t, filepath.Dir(LaunchAgentPath(home)), filepath.Base(LaunchAgentPath(home)), "<plist><dict><key>Label</key><string>com.example.other</string></dict></plist>")
	m := &Manager{Home: home, OS: "darwin", UID: "501", Runner: testutil.NewMockRunner(t)}

	_, err := m.Remove(context.Background())
	if err == nil || !strings.Contains(err.Error(), "refusing to remove non-dotstate LaunchAgent") {
		t.Fatalf("Remove error = %v, want non-dotstate refusal", err)
	}
	testutil.AssertFileExists(t, LaunchAgentPath(home))
}

func TestOptionsFromConfigUsesConfiguredInterval(t *testing.T) {
	repo := testutil.TempDir(t)
	cfgPath := testutil.TempDotToml(t, repo, testutil.MinimalDotToml())
	cfg, err := config.Load(cfgPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	cfg.Sync.IntervalMinutes = 42

	opts := OptionsFromConfig(cfg, "/bin/dot")
	if opts.IntervalMinutes != 42 {
		t.Fatalf("IntervalMinutes = %d, want 42", opts.IntervalMinutes)
	}
	if opts.ConfigPath != cfgPath || opts.RepoRoot != repo || opts.LogDir != filepath.Join(repo, "state", "logs") {
		t.Fatalf("unexpected paths: %#v", opts)
	}
}

func TestUnsupportedPlatform(t *testing.T) {
	m := &Manager{Home: testutil.TempDir(t), OS: "linux", UID: "501"}
	_, err := m.Inspect(context.Background())
	if err != ErrUnsupported {
		t.Fatalf("Inspect error = %v, want ErrUnsupported", err)
	}
}
