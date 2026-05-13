package macos

import (
	"context"
	"strings"
	"testing"

	"github.com/dnery/dotstate/dot/internal/testutil"
)

func TestRunBrewBundleCheckUsesReadOnlyCommand(t *testing.T) {
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("brew", "bundle", "check", "--file", "/tmp/Brewfile"), "The Brewfile's dependencies are satisfied.\n")

	res, err := RunBrewBundleCheck(context.Background(), r, "", "/tmp/Brewfile")
	if err != nil {
		t.Fatalf("RunBrewBundleCheck error = %v", err)
	}
	if !strings.Contains(res.Stdout, "satisfied") {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	r.AssertCalled(testutil.MatchExact("brew", "bundle", "check", "--file", "/tmp/Brewfile"))
}

func TestRunDefaultsReadUsesSingleDomainKeyRead(t *testing.T) {
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("defaults", "read", "NSGlobalDomain", "AppleShowAllExtensions"), "1\n")

	res, err := RunDefaultsRead(context.Background(), r, "", "NSGlobalDomain", "AppleShowAllExtensions")
	if err != nil {
		t.Fatalf("RunDefaultsRead error = %v", err)
	}
	if strings.TrimSpace(res.Stdout) != "1" {
		t.Fatalf("stdout = %q, want 1", res.Stdout)
	}
	r.AssertCalled(testutil.MatchExact("defaults", "read", "NSGlobalDomain", "AppleShowAllExtensions"))
	r.AssertNotCalled(testutil.MatchCommandPrefix("defaults", "export"))
	r.AssertNotCalled(testutil.MatchCommandPrefix("defaults", "write"))
}

func TestRunLaunchctlPrintUsesUserServiceRead(t *testing.T) {
	r := testutil.NewMockRunner(t)
	r.OnCommandSuccess(testutil.MatchExact("launchctl", "print", "gui/501/com.dnery.dotstate.sync"), "service state = running\n")

	res, err := RunLaunchctlPrint(context.Background(), r, "", "501", "com.dnery.dotstate.sync")
	if err != nil {
		t.Fatalf("RunLaunchctlPrint error = %v", err)
	}
	if !strings.Contains(res.Stdout, "running") {
		t.Fatalf("unexpected stdout: %q", res.Stdout)
	}
	r.AssertCalled(testutil.MatchExact("launchctl", "print", "gui/501/com.dnery.dotstate.sync"))
	r.AssertNotCalled(testutil.MatchCommandPrefix("launchctl", "bootstrap"))
	r.AssertNotCalled(testutil.MatchCommandPrefix("launchctl", "bootout"))
}

func TestMacOSReadOnlyRunnerChecksRequireExplicitTargets(t *testing.T) {
	r := testutil.NewMockRunner(t)
	if _, err := RunBrewBundleCheck(context.Background(), r, "", ""); err == nil || !strings.Contains(err.Error(), "brewfile path") {
		t.Fatalf("RunBrewBundleCheck error = %v, want missing brewfile", err)
	}
	if _, err := RunDefaultsRead(context.Background(), r, "", "NSGlobalDomain", ""); err == nil || !strings.Contains(err.Error(), "domain and key") {
		t.Fatalf("RunDefaultsRead error = %v, want missing key", err)
	}
	if _, err := RunLaunchctlPrint(context.Background(), r, "", "501", ""); err == nil || !strings.Contains(err.Error(), "uid and label") {
		t.Fatalf("RunLaunchctlPrint error = %v, want missing label", err)
	}
	r.AssertCallCount(0)
}
