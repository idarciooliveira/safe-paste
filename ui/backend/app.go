// Package backend is the Wails-bound bridge over the redact engine.
// It imports no Wails packages so it stays testable headless;
// ui/main.go handles window wiring.
package backend

import (
	"fmt"
	"sort"
	"strings"

	"github.com/atotto/clipboard"
	"github.com/idarciooliveira/safe-past/internal/redact"
)

// App is bound into the Wails frontend as window.go.backend.App.
type App struct{}

// New returns a bound App.
func New() *App { return &App{} }

// RuleInfo describes one detector for the UI toggle list.
type RuleInfo struct {
	Name        string `json:"name"`
	Placeholder string `json:"placeholder"`
}

// ListRules returns built-in detectors in stable order.
func (a *App) ListRules() []RuleInfo {
	rules := redact.Rules()
	out := make([]RuleInfo, 0, len(rules))
	for _, r := range rules {
		out = append(out, RuleInfo{Name: r.Name, Placeholder: r.Placeholder})
	}
	return out
}

// RedactRequest mirrors the UI state: text + operator options.
type RedactRequest struct {
	Text     string   `json:"text"`
	Mode     string   `json:"mode"`
	Disabled []string `json:"disabled"`
	MaskKeep int      `json:"maskKeep"`
	MaskChar string   `json:"maskChar"`
	Extra    []string `json:"extra"`
}

// RedactResponse is the de-identified result plus stats.
type RedactResponse struct {
	Text     string         `json:"text"`
	Replaces int            `json:"replaces"`
	Counts   map[string]int `json:"counts"`
	Stats    string         `json:"stats"`
}

// Redact scrubs req.Text per req options. Empty mode means replace.
func (a *App) Redact(req RedactRequest) (RedactResponse, error) {
	mode := redact.ModeReplace
	if strings.TrimSpace(req.Mode) != "" {
		m, err := redact.ParseMode(req.Mode)
		if err != nil {
			return RedactResponse{}, err
		}
		mode = m
	}
	cfg := redact.DefaultConfig()
	cfg.Default = mode
	if req.MaskKeep > 0 {
		cfg.MaskKeep = req.MaskKeep
	}
	if req.MaskChar != "" {
		cfg.MaskChar = req.MaskChar
	}
	for _, d := range req.Disabled {
		if d = strings.TrimSpace(strings.ToUpper(d)); d != "" {
			cfg.Disabled[d] = true
		}
	}
	res := redact.CleanWithConfig(req.Text, req.Extra, cfg)
	stats := fmt.Sprintf("scrubbed %d value(s): %s", res.Replaces, strings.Join(redact.SortedCounts(res.Counts), " "))
	if res.Replaces == 0 {
		stats = "scrubbed 0 value(s): clean"
	}
	return RedactResponse{Text: res.Text, Replaces: res.Replaces, Counts: res.Counts, Stats: stats}, nil
}

// SortedRuleNames validates toggle names against built-ins.
func SortedRuleNames() []string {
	seen := map[string]bool{}
	for _, r := range redact.Rules() {
		seen[r.Placeholder] = true
	}
	out := make([]string, 0, len(seen))
	for k := range seen {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// ReadClipboard returns the current clipboard text.
func (a *App) ReadClipboard() (string, error) {
	return clipboard.ReadAll()
}

// WriteClipboard replaces the clipboard with the clean text.
func (a *App) WriteClipboard(s string) error {
	return clipboard.WriteAll(s)
}
