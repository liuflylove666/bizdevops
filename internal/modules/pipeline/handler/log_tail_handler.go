// Package handler 流水线模块处理器
//
// log_tail_handler.go ships GET /pipeline/runs/:id/log/tail (Sprint 2
// BE-13). Used by FE-08 列表 hover 预览 to show the last N lines of a
// run's combined step logs without loading full logs.
package handler

import (
	"errors"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/service/pipeline"
	"devops/pkg/response"
)

// LogTailHandler implements the read API for run log tail preview.
type LogTailHandler struct {
	logSvc *pipeline.LogService
}

// NewLogTailHandler wires the handler with its log service.
func NewLogTailHandler(db *gorm.DB) *LogTailHandler {
	return &LogTailHandler{
		logSvc: pipeline.NewLogService(db),
	}
}

// RegisterRoutes attaches the single read endpoint.
func (h *LogTailHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/pipeline/runs/:id/log/tail", h.GetLogTail)
}

// GetLogTail godoc
// @Summary 获取 run 末尾 N 行日志（去敏后）
// @Description 列表页 hover 预览专用；不返回完整日志。n 默认 50，范围 [1, 500]。
// @Tags 流水线
// @Param id path int true "run ID"
// @Param n query int false "末 N 行（默认 50，最大 500）"
// @Success 200 {object} response.Response{data=pipeline.LogTailResult}
// @Failure 400 {object} response.Response "run_id 非数字 / n 超范围"
// @Failure 404 {object} response.Response "run 不存在"
// @Router /pipeline/runs/{id}/log/tail [get]
// @Security BearerAuth
func (h *LogTailHandler) GetLogTail(c *gin.Context) {
	id64, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id64 == 0 {
		response.BadRequest(c, "run_id 必须为正整数")
		return
	}

	n := 0 // 0 → service uses LogTailDefaultN
	if raw := c.Query("n"); raw != "" {
		parsed, perr := strconv.Atoi(raw)
		if perr != nil {
			response.BadRequest(c, "n 必须为整数")
			return
		}
		n = parsed
	}

	out, err := h.logSvc.GetRunLogTail(c, uint(id64), n)
	if err != nil {
		switch {
		case errors.Is(err, pipeline.ErrRunNotFound):
			response.NotFound(c, "run 不存在")
		case errors.Is(err, pipeline.ErrInvalidTailN):
			response.BadRequest(c, err.Error())
		default:
			response.InternalError(c, err.Error())
		}
		return
	}
	response.Success(c, out)
}
