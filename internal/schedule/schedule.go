// Package schedule manages user-level scheduled dotstate sync jobs.
package schedule

import (
	"bytes"
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/dnery/dotstate/dot/internal/config"
	"github.com/dnery/dotstate/dot/internal/runner"
)

const (
	// Label is the launchd label used for the macOS user LaunchAgent.
	Label = "com.dnery.dotstate.sync"

	launchAgentRelPath = "Library/LaunchAgents/" + Label + ".plist"
	defaultPATH        = "/opt/homebrew/bin:/usr/local/bin:/usr/bin:/bin:/usr/sbin:/sbin"
)

// ErrUnsupported is returned when scheduling is requested on a platform that
// does not have an implementation yet.
var ErrUnsupported = errors.New("schedule is currently implemented for macOS user LaunchAgents only")

// Manager installs, inspects, and removes a user-level schedule.
type Manager struct {
	Home   string
	OS     string
	UID    string
	Runner runner.Runner
}

// InstallOptions describes the LaunchAgent that should be installed.
type InstallOptions struct {
	DotBin          string
	ConfigPath      string
	RepoRoot        string
	LogDir          string
	IntervalMinutes int
	NoLoad          bool
}

// Status reports the observed schedule state.
type Status struct {
	Label           string
	Path            string
	Installed       bool
	Loaded          bool
	IntervalMinutes int
	ProgramArgs     []string
	Message         string
}

// NewManager constructs a Manager for the current process user.
func NewManager(home string, r runner.Runner) *Manager {
	return &Manager{
		Home:   home,
		OS:     runtime.GOOS,
		UID:    strconv.Itoa(os.Getuid()),
		Runner: r,
	}
}

// LaunchAgentPath returns the LaunchAgent path for a home directory.
func LaunchAgentPath(home string) string {
	return filepath.Join(home, launchAgentRelPath)
}

// OptionsFromConfig builds install options from dotstate config and a dot binary path.
func OptionsFromConfig(cfg *config.Config, dotBin string) (InstallOptions, error) {
	interval := cfg.Sync.IntervalMinutes
	if interval <= 0 {
		interval = config.DefaultSyncInterval
	}
	configPath, err := filepath.Abs(cfg.ConfigPath())
	if err != nil {
		return InstallOptions{}, fmt.Errorf("resolve absolute config path: %w", err)
	}
	repoRoot, err := filepath.Abs(cfg.RepoRoot())
	if err != nil {
		return InstallOptions{}, fmt.Errorf("resolve absolute repo root: %w", err)
	}
	logDir, err := filepath.Abs(cfg.LogPath())
	if err != nil {
		return InstallOptions{}, fmt.Errorf("resolve absolute schedule log dir: %w", err)
	}
	return InstallOptions{
		DotBin:          dotBin,
		ConfigPath:      configPath,
		RepoRoot:        repoRoot,
		LogDir:          logDir,
		IntervalMinutes: interval,
	}, nil
}

// Install writes the LaunchAgent plist and registers it with launchd unless NoLoad is set.
func (m *Manager) Install(ctx context.Context, opts InstallOptions) (*Status, error) {
	if err := m.requireDarwin(); err != nil {
		return nil, err
	}
	if err := validateInstallOptions(opts); err != nil {
		return nil, err
	}

	path := LaunchAgentPath(m.Home)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create LaunchAgents directory: %w", err)
	}
	owned, err := isDotstateLaunchAgent(path)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, fmt.Errorf("refusing to overwrite non-dotstate LaunchAgent at %s", path)
	}
	if opts.LogDir != "" {
		if err := os.MkdirAll(opts.LogDir, 0o755); err != nil {
			return nil, fmt.Errorf("create schedule log directory: %w", err)
		}
	}

	plist := RenderLaunchAgent(opts)
	if err := os.WriteFile(path, []byte(plist), 0o644); err != nil {
		return nil, fmt.Errorf("write LaunchAgent: %w", err)
	}

	status := &Status{
		Label:           Label,
		Path:            path,
		Installed:       true,
		Loaded:          false,
		IntervalMinutes: normalizedInterval(opts.IntervalMinutes),
		ProgramArgs:     programArgs(opts),
		Message:         "LaunchAgent written; shutdown flush is not installed because macOS cannot guarantee a safe non-destructive shutdown hook.",
	}

	if opts.NoLoad {
		status.Message = "LaunchAgent written but not loaded (--no-load)."
		return status, nil
	}
	if m.Runner == nil {
		return nil, errors.New("schedule install requires a command runner")
	}

	// bootout is intentionally best-effort: it fails when the agent is not loaded yet.
	_, _ = m.Runner.Run(ctx, "", "launchctl", "bootout", m.launchctlDomain(), path)
	if _, err := m.Runner.Run(ctx, "", "launchctl", "bootstrap", m.launchctlDomain(), path); err != nil {
		return nil, fmt.Errorf("load LaunchAgent: %w", err)
	}
	if _, err := m.Runner.Run(ctx, "", "launchctl", "enable", m.launchctlService()); err != nil {
		return nil, fmt.Errorf("enable LaunchAgent: %w", err)
	}
	status.Loaded = true
	status.Message = "LaunchAgent installed and loaded. It runs `dot sync` every interval; use `dot sync now` for an immediate manual flush."
	return status, nil
}

