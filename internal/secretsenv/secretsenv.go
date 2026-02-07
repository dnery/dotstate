package secretsenv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	defaultTimeout = 45 * time.Second
	aggregateName  = "secrets"
)

var envNameRE = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

type Config struct {
	OPBin             string           `json:"op_bin"`
	CacheDir          string           `json:"cache_dir"`
	AggregateExclude  []string         `json:"aggregate_exclude"`
	Scopes            []ScopeConfig    `json:"scopes"`
	MutationAllowlist []MutationTarget `json:"mutation_allowlist"`
	Migration         MigrationConfig  `json:"migration"`
}

type ScopeConfig struct {
	Name    string `json:"name"`
	Account string `json:"account"`
	Vault   string `json:"vault"`
	Item    string `json:"item"`
	Section string `json:"section"`
	Mutate  bool   `json:"mutate"`
}

type MutationTarget struct {
	Account string `json:"account"`
	Vault   string `json:"vault"`
}

type MigrationConfig struct {
	PersonalEnvPath string            `json:"personal_env_path"`
	Sources         []MigrationSource `json:"sources"`
}

type MigrationSource struct {
	Scope   string         `json:"scope"`
	Account string         `json:"account"`
	Vault   string         `json:"vault"`
	Item    string         `json:"item"`
	Fields  []FieldMapping `json:"fields"`
}

type FieldMapping struct {
	From string `json:"from"`
	To   string `json:"to"`
}

type fieldValue struct {
	Label string
	Value string
	Type  string
}

type opItem struct {
	ID       string    `json:"id,omitempty"`
	Title    string    `json:"title"`
	Category string    `json:"category"`
	Fields   []opField `json:"fields"`
}

type opField struct {
	ID      string     `json:"id,omitempty"`
	Type    string     `json:"type"`
	Purpose string     `json:"purpose,omitempty"`
	Label   string     `json:"label"`
	Value   string     `json:"value,omitempty"`
	Section *opSection `json:"section,omitempty"`
}

type opSection struct {
	ID    string `json:"id,omitempty"`
	Label string `json:"label"`
}

type opListItem struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Category string `json:"category"`
}

type app struct {
	stdout io.Writer
	stderr io.Writer
	getenv func(string) string
}

func Execute(args []string) int {
	a := &app{
		stdout: os.Stdout,
		stderr: os.Stderr,
		getenv: os.Getenv,
	}
	if err := a.run(args); err != nil {
		fmt.Fprintf(a.stderr, "error: %v\n", err)
		return 1
	}
	return 0
}

func (a *app) run(args []string) error {
	if len(args) == 0 || args[0] == "-h" || args[0] == "--help" {
		a.usage()
		return nil
	}

	configPath := ""
	if len(args) >= 2 && args[0] == "--config" {
		configPath = args[1]
		args = args[2:]
	}
	if len(args) == 0 {
		a.usage()
		return nil
	}

	cfg, err := loadConfig(configPath)
	if err != nil {
		return err
	}
	if err := cfg.validate(); err != nil {
		return err
	}

	switch args[0] {
	case "inventory":
		return a.inventory(cfg)
	case "migrate":
		return a.migrate(cfg, args[1:])
	case "archive-sources":
		return a.archiveSources(cfg, args[1:])
	case "refresh":
		return a.refresh(cfg, args[1:])
	case "status":
		return a.status(cfg)
	case "run":
		return a.runCommand(cfg, args[1:])
	default:
		a.usage()
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func (a *app) usage() {
	fmt.Fprintln(a.stdout, `Usage: secrets-env [--config path] <command>

Commands:
  inventory          Print redacted 1Password target and source metadata
  migrate            Create or update allowed Secure Notes from configured sources
  archive-sources    Archive migrated source API Credential items after verification
  refresh            Refresh mode 600 cache files from Secure Notes
  status             Show cache file status without printing values
  run -- <command>   Run a command with the aggregate POSIX cache sourced`)
}

func loadConfig(path string) (*Config, error) {
	if path == "" {
		path = defaultConfigPath()
	}
	b, err := os.ReadFile(expandPath(path))
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg Config
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	if cfg.OPBin == "" {
		cfg.OPBin = "op"
	}
	cfg.CacheDir = expandPath(cfg.CacheDir)
	cfg.Migration.PersonalEnvPath = expandPath(cfg.Migration.PersonalEnvPath)
	return &cfg, nil
}

func defaultConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "dotstate", "secrets-env.json")
}

func defaultCacheDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".local", "state", "dotstate", "secrets")
}

func expandPath(path string) string {
	if path == "" {
		return path
	}
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	}
	return filepath.Clean(os.ExpandEnv(path))
}

func (c *Config) validate() error {
	if c.CacheDir == "" {
		c.CacheDir = defaultCacheDir()
	}
	seen := map[string]bool{}
	for _, scope := range c.Scopes {
		if scope.Name == "" || scope.Account == "" || scope.Vault == "" || scope.Item == "" {
			return fmt.Errorf("scope entries require name, account, vault, and item")
		}
		if seen[scope.Name] {
			return fmt.Errorf("duplicate scope %q", scope.Name)
		}
		seen[scope.Name] = true
		if scope.Mutate && !c.mutationAllowed(scope.Account, scope.Vault) {
			return fmt.Errorf("scope %q is mutable but not allowlisted: %s/%s", scope.Name, scope.Account, scope.Vault)
		}
	}
	for _, src := range c.Migration.Sources {
		if !seen[src.Scope] {
			return fmt.Errorf("migration source %q references unknown scope %q", src.Item, src.Scope)
		}
		for _, f := range src.Fields {
			if !validEnvName(f.To) {
				return fmt.Errorf("invalid target env var %q for source %q", f.To, src.Item)
			}
		}
	}
	return nil
}

func (c *Config) mutationAllowed(account, vault string) bool {
	for _, allowed := range c.MutationAllowlist {
		if allowed.Account == account && allowed.Vault == vault {
			return true
		}
	}
	return false
}

func (c *Config) scope(name string) (ScopeConfig, bool) {
	for _, scope := range c.Scopes {
		if scope.Name == name {
			return scope, true
		}
	}
	return ScopeConfig{}, false
}

func (a *app) inventory(cfg *Config) error {
	fmt.Fprintln(a.stdout, "Scopes")
	for _, scope := range cfg.Scopes {
		item, err := a.getItem(context.Background(), cfg, scope.Account, scope.Vault, scope.Item, false)
		status := "missing"
		count := 0
		if err == nil {
			status = "present"
			count = countExportableFieldLabels(item, scope.Section)
		}
		mutability := "read-only"
		if scope.Mutate {
			mutability = "mutable"
		}
		fmt.Fprintf(a.stdout, "- %s: %s/%s %q [%s, %s, fields=%d]\n", scope.Name, scope.Account, scope.Vault, scope.Item, status, mutability, count)
	}

	fmt.Fprintln(a.stdout, "Migration sources")
	if cfg.Migration.PersonalEnvPath != "" {
		fmt.Fprintf(a.stdout, "- personal env file: %s -> scope personal [values redacted]\n", cfg.Migration.PersonalEnvPath)
	}
	for _, src := range cfg.Migration.Sources {
		mapped := make([]string, 0, len(src.Fields))
		for _, f := range src.Fields {
			mapped = append(mapped, f.From+"->"+f.To)
		}
		sort.Strings(mapped)
		fmt.Fprintf(a.stdout, "- %s/%s %q -> scope %s [%s]\n", src.Account, src.Vault, src.Item, src.Scope, strings.Join(mapped, ", "))
	}
	return nil
}

func countExportableFieldLabels(item opItem, section string) int {
	count := 0
	for _, f := range item.Fields {
		if f.Label != "notesPlain" && validEnvName(f.Label) && (section == "" || sameSection(f, section) || emptySection(f)) {
			count++
		}
	}
	return count
}

