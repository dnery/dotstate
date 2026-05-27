package macos

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/runner"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestMacOSCaptureArtifactsGolden(t *testing.T) {
	caseDir := macOSFixtureDir(t, "macos", "capture-artifacts")
	brew, ok, err := renderBrewfileArtifact([]modules.Fact{
		{ID: "brew:tap/homebrew/cask", Current: map[string]any{"name": "homebrew/cask"}},
		{ID: "brew:formula/git", Current: map[string]any{"name": "git"}},
		{ID: "brew:cask/visual-studio-code", Current: map[string]any{"name": "visual-studio-code"}},
	})
	if err != nil || !ok {
		t.Fatalf("render brew artifact ok=%v err=%v", ok, err)
	}
	compareTextGolden(t, filepath.Join(caseDir, "Brewfile.golden"), brew)

	mas, ok, err := renderMASArtifact([]modules.Fact{{Current: map[string]any{"id": "497799835", "name": "Xcode", "version": "15.4"}}})
	if err != nil || !ok {
		t.Fatalf("render mas artifact ok=%v err=%v", ok, err)
	}
	compareTextGolden(t, filepath.Join(caseDir, "mas.golden.toml"), mas)

	apps, ok, err := renderAppsArtifact([]modules.Fact{{Current: map[string]any{"bundle_id": "com.example.TestApp", "name": "Test App", "version": "1.2.3", "source_hint": "user_applications", "path": "~/Applications/Test App.app"}}})
	if err != nil || !ok {
		t.Fatalf("render apps artifact ok=%v err=%v", ok, err)
	}
	compareTextGolden(t, filepath.Join(caseDir, "apps.golden.toml"), apps)

	defaults, ok, err := renderDefaultsArtifact([]modules.Fact{{Current: map[string]any{"domain": "NSGlobalDomain", "key": "AppleShowAllExtensions", "value": "1"}}})
	if err != nil || !ok {
		t.Fatalf("render defaults artifact ok=%v err=%v", ok, err)
	}
	compareTextGolden(t, filepath.Join(caseDir, "defaults.golden.toml"), defaults)

	secrets, ok, err := renderSecretsArtifact(nil)
	if err != nil || !ok {
		t.Fatalf("render secrets artifact ok=%v err=%v", ok, err)
	}
	compareTextGolden(t, filepath.Join(caseDir, "secrets.golden.toml"), secrets)
	assertMacOSFixtureSentinelsAbsent(t, caseDir)
}

func TestMacOSArtifactModulesCapturePortableDesiredState(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	brewfile := filepath.Join(repoDir, "state", "macos", "brew", "Brewfile")
	appInfo := testutil.TempFile(t, homeDir, "Applications/Test App.app/Contents/Info.plist", "{}")

	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("brew", "tap"), "homebrew/cask\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--formula", "--versions"), "git 2.51.0\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--cask", "--versions"), "visual-studio-code 1.99.0\n")
	r.OnCommandSuccess(testutil.MatchExact("mas", "list"), "497799835 Xcode (15.4)\n")
	r.OnCommandSuccess(testutil.MatchExact("plutil", "-convert", "json", "-o", "-", appInfo), `{"CFBundleIdentifier":"com.example.TestApp","CFBundleDisplayName":"Test App","CFBundleShortVersionString":"1.2.3"}`)
	r.OnCommandSuccess(testutil.MatchExact("brew", "services", "list", "--json"), `[]`)
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "NSGlobalDomain", "AppleShowAllExtensions"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.finder", "AppleShowAllFiles"), "0\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.dock", "autohide"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("profiles", "status", "-type", "enrollment"), "Enrolled via DEP: No\nMDM enrollment: No\n")

	cache := &auditCache{opts: AuditOptions{
		GOOS:           "darwin",
		Arch:           "arm64",
		Host:           "fixture-host.local",
		GeneratedAt:    time.Date(2026, 5, 13, 0, 0, 0, 0, time.UTC),
		Runner:         r,
		RepoRoot:       repoDir,
		HomeDir:        homeDir,
		BrewfilePath:   brewfile,
		AppDirs:        []string{filepath.Join(homeDir, "Applications")},
		LaunchAgentDir: filepath.Join(homeDir, "Library", "LaunchAgents"),
		UID:            "501",
	}}
	orch := modules.NewOrchestrator(
		newArtifactModule(surfaceBrew, brewfile, cache, renderBrewfileArtifact),
		newArtifactModule(surfaceMAS, filepath.Join(repoDir, "state", "macos", "mas.toml"), cache, renderMASArtifact),
		newArtifactModule(surfaceApps, filepath.Join(repoDir, "state", "macos", "apps.toml"), cache, renderAppsArtifact),
		newArtifactModule(surfaceDefaults, filepath.Join(repoDir, "state", "macos", "defaults.toml"), cache, renderDefaultsArtifact),
		newSecretsModule(filepath.Join(repoDir, "state", "secrets", "generated.toml"), cache),
	)

	report, err := orch.Run(ctx, modules.OperationCapture, modules.RunOptions{})
	if err != nil {
		t.Fatalf("Run capture error = %v", err)
	}
	assertFileContains(t, brewfile, `tap "homebrew/cask"`)
	assertFileContains(t, brewfile, `brew "git"`)
	assertFileContains(t, filepath.Join(repoDir, "state", "macos", "mas.toml"), `id = '497799835'`)
	assertFileContains(t, filepath.Join(repoDir, "state", "macos", "apps.toml"), `bundle_id = 'com.example.TestApp'`)
	assertFileContains(t, filepath.Join(repoDir, "state", "macos", "defaults.toml"), `domain = 'NSGlobalDomain'`)
	assertFileContains(t, filepath.Join(repoDir, "state", "secrets", "generated.toml"), `secret_values_serialized = false`)

	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(encoded), "fixture-host.local") || strings.Contains(string(encoded), "DOTSTATE_TEST_SECRET_DO_NOT_PRINT") {
		t.Fatalf("capture report leaked sensitive value: %s", encoded)
	}
}

