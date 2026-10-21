package redact

import (
	"strings"
	"testing"
)

// realisticLine mirrors a prod log paste: nginx/k8s/app fields, tokens,
// URLs and timestamps in one line — the shape the loadtest corpus uses.
var realisticLine = `2024-01-15T10:00:01Z ERROR deploy/api-7d9f8b6c4-x2k9j user bob@example.com src=192.168.1.10 password=hunter2 token ghp_abcdefghijklmnopqrstuv key AKIAIOSFODNN7EXAMPLE GET https://admin:s3cret@internal:8443/v1/orders 200 "jwt eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c" card 4111-1111-1111-1111 phone +1-555-123-4567 ssn 123-45-6789 mac AA:BB:CC:DD:EE:FF`

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
	for _, leak := range []string{"AKIAIOSFODNN7EXAMPLE", "abcdefghijklmnopqrstuvwx", "hunter2", "s3cret@host"} {
		if strings.Contains(res.Text, leak) {
			t.Fatalf("leaked %q in %q", leak, res.Text)
		}
	}
	if !strings.Contains(res.Text, "Bearer [BEARER_TOKEN_1]") {
		t.Fatalf("bearer scheme should be kept, got: %q", res.Text)
	}
}

// Secrets stay quoted in their original framing (JSON, .env, flags).
type stringpair struct {
	in, leak string
}

func TestQuotedSecrets(t *testing.T) {
	cases := map[string]stringpair{
		"json double":  {in: `{"user":"bob","password": "hunter2"}`, leak: "hunter2"},
		"json api_key": {in: `{"api_key": "sk_test_123"}`, leak: "sk_test_123"},
		"json single":  {in: `{'passwd': 's3cret!'}`, leak: "s3cret!"},
		"env double":   {in: `PASSWORD="hunter2"`, leak: "hunter2"},
		"env single":   {in: `API_KEY='abc123'`, leak: "abc123"},
		"unquoted":     {in: `password=hunter2`, leak: "hunter2"},
		"colon spaced": {in: `api_key: MYKEY123`, leak: "MYKEY123"},
	}
	for name, c := range cases {
		res := Clean(c.in, nil)
		if strings.Contains(res.Text, c.leak) {
			t.Errorf("%s: leaked %q in %q", name, c.leak, res.Text)
		}
		if !strings.Contains(res.Text, "[SECRET_") {
			t.Errorf("%s: missing [SECRET_N] placeholder in %q", name, res.Text)
		}
	}
}

func TestTimestampsPreserved(t *testing.T) {
	in := "at 12:34:56 duration 2024-01-15T10:00:00Z trace 9f2c7a1e"
	res := Clean(in, nil)
	if res.Text != in {
		t.Fatalf("timestamp corrupted: %q", res.Text)
	}
	for _, ip6 := range []string{"fe80::1", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "1:2:3:4:5:6:7:8"} {
		res := Clean("from "+ip6+" ok", nil)
		if strings.Contains(res.Text, ip6) {
			t.Errorf("ipv6 %q not redacted: %q", ip6, res.Text)
		}
	}
}

func TestMACLabel(t *testing.T) {
	res := Clean("node mac AA:BB:CC:DD:EE:FF up", nil)
	if !strings.Contains(res.Text, "[MAC_1]") {
		t.Fatalf("mac mislabeled, got: %q", res.Text)
	}
}

// Query strings: only the secret value goes, sibling params survive.
func TestURLQueryParams(t *testing.T) {
	res := Clean("GET https://api/x?token=abc123&api_key=MYKEY123&foo=bar ok", nil)
	if strings.Contains(res.Text, "MYKEY123") {
		t.Fatalf("leaked api key: %q", res.Text)
	}
	if !strings.Contains(res.Text, "&foo=bar") {
		t.Fatalf("sibling param destroyed: %q", res.Text)
	}
}

func TestCardSpacing(t *testing.T) {
	res := Clean("card 4111 1111 1111 1111 charged", nil)
	if !strings.Contains(res.Text, "[CARD_1] charged") {
		t.Fatalf("trailing space eaten: %q", res.Text)
	}
}

