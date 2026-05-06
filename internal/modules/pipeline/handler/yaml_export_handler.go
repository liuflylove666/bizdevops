// Package handler 流水线模块处理器
//
// yaml_export_handler.go ships GET /pipeline/:id/yaml (Sprint 2 BE-12).
// Pipelines DB row → IR → YAML, with optional designer Layout block.
// Contract: docs/api/sprint2_v1.md §4.
package handler

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/service/pipeline"
	"devops/internal/service/pipeline/ir"
	"devops/pkg/response"
)

// YAMLExportHandler turns a stored Pipeline into exportable YAML text.
type YAMLExportHandler struct {
	db *gorm.DB
}

// NewYAMLExportHandler wires the handler with its database handle.
func NewYAMLExportHandler(db *gorm.DB) *YAMLExportHandler {
	return &YAMLExportHandler{db: db}
}

// RegisterRoutes attaches the single read endpoint.
func (h *YAMLExportHandler) RegisterRoutes(r *gin.RouterGroup) {
	r.GET("/pipeline/:id/yaml", h.ExportYAML)
}

// ExportYAML godoc
// @Summary 导出流水线为 YAML 文本
// @Description DB → IR → YAML。include_layout=true 时附带设计器画布坐标。Content-Type 是 text/yaml，body 是 raw YAML。
// @Tags 流水线
// @Param id path int true "Pipeline ID"
// @Param include_layout query bool false "是否包含 __layout 字段（默认 false）"
// @Produce text/yaml
// @Success 200 {string} string "YAML body"
// @Failure 400 {object} response.Response "id 非数字或 include_layout 非 bool"
// @Failure 404 {object} response.Response "pipeline 不存在"
// @Failure 500 {object} response.Response "DB → IR 转换失败 / YAML 序列化失败"
// @Router /pipeline/{id}/yaml [get]
// @Security BearerAuth
func (h *YAMLExportHandler) ExportYAML(c *gin.Context) {
	idStr := c.Param("id")
	id64, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id64 == 0 {
		response.BadRequest(c, "pipeline id 必须为正整数")
		return
	}

	includeLayout, err := parseBoolQuery(c, "include_layout", false)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	var p models.Pipeline
	if err := h.db.WithContext(c).First(&p, uint(id64)).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			response.NotFound(c, "pipeline 不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	irPipeline, err := pipeline.IRFromDB(&p)
	if err != nil {
		// IR_BUILD_FAILED per contract — but we don't have a dedicated
		// app-error code, so fall through to InternalError with a
		// human-readable prefix the frontend can match on.
		response.InternalError(c, "IR_BUILD_FAILED: "+err.Error())
		return
	}

	yamlBytes, err := ir.MarshalYAML(irPipeline, ir.MarshalOptions{IncludeLayout: includeLayout})
	if err != nil {
		response.InternalError(c, "YAML_MARSHAL_FAILED: "+err.Error())
		return
	}

	filename := sanitizeFilename(p.Name) + ".yaml"
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", filename))
	c.Data(http.StatusOK, "text/yaml; charset=utf-8", yamlBytes)
}

// parseBoolQuery reads a query param as bool with a default. Returns an
// error for non-bool values (e.g., include_layout=foo) so the client
// learns it's a typo, not silent default-fallback.
func parseBoolQuery(c *gin.Context, key string, def bool) (bool, error) {
	raw := c.Query(key)
	if raw == "" {
		return def, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return def, fmt.Errorf("query param %q 必须为 true/false/1/0", key)
	}
	return v, nil
}

// sanitizeFilename replaces characters that complicate Content-Disposition
// quoting. Keeps the YAML readable for casual download. ASCII-only by
// design (UTF-8 in headers needs RFC 5987, overkill for V1).
func sanitizeFilename(name string) string {
	if name == "" {
		return "pipeline"
	}
	repl := strings.NewReplacer(
		" ", "_",
		"/", "_",
		"\\", "_",
		"\"", "",
		"\n", "",
		"\r", "",
		"\t", "_",
	)
	out := repl.Replace(name)
	if out == "" {
		return "pipeline"
	}
	return out
}