func (a *app) migrate(cfg *Config, args []string) error {
	apply := hasFlag(args, "--apply")
	if !apply && !hasFlag(args, "--dry-run") {
		fmt.Fprintln(a.stdout, "Dry run only. Re-run with --apply to mutate allowed Secure Notes.")
	}

	byScope, err := a.collectMigrationValues(cfg)
	if err != nil {
		return err
	}

	for scopeName, values := range byScope {
		scope, ok := cfg.scope(scopeName)
		if !ok {
			return fmt.Errorf("unknown migration scope %q", scopeName)
		}
		if !scope.Mutate {
			return fmt.Errorf("scope %q is not marked mutable", scopeName)
		}
		if !cfg.mutationAllowed(scope.Account, scope.Vault) {
			return fmt.Errorf("refusing to mutate non-allowlisted vault %s/%s", scope.Account, scope.Vault)
		}
		fmt.Fprintf(a.stdout, "- %s: %d variables -> %s/%s %q\n", scopeName, len(values), scope.Account, scope.Vault, scope.Item)
		if apply {
			if err := a.upsertSecureNote(context.Background(), cfg, scope, values); err != nil {
				return err
			}
		}
	}

	if apply {
		fmt.Fprintln(a.stdout, "Migration applied. Values were not printed.")
	}
	return nil
}

func (a *app) archiveSources(cfg *Config, args []string) error {
	if !hasFlag(args, "--apply") {
		fmt.Fprintln(a.stdout, "Dry run only. Re-run with --apply after cache parity is verified.")
	}
	apply := hasFlag(args, "--apply")
	for _, src := range cfg.Migration.Sources {
		item, err := a.getItem(context.Background(), cfg, src.Account, src.Vault, src.Item, false)
		if err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "- archive source: %s/%s %q [%s]\n", src.Account, src.Vault, src.Item, item.ID)
		if apply {
			if err := a.op(context.Background(), cfg, nil, "item", "delete", item.ID, "--archive", "--vault", src.Vault, "--account", src.Account); err != nil {
				return err
			}
		}
	}
	if apply {
		fmt.Fprintln(a.stdout, "Source items archived.")
	}
	return nil
}

func (a *app) collectMigrationValues(cfg *Config) (map[string]map[string]fieldValue, error) {
	byScope := map[string]map[string]fieldValue{}
	put := func(scope, label, value string) {
		if !validEnvName(label) || value == "" {
			return
		}
		if byScope[scope] == nil {
			byScope[scope] = map[string]fieldValue{}
		}
		byScope[scope][label] = fieldValue{Label: label, Value: value, Type: fieldTypeForLabel(label)}
	}

	if cfg.Migration.PersonalEnvPath != "" {
		env, err := readEnvFileWithTimeout(cfg.Migration.PersonalEnvPath)
		if err != nil {
			return nil, err
		}
		for k, v := range env {
			put("personal", k, v)
		}
	}

	for _, src := range cfg.Migration.Sources {
		item, err := a.getItem(context.Background(), cfg, src.Account, src.Vault, src.Item, true)
		if err != nil {
			return nil, err
		}
		fields := map[string]string{}
		for _, f := range item.Fields {
			fields[f.Label] = f.Value
		}
		for _, mapping := range src.Fields {
			value := fields[mapping.From]
			if value == "" {
				return nil, fmt.Errorf("source %q missing field %q", src.Item, mapping.From)
			}
			put(src.Scope, mapping.To, value)
		}
	}
	return byScope, nil
}

func readEnvFileWithTimeout(path string) (map[string]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "cat", path)
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("read env file %s timed out", path)
		}
		return nil, fmt.Errorf("read env file %s: %w", path, err)
	}
	return parseDotenv(out.String())
}

func parseDotenv(s string) (map[string]string, error) {
	env := map[string]string{}
	for _, raw := range strings.Split(s, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if !validEnvName(key) {
			continue
		}
		parsed, err := parseDotenvValue(strings.TrimSpace(value))
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", key, err)
		}
		env[key] = parsed
	}
	return env, nil
}

func parseDotenvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}
	if value[0] == '\'' {
		if !strings.HasSuffix(value, "'") || len(value) == 1 {
			return "", errors.New("unterminated single quote")
		}
		return value[1 : len(value)-1], nil
	}
	if value[0] == '"' {
		if !strings.HasSuffix(value, "\"") || len(value) == 1 {
			return "", errors.New("unterminated double quote")
		}
		return unescapeDoubleQuoted(value[1 : len(value)-1]), nil
	}
	return strings.TrimSpace(value), nil
}

func unescapeDoubleQuoted(s string) string {
	var b strings.Builder
	escaped := false
	for _, r := range s {
		if escaped {
			switch r {
			case 'n':
				b.WriteByte('\n')
			case 'r':
				b.WriteByte('\r')
			case 't':
				b.WriteByte('\t')
			default:
				b.WriteRune(r)
			}
			escaped = false
			continue
		}
		if r == '\\' {
			escaped = true
			continue
		}
		b.WriteRune(r)
	}
	if escaped {
		b.WriteByte('\\')
	}
	return b.String()
}

