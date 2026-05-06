package diagnosis

import (
	"context"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Service writes failure signatures to MySQL (tables created by
// migrations/patch_401_failure_signatures.sql).
//
// It is invoked by the pipeline executor on terminal failure and by the
// diagnosis API handler (Sprint 1 BE-05) for read-back.
type Service struct {
	db *gorm.DB
}

// NewService wires a new diagnosis Service. The caller passes the same
// *gorm.DB used by the rest of the pipeline package.
func NewService(db *gorm.DB) *Service {
	return &Service{db: db}
}

// FailureSignature is the GORM-side mapping for the failure_signatures
// table. The schema is defined by patch_401; this struct is read-only
// from the migration's perspective — do not include it in AutoMigrate.
type FailureSignature struct {
	ID               uint      `gorm:"primaryKey;column:id"`
	CreatedAt        time.Time `gorm:"column:created_at"`
	UpdatedAt        time.Time `gorm:"column:updated_at"`
	Signature        string    `gorm:"column:signature;size:40;uniqueIndex:uk_signature"`
	NormalizedSample string    `gorm:"column:normalized_sample;type:text"`
	FirstSeenRunID   uint      `gorm:"column:first_seen_run_id"`
	FirstSeenAt      time.Time `gorm:"column:first_seen_at"`
	LastSeenRunID    uint      `gorm:"column:last_seen_run_id"`
	LastSeenAt       time.Time `gorm:"column:last_seen_at"`
	Occurrences      uint      `gorm:"column:occurrences;default:1"`
	DistinctCommits  uint      `gorm:"column:distinct_commits;default:1"`
}

// TableName matches patch_401_failure_signatures.sql.
func (FailureSignature) TableName() string { return "failure_signatures" }

// PipelineRunFailure is the run ↔ signature association row.
//
// One pipeline_runs row has at most one PipelineRunFailure (run_id is the
// primary key). Subsequent attempts to record the same run are no-ops via
// ON CONFLICT DO NOTHING.
type PipelineRunFailure struct {
	RunID         uint      `gorm:"primaryKey;column:run_id"`
	SignatureID   uint      `gorm:"column:signature_id;index:idx_signature_id"`
	PipelineID    uint      `gorm:"column:pipeline_id;index:idx_pipeline_id"`
	CommitSHA     string    `gorm:"column:commit_sha;size:40;index:idx_commit_sha"`
	IsFlakyRetry  bool      `gorm:"column:is_flaky_retry;default:0"`
	FixedByCommit string    `gorm:"column:fixed_by_commit;size:40"`
	LogTail       string    `gorm:"column:log_tail;type:text"`
	CreatedAt     time.Time `gorm:"column:created_at"`
}

// TableName matches patch_401_failure_signatures.sql.
func (PipelineRunFailure) TableName() string { return "pipeline_run_failures" }

// RecordFailureInput captures everything needed to persist one run's failure.
//
// LogTail is expected to be the raw lines as captured by the executor (with
// timestamps still attached). The service applies normalization internally;
// callers do not pre-normalize.
type RecordFailureInput struct {
	RunID      uint
	PipelineID uint
	CommitSHA  string
	LogTail    []string
}

// ErrInvalidInput is returned when RecordFailure is called with a zero RunID
// or no log lines.
var ErrInvalidInput = errors.New("diagnosis: RunID and LogTail are required")

// RecordFailure persists a run's failure: upserts the failure_signatures row
// and inserts the pipeline_run_failures association.
//
// Behavior:
//
//   - If the signature exists, occurrences is incremented and last_seen_*
//     fields are updated; distinct_commits is incremented only when the
//     commit_sha is new for that signature.
//   - If the signature does not exist, a new row is inserted.
//   - If the run already has a recorded failure (RunID collision), the
//     association insert is a no-op (ON CONFLICT DO NOTHING). Signature
//     counters are still updated, which is intentional: the executor may
//     legitimately call this once per failed run.
//
// The whole operation runs in a single transaction.
func (s *Service) RecordFailure(ctx context.Context, in RecordFailureInput) (*FailureSignature, error) {
	if in.RunID == 0 || len(in.LogTail) == 0 {
		return nil, ErrInvalidInput
	}

	sigHex, normalized, err := ComputeSignature(in.LogTail, SignatureOptions{})
	if err != nil {
		return nil, err
	}

	now := time.Now()
	var resolved FailureSignature

	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing FailureSignature
		findErr := tx.Where("signature = ?", sigHex).First(&existing).Error

		switch {
		case findErr == nil:
			// Update last_seen + occurrences.
			updates := map[string]interface{}{
				"last_seen_run_id": in.RunID,
				"last_seen_at":     now,
				"occurrences":      gorm.Expr("occurrences + 1"),
			}
			if err := tx.Model(&existing).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
				return err
			}

			// Increment distinct_commits only if this commit is new for this signature.
			if in.CommitSHA != "" {
				var seen int64
				if err := tx.Model(&PipelineRunFailure{}).
					Where("signature_id = ? AND commit_sha = ?", existing.ID, in.CommitSHA).
					Count(&seen).Error; err != nil {
					return err
				}
				if seen == 0 {
					if err := tx.Model(&FailureSignature{}).
						Where("id = ?", existing.ID).
						Update("distinct_commits", gorm.Expr("distinct_commits + 1")).Error; err != nil {
						return err
					}
				}
			}

			// Reload for return value.
			if err := tx.Where("id = ?", existing.ID).First(&existing).Error; err != nil {
				return err
			}

		case errors.Is(findErr, gorm.ErrRecordNotFound):
			distinct := uint(1)
			if in.CommitSHA == "" {
				distinct = 0
			}
			existing = FailureSignature{
				Signature:        sigHex,
				NormalizedSample: normalized,
				FirstSeenRunID:   in.RunID,
				FirstSeenAt:      now,
				LastSeenRunID:    in.RunID,
				LastSeenAt:       now,
				Occurrences:      1,
				DistinctCommits:  distinct,
			}
			if err := tx.Create(&existing).Error; err != nil {
				return err
			}

		default:
			return findErr
		}

		assoc := PipelineRunFailure{
			RunID:       in.RunID,
			SignatureID: existing.ID,
			PipelineID:  in.PipelineID,
			CommitSHA:   in.CommitSHA,
			LogTail:     strings.Join(in.LogTail, "\n"),
			CreatedAt:   now,
		}
		// Idempotent: if the run already has a row, do nothing.
		if err := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&assoc).Error; err != nil {
			return err
		}

		resolved = existing
		return nil
	})

	if err != nil {
		return nil, err
	}
	return &resolved, nil
}

