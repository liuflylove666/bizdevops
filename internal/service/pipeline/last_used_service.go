package pipeline

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
)

// LastUsedRunSummary captures the smart-default values pulled from the most
// recent run of a pipeline. The frontend renders a "use last config" button
// that pre-fills branch + parameters when HasValue is true.
//
// HasValue is false (and other fields zero-valued) when the pipeline has
// never been run; the frontend then hides the button.
type LastUsedRunSummary struct {
	HasValue   bool                   `json:"has_value"`
	RunID      uint                   `json:"run_id,omitempty"`
	Branch     string                 `json:"branch,omitempty"`
	Commit     string                 `json:"commit,omitempty"`
	Parameters map[string]interface{} `json:"parameters,omitempty"`
	Status     string                 `json:"status,omitempty"`
	HappenedAt time.Time              `json:"happened_at,omitempty"`
}

// GetLastUsed returns the smart-default summary for a pipeline's most recent
// run regardless of run status (success/failed both count — users often want
// to retry a failed config).
//
// A pipeline with no runs returns &LastUsedRunSummary{HasValue: false}, nil
// rather than an error: "never run" is a normal state, not an exception.
func (s *RunService) GetLastUsed(ctx context.Context, pipelineID uint) (*LastUsedRunSummary, error) {
	var run models.PipelineRun
	err := s.db.WithContext(ctx).
		Where("pipeline_id = ?", pipelineID).
		Order("created_at DESC").
		First(&run).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return &LastUsedRunSummary{HasValue: false}, nil
		}
		return nil, err
	}

	out := &LastUsedRunSummary{
		HasValue:   true,
		RunID:      run.ID,
		Branch:     run.GitBranch,
		Commit:     run.GitCommit,
		Status:     run.Status,
		HappenedAt: run.CreatedAt,
	}

	if run.ParametersJSON != "" {
		var params map[string]interface{}
		if err := json.Unmarshal([]byte(run.ParametersJSON), &params); err == nil {
			out.Parameters = params
		}
	}

	return out, nil
}