func (a *app) upsertSecureNote(ctx context.Context, cfg *Config, scope ScopeConfig, values map[string]fieldValue) error {
	itemID, exists, err := a.findItemID(ctx, cfg, scope.Account, scope.Vault, scope.Item)
	if err != nil {
		return err
	}
	item := opItem{
		Title:    scope.Item,
		Category: "SECURE_NOTE",
		Fields: []opField{
			{ID: "notesPlain", Type: "STRING", Purpose: "NOTES", Label: "notesPlain", Value: "Managed by secrets-env on this machine. Values intentionally redacted from command output."},
		},
	}
	if exists {
		existing, err := a.getItem(ctx, cfg, scope.Account, scope.Vault, itemID, true)
		if err != nil {
			return err
		}
		item.ID = existing.ID
		item.Fields = preserveFields(existing.Fields, scope.Section)
	}
	item.Fields = mergeSecretFields(item.Fields, scope.Section, values)
	payload, err := json.Marshal(item)
	if err != nil {
		return fmt.Errorf("marshal secure note: %w", err)
	}

	if exists {
		return a.op(ctx, cfg, payload, "item", "edit", itemID, "--vault", scope.Vault, "--account", scope.Account)
	}
	return a.op(ctx, cfg, payload, "item", "create", "--vault", scope.Vault, "--account", scope.Account, "-")
}

func preserveFields(fields []opField, section string) []opField {
	out := make([]opField, 0, len(fields))
	for _, f := range fields {
		if f.Label == "notesPlain" || !sameSection(f, section) {
			out = append(out, f)
		}
	}
	if len(out) == 0 {
		out = append(out, opField{ID: "notesPlain", Type: "STRING", Purpose: "NOTES", Label: "notesPlain", Value: "Managed by secrets-env on this machine."})
	}
	return out
}

func mergeSecretFields(fields []opField, section string, values map[string]fieldValue) []opField {
	labels := make([]string, 0, len(values))
	for label := range values {
		labels = append(labels, label)
	}
	sort.Strings(labels)
	sec := &opSection{ID: sectionID(section), Label: section}
	if section == "" {
		sec = nil
	}
	for _, label := range labels {
		fv := values[label]
		fields = append(fields, opField{
			ID:      fieldID(label),
			Type:    fv.Type,
			Label:   label,
			Value:   fv.Value,
			Section: sec,
		})
	}
	return fields
}

func sectionID(section string) string {
	return strings.ToLower(regexp.MustCompile(`[^A-Za-z0-9_]+`).ReplaceAllString(section, "_"))
}

func fieldID(label string) string {
	return strings.ToLower(regexp.MustCompile(`[^A-Za-z0-9_]+`).ReplaceAllString(label, "_"))
}

func (a *app) refresh(cfg *Config, args []string) error {
	all := hasFlag(args, "--all")
	scopeName := flagValue(args, "--scope")
	if !all && scopeName == "" {
		return fmt.Errorf("refresh requires --all or --scope <name>")
	}
	var scopes []ScopeConfig
	if all {
		scopes = cfg.Scopes
	} else {
		scope, ok := cfg.scope(scopeName)
		if !ok {
			return fmt.Errorf("unknown scope %q", scopeName)
		}
		scopes = []ScopeConfig{scope}
	}

	refreshed := map[string]map[string]fieldValue{}
	for _, scope := range scopes {
		item, err := a.getItem(context.Background(), cfg, scope.Account, scope.Vault, scope.Item, true)
		if err != nil {
			return err
		}
		values := exportableValues(item, scope.Section)
		if len(values) == 0 {
			return fmt.Errorf("scope %q returned no exportable fields", scope.Name)
		}
		if err := writeCacheSet(cfg.CacheDir, scope.Name, values); err != nil {
			return err
		}
		refreshed[scope.Name] = values
		fmt.Fprintf(a.stdout, "- refreshed %s (%d variables)\n", scope.Name, len(values))
	}

	if all {
		merged := map[string]fieldValue{}
		exclude := cfg.aggregateExcludeSet()
		for _, scope := range cfg.Scopes {
			for k, v := range refreshed[scope.Name] {
				if exclude[k] {
					continue
				}
				merged[k] = v
			}
		}
		if err := writeCacheSet(cfg.CacheDir, aggregateName, merged); err != nil {
			return err
		}
		fmt.Fprintf(a.stdout, "- refreshed %s aggregate (%d variables)\n", aggregateName, len(merged))
	}
	return nil
}

