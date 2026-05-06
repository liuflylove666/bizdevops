// Package handler 流水线模块处理器
//
// last_used_handler.go ships GET /pipelines/:id/last-run-config (Sprint 1
// BE-06 / FE-03). The endpoint reads the most recent run of a pipeline
// (regardless of status) and returns its branch + parameters as smart
// defaults for the next run.
package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/service/pipeline"
	"devops/pkg/response"
)

// LastUsedHandler returns the smart-default run config for a pipeline.
type LastUsedHandler struct {
	runSvc *pipeline.RunService
}

// NewLastUsedHandler wires the handler with its run service.
func NewLastUsedHandler(db *gorm.DB) *LastUsedHandler {
	return &LastUsedHandler{
		runSvc: pipeline.NewRunService(db),
	}
}

// RegisterRoutes attaches the single read endpoint.
func (h *LastUsedHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/pipelines/:id/last-run-config", h.GetLastRunConfig)
}

// GetLastRunConfig godoc
// @Summary 获取流水线最近一次运行的配置（智能默认）
// @Description 用于 run 触发弹窗的"使用上次配置"按钮。Pipeline 从未运行过时返回 has_value=false。
// @Tags 流水线
// @Param id path int true "Pipeline ID"
// @Success 200 {object} response.Response{data=pipeline.LastUsedRunSummary}
// @Failure 400 {object} response.Response "id 非数字"
// @Router /pipelines/{id}/last-run-config [get]
// @Security BearerAuth
func (h *LastUsedHandler) GetLastRunConfig(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		response.BadRequest(c, "pipeline id 必须为正整数")
		return
	}

	out, err := h.runSvc.GetLastUsed(c, uint(id64))
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, out)
}
