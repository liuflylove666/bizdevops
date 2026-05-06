// Package diagnosis computes a stable failure signature from a CI run's log
// tail. It is the foundation of the data-driven (no-AI) failure diagnosis
// card delivered in Sprint 1.
//
// Signature is SHA1 over a normalized form of the last N log lines. The
// normalization rules deliberately strip *only* fields that are guaranteed
// to vary across reruns of the same failure (timestamps, PIDs, addresses,
// UUIDs, line numbers in stack traces) while preserving everything that
// carries semantic meaning (function names, error strings, file paths,
// exit codes).
//
// Goals:
//
//  1. Same root cause across reruns -> same signature
//  2. Genuinely different failures   -> different signatures
//  3. Idempotent: Normalize(Normalize(x)) == Normalize(x)
//
// See docs/api/diagnosis_v1.md for the surfaced contract.
package diagnosis

import (
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"regexp"
	"strings"
)

// DefaultTailLines is the upper bound on how many trailing log lines are
// fed into normalization. Tuned for representative test/build failures;
// override via SignatureOptions.
const DefaultTailLines = 50

// SignatureOptions tunes signature computation. Zero-value is valid and
// equivalent to defaults.
type SignatureOptions struct {
	// TailLines caps the number of trailing lines considered. <=0 means
	// DefaultTailLines.
	TailLines int
}

// ErrEmptyInput is returned when the input log is empty after trimming.
var ErrEmptyInput = errors.New("diagnosis: log tail is empty")

// Normalization rules, applied in order. Each rule is a regex + replacement.
// The order matters when patterns can overlap (e.g. timestamps that contain
// digits should run before the bare digit-sequence rule).
var rules = []struct {
	re   *regexp.Regexp
	with string
}{
	// ANSI escape sequences.
	{regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`), ""},

	// ISO 8601 / RFC 3339 timestamps with optional fractional seconds + zone.
	// 2026-04-29T02:11:42.812Z, 2026-04-29 02:11:42.812+08:00, etc.
	{regexp.MustCompile(`\d{4}-\d{2}-\d{2}[T ]\d{2}:\d{2}:\d{2}(?:\.\d+)?(?:Z|[+-]\d{2}:?\d{2})?`), "<TS>"},

	// Bracketed wall-clock times: [02:11:42] or [02:11:42.812].
	{regexp.MustCompile(`\[\d{2}:\d{2}:\d{2}(?:\.\d+)?\]`), "<TS>"},

	// Bare wall-clock times: 02:11:42.812 or 02:11:42.
	{regexp.MustCompile(`\b\d{2}:\d{2}:\d{2}(?:\.\d+)?\b`), "<TS>"},

	// UUIDs (8-4-4-4-12 hex).
	{regexp.MustCompile(`\b[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}\b`), "<UUID>"},

	// Hex memory addresses: 0xabcdef00.
	{regexp.MustCompile(`\b0x[0-9a-fA-F]{6,}\b`), "<ADDR>"},

	// PIDs / TIDs in common forms.
	{regexp.MustCompile(`\bpid[:= ]\s*\d+\b`), "pid=<PID>"},
	{regexp.MustCompile(`\btid[:= ]\s*\d+\b`), "tid=<TID>"},
	{regexp.MustCompile(`\[pid \d+\]`), "[pid <PID>]"},

	// Source file:line references in stack traces (Go, Node, Python, etc.).
	// foo/bar.go:123  ->  foo/bar.go:<LINE>
	// preserves the path so different files in a trace stay distinct.
	{regexp.MustCompile(`(\.[a-zA-Z]{1,5}):\d+\b`), "${1}:<LINE>"},

	// Random-looking suffix on tmp paths: /tmp/build-XXXXXXXX
	{regexp.MustCompile(`(/tmp/[a-zA-Z0-9_-]+?-)[a-zA-Z0-9]{6,}\b`), "${1}<RAND>"},

	// Trailing whitespace.
	{regexp.MustCompile(`[ \t]+$`), ""},
}

// NormalizeLine applies all rules to a single line.
func NormalizeLine(line string) string {
	out := line
	for _, r := range rules {
		out = r.re.ReplaceAllString(out, r.with)
	}
	return out
}

// NormalizeLogTail returns a single string ready for hashing:
//
//   - keeps only the last opts.TailLines lines
//   - applies NormalizeLine to each
//   - drops empty lines after normalization
//   - joins with '\n'
//
// The output is deterministic for a given input and options. It is also
// idempotent: feeding the result back through NormalizeLogTail yields the
// same string (modulo line splitting on '\n').
func NormalizeLogTail(lines []string, opts SignatureOptions) string {
	tail := opts.TailLines
	if tail <= 0 {
		tail = DefaultTailLines
	}
	if len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}

	out := make([]string, 0, len(lines))
	for _, ln := range lines {
		n := NormalizeLine(ln)
		n = strings.TrimRight(n, " \t")
		if n == "" {
			continue
		}
		out = append(out, n)
	}
	return strings.Join(out, "\n")
}

// ComputeSignature returns (sigHex, normalized, err).
//
// sigHex is the full 40-char SHA1; the API surface displays "sig_<first12>".
// Callers that want the short form should slice on their own.
func ComputeSignature(lines []string, opts SignatureOptions) (string, string, error) {
	normalized := NormalizeLogTail(lines, opts)
	if strings.TrimSpace(normalized) == "" {
		return "", "", ErrEmptyInput
	}
	sum := sha1.Sum([]byte(normalized))
	return hex.EncodeToString(sum[:]), normalized, nil
}

// ShortSignature returns the user-facing form: "sig_" + first 12 hex chars.
// Returns "" if hex is shorter than 12 chars.
func ShortSignature(sigHex string) string {
	if len(sigHex) < 12 {
		return ""
	}
	return "sig_" + sigHex[:12]
}