func exportableValues(item opItem, section string) map[string]fieldValue {
	values := map[string]fieldValue{}
	for _, f := range item.Fields {
		if !exportableField(f, section) {
			continue
		}
		values[f.Label] = fieldValue{Label: f.Label, Value: f.Value, Type: f.Type}
	}
	return values
}

func exportableField(f opField, section string) bool {
	if f.Label == "notesPlain" || f.Value == "" || !validEnvName(f.Label) {
		return false
	}
	if section == "" {
		return true
	}
	return sameSection(f, section) || emptySection(f)
}

func sameSection(f opField, section string) bool {
	if section == "" {
		return emptySection(f)
	}
	return f.Section != nil && f.Section.Label == section
}

func emptySection(f opField) bool {
	return f.Section == nil || f.Section.Label == ""
}

func writeCacheSet(cacheDir, name string, values map[string]fieldValue) error {
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}
	if err := writeAtomic(filepath.Join(cacheDir, name+".env"), []byte(renderPOSIX(values))); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(cacheDir, name+".fish"), []byte(renderFish(values))); err != nil {
		return err
	}
	if err := writeAtomic(filepath.Join(cacheDir, name+".ps1"), []byte(renderPowerShell(values))); err != nil {
		return err
	}
	return nil
}

func writeAtomic(path string, content []byte) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp cache: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp cache: %w", err)
	}
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp cache: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp cache: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("install cache: %w", err)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return fmt.Errorf("chmod cache: %w", err)
	}
	cleanup = false
	return nil
}

func renderPOSIX(values map[string]fieldValue) string {
	var b strings.Builder
	b.WriteString("# Generated by secrets-env. Do not edit. Contains secret values.\n")
	for _, key := range sortedKeys(values) {
		b.WriteString("export ")
		b.WriteString(key)
		b.WriteByte('=')
		b.WriteString(shellSingleQuote(values[key].Value))
		b.WriteByte('\n')
	}
	return b.String()
}

func renderFish(values map[string]fieldValue) string {
	var b strings.Builder
	b.WriteString("# Generated by secrets-env. Do not edit. Contains secret values.\n")
	for _, key := range sortedKeys(values) {
		b.WriteString("set -gx ")
		b.WriteString(key)
		b.WriteByte(' ')
		b.WriteString(fishSingleQuote(values[key].Value))
		b.WriteByte('\n')
	}
	return b.String()
}

func renderPowerShell(values map[string]fieldValue) string {
	var b strings.Builder
	b.WriteString("# Generated by secrets-env. Do not edit. Contains secret values.\n")
	for _, key := range sortedKeys(values) {
		b.WriteString("$env:")
		b.WriteString(key)
		b.WriteString(" = ")
		b.WriteString(powerShellSingleQuote(values[key].Value))
		b.WriteByte('\n')
	}
	return b.String()
}

func sortedKeys(values map[string]fieldValue) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func shellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}

func fishSingleQuote(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "'", "\\'")
	return "'" + s + "'"
}

func powerShellSingleQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func (a *app) status(cfg *Config) error {
	names := []string{aggregateName}
	for _, scope := range cfg.Scopes {
		names = append(names, scope.Name)
	}
	for _, name := range names {
		path := filepath.Join(cfg.CacheDir, name+".env")
		info, err := os.Stat(path)
		if err != nil {
			fmt.Fprintf(a.stdout, "- %s: missing\n", name)
			continue
		}
		mode := info.Mode().Perm()
		count := countCacheExports(path)
		fmt.Fprintf(a.stdout, "- %s: present mode=%04o vars=%d mtime=%s\n", name, mode, count, info.ModTime().Format(time.RFC3339))
	}
	return nil
}

func countCacheExports(path string) int {
	b, err := os.ReadFile(path)
	if err != nil {
		return 0
	}
	count := 0
	for _, line := range strings.Split(string(b), "\n") {
		if strings.HasPrefix(line, "export ") {
			count++
		}
	}
	return count
}

