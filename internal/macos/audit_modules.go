package macos

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/dnery/dotstate/dot/internal/modules"
	"github.com/dnery/dotstate/dot/internal/redact"
	"github.com/dnery/dotstate/dot/internal/runner"
)

const (
	surfaceBrew     = "brew"
	surfaceMAS      = "mas"
	surfaceApps     = "apps"
	surfaceLaunchd  = "launchd"
	surfaceDefaults = "defaults"
	surfaceProfiles = "profiles"
	surfacePrivacy  = "privacy_tcc"
	surfaceSecrets  = "secrets"
	surfaceSubrepos = "subrepos"
)

// AuditOptions configures the read-only macOS audit collector. Empty fields use
// safe defaults and never cause the audit command to request elevation.
type AuditOptions struct {
	GOOS        string
	Arch        string
	Host        string
	GeneratedAt time.Time
	Runner      runner.Runner
	RepoRoot    string
	HomeDir     string
	UID         string

	BrewBin      string
	MasBin       string
	DefaultsBin  string
	LaunchctlBin string
	ProfilesBin  string
	PlutilBin    string

	BrewfilePath   string
	AppDirs        []string
	LaunchAgentDir string
	ExtraModules   []modules.Module
}

// NewAudit runs the macOS read-only audit modules and returns a stable
// dotstate.audit.v1 envelope. Collection failures are represented as diagnostics
// so bootstrap and onboarding can keep going without elevated privileges.
func NewAudit(ctx context.Context, opts AuditOptions) AuditEnvelope {
	opts = opts.withDefaults()
	envelope := newAuditEnvelope(opts.GOOS, opts.Arch, opts.Host, opts.GeneratedAt)
	if opts.GOOS != "darwin" {
		diag := modules.NewDiagnostic(
			modules.SeverityWarning,
			"macos_audit_unsupported_platform",
			"macOS audit is only available on darwin; no elevated checks were attempted.",
			"macos",
			"macos:platform",
		)
		diag.Capability = []modules.Capability{modules.CapabilityUnsupported}
		diag.Remediation = "Run this command on macOS, or use platform-specific audit commands when they are implemented."
		envelope.Diagnostics = append(envelope.Diagnostics, diag)
		envelope.updateSummary()
		return envelope
	}

	mods := append([]modules.Module{}, opts.ExtraModules...)
	mods = append(mods,
		newBrewAuditModule(opts),
		newMASAuditModule(opts),
		newAppsAuditModule(opts),
		newLaunchdAuditModule(opts),
		newDefaultsAuditModule(opts),
		newProfilesAuditModule(opts),
		newPrivacyAuditModule(opts),
		newSubreposAuditModule(opts),
		newSecretsAuditModule(opts),
	)

	facts, diagnostics, err := modules.NewOrchestrator(mods...).Audit(ctx)
	if err != nil {
		diag := modules.NewDiagnostic(
			modules.SeverityWarning,
			"macos.audit.partial_failure",
			"One or more macOS audit surfaces failed; partial facts and diagnostics were still returned.",
			"macos",
			"macos:audit",
		)
		diag.Current = map[string]any{"error": redact.Text(err.Error())}
		diag.Capability = []modules.Capability{modules.CapabilityReadOnly}
		diag.Remediation = "Review the surface-specific diagnostics, then rerun dot macos audit --json after fixing missing tools or permissions."
		diagnostics = append(diagnostics, diag)
	}

	modules.SanitizeFacts(facts)
	modules.SanitizeDiagnostics(diagnostics)
	sortFacts(facts)
	sortDiagnostics(diagnostics)
	envelope.Facts = facts
	envelope.Diagnostics = diagnostics
	envelope.updateSummary()
	return envelope
}

