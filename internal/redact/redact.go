package redact

import (
	"crypto/sha256"
	"encoding/hex"
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
	// Angola IBAN: AO + 2 check digits + 21-digit BBAN (25 chars). The shape
	// alone also matches any AO + 23 digits, so mod-97 is enforced in the
	// guard below. Runs before CARD/PHONE so spaced groups are not eaten
	// as cards or phone fragments.
	{"IBAN_AO", regexp.MustCompile(`(?i)\bAO\d{2}(?:[\s.\-]?\d{4}){5}[\s.\-]?\d\b`), "IBAN"},
	// Legacy singular NIF (pre-2019 AGT migration, still in old dumps):
	// 1 + 8 digits + 2 letters + 4 digits. Before BI_AO: the BI shape
	// would otherwise match its first 14 chars and leave a digit over.
	{"NIF_AO_LEGACY", regexp.MustCompile(`(?i)\b1\d{8}[A-Z]{2}\d{4}\b`), "NIF_AO"},
	// Bilhete de Identidade — also the singular NIF since the 2019 AGT
	// migration: 9 digits + 2 letters + 3 digits, ex 004281770LA042.
	{"BI_AO", regexp.MustCompile(`(?i)\b\d{9}\s?[A-Z]{2}\s?\d{3}\b`), "BI_AO"},
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
	// Collective-entity NIF (e.g. NIF: 5417001234): a bare 10-digit run would
	// be eaten by the US phone rule, so the labeled form is claimed first.
	// Singular NIFs need no rule — since 2019 they equal the BI number.
	{"NIF_AO_LABELED", regexp.MustCompile(`(?i)\b(nif|contribuinte)\s*[:=]?\s*["']?(?P<v>\d{9,12})\b`), "NIF_AO"},
	// Angola (AO) mobile 9XX XXX XXX + Luanda landline 222 XXX XXX, local or
	// +244/00244 international. Runs before the US rule: the spaced/dashed
	// shapes overlap otherwise. Leading (?:^|[^\d+]) keeps a global +244
	// attached (a bare \b cannot sit between a space and '+').
	{"PHONE_AO_INTL", regexp.MustCompile(`(?:^|[^\d+])(?P<v>(?:\+244|00244)[-.\s]?(?:9\d{2}|2\d{2})[-.\s]?\d{3}[-.\s]?\d{3})\b`), "PHONE_AO"},
	{"PHONE_AO_LOCAL", regexp.MustCompile(`(?:^|[^\d()])(?P<v>\(?(?:9\d{2}|222)\)?[-.\s]?\d{3}[-.\s]?\d{3})\b`), "PHONE_AO"},
	{"PHONE", regexp.MustCompile(`(?i)(?:^|[^\d+])(?P<v>(?:\+?1[-.\s]?)?\(?\d{3}\)?[-.\s]?\d{3}[-.\s]?\d{4})\b`), "PHONE"},
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

// Mode selects how a detected value is rendered, mirroring Presidio's
// anonymizer operators: replace ([TYPE_N]), mask (****0152), hash
// (salted sha256, stable across runs), presidio (<TYPE> tags for
// pipeline compat).
type Mode string

const (
	ModeReplace  Mode = "replace"
	ModeMask     Mode = "mask"
	ModeHash     Mode = "hash"
	ModePresidio Mode = "presidio"
)

// OpConfig tunes rendering. Entities maps a placeholder kind (e.g. PHONE_AO,
// IBAN, SECRET — see --list-rules) to a per-entity mode that overrides
// Default.
type OpConfig struct {
	Default  Mode
	Entities map[string]Mode
	// Mask: keep the last MaskKeep runes, mask the rest with MaskChar.
	MaskKeep int
	MaskChar string
	// Hash: hex(sha256(salt + value)) truncated to HashLen hex chars
	// (<=0 means full 64). Default salt is a documented constant so
	// output is reproducible; pass --salt for your own domain separation.
	Salt    string
	HashLen int
}

// DefaultConfig is replace mode — the historical behavior.
func DefaultConfig() OpConfig {
	return OpConfig{
		Default:  ModeReplace,
		Entities: map[string]Mode{},
		MaskKeep: 4,
		MaskChar: "*",
		Salt:     "safe-past-v1",
		HashLen:  16,
	}
}

// ParseMode validates a --mode / --entity-mode value.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case ModeReplace, ModeMask, ModeHash, ModePresidio:
		return Mode(s), nil
	default:
		return "", fmt.Errorf("unknown mode %q (want replace|mask|hash|presidio)", s)
	}
}

func (c OpConfig) modeFor(kind string) Mode {
	if m, ok := c.Entities[kind]; ok {
		return m
	}
	return c.Default
}

// Clean redacts text with default (replace) operators.
func Clean(text string, extraPatterns []string) Result {
	return CleanWithConfig(text, extraPatterns, DefaultConfig())
}