func (a *app) runCommand(cfg *Config, args []string) error {
	if len(args) > 0 && args[0] == "--" {
		args = args[1:]
	}
	if len(args) == 0 {
		return fmt.Errorf("run requires -- <command>")
	}
	cache := filepath.Join(cfg.CacheDir, aggregateName+".env")
	if _, err := os.Stat(cache); err != nil {
		return fmt.Errorf("aggregate cache missing; run `secrets-env refresh --all` first")
	}
	shArgs := append([]string{"-c", `. "$1" && shift && exec "$@"`, "secrets-env-run", cache}, args...)
	cmd := exec.Command("/bin/sh", shArgs...)
	cmd.Stdout = a.stdout
	cmd.Stderr = a.stderr
	cmd.Stdin = os.Stdin
	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return fmt.Errorf("command exited with status %d", exitErr.ExitCode())
		}
		return err
	}
	return nil
}

func (a *app) getItem(ctx context.Context, cfg *Config, account, vault, item string, reveal bool) (opItem, error) {
	args := []string{"item", "get", item, "--vault", vault, "--account", account, "--format", "json"}
	if reveal {
		args = append(args, "--reveal")
	}
	out, err := a.opOutput(ctx, cfg, args...)
	if err != nil {
		return opItem{}, err
	}
	var parsed opItem
	if err := json.Unmarshal(out, &parsed); err != nil {
		return opItem{}, fmt.Errorf("parse op item %q: %w", item, err)
	}
	return parsed, nil
}

func (a *app) findItemID(ctx context.Context, cfg *Config, account, vault, title string) (string, bool, error) {
	out, err := a.opOutput(ctx, cfg, "item", "list", "--vault", vault, "--account", account, "--format", "json")
	if err != nil {
		return "", false, err
	}
	var items []opListItem
	if err := json.Unmarshal(out, &items); err != nil {
		return "", false, fmt.Errorf("parse op item list for %s/%s: %w", account, vault, err)
	}
	for _, item := range items {
		if item.Title == title {
			return item.ID, true, nil
		}
	}
	return "", false, nil
}

func (a *app) op(ctx context.Context, cfg *Config, stdin []byte, args ...string) error {
	_, err := a.opOutputWithStdin(ctx, cfg, stdin, args...)
	return err
}

func (a *app) opOutput(ctx context.Context, cfg *Config, args ...string) ([]byte, error) {
	return a.opOutputWithStdin(ctx, cfg, nil, args...)
}

func (a *app) opOutputWithStdin(ctx context.Context, cfg *Config, stdin []byte, args ...string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, cfg.OPBin, args...)
	cmd.Env = scrubOPEnv(os.Environ())
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var out bytes.Buffer
	var errBuf bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("op %s timed out", strings.Join(args, " "))
		}
		msg := strings.TrimSpace(errBuf.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("op %s failed: %s", strings.Join(args, " "), msg)
	}
	return out.Bytes(), nil
}

func (c *Config) aggregateExcludeSet() map[string]bool {
	out := map[string]bool{}
	for _, name := range c.AggregateExclude {
		out[name] = true
	}
	return out
}

func scrubOPEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, entry := range env {
		key, _, _ := strings.Cut(entry, "=")
		switch key {
		case "OP_ACCOUNT", "OP_SERVICE_ACCOUNT_TOKEN":
			continue
		default:
			out = append(out, entry)
		}
	}
	return out
}

func validEnvName(name string) bool {
	return envNameRE.MatchString(name)
}

func fieldTypeForLabel(label string) string {
	upper := strings.ToUpper(label)
	sensitiveTokens := []string{"KEY", "TOKEN", "SECRET", "PASSWORD", "PASS", "PWD", "PSWD", "CREDENTIAL", "AUTH", "SID", "SERVICE_ACCOUNT"}
	for _, token := range sensitiveTokens {
		if strings.Contains(upper, token) {
			return "CONCEALED"
		}
	}
	return "STRING"
}

func hasFlag(args []string, flag string) bool {
	for _, arg := range args {
		if arg == flag {
			return true
		}
	}
	return false
}

func flagValue(args []string, flag string) string {
	for i, arg := range args {
		if arg == flag && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}