func (opts AuditOptions) withDefaults() AuditOptions {
	if opts.GOOS == "" {
		opts.GOOS = runtime.GOOS
	}
	if opts.Arch == "" {
		opts.Arch = runtime.GOARCH
	}
	if opts.Host == "" {
		if host, err := os.Hostname(); err == nil {
			opts.Host = host
		}
	}
	if opts.GeneratedAt.IsZero() {
		opts.GeneratedAt = time.Now()
	}
	if opts.Runner == nil {
		opts.Runner = runner.New()
	}
	if opts.HomeDir == "" {
		if home, err := os.UserHomeDir(); err == nil {
			opts.HomeDir = home
		}
	}
	if opts.UID == "" {
		opts.UID = strconv.Itoa(os.Getuid())
	}
	if opts.BrewBin == "" {
		opts.BrewBin = "brew"
	}
	if opts.MasBin == "" {
		opts.MasBin = "mas"
	}
	if opts.DefaultsBin == "" {
		opts.DefaultsBin = "defaults"
	}
	if opts.LaunchctlBin == "" {
		opts.LaunchctlBin = "launchctl"
	}
	if opts.ProfilesBin == "" {
		opts.ProfilesBin = "profiles"
	}
	if opts.PlutilBin == "" {
		opts.PlutilBin = "plutil"
	}
	if opts.BrewfilePath == "" && opts.RepoRoot != "" {
		opts.BrewfilePath = filepath.Join(opts.RepoRoot, "state", "macos", "brew", "Brewfile")
	}
	if opts.LaunchAgentDir == "" && opts.HomeDir != "" {
		opts.LaunchAgentDir = filepath.Join(opts.HomeDir, "Library", "LaunchAgents")
	}
	if len(opts.AppDirs) == 0 {
		opts.AppDirs = []string{"/Applications"}
		if opts.HomeDir != "" {
			opts.AppDirs = append(opts.AppDirs, filepath.Join(opts.HomeDir, "Applications"))
		}
	}
	return opts
}

type readOnlyAuditModule struct {
	surface string
	audit   func(context.Context) ([]modules.Fact, []modules.Diagnostic, error)
}

func (m readOnlyAuditModule) Surface() string { return m.surface }

func (m readOnlyAuditModule) Audit(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
	return m.audit(ctx)
}

func (m readOnlyAuditModule) Plan(context.Context, modules.Operation) ([]modules.Change, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhasePlan)}, nil
}

func (m readOnlyAuditModule) Backup(context.Context, []modules.Change, *modules.Plan) ([]modules.Backup, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhaseBackup)}, nil
}

func (m readOnlyAuditModule) Apply(context.Context, []modules.Change, *modules.Plan) ([]modules.Result, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhaseApply)}, nil
}

func (m readOnlyAuditModule) Capture(context.Context, []modules.Change, *modules.Plan) ([]modules.Result, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhaseCapture)}, nil
}

func (m readOnlyAuditModule) Verify(context.Context, modules.Operation, []modules.Change, *modules.Plan) ([]modules.Result, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhaseVerify)}, nil
}

func (m readOnlyAuditModule) Restore(context.Context, []modules.Backup) ([]modules.Result, []modules.Diagnostic, error) {
	return nil, []modules.Diagnostic{unsupportedPhaseDiagnostic(m.surface, modules.PhaseRestore)}, nil
}

func unsupportedPhaseDiagnostic(surface string, phase modules.Phase) modules.Diagnostic {
	diag := modules.NewDiagnostic(
		modules.SeverityInfo,
		"macos."+surface+"."+string(phase)+"_unsupported",
		"This macOS audit module is read-only for this phase.",
		surface,
		surface+":"+string(phase),
	)
	diag.Capability = []modules.Capability{modules.CapabilityUnsupported, modules.CapabilityReadOnly}
	diag.Remediation = "Use dot macos audit --json for read-only facts; mutating support must go through module plan/apply/verify tests first."
	return diag
}

func newBrewAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceBrew, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		var facts []modules.Fact
		var diagnostics []modules.Diagnostic
		observedAt := modules.Timestamp(opts.GeneratedAt)

		taps, diag, _ := commandLines(ctx, opts.Runner, surfaceBrew, "brew:tap", opts.BrewBin, []string{"tap"}, observedAt)
		if diag != nil {
			return brewfilePresenceFacts(opts, observedAt), []modules.Diagnostic{*diag}, nil
		}
		for _, tap := range taps {
			facts = append(facts, auditFact(surfaceBrew, "brew:tap/"+tap, modules.Source{Kind: "command", Value: "brew tap", ObservedAt: observedAt}, map[string]any{"name": tap, "installed": true}, nil, []string{"homebrew"}, modules.SensitivityPublic, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly}, modules.LowRisk(true)))
		}

		formulae, diag := brewListFacts(ctx, opts, "formula", []string{"list", "--formula", "--versions"}, observedAt)
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
		} else {
			facts = append(facts, formulae...)
		}
		casks, diag := brewListFacts(ctx, opts, "cask", []string{"list", "--cask", "--versions"}, observedAt)
		if diag != nil {
			diagnostics = append(diagnostics, *diag)
		} else {
			facts = append(facts, casks...)
		}
		brewfileFacts, brewfileDiagnostics := brewfileFacts(ctx, opts, observedAt)
		facts = append(facts, brewfileFacts...)
		diagnostics = append(diagnostics, brewfileDiagnostics...)
		return facts, diagnostics, nil
	}}
}

