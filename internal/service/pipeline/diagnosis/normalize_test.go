package diagnosis

import (
	"strings"
	"testing"
)

func TestNormalizeLine_Timestamps(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"iso8601 z", "2026-04-29T02:11:42.812Z FATAL boom"},
		{"iso8601 offset", "2026-04-29T02:11:42+08:00 FATAL boom"},
		{"iso8601 space", "2026-04-29 02:11:42.812 FATAL boom"},
		{"bracketed", "[02:11:42] FATAL boom"},
		{"bare hms", "02:11:42 FATAL boom"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := NormalizeLine(c.in)
			if !strings.Contains(got, "<TS>") {
				t.Errorf("expected <TS> placeholder in %q", got)
			}
			if strings.Contains(got, "2026") || strings.Contains(got, "02:11") {
				t.Errorf("expected timestamp stripped, got %q", got)
			}
		})
	}
}

func TestNormalizeLine_UUID(t *testing.T) {
	in := "request-id 550e8400-e29b-41d4-a716-446655440000 failed"
	got := NormalizeLine(in)
	if !strings.Contains(got, "<UUID>") {
		t.Errorf("want <UUID>, got %q", got)
	}
	if strings.Contains(got, "550e8400") {
		t.Errorf("UUID not stripped: %q", got)
	}
}

func TestNormalizeLine_HexAddress(t *testing.T) {
	in := "panic at 0xc0001fe000 in routine"
	got := NormalizeLine(in)
	if !strings.Contains(got, "<ADDR>") || strings.Contains(got, "0xc0001fe000") {
		t.Errorf("want <ADDR> only, got %q", got)
	}
}

func TestNormalizeLine_PIDs(t *testing.T) {
	cases := []string{
		"pid=12345 SIGSEGV",
		"pid: 12345 SIGSEGV",
		"[pid 12345] SIGSEGV",
	}
	for _, c := range cases {
		got := NormalizeLine(c)
		if strings.Contains(got, "12345") {
			t.Errorf("PID not stripped: in=%q got=%q", c, got)
		}
	}
}

func TestNormalizeLine_FileLineNumbers(t *testing.T) {
	in := "    /src/internal/service/foo.go:123 +0x42"
	got := NormalizeLine(in)
	if !strings.Contains(got, "foo.go:<LINE>") {
		t.Errorf("want foo.go:<LINE>, got %q", got)
	}
	// path preserved (different files in a stack must stay distinct).
	if !strings.Contains(got, "internal/service/foo.go") {
		t.Errorf("path was incorrectly stripped: %q", got)
	}
}

func TestNormalizeLine_TmpPathRandSuffix(t *testing.T) {
	a := NormalizeLine("output dir /tmp/build-Ab12CdEf done")
	b := NormalizeLine("output dir /tmp/build-Zz99XyW1 done")
	if a != b {
		t.Errorf("tmp suffix should normalize equal:\n  a=%q\n  b=%q", a, b)
	}
}

func TestNormalizeLine_ANSI(t *testing.T) {
	in := "\x1b[31mFAIL\x1b[0m: TestFoo"
	got := NormalizeLine(in)
	if got != "FAIL: TestFoo" {
		t.Errorf("want %q, got %q", "FAIL: TestFoo", got)
	}
}

func TestNormalizeLogTail_TailCap(t *testing.T) {
	lines := make([]string, 200)
	for i := range lines {
		lines[i] = "noise"
	}
	lines[150] = "DISTINCT_MARKER"
	out := NormalizeLogTail(lines, SignatureOptions{TailLines: 10})

	// Tail of 10 starts at index 190; marker at 150 should NOT be in output.
	if strings.Contains(out, "DISTINCT_MARKER") {
		t.Error("tail cap not respected; marker leaked")
	}
}

func TestNormalizeLogTail_DefaultTail(t *testing.T) {
	// 100 lines, default tail = 50 => first 50 dropped.
	lines := make([]string, 100)
	for i := range lines {
		lines[i] = "noise"
	}
	lines[10] = "EARLY_MARKER"
	lines[80] = "LATE_MARKER"
	out := NormalizeLogTail(lines, SignatureOptions{})
	if strings.Contains(out, "EARLY_MARKER") {
		t.Error("default tail did not drop early lines")
	}
	if !strings.Contains(out, "LATE_MARKER") {
		t.Error("default tail incorrectly dropped late lines")
	}
}

