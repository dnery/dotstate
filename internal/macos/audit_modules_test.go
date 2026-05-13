package macos

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestNewAuditCollectsReadOnlyMacOSFacts(t *testing.T) {
	ctx := context.Background()
	repoDir := testutil.TempDir(t)
	homeDir := testutil.TempDir(t)
	brewfile := testutil.TempFile(t, repoDir, "state/macos/brew/Brewfile", "tap \"homebrew/cask\"\nbrew \"git\"\n")
	appInfo := testutil.TempFile(t, homeDir, "Applications/Test App.app/Contents/Info.plist", "{}")
	agentPlist := testutil.TempFile(t, homeDir, "Library/LaunchAgents/com.example.agent.plist", "{}")

	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("brew", "tap"), "homebrew/cask\nexample/private\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--formula", "--versions"), "git 2.51.0\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--cask", "--versions"), "visual-studio-code 1.99.0\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "bundle", "check", "--file", brewfile), "The Brewfile's dependencies are satisfied.\n")
	r.OnCommandSuccess(testutil.MatchExact("mas", "list"), "497799835 Xcode (15.4)\n")
	r.OnCommandSuccess(testutil.MatchExact("plutil", "-convert", "json", "-o", "-", appInfo), `{"CFBundleIdentifier":"com.example.TestApp","CFBundleDisplayName":"Test App","CFBundleShortVersionString":"1.2.3"}`)
	r.OnCommandSuccess(testutil.MatchExact("plutil", "-convert", "json", "-o", "-", agentPlist), `{"Label":"com.example.agent"}`)
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "print", "gui/501/com.example.agent"), "service state = running\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "services", "list", "--json"), `[{"name":"postgresql@16","status":"started","user":"fixture-user","file":"`+homeDir+`/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist"}]`)
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "NSGlobalDomain", "AppleShowAllExtensions"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.finder", "AppleShowAllFiles"), "0\n")
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "com.apple.dock", "autohide"), "1\n")
	r.OnCommandSuccess(testutil.MatchExact("profiles", "status", "-type", "enrollment"), "Enrolled via DEP: No\nMDM enrollment: Yes\n")

	audit := NewAudit(ctx, AuditOptions{
		GOOS:           "darwin",
		Arch:           "arm64",
		Host:           "fixture-host.local",
		GeneratedAt:    time.Date(2026, 5, 13, 1, 2, 3, 0, time.UTC),
		Runner:         r,
		RepoRoot:       repoDir,
		HomeDir:        homeDir,
		UID:            "501",
		BrewfilePath:   brewfile,
		AppDirs:        []string{homeDir + "/Applications"},
		LaunchAgentDir: homeDir + "/Library/LaunchAgents",
	})

	if audit.SchemaVersion != modules.SchemaAuditV1 {
		t.Fatalf("schema = %q", audit.SchemaVersion)
	}
	if audit.Target.Host != "<redacted:hostname>" {
		t.Fatalf("host = %q", audit.Target.Host)
	}
	facts := factsByID(audit.Facts)
	for _, id := range []string{
		"brew:tap/homebrew/cask",
		"brew:formula/git",
		"brew:cask/visual-studio-code",
		"brew:brewfile",
		"mas:app/497799835",
		"apps:bundle/com.example.TestApp",
		"launchd:user/com.example.agent",
		"launchd:brew/postgresql@16",
		"defaults:domain/NSGlobalDomain/key/AppleShowAllExtensions",
		"profiles:mdm/enrollment",
		"privacy_tcc:service/FullDiskAccess/client/manual",
		"subrepos:manifest",
		"secrets:keychain/reference-only",
	} {
		if _, ok := facts[id]; !ok {
			t.Fatalf("missing fact %s in %#v", id, facts)
		}
	}
	if got := facts["brew:brewfile"].Current["satisfied"]; got != true {
		t.Fatalf("brewfile satisfied = %#v", got)
	}
	if got := facts["apps:bundle/com.example.TestApp"].Current["path"]; got != "~/Applications/Test App.app" {
		t.Fatalf("app path = %#v", got)
	}
	if got := facts["launchd:brew/postgresql@16"].Current["file"]; got != "~/Library/LaunchAgents/homebrew.mxcl.postgresql@16.plist" {
		t.Fatalf("brew service file path = %#v", got)
	}
	if audit.Summary.Facts != len(audit.Facts) || audit.Summary.Errors != 0 {
		t.Fatalf("summary = %#v facts=%d", audit.Summary, len(audit.Facts))
	}
	for _, diag := range audit.Diagnostics {
		if diag.Code == "macos_audit_surface_pending" {
			t.Fatalf("audit still emitted pending diagnostic: %#v", diag)
		}
	}
	encoded, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("Marshal audit: %v", err)
	}
	if strings.Contains(string(encoded), "fixture-host.local") || strings.Contains(string(encoded), "DOTSTATE_TEST_SECRET_DO_NOT_PRINT") {
		t.Fatalf("audit leaked redaction sentinel/host: %s", encoded)
	}
}