func brewListFacts(ctx context.Context, opts AuditOptions, kind string, args []string, observedAt string) ([]modules.Fact, *modules.Diagnostic) {
	res, err := opts.Runner.Run(ctx, "", opts.BrewBin, args...)
	if err != nil {
		diag := commandFailureDiagnostic(surfaceBrew, "brew:"+kind, opts.BrewBin, args, res, err)
		return nil, &diag
	}
	var facts []modules.Fact
	for _, item := range parseVersionLines(res.Stdout) {
		id := fmt.Sprintf("brew:%s/%s", kind, item.name)
		facts = append(facts, auditFact(surfaceBrew, id, modules.Source{Kind: "command", Value: strings.Join(append([]string{"brew"}, args...), " "), ObservedAt: observedAt}, map[string]any{"kind": kind, "name": item.name, "versions": item.versions, "installed": true}, nil, []string{"homebrew"}, modules.SensitivityPublic, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly}, modules.LowRisk(true)))
	}
	return facts, nil
}

func brewfilePresenceFacts(opts AuditOptions, observedAt string) []modules.Fact {
	if opts.BrewfilePath == "" {
		return nil
	}
	current := map[string]any{"path": displayArtifactPath(opts, opts.BrewfilePath), "exists": fileExists(opts.BrewfilePath)}
	return []modules.Fact{auditFact(surfaceBrew, "brew:brewfile", modules.Source{Kind: "path", Value: displayArtifactPath(opts, opts.BrewfilePath), ObservedAt: observedAt}, current, nil, []string{"dotstate", "homebrew"}, modules.SensitivityLocalPath, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly}, modules.LowRisk(true))}
}

func brewfileFacts(ctx context.Context, opts AuditOptions, observedAt string) ([]modules.Fact, []modules.Diagnostic) {
	facts := brewfilePresenceFacts(opts, observedAt)
	if opts.BrewfilePath == "" || !fileExists(opts.BrewfilePath) {
		return facts, nil
	}
	res, err := RunBrewBundleCheck(ctx, opts.Runner, opts.BrewBin, opts.BrewfilePath)
	if len(facts) > 0 {
		facts[0].Source = modules.Source{Kind: "command", Value: "brew bundle check --file " + displayArtifactPath(opts, opts.BrewfilePath), ObservedAt: observedAt}
		facts[0].Current["satisfied"] = err == nil
	}
	if err == nil {
		return facts, nil
	}
	diag := commandFailureDiagnostic(surfaceBrew, "brew:brewfile", opts.BrewBin, []string{"bundle", "check", "--file", opts.BrewfilePath}, res, err)
	diag.Source.Value = "brew bundle check --file " + displayArtifactPath(opts, opts.BrewfilePath)
	diag.Code = "macos.brew.brewfile_unsatisfied"
	diag.Severity = modules.SeverityWarning
	diag.Remediation = "Run brew bundle check --file <Brewfile> locally to review missing or changed Homebrew dependencies."
	diag.Capability = []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityDryRunOnly}
	return facts, []modules.Diagnostic{diag}
}

func newMASAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceMAS, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		res, err := opts.Runner.Run(ctx, "", opts.MasBin, "list")
		if err != nil {
			diag := commandFailureDiagnostic(surfaceMAS, "mas:list", opts.MasBin, []string{"list"}, res, err)
			diag.Code = "macos.mas.tool_unavailable"
			if !isCommandUnavailable(err) {
				diag.Code = "macos.mas.account_or_command_unavailable"
				diag.Remediation = "Install/sign in to mas if you want Mac App Store inventory; audit continues without Apple ID details."
			}
			return nil, []modules.Diagnostic{diag}, nil
		}
		var facts []modules.Fact
		for _, app := range parseMASList(res.Stdout) {
			facts = append(facts, auditFact(surfaceMAS, "mas:app/"+app.id, modules.Source{Kind: "command", Value: "mas list", ObservedAt: observedAt}, map[string]any{"id": app.id, "name": app.name, "version": app.version, "installed": true}, nil, []string{"mas"}, modules.SensitivityPersonal, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly}, modules.LowRisk(true)))
		}
		return facts, nil, nil
	}}
}

func newAppsAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceApps, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		var facts []modules.Fact
		var diagnostics []modules.Diagnostic
		for _, dir := range sortedStrings(opts.AppDirs) {
			entries, err := os.ReadDir(dir)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				diagnostics = append(diagnostics, filesystemDiagnostic(surfaceApps, "apps:dir/"+displayPath(opts.HomeDir, dir), "read applications directory", err))
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() || !strings.HasSuffix(entry.Name(), ".app") {
					continue
				}
				appPath := filepath.Join(dir, entry.Name())
				infoPath := filepath.Join(appPath, "Contents", "Info.plist")
				info, diag := readBundleInfo(ctx, opts, infoPath)
				if diag != nil {
					diagnostics = append(diagnostics, *diag)
				}
				bundleID := stringFromAny(info["CFBundleIdentifier"])
				name := firstNonEmpty(stringFromAny(info["CFBundleDisplayName"]), stringFromAny(info["CFBundleName"]), strings.TrimSuffix(entry.Name(), ".app"))
				version := firstNonEmpty(stringFromAny(info["CFBundleShortVersionString"]), stringFromAny(info["CFBundleVersion"]))
				id := "apps:path/" + displayPath(opts.HomeDir, appPath)
				if bundleID != "" {
					id = "apps:bundle/" + bundleID
				}
				current := map[string]any{"name": name, "path": displayPath(opts.HomeDir, appPath), "installed": true, "source_hint": appSourceHint(opts.HomeDir, appPath)}
				if bundleID != "" {
					current["bundle_id"] = bundleID
				}
				if version != "" {
					current["version"] = version
				}
				facts = append(facts, auditFact(surfaceApps, id, modules.Source{Kind: "path", Value: displayPath(opts.HomeDir, appPath), ObservedAt: observedAt}, current, nil, []string{"macos", "manual"}, modules.SensitivityPersonal, modules.ConfidenceHigh, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityManual}, modules.LowRisk(true)))
			}
		}
		return facts, diagnostics, nil
	}}
}

func newLaunchdAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceLaunchd, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		var facts []modules.Fact
		var diagnostics []modules.Diagnostic
		if opts.LaunchAgentDir != "" {
			entries, err := os.ReadDir(opts.LaunchAgentDir)
			if err != nil && !os.IsNotExist(err) {
				diagnostics = append(diagnostics, filesystemDiagnostic(surfaceLaunchd, "launchd:user", "read user LaunchAgents", err))
			}
			if err == nil {
				for _, entry := range entries {
					if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".plist") {
						continue
					}
					plistPath := filepath.Join(opts.LaunchAgentDir, entry.Name())
					info, diag := readBundleInfo(ctx, opts, plistPath)
					if diag != nil {
						diagnostics = append(diagnostics, *diag)
					}
					label := firstNonEmpty(stringFromAny(info["Label"]), strings.TrimSuffix(entry.Name(), ".plist"))
					current := map[string]any{"label": label, "path": displayPath(opts.HomeDir, plistPath), "installed": true}
					if opts.UID != "" {
						if res, err := RunLaunchctlPrint(ctx, opts.Runner, opts.LaunchctlBin, opts.UID, label); err == nil {
							current["loaded"] = true
							current["state_summary"] = launchctlStateSummary(res.Stdout)
						} else {
							current["loaded"] = false
						}
					}
					facts = append(facts, auditFact(surfaceLaunchd, "launchd:user/"+label, modules.Source{Kind: "path", Value: displayPath(opts.HomeDir, plistPath), ObservedAt: observedAt}, current, nil, []string{"launchd"}, modules.SensitivityLocalPath, modules.ConfidenceHigh, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityDryRunOnly}, modules.LowRisk(true)))
				}
			}
		}
		serviceFacts, serviceDiagnostics := brewServiceFacts(ctx, opts, observedAt)
		facts = append(facts, serviceFacts...)
		diagnostics = append(diagnostics, serviceDiagnostics...)
		return facts, diagnostics, nil
	}}
}