func TestNormalizeLogTail_DropsEmptyLines(t *testing.T) {
	in := []string{"", "   ", "real content", "", "more"}
	out := NormalizeLogTail(in, SignatureOptions{})
	got := strings.Split(out, "\n")
	if len(got) != 2 {
		t.Errorf("want 2 non-empty lines, got %d: %q", len(got), got)
	}
}

func TestNormalizeLogTail_Idempotent(t *testing.T) {
	in := []string{
		"2026-04-29T02:11:42.812Z FATAL [pid 99] foo.go:42 0xabcd1234",
		"\x1b[31mFAIL\x1b[0m TestBar 550e8400-e29b-41d4-a716-446655440000",
	}
	once := NormalizeLogTail(in, SignatureOptions{})
	twice := NormalizeLogTail(strings.Split(once, "\n"), SignatureOptions{})
	if once != twice {
		t.Errorf("not idempotent:\n  once=%q\n  twice=%q", once, twice)
	}
}

func TestComputeSignature_Stable(t *testing.T) {
	a := []string{
		"2026-04-29T02:11:42Z [pid 1234] FAIL TestFoo at /src/foo.go:123",
		"exit status 1",
	}
	b := []string{
		"2026-04-30T05:55:01Z [pid 9876] FAIL TestFoo at /src/foo.go:124",
		"exit status 1",
	}
	sigA, _, errA := ComputeSignature(a, SignatureOptions{})
	sigB, _, errB := ComputeSignature(b, SignatureOptions{})
	if errA != nil || errB != nil {
		t.Fatalf("compute failed: %v / %v", errA, errB)
	}
	if sigA != sigB {
		t.Errorf("expected stable signature across reruns:\n  a=%s\n  b=%s", sigA, sigB)
	}
}

func TestComputeSignature_DifferentFailures(t *testing.T) {
	a := []string{"FATAL: connection refused to 127.0.0.1:5432"}
	b := []string{"FATAL: nil pointer dereference at /src/main.go:42"}
	sigA, _, _ := ComputeSignature(a, SignatureOptions{})
	sigB, _, _ := ComputeSignature(b, SignatureOptions{})
	if sigA == sigB {
		t.Errorf("expected distinct signatures, both = %s", sigA)
	}
}

func TestComputeSignature_EmptyInput(t *testing.T) {
	cases := [][]string{
		nil,
		{},
		{"", "  ", "\t"},
	}
	for _, in := range cases {
		_, _, err := ComputeSignature(in, SignatureOptions{})
		if err != ErrEmptyInput {
			t.Errorf("want ErrEmptyInput, got %v (input=%v)", err, in)
		}
	}
}

func TestComputeSignature_HexLength(t *testing.T) {
	sig, _, err := ComputeSignature([]string{"some failure"}, SignatureOptions{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(sig) != 40 { // SHA1 hex length
		t.Errorf("want 40-char hex, got len=%d (%s)", len(sig), sig)
	}
}

func TestShortSignature(t *testing.T) {
	if got := ShortSignature("a1b2c3d4e5f6g7h8i9j0klmnopqrstuvwxyz1234"); got != "sig_a1b2c3d4e5f6" {
		t.Errorf("ShortSignature = %q", got)
	}
	if got := ShortSignature("short"); got != "" {
		t.Errorf("expected empty for too-short hex, got %q", got)
	}
}

// TestNormalizeLine_PreservesSemantics asserts that edits to error strings,
// function names, file paths, and exit codes change the signature. This
// guards against rules being too aggressive.
func TestComputeSignature_PreservesSemantics(t *testing.T) {
	base := []string{"FATAL TestFoo at /src/internal/foo.go:42 exit status 1"}
	variants := map[string][]string{
		"different error":    {"FATAL TestFoo at /src/internal/foo.go:42 exit status 2"},
		"different test":     {"FATAL TestBar at /src/internal/foo.go:42 exit status 1"},
		"different file":     {"FATAL TestFoo at /src/internal/bar.go:42 exit status 1"},
		"different severity": {"WARN TestFoo at /src/internal/foo.go:42 exit status 1"},
	}
	baseSig, _, _ := ComputeSignature(base, SignatureOptions{})
	for name, v := range variants {
		t.Run(name, func(t *testing.T) {
			sig, _, _ := ComputeSignature(v, SignatureOptions{})
			if sig == baseSig {
				t.Errorf("rule too aggressive: %q produced same sig as base", name)
			}
		})
	}
}