// GetByRunID returns the failure association + parent signature for a run,
// or (nil, nil, nil) if the run had no recorded failure (e.g., succeeded).
//
// Callers must distinguish "no failure" from "lookup error" by inspecting
// both return values.
func (s *Service) GetByRunID(ctx context.Context, runID uint) (*PipelineRunFailure, *FailureSignature, error) {
	var assoc PipelineRunFailure
	err := s.db.WithContext(ctx).Where("run_id = ?", runID).First(&assoc).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}

	var sig FailureSignature
	if err := s.db.WithContext(ctx).Where("id = ?", assoc.SignatureID).First(&sig).Error; err != nil {
		return &assoc, nil, err
	}
	return &assoc, &sig, nil
}

// SimilarRunsLimit caps how many similar runs the API surface returns.
// 3 matches docs/api/diagnosis_v1.md. Centralized here so the upcoming
// BE-05 handler and any internal callers stay in sync.
const SimilarRunsLimit = 3

// CrossCommitFlakyThreshold is the count of distinct commits required within
// FlakyWindowDays for a signature to be considered "cross commit recurrence".
const CrossCommitFlakyThreshold = 3

// FlakyWindowDays is the rolling window over which CrossCommitFlakyThreshold
// is evaluated.
const FlakyWindowDays = 7

