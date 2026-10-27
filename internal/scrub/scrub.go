// Package scrub bridges daemon config to the redact engine: one mapping,
// no CLI flag parsing, shared with the future GUI Settings save path.
package scrub

import (
	"strings"

	"github.com/idarciooliveira/safe-past/internal/config"
	"github.com/idarciooliveira/safe-past/internal/redact"
)

// OpConfig maps daemon settings onto engine operators.
func OpConfig(c config.Config) (redact.OpConfig, error) {
	mode := c.Mode
	if strings.TrimSpace(mode) == "" {
		mode = "replace"
	}
	m, err := redact.ParseMode(strings.ToLower(strings.TrimSpace(mode)))
	if err != nil {
		return redact.OpConfig{}, err
	}
	cfg := redact.DefaultConfig()
	cfg.Default = m
	if c.MaskKeep > 0 {
		cfg.MaskKeep = c.MaskKeep
	}
	if strings.TrimSpace(c.MaskChar) != "" {
		cfg.MaskChar = c.MaskChar
	}
	for _, d := range c.Disabled {
		if d = strings.ToUpper(strings.TrimSpace(d)); d != "" {
			cfg.Disabled[d] = true
		}
	}
	return cfg, nil
}

// Result is the scrubbed text plus a one-line human summary.
type Result struct {
	Text    string
	Summary string // "scrubbed 3 value(s): EMAIL=1 IP=2" or "...: clean"
	Count   int
}

// ScrubText redacts input per c. Empty input returns an empty result.
func ScrubText(input string, c config.Config) (Result, error) {
	if strings.TrimSpace(input) == "" {
		return Result{}, nil
	}
	cfg, err := OpConfig(c)
	if err != nil {
		return Result{}, err
	}
	var extras []string
	for _, e := range c.Extra {
		if e = strings.TrimSpace(e); e != "" {
			extras = append(extras, e)
		}
	}
	res := redact.CleanWithConfig(input, extras, cfg)
	summary := "scrubbed 0 value(s): clean"
	if res.Replaces > 0 {
		summary = "scrubbed " + itoa(res.Replaces) + " value(s): " +
			strings.Join(redact.SortedCounts(res.Counts), " ")
	}
	return Result{Text: res.Text, Summary: summary, Count: res.Replaces}, nil
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}