func brewServiceFacts(ctx context.Context, opts AuditOptions, observedAt string) ([]modules.Fact, []modules.Diagnostic) {
	res, err := opts.Runner.Run(ctx, "", opts.BrewBin, "services", "list", "--json")
	if err != nil {
		diag := commandFailureDiagnostic(surfaceLaunchd, "launchd:brew", opts.BrewBin, []string{"services", "list", "--json"}, res, err)
		diag.Code = "macos.launchd.brew_services_unavailable"
		diag.Remediation = "Install Homebrew or run brew services list --json locally if you want Homebrew service inventory."
		return nil, []modules.Diagnostic{diag}
	}
	var rows []map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &rows); err != nil {
		diag := modules.NewDiagnostic(modules.SeverityWarning, "macos.launchd.brew_services_parse_failed", "Homebrew services JSON could not be parsed.", surfaceLaunchd, "launchd:brew")
		diag.Current = map[string]any{"error": redact.Text(err.Error())}
		return nil, []modules.Diagnostic{diag}
	}
	facts := make([]modules.Fact, 0, len(rows))
	for _, row := range rows {
		name := stringFromAny(row["name"])
		if name == "" {
			continue
		}
		current := map[string]any{"name": name, "managed_by_brew": true}
		for _, key := range []string{"status", "user", "file"} {
			if value := stringFromAny(row[key]); value != "" {
				if key == "file" {
					value = displayPath(opts.HomeDir, value)
				}
				current[key] = value
			}
		}
		facts = append(facts, auditFact(surfaceLaunchd, "launchd:brew/"+name, modules.Source{Kind: "command", Value: "brew services list --json", ObservedAt: observedAt}, current, nil, []string{"homebrew", "launchd"}, modules.SensitivityPersonal, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityDryRunOnly}, modules.LowRisk(true)))
	}
	return facts, nil
}

func newDefaultsAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceDefaults, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		var facts []modules.Fact
		var diagnostics []modules.Diagnostic
		for _, item := range curatedDefaults() {
			res, err := RunDefaultsRead(ctx, opts.Runner, opts.DefaultsBin, item.domain, item.key)
			id := fmt.Sprintf("defaults:domain/%s/key/%s", item.domain, item.key)
			current := map[string]any{"domain": item.domain, "key": item.key, "curated": true}
			if err != nil {
				current["available"] = false
				diag := commandFailureDiagnostic(surfaceDefaults, id, opts.DefaultsBin, []string{"read", item.domain, item.key}, res, err)
				diag.Code = "macos.defaults.key_unavailable"
				diag.Severity = modules.SeverityInfo
				diag.Remediation = "This curated default is absent or unreadable; audit records the gap without exporting full preference plists."
				diagnostics = append(diagnostics, diag)
			} else {
				current["available"] = true
				current["value"] = parseDefaultsValue(res.Stdout)
			}
			facts = append(facts, auditFact(surfaceDefaults, id, modules.Source{Kind: "command", Value: "defaults read " + item.domain + " " + item.key, ObservedAt: observedAt}, current, nil, []string{"macos", "manual"}, item.sensitivity, modules.ConfidenceHigh, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityDryRunOnly}, modules.LowRisk(true)))
		}
		return facts, diagnostics, nil
	}}
}

func newProfilesAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceProfiles, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		res, err := opts.Runner.Run(ctx, "", opts.ProfilesBin, "status", "-type", "enrollment")
		if err != nil {
			diag := commandFailureDiagnostic(surfaceProfiles, "profiles:mdm/enrollment", opts.ProfilesBin, []string{"status", "-type", "enrollment"}, res, err)
			diag.Code = "macos.profiles.tool_or_permission_unavailable"
			diag.Capability = []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityManual, modules.CapabilityRequiresMDM}
			diag.Remediation = "Configuration profile and MDM posture remains report-only; use System Settings or MDM tooling for details."
			return nil, []modules.Diagnostic{diag}, nil
		}
		current := parseProfilesStatus(res.Stdout)
		current["report_only"] = true
		fact := auditFact(surfaceProfiles, "profiles:mdm/enrollment", modules.Source{Kind: "command", Value: "profiles status -type enrollment", ObservedAt: observedAt}, current, nil, []string{"macos", "mdm"}, modules.SensitivityPersonal, modules.ConfidenceMedium, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityRequiresMDM, modules.CapabilityManual}, modules.LowRisk(false))
		return []modules.Fact{fact}, nil, nil
	}}
}

func newPrivacyAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfacePrivacy, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		services := []string{"Accessibility", "FullDiskAccess", "ScreenRecording", "Automation", "DeveloperTools"}
		facts := make([]modules.Fact, 0, len(services))
		for _, service := range services {
			facts = append(facts, auditFact(surfacePrivacy, "privacy_tcc:service/"+service+"/client/manual", modules.Source{Kind: "manual", Value: "macOS privacy settings", ObservedAt: observedAt}, map[string]any{"service": service, "status": "manual_checkpoint", "database_read": false}, nil, []string{"manual", "macos"}, modules.SensitivityRestricted, modules.ConfidenceMedium, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityManual}, modules.Risk{Level: modules.RiskMedium, Reasons: []string{"protected privacy surface"}, RequiresConfirmation: true, Reversible: false}))
		}
		return facts, privacySafetyDiagnostics(), nil
	}}
}

func newSubreposAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceSubrepos, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		if opts.RepoRoot == "" {
			return nil, nil, nil
		}
		manifest := filepath.Join(opts.RepoRoot, "state", "subrepos.toml")
		fact := auditFact(surfaceSubrepos, "subrepos:manifest", modules.Source{Kind: "path", Value: displayArtifactPath(opts, manifest), ObservedAt: observedAt}, map[string]any{"path": displayArtifactPath(opts, manifest), "exists": fileExists(manifest)}, nil, []string{"dotstate", "git"}, modules.SensitivityLocalPath, modules.ConfidenceMedium, []modules.Capability{modules.CapabilityReadOnly}, modules.LowRisk(true))
		return []modules.Fact{fact}, nil, nil
	}}
}

func newSecretsAuditModule(opts AuditOptions) modules.Module {
	return readOnlyAuditModule{surface: surfaceSecrets, audit: func(ctx context.Context) ([]modules.Fact, []modules.Diagnostic, error) {
		observedAt := modules.Timestamp(opts.GeneratedAt)
		fact := auditFact(surfaceSecrets, "secrets:keychain/reference-only", modules.Source{Kind: "policy", Value: "dotstate secret safety policy", ObservedAt: observedAt}, map[string]any{"keychain_contents_read": false, "secret_values_serialized": false, "recommended_reference_scheme": "op://"}, nil, []string{"dotstate", "manual"}, modules.SensitivityRestricted, modules.ConfidenceConfirmed, []modules.Capability{modules.CapabilityReadOnly, modules.CapabilityManual}, modules.Risk{Level: modules.RiskHigh, Reasons: []string{"decrypted Keychain values are secret material"}, RequiresConfirmation: true, Reversible: false})
		return []modules.Fact{fact}, nil, nil
	}}
}

func newAuditEnvelope(goos, arch, host string, generatedAt time.Time) AuditEnvelope {
	return AuditEnvelope{
		SchemaVersion: modules.SchemaAuditV1,
		GeneratedAt:   modules.Timestamp(generatedAt),
		Target: modules.Target{
			OS:   goos,
			Arch: arch,
			Host: redactHostname(host),
		},
		Facts:       []modules.Fact{},
		Diagnostics: []modules.Diagnostic{},
		Summary: AuditSummary{
			Redactions: 1,
		},
	}
}

func auditFact(surface, id string, source modules.Source, current, desired map[string]any, managedBy []string, sensitivity modules.Sensitivity, confidence modules.Confidence, capabilities []modules.Capability, risk modules.Risk) modules.Fact {
	return modules.Fact{
		SchemaVersion: modules.SchemaFactV1,
		Surface:       surface,
		ID:            id,
		Source:        source,
		Current:       current,
		Desired:       desired,
		ManagedBy:     managedBy,
		Sensitivity:   sensitivity,
		Confidence:    confidence,
		Capability:    capabilities,
		Risk:          risk,
		Diagnostics:   nil,
	}
}