// Flaky reason strings (also enumerated in docs/api/diagnosis_v1.md).
const (
	FlakyReasonSameCommitRetry = "same_commit_retry_succeeded"
	FlakyReasonCrossCommit     = "cross_commit_recurrence"
)

// flakyReasonFor is the pure-logic core of FlakyReason. Exposed for unit
// testing without a DB. Handlers should not call it directly; they should
// call (*Service).FlakyReason which loads the count.
func flakyReasonFor(isFlakyRetry bool, distinctCommitsLastWindow int64) string {
	if isFlakyRetry {
		return FlakyReasonSameCommitRetry
	}
	if distinctCommitsLastWindow >= CrossCommitFlakyThreshold {
		return FlakyReasonCrossCommit
	}
	return ""
}

// FlakyReason classifies whether a run's failure should be presented as
// flaky. Returns "" when not flaky; otherwise one of FlakyReason*.
//
// The cross-commit branch queries pipeline_run_failures for distinct commits
// within FlakyWindowDays. The same-commit branch is a single field read on
// the input record.
func (s *Service) FlakyReason(
	ctx context.Context,
	rec *PipelineRunFailure,
	sig *FailureSignature,
) (string, error) {
	if rec == nil {
		return "", nil
	}
	if rec.IsFlakyRetry {
		return FlakyReasonSameCommitRetry, nil
	}
	if sig == nil {
		return "", nil
	}

	cutoff := time.Now().AddDate(0, 0, -FlakyWindowDays)
	var distinct int64
	err := s.db.WithContext(ctx).
		Model(&PipelineRunFailure{}).
		Where("signature_id = ? AND created_at >= ? AND commit_sha <> ''", sig.ID, cutoff).
		Select("COUNT(DISTINCT commit_sha)").
		Scan(&distinct).Error
	if err != nil {
		return "", err
	}
	return flakyReasonFor(false, distinct), nil
}

// MarkSameCommitRetrySuccess is invoked by the executor when a run finishes
// with status=success. Earlier failed runs of the same pipeline + commit
// are marked is_flaky_retry=1: the same code worked on retry, so those
// failures retroactively gained "flaky" context.
//
// Returns the number of rows updated. No-op when pipelineID or commitSHA
// is zero/empty (also a no-op when no earlier matching failures exist).
//
// Note: BE-03 ships this method; wiring it into the run-finalize path is
// a separate change owned by the executor (BE-02 follow-up).
func (s *Service) MarkSameCommitRetrySuccess(
	ctx context.Context,
	pipelineID uint,
	commitSHA string,
) (int64, error) {
	if pipelineID == 0 || commitSHA == "" {
		return 0, nil
	}
	res := s.db.WithContext(ctx).
		Model(&PipelineRunFailure{}).
		Where("pipeline_id = ? AND commit_sha = ? AND is_flaky_retry = ?", pipelineID, commitSHA, false).
		Update("is_flaky_retry", true)
	return res.RowsAffected, res.Error
}

// FindSimilarRuns returns up to SimilarRunsLimit historical runs that share
// the same failure_signature, optionally restricted by pipelineID and
// excluding the run being diagnosed. Ordered by most recent first.
//
// The returned slice may be empty (never nil) when no peers exist.
func (s *Service) FindSimilarRuns(
	ctx context.Context,
	signatureID uint,
	excludeRunID uint,
	pipelineID uint,
	withinDays int,
) ([]PipelineRunFailure, error) {
	if withinDays <= 0 {
		withinDays = 30
	}
	cutoff := time.Now().AddDate(0, 0, -withinDays)

	q := s.db.WithContext(ctx).
		Model(&PipelineRunFailure{}).
		Where("signature_id = ? AND run_id <> ? AND created_at >= ?", signatureID, excludeRunID, cutoff).
		Order("created_at DESC").
		Limit(SimilarRunsLimit)

	if pipelineID > 0 {
		q = q.Where("pipeline_id = ?", pipelineID)
	}

	out := make([]PipelineRunFailure, 0, SimilarRunsLimit)
	if err := q.Find(&out).Error; err != nil {
		return nil, err
	}
	return out, nil
}
