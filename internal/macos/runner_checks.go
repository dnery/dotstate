package macos

import (
	"context"
	"fmt"
	"strings"

	"github.com/dnery/dotstate/dot/internal/runner"
)

// RunBrewBundleCheck runs Homebrew Bundle in check-only mode. It is a small
// read-only probe shared by tests and future audit collectors so command shape
// changes are caught before macOS module work depends on them.
func RunBrewBundleCheck(ctx context.Context, r runner.Runner, brewBin, brewfile string) (*runner.CmdResult, error) {
	if r == nil {
		r = runner.New()
	}
	if strings.TrimSpace(brewBin) == "" {
		brewBin = "brew"
	}
	if strings.TrimSpace(brewfile) == "" {
		return nil, fmt.Errorf("brewfile path is required")
	}
	return r.Run(ctx, "", brewBin, "bundle", "check", "--file", brewfile)
}

// RunDefaultsRead reads a single curated defaults domain/key pair. It never
// exports whole domains or writes preferences.
func RunDefaultsRead(ctx context.Context, r runner.Runner, defaultsBin, domain, key string) (*runner.CmdResult, error) {
	if r == nil {
		r = runner.New()
	}
	if strings.TrimSpace(defaultsBin) == "" {
		defaultsBin = "defaults"
	}
	if strings.TrimSpace(domain) == "" || strings.TrimSpace(key) == "" {
		return nil, fmt.Errorf("defaults domain and key are required")
	}
	return r.Run(ctx, "", defaultsBin, "read", domain, key)
}

// RunLaunchctlPrint reads launchd state for a user service. It does not unload,
// bootstrap, enable, or otherwise mutate launchd.
func RunLaunchctlPrint(ctx context.Context, r runner.Runner, launchctlBin, uid, label string) (*runner.CmdResult, error) {
	if r == nil {
		r = runner.New()
	}
	if strings.TrimSpace(launchctlBin) == "" {
		launchctlBin = "launchctl"
	}
	if strings.TrimSpace(uid) == "" || strings.TrimSpace(label) == "" {
		return nil, fmt.Errorf("launchctl uid and label are required")
	}
	return r.Run(ctx, "", launchctlBin, "print", "gui/"+uid+"/"+label)
}