func commandLines(ctx context.Context, r runner.Runner, surface, id, name string, args []string, observedAt string) ([]string, *modules.Diagnostic, bool) {
	res, err := r.Run(ctx, "", name, args...)
	if err != nil {
		diag := commandFailureDiagnostic(surface, id, name, args, res, err)
		return nil, &diag, false
	}
	return nonEmptyLines(res.Stdout), nil, true
}

func commandFailureDiagnostic(surface, id, name string, args []string, res *runner.CmdResult, err error) modules.Diagnostic {
	code := "macos." + surface + ".command_failed"
	severity := modules.SeverityWarning
	remediation := "Install or fix the required tool, then rerun dot macos audit --json."
	capabilities := []modules.Capability{modules.CapabilityReadOnly}
	if isCommandUnavailable(err) {
		code = "macos." + surface + ".tool_unavailable"
		capabilities = []modules.Capability{modules.CapabilityUnsupported, modules.CapabilityReadOnly}
	}
	diag := modules.NewDiagnostic(severity, code, "Read-only macOS audit command could not complete.", surface, id)
	diag.Source = modules.Source{Kind: "command", Value: strings.Join(append([]string{name}, args...), " ")}
	diag.Current = map[string]any{"error": redact.Text(err.Error())}
	if res != nil {
		diag.Current["exit_code"] = res.Code
		if strings.TrimSpace(res.Stderr) != "" {
			diag.Current["stderr"] = redact.Text(res.Stderr)
		}
	}
	diag.Capability = capabilities
	diag.Remediation = remediation
	return diag
}

func filesystemDiagnostic(surface, id, action string, err error) modules.Diagnostic {
	diag := modules.NewDiagnostic(modules.SeverityWarning, "macos."+surface+".read_failed", "Read-only macOS audit could not inspect a filesystem path.", surface, id)
	diag.Source = modules.Source{Kind: "path", Value: id}
	diag.Current = map[string]any{"action": action, "error": redact.Text(err.Error())}
	diag.Capability = []modules.Capability{modules.CapabilityReadOnly}
	if permissionDiagnostic, ok := modules.PermissionDiagnostic(surface, id, action, err); ok {
		return permissionDiagnostic
	}
	return diag
}

func isCommandUnavailable(err error) bool {
	if err == nil {
		return false
	}
	message := err.Error()
	return strings.Contains(message, "executable file not found") || strings.Contains(message, "no such file or directory") || errors.Is(err, os.ErrNotExist)
}

type versionedItem struct {
	name     string
	versions []string
}

func parseVersionLines(output string) []versionedItem {
	lines := nonEmptyLines(output)
	items := make([]versionedItem, 0, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		items = append(items, versionedItem{name: fields[0], versions: fields[1:]})
	}
	return items
}

type masApp struct {
	id      string
	name    string
	version string
}

func parseMASList(output string) []masApp {
	var apps []masApp
	for _, line := range nonEmptyLines(output) {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		id := fields[0]
		rest := strings.TrimSpace(strings.TrimPrefix(line, id))
		version := ""
		if open := strings.LastIndex(rest, "("); open > 0 && strings.HasSuffix(rest, ")") {
			version = strings.TrimSuffix(rest[open+1:], ")")
			rest = strings.TrimSpace(rest[:open])
		}
		apps = append(apps, masApp{id: id, name: rest, version: version})
	}
	return apps
}

func readBundleInfo(ctx context.Context, opts AuditOptions, plistPath string) (map[string]any, *modules.Diagnostic) {
	res, err := opts.Runner.Run(ctx, "", opts.PlutilBin, "-convert", "json", "-o", "-", plistPath)
	if err != nil {
		diag := commandFailureDiagnostic(surfaceApps, "apps:plist/"+displayPath(opts.HomeDir, plistPath), opts.PlutilBin, []string{"-convert", "json", "-o", "-", plistPath}, res, err)
		diag.Code = "macos.apps.plist_unreadable"
		diag.Remediation = "The app or plist is not readable without additional permissions; audit continues with path-only metadata."
		return map[string]any{}, &diag
	}
	var info map[string]any
	if err := json.Unmarshal([]byte(res.Stdout), &info); err != nil {
		diag := modules.NewDiagnostic(modules.SeverityWarning, "macos.apps.plist_parse_failed", "App Info.plist JSON could not be parsed.", surfaceApps, "apps:plist/"+displayPath(opts.HomeDir, plistPath))
		diag.Current = map[string]any{"error": redact.Text(err.Error())}
		return map[string]any{}, &diag
	}
	return info, nil
}

