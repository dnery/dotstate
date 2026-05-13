// Package redact centralizes output sanitization for reports, logs, errors, and
// module records. It is deliberately conservative: values that look like
// credentials are replaced before they cross a display or serialization
// boundary, while intentional references such as op:// URLs remain intact and
// are tagged as credential references by callers that track sensitivity.
package redact

import (
	"fmt"
	"reflect"
	"regexp"
	"strings"
)

// Sensitivity is an ordered view of the data classes used by callers. The
// numeric order is only for monotonic promotion while redacting nested values.
type Sensitivity int

const (
	SensitivityPublic Sensitivity = iota
	SensitivityLocalPath
	SensitivityPersonal
	SensitivityCredentialReference
	SensitivitySecret
	SensitivityRestricted
)

// Report describes what a redaction pass found.
type Report struct {
	Redactions  int
	Sensitivity Sensitivity
}

// Merge combines two reports, preserving the highest sensitivity.
func (r Report) Merge(other Report) Report {
	r.Redactions += other.Redactions
	if other.Sensitivity > r.Sensitivity {
		r.Sensitivity = other.Sensitivity
	}
	return r
}

type pattern struct {
	re          *regexp.Regexp
	marker      string
	sensitivity Sensitivity
}

var (
	credentialedURL = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)([^\s/@]+@)`) // e.g. https://token@host
	opReference     = regexp.MustCompile(`\bop://[^\s"'<>]+`)

	secretPatterns = []pattern{
		{regexp.MustCompile(`DOTSTATE_TEST_SECRET_DO_NOT_PRINT`), `<redacted:secret>`, SensitivitySecret},
		{regexp.MustCompile(`-----BEGIN (RSA |OPENSSH |DSA |EC |PGP )?PRIVATE KEY( BLOCK)?-----`), `<redacted:private-key>`, SensitivitySecret},
		{regexp.MustCompile(`AGE-SECRET-KEY-[A-Z0-9]{20,}`), `<redacted:private-key>`, SensitivitySecret},
		{regexp.MustCompile(`gh[pousr]_[A-Za-z0-9_]{30,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`glpat-[A-Za-z0-9\-_]{20,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`xox[baprs]-[0-9]{8,13}-[0-9]{8,13}[A-Za-z0-9-]*`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`sk_live_[A-Za-z0-9]{16,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`rk_live_[A-Za-z0-9]{16,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`SG\.[A-Za-z0-9_-]{16,}\.[A-Za-z0-9_-]{24,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`npm_[A-Za-z0-9]{24,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`pypi-AgEIcHlwaS5vcmc[A-Za-z0-9_-]{30,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`ops_[A-Za-z0-9_-]{30,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`AKIA[0-9A-Z]{16}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`AIza[0-9A-Za-z_-]{20,}`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`eyJ[A-Za-z0-9_-]*\.eyJ[A-Za-z0-9_-]*\.[A-Za-z0-9_-]*`), `<redacted:token>`, SensitivitySecret},
		{regexp.MustCompile(`(?i)(postgres(?:ql)?|mysql|mongodb(?:\+srv)?|redis)://[^\s:]+:[^\s@]+@[^\s]+`), `<redacted:credential-uri>`, SensitivitySecret},
		{regexp.MustCompile(`(?i)\b(password|passwd|pwd|secret|api[_-]?token|token|auth[_-]?(key|token|secret)?|credential[_-]?(key|token|secret)?)\s*[:=]\s*['"]?[^\s'"#]{8,}['"]?`), `<redacted:secret>`, SensitivitySecret},
	}

	restrictedHints = []string{
		"TCC.db",
		"/Library/Application Support/com.apple.TCC/",
		"/Library/Keychains/",
		".keychain-db",
		"Keychain",
	}
)

// String returns a sanitized copy of s and a report of any sensitivity found.
func String(s string) (string, Report) {
	out := s
	report := Report{Sensitivity: SensitivityPublic}

	out = credentialedURL.ReplaceAllStringFunc(out, func(match string) string {
		report.Redactions++
		if SensitivitySecret > report.Sensitivity {
			report.Sensitivity = SensitivitySecret
		}
		parts := credentialedURL.FindStringSubmatch(match)
		if len(parts) >= 2 {
			return parts[1] + "<redacted:credential>@"
		}
		return "<redacted:credential>"
	})

	for _, p := range secretPatterns {
		out = p.re.ReplaceAllStringFunc(out, func(string) string {
			report.Redactions++
			if p.sensitivity > report.Sensitivity {
				report.Sensitivity = p.sensitivity
			}
			return p.marker
		})
	}

	if report.Sensitivity == SensitivityPublic && opReference.MatchString(out) {
		report.Sensitivity = SensitivityCredentialReference
	}
	if report.Sensitivity < SensitivityRestricted {
		for _, hint := range restrictedHints {
			if strings.Contains(out, hint) {
				report.Sensitivity = SensitivityRestricted
				break
			}
		}
	}

	return out, report
}

// Text returns just the sanitized text for call sites that do not need the
// sensitivity report.
func Text(s string) string {
	out, _ := String(s)
	return out
}

// Value recursively sanitizes strings inside common JSON-like values.
func Value(v any) (any, Report) {
	return value(reflect.ValueOf(v))
}

func value(rv reflect.Value) (any, Report) {
	if !rv.IsValid() {
		return nil, Report{Sensitivity: SensitivityPublic}
	}
	if rv.Kind() == reflect.Interface || rv.Kind() == reflect.Pointer {
		if rv.IsNil() {
			return nil, Report{Sensitivity: SensitivityPublic}
		}
		return value(rv.Elem())
	}

	switch rv.Kind() {
	case reflect.String:
		return String(rv.String())
	case reflect.Bool:
		return rv.Bool(), Report{Sensitivity: SensitivityPublic}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return rv.Int(), Report{Sensitivity: SensitivityPublic}
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
		return rv.Uint(), Report{Sensitivity: SensitivityPublic}
	case reflect.Float32, reflect.Float64:
		return rv.Float(), Report{Sensitivity: SensitivityPublic}
	case reflect.Map:
		out := make(map[string]any, rv.Len())
		report := Report{Sensitivity: SensitivityPublic}
		iter := rv.MapRange()
		for iter.Next() {
			key, keyReport := String(fmt.Sprint(iter.Key().Interface()))
			val, valReport := value(iter.Value())
			report = report.Merge(keyReport).Merge(valReport)
			out[key] = val
		}
		return out, report
	case reflect.Slice, reflect.Array:
		out := make([]any, rv.Len())
		report := Report{Sensitivity: SensitivityPublic}
		for i := 0; i < rv.Len(); i++ {
			val, valReport := value(rv.Index(i))
			out[i] = val
			report = report.Merge(valReport)
		}
		return out, report
	case reflect.Struct:
		// Keep structs as-is. Module schema structs are sanitized explicitly so
		// callers do not accidentally lose static type information.
		return rv.Interface(), Report{Sensitivity: SensitivityPublic}
	default:
		return rv.Interface(), Report{Sensitivity: SensitivityPublic}
	}
}