// Inspect reports whether the LaunchAgent plist exists and whether launchd has it loaded.
func (m *Manager) Inspect(ctx context.Context) (*Status, error) {
	if err := m.requireDarwin(); err != nil {
		return nil, err
	}
	path := LaunchAgentPath(m.Home)
	status := &Status{Label: Label, Path: path}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			status.Message = "LaunchAgent is not installed."
			return status, nil
		}
		return nil, fmt.Errorf("stat LaunchAgent: %w", err)
	}
	status.Installed = true
	status.Message = "LaunchAgent plist exists."
	if m.Runner == nil {
		status.Message = "LaunchAgent plist exists; load status was not checked."
		return status, nil
	}
	res, err := m.Runner.Run(ctx, "", "launchctl", "print", m.launchctlService())
	if err != nil {
		msg := strings.TrimSpace(res.Stderr)
		if msg == "" {
			msg = err.Error()
		}
		status.Message = "LaunchAgent plist exists but launchd does not report it loaded: " + msg
		return status, nil
	}
	status.Loaded = true
	status.Message = "LaunchAgent is installed and loaded."
	return status, nil
}

// Remove unloads the LaunchAgent best-effort and removes the plist.
func (m *Manager) Remove(ctx context.Context) (*Status, error) {
	if err := m.requireDarwin(); err != nil {
		return nil, err
	}
	path := LaunchAgentPath(m.Home)
	owned, err := isDotstateLaunchAgent(path)
	if err != nil {
		return nil, err
	}
	if !owned {
		return nil, fmt.Errorf("refusing to remove non-dotstate LaunchAgent at %s", path)
	}
	if m.Runner != nil {
		_, _ = m.Runner.Run(ctx, "", "launchctl", "bootout", m.launchctlDomain(), path)
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("remove LaunchAgent: %w", err)
	}
	return &Status{
		Label:     Label,
		Path:      path,
		Installed: false,
		Loaded:    false,
		Message:   "LaunchAgent removed. No shutdown hook was removed because dotstate does not install one on macOS.",
	}, nil
}

// RenderLaunchAgent renders a launchd plist for the provided options.
func RenderLaunchAgent(opts InstallOptions) string {
	intervalSeconds := normalizedInterval(opts.IntervalMinutes) * 60
	args := programArgs(opts)
	stdout := filepath.Join(opts.LogDir, "schedule.out.log")
	stderr := filepath.Join(opts.LogDir, "schedule.err.log")

	var b strings.Builder
	b.WriteString("<?xml version=\"1.0\" encoding=\"UTF-8\"?>\n")
	b.WriteString("<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n")
	b.WriteString("<plist version=\"1.0\">\n")
	b.WriteString("<dict>\n")
	writeKeyString(&b, "Label", Label)
	b.WriteString("  <key>ProgramArguments</key>\n")
	b.WriteString("  <array>\n")
	for _, arg := range args {
		b.WriteString("    <string>")
		b.WriteString(xmlEscape(arg))
		b.WriteString("</string>\n")
	}
	b.WriteString("  </array>\n")
	writeKeyString(&b, "WorkingDirectory", opts.RepoRoot)
	writeKeyInteger(&b, "StartInterval", intervalSeconds)
	writeKeyBool(&b, "RunAtLoad", true)
	writeKeyString(&b, "StandardOutPath", stdout)
	writeKeyString(&b, "StandardErrorPath", stderr)
	b.WriteString("  <key>EnvironmentVariables</key>\n")
	b.WriteString("  <dict>\n")
	writeKeyString(&b, "PATH", defaultPATH)
	writeKeyString(&b, "DOTSTATE_SCHEDULED", "1")
	b.WriteString("  </dict>\n")
	b.WriteString("</dict>\n")
	b.WriteString("</plist>\n")
	return b.String()
}

func validateInstallOptions(opts InstallOptions) error {
	var missing []string
	if opts.DotBin == "" {
		missing = append(missing, "dot binary")
	}
	if opts.ConfigPath == "" {
		missing = append(missing, "config path")
	}
	if opts.RepoRoot == "" {
		missing = append(missing, "repo root")
	}
	if opts.LogDir == "" {
		missing = append(missing, "log directory")
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing schedule install option(s): %s", strings.Join(missing, ", "))
	}
	return nil
}

func isDotstateLaunchAgent(path string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return true, nil
		}
		return false, fmt.Errorf("read existing LaunchAgent: %w", err)
	}
	return bytes.Contains(content, []byte(Label)), nil
}

func (m *Manager) requireDarwin() error {
	if m.OS != "darwin" {
		return ErrUnsupported
	}
	if m.Home == "" {
		return errors.New("home directory is required")
	}
	if m.UID == "" {
		return errors.New("uid is required")
	}
	return nil
}

func (m *Manager) launchctlDomain() string {
	return "gui/" + m.UID
}

func (m *Manager) launchctlService() string {
	return m.launchctlDomain() + "/" + Label
}

func normalizedInterval(minutes int) int {
	if minutes <= 0 {
		return config.DefaultSyncInterval
	}
	return minutes
}

func programArgs(opts InstallOptions) []string {
	return []string{opts.DotBin, "--config", opts.ConfigPath, "sync"}
}

func writeKeyString(b *strings.Builder, key, value string) {
	b.WriteString("  <key>")
	b.WriteString(xmlEscape(key))
	b.WriteString("</key>\n")
	b.WriteString("  <string>")
	b.WriteString(xmlEscape(value))
	b.WriteString("</string>\n")
}

func writeKeyInteger(b *strings.Builder, key string, value int) {
	b.WriteString("  <key>")
	b.WriteString(xmlEscape(key))
	b.WriteString("</key>\n")
	b.WriteString("  <integer>")
	b.WriteString(strconv.Itoa(value))
	b.WriteString("</integer>\n")
}

func writeKeyBool(b *strings.Builder, key string, value bool) {
	b.WriteString("  <key>")
	b.WriteString(xmlEscape(key))
	b.WriteString("</key>\n")
	if value {
		b.WriteString("  <true/>\n")
		return
	}
	b.WriteString("  <false/>\n")
}

func xmlEscape(value string) string {
	var buf bytes.Buffer
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}