type curatedDefault struct {
	domain      string
	key         string
	sensitivity modules.Sensitivity
}

func curatedDefaults() []curatedDefault {
	return []curatedDefault{
		{domain: "NSGlobalDomain", key: "AppleShowAllExtensions", sensitivity: modules.SensitivityPersonal},
		{domain: "com.apple.finder", key: "AppleShowAllFiles", sensitivity: modules.SensitivityPersonal},
		{domain: "com.apple.dock", key: "autohide", sensitivity: modules.SensitivityPersonal},
	}
}

func parseDefaultsValue(output string) string {
	return strings.TrimSpace(output)
}

func parseProfilesStatus(output string) map[string]any {
	current := map[string]any{}
	for _, line := range nonEmptyLines(output) {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(parts[0]))
		key = strings.NewReplacer(" ", "_", "-", "_").Replace(key)
		value := strings.TrimSpace(parts[1])
		switch strings.ToLower(value) {
		case "yes", "true":
			current[key] = true
		case "no", "false":
			current[key] = false
		default:
			current[key] = redact.Text(value)
		}
	}
	if len(current) == 0 {
		current["status_observed"] = true
	}
	return current
}

func launchctlStateSummary(output string) string {
	for _, line := range nonEmptyLines(output) {
		if strings.Contains(strings.ToLower(line), "state") {
			return redact.Text(strings.TrimSpace(line))
		}
	}
	return "observed"
}

func nonEmptyLines(output string) []string {
	var lines []string
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func sortedStrings(values []string) []string {
	out := append([]string{}, values...)
	sort.Strings(out)
	return out
}

func sortFacts(facts []modules.Fact) {
	sort.SliceStable(facts, func(i, j int) bool {
		left, right := surfaceRank(facts[i].Surface), surfaceRank(facts[j].Surface)
		if left != right {
			return left < right
		}
		return facts[i].ID < facts[j].ID
	})
}

func sortDiagnostics(diagnostics []modules.Diagnostic) {
	sort.SliceStable(diagnostics, func(i, j int) bool {
		left, right := surfaceRank(diagnostics[i].Surface), surfaceRank(diagnostics[j].Surface)
		if left != right {
			return left < right
		}
		if diagnostics[i].Code != diagnostics[j].Code {
			return diagnostics[i].Code < diagnostics[j].Code
		}
		return diagnostics[i].ID < diagnostics[j].ID
	})
}

func surfaceRank(surface string) int {
	order := []string{"files", surfaceBrew, surfaceMAS, surfaceApps, surfaceLaunchd, surfaceDefaults, surfaceProfiles, surfacePrivacy, surfaceSubrepos, surfaceSecrets}
	for i, candidate := range order {
		if surface == candidate {
			return i
		}
	}
	return len(order)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func displayArtifactPath(opts AuditOptions, path string) string {
	if path == "" {
		return ""
	}
	if opts.RepoRoot != "" {
		if rel, err := filepath.Rel(filepath.Clean(opts.RepoRoot), filepath.Clean(path)); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(rel)
		}
	}
	return displayPath(opts.HomeDir, path)
}

func displayPath(homeDir, path string) string {
	if path == "" {
		return ""
	}
	clean := filepath.Clean(path)
	if homeDir != "" {
		home := filepath.Clean(homeDir)
		if clean == home {
			return "~"
		}
		if rel, err := filepath.Rel(home, clean); err == nil && rel != "." && !strings.HasPrefix(rel, "..") {
			return filepath.ToSlash(filepath.Join("~", rel))
		}
	}
	return filepath.ToSlash(clean)
}

func appSourceHint(homeDir, path string) string {
	clean := filepath.Clean(path)
	if homeDir != "" {
		userApps := filepath.Join(filepath.Clean(homeDir), "Applications")
		if strings.HasPrefix(clean, userApps+string(os.PathSeparator)) || clean == userApps {
			return "user_applications"
		}
	}
	if strings.HasPrefix(clean, "/Applications"+string(os.PathSeparator)) || clean == "/Applications" {
		return "applications"
	}
	return "curated_app_directory"
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return strings.TrimSpace(v.String())
	case nil:
		return ""
	default:
		return strings.TrimSpace(fmt.Sprint(v))
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
