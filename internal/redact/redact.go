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

func defaultRules() []Rule {
	return []Rule{
		// Order matters: most specific / longest secrets first.
		{"PRIVATE_KEY", regexp.MustCompile(`(?s)-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`), "PRIVATE_KEY"},
		{"AWS_ACCESS_KEY", regexp.MustCompile(`\b(AKIA|ASIA|ABIA|ACCA)[0-9A-Z]{16}\b`), "AWS_KEY"},
		{"GITHUB_TOKEN", regexp.MustCompile(`\b(ghp_[A-Za-z0-9]{20,}|gho_[A-Za-z0-9]{20,}|github_pat_[A-Za-z0-9_]{20,})\b`), "GITHUB_TOKEN"},
		{"SLACK_TOKEN", regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}\b`), "SLACK_TOKEN"},
		{"STRIPE_KEY", regexp.MustCompile(`\b(sk|pk)_(live|test)_[A-Za-z0-9]{10,}\b`), "STRIPE_KEY"},
		{"JWT", regexp.MustCompile(`\beyJ[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\.[A-Za-z0-9_-]{10,}\b`), "JWT"},
		{"BEARER", regexp.MustCompile(`(?i)\bearer\s+[A-Za-z0-9\-._~+/]{20,}={0,2}\b`), "BEARER_TOKEN"},
		{"BASIC_AUTH_URL", regexp.MustCompile(`([a-zA-Z][a-zA-Z0-9+.-]*://)([^/\s:@]+):([^/\s@]+)@`), "URL_CREDS"},
		{"EMAIL", regexp.MustCompile(`\b[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}\b`), "EMAIL"},
		{"IPV4", regexp.MustCompile(`\b(?:(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\.){3}(?:25[0-5]|2[0-4][0-9]|[01]?[0-9][0-9]?)\b`), "IP"},
		{"IPV6", regexp.MustCompile(`\b(?:[0-9a-fA-F]{0,4}:){2,7}[0-9a-fA-F]{0,4}\b`), "IPV6"},
		{"MAC", regexp.MustCompile(`\b(?:[0-9A-Fa-f]{2}[:-]){5}[0-9A-Fa-f]{2}\b`), "MAC"},
		{"PHONE", regexp.MustCompile(`(?i)\b(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4}\b`), "PHONE"},
		{"CREDIT_CARD", regexp.MustCompile(`\b(?:\d[ \-]?){13,19}\b`), "CARD"},
		{"SSN", regexp.MustCompile(`\b\d{3}-\d{2}-\d{4}\b`), "SSN"},
		// password=secret, pwd: secret, api_key: xxx (captures the secret part)
		{"ASSIGNED_SECRET", regexp.MustCompile(`(?i)(password|passwd|pwd|secret|api[_-]?key|auth[_-]?token)\s*[:=]\s*([^\s,;'"` + "`" + `}]+)`), "SECRET"},
	}
}

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
		// Special-case ASSIGNED_SECRET: only redact the value group, keep the key name.
		if r.Name == "ASSIGNED_SECRET" {
			out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
				loc := r.Pattern.FindStringSubmatchIndex(m)
				if loc == nil || len(loc) < 6 || loc[4] < 0 {
					return m
				}
				val := m[loc[4]:loc[5]]
				ph := placeholderFor(mapping, counters, counts, r.Placeholder, val)
				return m[:loc[4]] + ph + m[loc[5]:]
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
			if r.Name == "IPV6" && !strings.Contains(m, ":") {
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
