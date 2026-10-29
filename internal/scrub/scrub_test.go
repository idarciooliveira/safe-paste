package scrub

import (
	"strings"
	"testing"

	"github.com/idarciooliveira/safe-past/internal/config"
)

func TestScrubTextReplace(t *testing.T) {
	c := config.Default()
	r, err := ScrubText("host 192.168.1.10 user bob@example.com", c)
	if err != nil {
		t.Fatal(err)
	}
	if r.Count != 2 || strings.Contains(r.Text, "192.168.1.10") {
		t.Fatalf("not scrubbed: %+v", r)
	}
	if !strings.Contains(r.Summary, "scrubbed 2 value(s)") {
		t.Fatalf("bad summary: %q", r.Summary)
	}
}

func TestScrubTextEmpty(t *testing.T) {
	r, err := ScrubText("   ", config.Default())
	if err != nil || r.Text != "" || r.Count != 0 {
		t.Fatalf("got %+v, %v", r, err)
	}
}

func TestScrubTextDisabled(t *testing.T) {
	c := config.Default()
	c.Disabled = []string{"EMAIL"}
	r, err := ScrubText("bob@example.com 10.0.0.1", c)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Text, "bob@example.com") || strings.Contains(r.Text, "10.0.0.1") {
		t.Fatalf("disabled not honored: %q", r.Text)
	}
}

func TestScrubTextBadMode(t *testing.T) {
	c := config.Default()
	c.Mode = "nope"
	if _, err := ScrubText("x", c); err == nil {
		t.Fatal("expected error")
	}
}
