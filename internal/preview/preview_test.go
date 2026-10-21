package preview

import (
	"strings"
	"testing"
)

func TestDiffLines(t *testing.T) {
	lines := DiffLines("a\nb", "a\n[X]")
	if len(lines) != 2 || lines[0].Redacted || !lines[1].Redacted {
		t.Fatalf("bad diff: %+v", lines)
	}
}

func TestStatsLine(t *testing.T) {
	s := StatsLine(3, map[string]int{"IP": 1, "EMAIL": 2})
	if !strings.Contains(s, "scrubbed 3") || !strings.Contains(s, "EMAIL=2") {
		t.Fatalf("bad stats: %q", s)
	}
	if got := StatsLine(0, nil); !strings.Contains(got, "clean") {
		t.Fatalf("bad empty stats: %q", got)
	}
}