// Angola (AO) numbers: local 9XX XXX XXX in any separator (or none),
// +244/00244 international, Luanda 222 landlines. US shapes keep working.
func TestAngolaPhones(t *testing.T) {
	leaks := []string{
		"call me 923 123 456",
		"call me 923-123-456",
		"call me 923.123.456",
		"call me 923123456",
		"tel +244 923 123 456",
		"tel +244923123456",
		"tel +244-923-123-456",
		"tel 00244923123456",
		"movicel 991 234 567",
		"africell 940 111 222",
		"office 222 123 456",
		"zap 935 000 001",
	}
	for _, in := range leaks {
		res := Clean(in, nil)
		if !strings.Contains(res.Text, "[PHONE_AO_1]") {
			t.Errorf("AO number not redacted: %q -> %q", in, res.Text)
		}
	}
	// Consistency: same number twice -> same placeholder.
	res := Clean("a 923 123 456 b 923 123 456 c 991 234 567", nil)
	if strings.Count(res.Text, "[PHONE_AO_1]") != 2 || !strings.Contains(res.Text, "[PHONE_AO_2]") {
		t.Fatalf("inconsistent AO mapping: %q", res.Text)
	}
	// Must not break: US numbers, short/long digit runs.
	kept := map[string]string{
		"us +1-555-123-4567": "us [PHONE_1]",
		"us (555) 123-4567":  "us [PHONE_1]",
		"order 92312345 ok":  "order 92312345 ok",
		"id 923123456789 ok": "id 923123456789 ok",
	}
	for in, want := range kept {
		if res := Clean(in, nil); res.Text != want {
			t.Errorf("regression: %q -> %q, want %q", in, res.Text, want)
		}
	}
}

func TestBearerVariants(t *testing.T) {
	for _, tok := range []string{
		"abcdefghijklmnopqrstuvwx", // opaque
		"eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c", // jwt
	} {
		res := Clean("Authorization: Bearer "+tok, nil)
		if strings.Contains(res.Text, tok) {
			t.Errorf("bearer token leaked: %q", res.Text)
		}
	}
	// Padded JWT keeps the padding inside the placeholder.
	res := Clean("tok eyJhbGciOiJIUzI1NiJ9.eyJzdWIiOiIxIn0.SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c= end", nil)
	if !strings.Contains(res.Text, "[JWT_1] end") {
		t.Fatalf("jwt padding leaked outside placeholder: %q", res.Text)
	}
}

func TestRealisticLine(t *testing.T) {
	res := Clean(realisticLine, nil)
	for _, leak := range []string{
		"bob@example.com", "192.168.1.10", "hunter2", "ghp_abcdefghijklmnopqrstuv",
		"AKIAIOSFODNN7EXAMPLE", "admin:s3cret@", "4111-1111-1111-1111",
		"+1-555-123-4567", "123-45-6789", "AA:BB:CC:DD:EE:FF",
		"SflKxwRJSMeKKF2QT4fwpMeJf36POk6yJV_adQssw5c",
	} {
		if strings.Contains(res.Text, leak) {
			t.Errorf("leaked %q in %q", leak, res.Text)
		}
	}
	// Structure that must survive redaction.
	for _, keep := range []string{"2024-01-15T10:00:01Z", "deploy/api-7d9f8b6c4-x2k9j", "200"} {
		if !strings.Contains(res.Text, keep) {
			t.Errorf("destroyed benign %q in %q", keep, res.Text)
		}
	}
}

// ---- loadtest-style benchmarks: typical clipboard paste vs log file ----

func BenchmarkCleanClipboard4K(b *testing.B) {
	doc := strings.Repeat(realisticLine+"\n", 15) // ~4KB
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Clean(doc, nil)
	}
}

func BenchmarkCleanLog1M(b *testing.B) {
	doc := strings.Repeat(realisticLine+"\n", 3800) // ~1MB
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Clean(doc, nil)
	}
}
