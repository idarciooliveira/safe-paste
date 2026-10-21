package preview

import (
	"fmt"
	"strings"
)

// Line is one row of the before/after diff.
type Line struct {
	Orig     string
	Clean    string
	Redacted bool
}

// DiffLines splits orig/clean by line. Lines differ -> Redacted.
func DiffLines(orig, clean string) []Line {
	o := strings.Split(orig, "\n")
	c := strings.Split(clean, "\n")
	n := len(o)
	if len(c) > n {
		n = len(c)
	}
	out := make([]Line, 0, n)
	for i := 0; i < n; i++ {
		var ol, cl string
		if i < len(o) {
			ol = o[i]
		}
		if i < len(c) {
			cl = c[i]
		}
		out = append(out, Line{Orig: ol, Clean: cl, Redacted: ol != cl})
	}
	return out
}

// StatsLine renders "scrubbed N value(s): EMAIL=2 IP=1".
func StatsLine(replaces int, counts map[string]int) string {
	if len(counts) == 0 {
		return fmt.Sprintf("scrubbed %d value(s): clean", replaces)
	}
	keys := make([]string, 0, len(counts))
	for k := range counts {
		keys = append(keys, k)
	}
	// insertion sort to avoid importing sort for one call site
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	parts := make([]string, 0, len(keys))
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s=%d", k, counts[k]))
	}
	return fmt.Sprintf("scrubbed %d value(s): %s", replaces, strings.Join(parts, " "))
}