func TestMacOSArtifactApplyIsManualNotSystemMutation(t *testing.T) {
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	brewfile := testutil.TempFile(t, repoDir, "state/macos/brew/Brewfile", "brew \"git\"\n")
	cache := &auditCache{opts: AuditOptions{RepoRoot: repoDir, HomeDir: homeDir}}
	mod := newArtifactModule(surfaceBrew, brewfile, cache, renderBrewfileArtifact)
	orch := modules.NewOrchestrator(mod)

	report, err := orch.Run(context.Background(), modules.OperationApply, modules.RunOptions{})
	if err != nil {
		t.Fatalf("Run apply error = %v", err)
	}
	if report.Plan.Summary.Manual != 1 {
		t.Fatalf("plan summary = %#v, want one manual change", report.Plan.Summary)
	}
	if len(report.Results) == 0 || report.Results[0].Status != modules.StatusManual {
		t.Fatalf("results = %#v, want manual result", report.Results)
	}
}

func TestSubreposModuleClonesMissingRepoAndStatusReportsIt(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadMacOSTestConfig(t, repoDir)
	testutil.TempFile(t, repoDir, "state/subrepos.toml", "[[subrepo]]\npath = \"~/.config/nvim\"\nurl = \"https://github.com/example/nvim.git\"\nbranch = \"main\"\n")
	r := &cloneRunner{t: t}
	mod := newSubreposModule(cfg, r, homeDir)
	orch := modules.NewOrchestrator(mod)

	report, err := orch.Run(ctx, modules.OperationApply, modules.RunOptions{})
	if err != nil {
		t.Fatalf("Run apply error = %v", err)
	}
	if len(report.Results) == 0 || report.Results[0].Status != modules.StatusApplied {
		t.Fatalf("results = %#v, want applied clone", report.Results)
	}
	if !fileExists(filepath.Join(homeDir, ".config", "nvim", ".git")) {
		t.Fatalf("subrepo was not cloned into sandbox home")
	}
	statuses, err := SubrepoStatuses(cfg, homeDir)
	if err != nil {
		t.Fatalf("SubrepoStatuses error = %v", err)
	}
	if len(statuses) != 1 || statuses[0].Status != "present" {
		t.Fatalf("statuses = %#v, want present", statuses)
	}
}

func TestSubreposModuleKeepsCredentialedRemoteManual(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	cfg := loadMacOSTestConfig(t, repoDir)
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	testutil.TempFile(t, repoDir, "state/subrepos.toml", "[[subrepo]]\npath = \"~/.config/private\"\nurl = \"ssh://ghp_"+sentinel+"@github.com/example/private.git\"\nbranch = \"main\"\n")
	r := testutil.NewMockRunner(t)
	mod := newSubreposModule(cfg, r, homeDir)
	orch := modules.NewOrchestrator(mod)

	report, err := orch.Run(ctx, modules.OperationApply, modules.RunOptions{})
	if err != nil {
		t.Fatalf("Run apply error = %v", err)
	}
	r.AssertCallCount(0)
	if report.Plan.Summary.Manual != 1 {
		t.Fatalf("plan summary = %#v, want manual credentialed subrepo", report.Plan.Summary)
	}
	if len(report.Results) == 0 || report.Results[0].Status != modules.StatusManual {
		t.Fatalf("results = %#v, want manual result", report.Results)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("Marshal report: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("credentialed subrepo remote leaked sentinel: %s", encoded)
	}
}

func loadMacOSTestConfig(t *testing.T, repoDir string) *config.Config {
	t.Helper()
	configPath := testutil.TempFile(t, repoDir, config.ConfigFileName, "[repo]\npath = \""+repoDir+"\"\n")
	cfg, err := config.Load(configPath)
	if err != nil {
		t.Fatalf("Load config: %v", err)
	}
	return cfg
}

type cloneRunner struct{ t *testing.T }

func (r *cloneRunner) Run(ctx context.Context, dir, name string, args ...string) (*runner.CmdResult, error) {
	r.t.Helper()
	want := []string{"clone", "--branch", "main", "https://github.com/example/nvim.git"}
	if name != "git" || len(args) != len(want)+1 {
		r.t.Fatalf("unexpected command: %s %v", name, args)
	}
	for i, arg := range want {
		if args[i] != arg {
			r.t.Fatalf("arg %d = %q, want %q (all args %#v)", i, args[i], arg, args)
		}
	}
	dest := args[len(args)-1]
	if _, err := os.Stat(filepath.Dir(dest)); err != nil {
		r.t.Fatalf("clone parent dir was not prepared: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(dest, ".git"), 0o755); err != nil {
		return nil, err
	}
	return &runner.CmdResult{}, nil
}

func compareTextGolden(t *testing.T, path string, got []byte) {
	t.Helper()
	if os.Getenv("DOTSTATE_UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("create golden dir: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v", path, err)
	}
	if string(want) != string(got) {
		t.Fatalf("golden mismatch for %s\nwant:\n%s\nactual:\n%s", path, want, got)
	}
}

func assertFileContains(t *testing.T, path, needle string) {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	if !strings.Contains(string(content), needle) {
		t.Fatalf("%s missing %q:\n%s", path, needle, content)
	}
}
