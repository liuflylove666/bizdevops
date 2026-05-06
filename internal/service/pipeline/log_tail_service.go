package pipeline

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"

	"devops/internal/models"
)

// LogTailLine is the structured form returned by GetRunLogTail. ts and
// stream are populated when the executor wrote them as a structured
// prefix; otherwise both are empty strings and Line carries the raw
// text. Sprint 2 BE-13 ships the empty-prefix version; future executor
// changes can backfill ts/stream without breaking the schema.
type LogTailLine struct {
	TS     string `json:"ts,omitempty"`
	Stream string `json:"stream,omitempty"`
	Line   string `json:"line"`
}

// LogTailResult is the assembled response shape for the BE-13 endpoint.
type LogTailResult struct {
	RunID          uint          `json:"run_id"`
	Status         string        `json:"status"`
	LinesTotal     int           `json:"lines_total"`
	LinesTruncated bool          `json:"lines_truncated"`
	Lines          []LogTailLine `json:"lines"`
}

// LogTailMaxN caps the requested number of trailing lines (matches the
// docs/api/sprint2_v1.md contract).
const LogTailMaxN = 500

// LogTailDefaultN is the default tail length when n is omitted or 0.
const LogTailDefaultN = 50

// ErrInvalidTailN is returned when n is outside [1, LogTailMaxN].
var ErrInvalidTailN = errors.New("log tail: n must be in [1, 500]")

// ErrRunNotFound is returned when the requested run does not exist.
var ErrRunNotFound = errors.New("log tail: run not found")

// GetRunLogTail returns up to n trailing lines from a run's combined step
// logs. Run with no logs (e.g., still pending) yields an empty Lines slice
// rather than an error: "no logs yet" is a normal state.
//
// Lines are concatenated stage-by-stage in execution order, then split on
// '\n', empty lines dropped, last n kept. ts/stream remain empty in V1.
//
// Returns ErrRunNotFound when the run does not exist; ErrInvalidTailN
// when n is out of range.
func (s *LogService) GetRunLogTail(ctx context.Context, runID uint, n int) (*LogTailResult, error) {
	if n == 0 {
		n = LogTailDefaultN
	}
	if n < 1 || n > LogTailMaxN {
		return nil, fmt.Errorf("%w: got %d", ErrInvalidTailN, n)
	}
	if runID == 0 {
		return nil, fmt.Errorf("log tail: run_id must be > 0")
	}

	// Confirm run exists and capture its status for the response.
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).Select("id", "status").First(&run, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrRunNotFound
		}
		return nil, err
	}

	// Pull all step logs in deterministic order. JOIN keeps it one round-trip;
	// ordering by stage_run.id then step_run.id reflects creation order, which
	// matches execution order in the project's executor.
	type row struct {
		Logs string
	}
	var rows []row
	err := s.db.WithContext(ctx).
		Table("step_runs").
		Joins("JOIN stage_runs ON stage_runs.id = step_runs.stage_run_id").
		Where("stage_runs.pipeline_run_id = ?", runID).
		Order("stage_runs.id ASC, step_runs.id ASC").
		Select("step_runs.logs AS logs").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}

	// Concatenate non-empty per-step logs; sanitize secrets via the existing
	// SanitizeLogs path so this endpoint can never leak credentials.
	var combined strings.Builder
	for _, r := range rows {
		if r.Logs == "" {
			continue
		}
		if combined.Len() > 0 {
			combined.WriteByte('\n')
		}
		combined.WriteString(s.SanitizeLogs(r.Logs))
	}

	out := &LogTailResult{
		RunID:  run.ID,
		Status: run.Status,
		Lines:  []LogTailLine{},
	}
	if combined.Len() == 0 {
		return out, nil
	}

	// Split, drop empty, take last n.
	all := strings.Split(combined.String(), "\n")
	clean := make([]string, 0, len(all))
	for _, ln := range all {
		ln = strings.TrimRight(ln, " \t\r")
		if ln == "" {
			continue
		}
		clean = append(clean, ln)
	}

	totalNonEmpty := len(clean)
	if totalNonEmpty > n {
		out.LinesTruncated = true
		clean = clean[totalNonEmpty-n:]
	}

	out.LinesTotal = len(clean)
	for _, ln := range clean {
		out.Lines = append(out.Lines, LogTailLine{Line: ln})
	}
	return out, nil
}
