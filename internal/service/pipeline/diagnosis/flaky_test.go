package diagnosis

import "testing"

func TestFlakyReasonFor(t *testing.T) {
	cases := []struct {
		name              string
		isFlakyRetry      bool
		distinctLastWindow int64
		want              string
	}{
		{"same commit retry trumps everything", true, 0, FlakyReasonSameCommitRetry},
		{"same commit retry trumps cross commit", true, 99, FlakyReasonSameCommitRetry},
		{"cross commit at threshold", false, int64(CrossCommitFlakyThreshold), FlakyReasonCrossCommit},
		{"cross commit above threshold", false, int64(CrossCommitFlakyThreshold) + 5, FlakyReasonCrossCommit},
		{"below threshold", false, int64(CrossCommitFlakyThreshold) - 1, ""},
		{"zero distinct commits", false, 0, ""},
		{"single commit", false, 1, ""},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := flakyReasonFor(c.isFlakyRetry, c.distinctLastWindow)
			if got != c.want {
				t.Errorf("flakyReasonFor(%v, %d) = %q; want %q",
					c.isFlakyRetry, c.distinctLastWindow, got, c.want)
			}
		})
	}
}

// TestFlakyReason_NilInputs verifies the public method's defensive nil checks.
// Service is constructed without a DB; the method must short-circuit before
// any DB access on the same-commit-retry path or on nil inputs.
func TestFlakyReason_NilInputs(t *testing.T) {
	s := NewService(nil) // intentional: must not be dereferenced on these paths

	if reason, err := s.FlakyReason(nil, nil, nil); err != nil || reason != "" {
		t.Errorf("nil rec: got (%q, %v), want empty", reason, err)
	}

	rec := &PipelineRunFailure{IsFlakyRetry: true}
	if reason, err := s.FlakyReason(nil, rec, nil); err != nil || reason != FlakyReasonSameCommitRetry {
		t.Errorf("flaky retry shortcut: got (%q, %v)", reason, err)
	}

	rec.IsFlakyRetry = false
	if reason, err := s.FlakyReason(nil, rec, nil); err != nil || reason != "" {
		t.Errorf("nil sig + not flaky: got (%q, %v), want empty", reason, err)
	}
}

// TestMarkSameCommitRetrySuccess_NoOpOnZeroes ensures the early-exit guard
// works without DB access.
func TestMarkSameCommitRetrySuccess_NoOpOnZeroes(t *testing.T) {
	s := NewService(nil)
	cases := []struct {
		name       string
		pipelineID uint
		commitSHA  string
	}{
		{"zero pipeline id", 0, "abc"},
		{"empty commit", 100, ""},
		{"both zero", 0, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			n, err := s.MarkSameCommitRetrySuccess(nil, c.pipelineID, c.commitSHA)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if n != 0 {
				t.Errorf("expected 0 rows, got %d", n)
			}
		})
	}
}

// TestConstants is a regression guard: changing these knobs is a behavior
// change, not a refactor. Keeping the test as a tripwire.
func TestConstants(t *testing.T) {
	if CrossCommitFlakyThreshold != 3 {
		t.Errorf("CrossCommitFlakyThreshold = %d; spec says 3", CrossCommitFlakyThreshold)
	}
	if FlakyWindowDays != 7 {
		t.Errorf("FlakyWindowDays = %d; spec says 7", FlakyWindowDays)
	}
	if SimilarRunsLimit != 3 {
		t.Errorf("SimilarRunsLimit = %d; spec says 3", SimilarRunsLimit)
	}
}
