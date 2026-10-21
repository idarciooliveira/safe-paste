package redact

import (
	"strings"
	"testing"
)

func TestCleanBasics(t *testing.T) {
	in := "error from 192.168.1.10 user bob@example.com token ghp_abcdefghijklmnopqrstuv phone +1-555-123-4567"
	res := Clean(in, nil)
	if strings.Contains(res.Text, "192.168.1.10") || strings.Contains(res.Text, "bob@example.com") {
		t.Fatalf("PII not redacted: %q", res.Text)
	}
	if !strings.Contains(res.Text, "[IP_1]") || !strings.Contains(res.Text, "[EMAIL_1]") {
		t.Fatalf("expected placeholders, got: %q", res.Text)
	}
	// Consistency: same IP twice -> same placeholder
	in2 := "a 10.0.0.1 b 10.0.0.1 c 10.0.0.2"
	res2 := Clean(in2, nil)
	if strings.Count(res2.Text, "[IP_1]") != 2 || !strings.Contains(res2.Text, "[IP_2]") {
		t.Fatalf("inconsistent mapping: %q", res2.Text)
	}
}

func TestSecrets(t *testing.T) {
	in := "key AKIAIOSFODNN7EXAMPLE Bearer abcdefghijklmnopqrstuvwx password=hunter2 url https://admin:s3cret@host/x"
	res := Clean(in, nil)
	for _, leak := range []string{"AKIAIOSFODNN7EXAMPLE", "hunter2", "s3cret@host"} {
		if strings.Contains(res.Text, leak) {
			t.Fatalf("leaked %q in %q", leak, res.Text)
		}
	}
}