// CleanWithConfig redacts text, rendering each finding per OpConfig.
// In replace mode the same value always maps to the same [TYPE_N] token,
// which keeps logs correlatable after redaction. Mask and hash modes are
// deterministic by construction, so they need no mapping table.
func CleanWithConfig(text string, extraPatterns []string, cfg OpConfig) Result {
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
		mode := cfg.modeFor(r.Placeholder)
		// Special-case ASSIGNED_SECRET / BEARER: only transform the value
		// group, keep the key/scheme name so the log stays readable.
		if vi := r.Pattern.SubexpIndex("v"); vi >= 0 {
			out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
				loc := r.Pattern.FindStringSubmatchIndex(m)
				if loc == nil || len(loc) <= 2*vi+1 || loc[2*vi] < 0 {
					return m
				}
				val := m[loc[2*vi]:loc[2*vi+1]]
				ph := render(cfg, mode, mapping, counters, counts, r.Placeholder, val)
				return m[:loc[2*vi]] + ph + m[loc[2*vi+1]:]
			})
			continue
		}
		// Special-case BASIC_AUTH_URL: keep scheme, transform user:pass
		// (capture groups 2+3 span loc[4]:loc[7]).
		if r.Name == "BASIC_AUTH_URL" {
			out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
				loc := r.Pattern.FindStringSubmatchIndex(m)
				if loc == nil || len(loc) < 8 || loc[4] < 0 || loc[7] < 0 {
					return m
				}
				creds := m[loc[4]:loc[7]]
				ph := render(cfg, mode, mapping, counters, counts, r.Placeholder, creds)
				return m[:loc[4]] + ph + m[loc[7]:]
			})
			continue
		}
		out = r.Pattern.ReplaceAllStringFunc(out, func(m string) string {
			// Guard against obvious false positives:
			if r.Name == "IBAN_AO" && !validIBAN(m) {
				return m
			}
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
			return render(cfg, mode, mapping, counters, counts, r.Placeholder, m)
		})
	}

	replaces := 0
	for _, c := range counts {
		replaces += c
	}
	return Result{Text: out, Replaces: replaces, Counts: counts, Mapping: mapping}
}

// render produces the replacement text for one detected value.
func render(cfg OpConfig, mode Mode, mapping map[string]string, counters map[string]int, counts map[string]int, kind, original string) string {
	counts[kind]++
	switch mode {
	case ModeMask:
		return maskValue(original, cfg.MaskKeep, cfg.MaskChar)
	case ModeHash:
		return hashValue(original, cfg.Salt, cfg.HashLen)
	case ModePresidio:
		return "<" + kind + ">"
	default: // ModeReplace
		if ph, ok := mapping[original]; ok {
			return ph
		}
		counters[kind]++
		ph := fmt.Sprintf("[%s_%d]", kind, counters[kind])
		mapping[original] = ph
		return ph
	}
}

// maskValue keeps the last keep runes, masking the rest (Presidio mask
// operator semantics, from_end variant).
func maskValue(s string, keep int, char string) string {
	if char == "" {
		char = "*"
	}
	mc := []rune(char)[0]
	runes := []rune(s)
	if keep < 0 {
		keep = 0
	}
	if len(runes) <= keep {
		return strings.Repeat(string(mc), len(runes))
	}
	return strings.Repeat(string(mc), len(runes)-keep) + string(runes[len(runes)-keep:])
}

// hashValue is sha256(salt + value) hex, truncated to hashLen chars
// (<=0 means full 64). Deterministic: same value + salt always agree,
// so findings stay correlatable across runs and pastes.
func hashValue(s, salt string, hashLen int) string {
	sum := sha256.Sum256([]byte(salt + "\x00" + s))
	hexed := hex.EncodeToString(sum[:])
	if hashLen > 0 && hashLen < len(hexed) {
		return hexed[:hashLen]
	}
	return hexed
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

// validIBAN enforces AO structure (25 chars: AO + 2 check + 21 BBAN) plus
// the ISO 13616 mod-97 checksum, so random AO + 23-digit runs don't redact.
func validIBAN(m string) bool {
	var compact strings.Builder
	compact.Grow(25)
	for i := 0; i < len(m); i++ {
		c := m[i]
		switch {
		case c >= '0' && c <= '9':
			compact.WriteByte(c)
		case c >= 'A' && c <= 'Z':
			compact.WriteByte(c)
		case c >= 'a' && c <= 'z':
			compact.WriteByte(c - 32)
		case c == ' ' || c == '.' || c == '-' || c == '\t':
			// display separator, skip
		default:
			return false
		}
	}
	s := compact.String()
	if len(s) != 25 || s[0] != 'A' || s[1] != 'O' {
		return false
	}
	for i := 2; i < 25; i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	rearr := s[4:] + s[:4]
	rem := 0
	for i := 0; i < len(rearr); i++ {
		c := rearr[i]
		if c >= '0' && c <= '9' {
			rem = (rem*10 + int(c-'0')) % 97
		} else {
			rem = (rem*100 + int(c-55)) % 97 // A=10 … Z=35
		}
	}
	return rem == 1
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