func TestNewAuditLabelsUnavailableToolsWithoutFailing(t *testing.T) {
	r := testutil.NewMockRunner(t)
	r.OnCommandFailure(testutil.MatchExact("missing-brew", "tap"), "exec: missing-brew: executable file not found", -1)
	r.OnCommandFailure(testutil.MatchExact("missing-mas", "list"), "exec: missing-mas: executable file not found", -1)
	r.OnCommandFailure(testutil.MatchExact("missing-brew", "services", "list", "--json"), "exec: missing-brew: executable file not found", -1)
	r.OnCommandFailure(testutil.MatchCommand("defaults"), "Domain not found", 1)
	r.OnCommandFailure(testutil.MatchExact("profiles", "status", "-type", "enrollment"), "profiles unavailable", 1)

	audit := NewAudit(context.Background(), AuditOptions{
		GOOS:        "darwin",
		Arch:        "arm64",
		GeneratedAt: time.Unix(0, 0),
		Runner:      r,
		BrewBin:     "missing-brew",
		MasBin:      "missing-mas",
		AppDirs:     []string{testutil.TempDir(t)},
		HomeDir:     testutil.TempDir(t),
		UID:         "501",
	})

	codes := map[string]bool{}
	for _, diag := range audit.Diagnostics {
		codes[diag.Code] = true
	}
	for _, want := range []string{
		"macos.brew.tool_unavailable",
		"macos.mas.tool_unavailable",
		"macos.launchd.brew_services_unavailable",
		"macos.defaults.key_unavailable",
		"macos.profiles.tool_or_permission_unavailable",
	} {
		if !codes[want] {
			t.Fatalf("missing diagnostic %s in %#v", want, codes)
		}
	}
	if audit.Summary.Errors != 0 {
		t.Fatalf("summary = %#v, want no hard errors", audit.Summary)
	}
}

func TestNewAuditRedactsCommandOutputSentinel(t *testing.T) {
	const sentinel = "DOTSTATE_TEST_SECRET_DO_NOT_PRINT"
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("brew", "tap"), "https://user:"+sentinel+"@github.com/example/tap\n")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--formula", "--versions"), "")
	r.OnCommandSuccess(testutil.MatchExact("brew", "list", "--cask", "--versions"), "")
	r.OnCommandSuccess(testutil.MatchExact("mas", "list"), "")
	r.OnCommandFailure(testutil.MatchExact("brew", "services", "list", "--json"), sentinel, 1)
	r.OnCommandFailure(testutil.MatchCommand("defaults"), sentinel, 1)
	r.OnCommandFailure(testutil.MatchExact("profiles", "status", "-type", "enrollment"), sentinel, 1)

	audit := NewAudit(context.Background(), AuditOptions{
		GOOS:        "darwin",
		Arch:        "arm64",
		Host:        sentinel,
		GeneratedAt: time.Unix(0, 0),
		Runner:      r,
		AppDirs:     []string{testutil.TempDir(t)},
		HomeDir:     testutil.TempDir(t),
	})
	encoded, err := json.Marshal(audit)
	if err != nil {
		t.Fatalf("Marshal audit: %v", err)
	}
	if strings.Contains(string(encoded), sentinel) {
		t.Fatalf("audit leaked sentinel: %s", encoded)
	}
}

func factsByID(facts []modules.Fact) map[string]modules.Fact {
	out := make(map[string]modules.Fact, len(facts))
	for _, fact := range facts {
		out[fact.ID] = fact
	}
	return out
}
