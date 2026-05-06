// Package handler 流水线模块处理器
//
// diagnosis_handler.go ships GET /pipeline/runs/:id/diagnosis (Sprint 1
// BE-05). Contract: docs/api/diagnosis_v1.md. The endpoint is purely
// read-only: it loads the failed run, its signature association, the
// signature itself, similar peers, and the prior successful run, then
// emits a structured payload. No AI is called, ever.
package handler

import (
	"errors"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/service/pipeline/diagnosis"
	"devops/pkg/response"
)

// DiagnosisHandler implements the read API for failed run diagnosis.
type DiagnosisHandler struct {
	db  *gorm.DB
	svc *diagnosis.Service
}

// NewDiagnosisHandler wires the handler with its service.
func NewDiagnosisHandler(db *gorm.DB) *DiagnosisHandler {
	return &DiagnosisHandler{
		db:  db,
		svc: diagnosis.NewService(db),
	}
}

// RegisterRoutes attaches the single read endpoint.
func (h *DiagnosisHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/pipeline/runs/:id/diagnosis", h.GetDiagnosis)
}

// ----- Response DTOs (mirror docs/api/diagnosis_v1.md) -----

type diagnosisResponse struct {
	RunID      uint   `json:"run_id"`
	PipelineID uint   `json:"pipeline_id"`
	Status     string `json:"status"`

	FailureSignature         *string    `json:"failure_signature"`
	SignatureFirstSeenAt     *time.Time `json:"signature_first_seen_at,omitempty"`
	SignatureOccurrences     uint       `json:"signature_occurrences,omitempty"`
	SignatureDistinctCommits uint       `json:"signature_distinct_commits,omitempty"`

	IsFlaky     bool    `json:"is_flaky"`
	FlakyReason *string `json:"flaky_reason"`

	LastSuccess  *lastSuccessInfo `json:"last_success"`
	ChangedFiles []changedFile    `json:"changed_files"`
	SimilarRuns  []similarRun     `json:"similar_runs"`
	FixReferences []fixReference  `json:"fix_references"`
	LogTail      []logTailLine    `json:"log_tail"`
}

type lastSuccessInfo struct {
	RunID      uint      `json:"run_id"`
	Commit     string    `json:"commit"`
	HappenedAt time.Time `json:"happened_at"`
	DiffURL    string    `json:"diff_url,omitempty"`
}

// changedFile is a placeholder per the V1 contract: the field is required
// to be present (so frontend renders an empty list rather than a missing
// section), but population is owned by BE-09 in Sprint 2. Defining the
// schema here lets the handler remain stable when BE-09 lands.
type changedFile struct {
	Path      string `json:"path"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
}

type similarRun struct {
	RunID         uint      `json:"run_id"`
	HappenedAt    time.Time `json:"happened_at"`
	FixedByCommit string    `json:"fixed_by_commit,omitempty"`
	FixDiffURL    string    `json:"fix_diff_url,omitempty"`
}

type fixReference struct {
	Kind  string `json:"kind"`
	Key   string `json:"key,omitempty"`
	ID    string `json:"id,omitempty"`
	Title string `json:"title"`
	URL   string `json:"url,omitempty"`
}

type logTailLine struct {
	TS     string `json:"ts,omitempty"`
	Stream string `json:"stream,omitempty"`
	Line   string `json:"line"`
}

// ----- Handler -----

// GetDiagnosis godoc
// @Summary 获取失败 run 的结构化诊断（去 AI 化）
// @Description 返回失败签名、上次成功 commit、相似历史、修复参考。无 AI 字段。
// @Tags 流水线
// @Param id path int true "run ID"
// @Success 200 {object} response.Response{data=object}
// @Failure 400 {object} response.Response "run_id 非数字"
// @Failure 404 {object} response.Response "run 不存在"
// @Failure 409 {object} response.Response "run 状态不是 failed/cancelled"
// @Router /pipeline/runs/{id}/diagnosis [get]
// @Security BearerAuth
func (h *DiagnosisHandler) GetDiagnosis(c *gin.Context) {
	runID64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || runID64 == 0 {
		response.BadRequest(c, "run_id 必须为正整数")
		return
	}
	runID := uint(runID64)

	var run models.PipelineRun
	if err := h.db.WithContext(c).First(&run, runID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "run 不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	if run.Status != "failed" && run.Status != "cancelled" {
		response.Conflict(c, "run 状态非 failed/cancelled，不允许诊断")
		return
	}

	// Always-present skeleton; populated below as data becomes available.
	out := diagnosisResponse{
		RunID:         run.ID,
		PipelineID:    run.PipelineID,
		Status:        run.Status,
		ChangedFiles:  []changedFile{},
		SimilarRuns:   []similarRun{},
		FixReferences: []fixReference{},
		LogTail:       []logTailLine{},
	}

	// Last successful run on same pipeline, irrespective of failure record.
	var lastSuccess models.PipelineRun
	err = h.db.WithContext(c).
		Where("pipeline_id = ? AND status = ? AND id < ?", run.PipelineID, "success", run.ID).
		Order("id DESC").
		First(&lastSuccess).Error
	if err == nil {
		out.LastSuccess = &lastSuccessInfo{
			RunID:      lastSuccess.ID,
			Commit:     lastSuccess.GitCommit,
			HappenedAt: lastSuccess.CreatedAt,
		}
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		response.InternalError(c, err.Error())
		return
	}

	// Failure record + signature. May be nil if executor hasn't recorded
	// (e.g., normalize produced empty input). The contract permits this:
	// the response degrades to "status + log_tail only" form.
	assoc, sig, err := h.svc.GetByRunID(c, runID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	if assoc != nil && sig != nil {
		short := diagnosis.ShortSignature(sig.Signature)
		out.FailureSignature = &short
		fseen := sig.FirstSeenAt
		out.SignatureFirstSeenAt = &fseen
		out.SignatureOccurrences = sig.Occurrences
		out.SignatureDistinctCommits = sig.DistinctCommits

		if reason, fErr := h.svc.FlakyReason(c, assoc, sig); fErr == nil && reason != "" {
			out.IsFlaky = true
			r := reason
			out.FlakyReason = &r
		}

		similar, sErr := h.svc.FindSimilarRuns(c, sig.ID, run.ID, run.PipelineID, 30)
		if sErr == nil {
			for _, s := range similar {
				out.SimilarRuns = append(out.SimilarRuns, similarRun{
					RunID:         s.RunID,
					HappenedAt:    s.CreatedAt,
					FixedByCommit: s.FixedByCommit,
				})
			}
		}

		if assoc.LogTail != "" {
			for _, ln := range strings.Split(assoc.LogTail, "\n") {
				if ln == "" {
					continue
				}
				out.LogTail = append(out.LogTail, logTailLine{Line: ln})
			}
		}
	}

	response.Success(c, out)
}
