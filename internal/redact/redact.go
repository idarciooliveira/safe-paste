package redact

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// Rule is a named detection pattern.
type Rule struct {
	Name        string
	Pattern     *regexp.Regexp
	Placeholder string // e.g. "IP", "EMAIL"
}

// Result holds the cleaned text plus stats.
type Result struct {
	Text     string
	Replaces int
	Counts   map[string]int
	// Mapping maps original value -> placeholder (per-run, consistent).
	// Only kept in memory so the same IP always becomes the same token,
	// which keeps logs correlatable after redaction.
	Mapping map[string]string
}

// Precompiled once at startup: Clean() is called per paste / per loadtest
// iteration, and recompiling 16+ patterns per call dominated profiles
// (baseline: 1202 allocs for a 4KB paste, most of them compiles).
var builtinRules = []Rule{
	// Order matters: most specific / longest secrets first.
	// MAC before IPV6: a MAC (AA:BB:CC:DD:EE:FF) also matches the loose
	// IPv6 shape, so it must be claimed first to get the right label.
	{"PRIVATE_KEY", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "PRIVATE_KEY"},
	{"AWS_ACCESS_KEY", regexp.MustCompile(`\b(AKIA|ASIA|ABIA|ACCA)[0-9A-Z]{16}\b`), "AWS_KEY"},
	{"GITHUB_TOKEN", regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`), "GITHUB_TOKEN"},
	{"SLACK_TOKEN", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), "SLACK_TOKEN"},
	{"STRIPE_KEY", regexp.MustCompile(`\b(sk|pk)_(live|test)_[A-Za-z0-9]{10,}\b`), "STRIPE_KEY"},
	{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}={0,2}`), "JWT"},
	{"BEARER", regexp.MustCompile(`(?i)\bBearer\s+(?P<v>[A-Za-z0-9\-._~+/]{20,}={0,2})`), "BEARER_TOKEN"},
	{"BASIC_AUTH_URL", regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s:@]+):([^/\s@]+)@`), "URL_CREDS"},
	{"EMAIL", regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`), "EMAIL"},
	{"IPV4", regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`), "IP"},
	{"MAC", regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`), "MAC"},
	{"IPV6", regexp.MustCompile(`\b(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}\b`), "IPV6"},
	{"PHONE", regexp.MustCompile(`(?i)\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`), "PHONE"},
	// Ends with \d so the match never swallows a trailing space.
	{"CREDIT_CARD", regexp.MustCompile(`\b(?:\d[ \-]?){12,18}\d\b`), "CARD"},
	{"SSN", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), "SSN"},
	// password=secret, pwd: secret, api_key: xxx — key and value may each be
	// single/double quoted (JSON, .env). Value stops at whitespace, quotes,
	// commas, braces and & (so ?a=1&api_key=K&b=2 keeps b=2 intact).
	{"ASSIGNED_SECRET", regexp.MustCompile(`(?i)["']?(password|passwd|pwd|secret|api[_-]?key|auth[_-]?token)["']?\s*[:=]\s*["']?(?P<v>[^\s,;&'"` + "`" + `}]+)`), "SECRET"},
}

func defaultRules() []Rule {
	out := make([]Rule, len(builtinRules))
	copy(out, builtinRules)
	return out
}

// Rules exposes the built-in detection list (for --list-rules).
func Rules() []Rule { return defaultRules() }

// Clean redacts text, keeping a consistent placeholder per unique value.
func Clean(text string, extraPatterns []string) Result {
	rules := defaultRules()
	for i, p := range extraPatterns {
		if re, err := regexp.Compile(p); err == nil {
			rules = append(rules, Rule{
				Name:        fmt.Sprintf("CUSTOM_%d", i+1),
				Pattern:     re,
				Placeholder: fmt.Sprintf("CUSTOM_%d", i+1),
			})
		}
	}

	counts := map[string]int{}
	mapping := map[string]string{}
	counters := map[string]int{}
	out := text

	// Collect all matches first so overlapping rules resolve deterministically:
	// earlier rules (more specific) win. We apply sequentially, which is simple
	// and predictable for logs.
	for _, r := range rules {
		// Special-case ASSIGNED_SECRET / BEARER: only redact the value group,
		// keep the key/scheme name so the log stays readable.
		if vi := r.Pattern.SubexpIndex("v"); vi >= 0 {
			out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
				loc := r.Pattern.FindStringSubmatchIndex(m)
				if loc == nil || len(loc) <= 2*vi+1 || loc[2*vi] < 0 {
					return m
				}
				val := m[loc[2*vi]:loc[2*vi+1]]
				ph := placeholderFor(mapping, counters, counts, r.Placeholder, val)
				return m[:loc[2*vi]] + ph + m[loc[2*vi+1]:]
			})
			continue
		}
		// Special-case BASIC_AUTH_URL: keep scheme, redact user:pass.
		if r.Name == "BASIC_AUTH_URL" {
			n := len(r.Pattern.FindAllString(out, -1))
			if n > 0 {
				out = r.Pattern.ReplaceAllString(out, `$1[URL_CREDS]@`)
				counts["URL_CREDS"] += n
			}
			continue
		}
		out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
			// Guard against obvious false positives:
			if r.Name == "IPV6" && !looksLikeIPV6(m) {
				return m
			}
			if r.Name == "CREDIT_CARD" {
				digits := strings.Map(func(c rune) rune {
					if c >= '0' && c <= '9' {
						return c
					}
					return -1
				}, m)
				if len(digits) < 13 || len(digits) > 19 || !luhnLike(digits) {
					return m
				}
			}
			return placeholderFor(mapping, counters, counts, r.Placeholder, m)
		})
	}

	replaces := 0
	for _, c := range counts {
		replaces += c
	}
	return Result{Text: out, Replaces: replaces, Counts: counts, Mapping: mapping}
}

func placeholderFor(mapping map[string]string, counters map[string]int, counts map[string]int, kind, original string) string {
	if ph, ok := mapping[original]; ok {
		counts[kind]++
		return ph
	}
	counters[kind]++
	ph := fmt.Sprintf("[%s_%d]", kind, counters[kind])
	mapping[original] = ph
	counts[kind]++
	return ph
}

// looksLikeIPV6 rejects clock times and fragments the loose pattern matches:
// 12:34:56 or the :00:00 inside an ISO timestamp are not addresses. A real
// match must contain "::", a hex letter, or at least 3 colons (an all-digit
// full address like 1:2:3:4:5:6:7:8 has 7).
func looksLikeIPV6(m string) bool {
	if strings.Contains(m, "::") {
		return true
	}
	for i := 0; i < len(m); i++ {
		if (m[i] >= 'a' && m[i] <= 'f') || (m[i] >= 'A' && m[i] <= 'F') {
			return true
		}
	}
	return strings.Count(m, ":") >= 3
}

func luhnLike(digits string) bool {
	// Lightweight Luhn check to avoid redacting every 13-19 digit run (e.g. timestamps/IDs).
	sum := 0
	alt := false
	for i := len(digits) - 1; i >= 0; i-- {
		d := int(digits[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}

// SortedCounts returns counts sorted by key for stable CLI output.
func SortedCounts(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		out = append(out, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return out
}
