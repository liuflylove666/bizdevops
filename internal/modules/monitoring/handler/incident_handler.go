// Package handler
//
// incident_handler.go: 生产事故 HTTP 接入层（v2.1）。
//
// 端点：
//   GET    /api/v1/incidents
//   GET    /api/v1/incidents/:id
//   POST   /api/v1/incidents
//   PUT    /api/v1/incidents/:id
//   DELETE /api/v1/incidents/:id
//   POST   /api/v1/incidents/:id/mitigate
//   POST   /api/v1/incidents/:id/resolve
package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/monitoring"
	monRepo "devops/internal/modules/monitoring/repository"
	"devops/internal/service/incident"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("IncidentHandler", &IncidentApiHandler{})
}

// IncidentApiHandler IoC 入口。
type IncidentApiHandler struct {
	handler *IncidentHandler
}

func (h *IncidentApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	incidentRepo := monRepo.NewIncidentRepository(db)
	svc := incident.NewService(incidentRepo)
	h.handler = &IncidentHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("incidents")
	root.Use(middleware.AuthMiddleware())
	root.GET("", h.handler.List)
	root.GET("/:id", h.handler.Get)
	root.POST("", h.handler.Create)
	root.PUT("/:id", h.handler.Update)
	root.DELETE("/:id", h.handler.Delete)
	root.POST("/:id/mitigate", h.handler.Mitigate)
	root.POST("/:id/resolve", h.handler.Resolve)
	root.GET("/:id/postmortem", h.handler.Postmortem)                     // v2.2: 导出 Markdown 复盘
	return nil
}

// IncidentHandler 业务方法。
type IncidentHandler struct {
	svc *incident.Service
}

func (h *IncidentHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	f := monRepo.IncidentFilter{
		Env:      c.Query("env"),
		Status:   c.Query("status"),
		Severity: c.Query("severity"),
		Keyword:  c.Query("keyword"),
	}
	if v := c.Query("application_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			id := uint(n)
			f.AppID = &id
		}
	}
	if v := c.Query("project_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			id := uint(n)
			f.ProjectID = &id
		}
	}
	if v := c.Query("release_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			id := uint(n)
			f.ReleaseID = &id
		}
	}
	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.From = &t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			f.To = &t
		}
	}
	list, total, err := h.svc.List(f, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "查询事故列表失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":     list,
		"total":    total,
		"page":     page,
		"pageSize": pageSize,
	})
}

func (h *IncidentHandler) Get(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	inc, err := h.svc.GetByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "事故不存在: "+err.Error())
		return
	}
	response.Success(c, inc)
}

func (h *IncidentHandler) Create(c *gin.Context) {
	var req struct {
		Title            string  `json:"title" binding:"required"`
		Description      string  `json:"description"`
		ApplicationID    *uint   `json:"application_id"`
		ApplicationName  string  `json:"application_name"`
		Env              string  `json:"env"`
		Severity         string  `json:"severity"`
		DetectedAt       *string `json:"detected_at"`
		Source           string  `json:"source"`
		ReleaseID        *uint   `json:"release_id"`
		AlertFingerprint string  `json:"alert_fingerprint"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	in := &incident.CreateInput{
		Title:            req.Title,
		Description:      req.Description,
		ApplicationID:    req.ApplicationID,
		ApplicationName:  req.ApplicationName,
		Env:              req.Env,
		Severity:         req.Severity,
		Source:           req.Source,
		ReleaseID:        req.ReleaseID,
		AlertFingerprint: req.AlertFingerprint,
		CreatedBy:        userID,
		CreatedByName:    username,
	}
	if req.DetectedAt != nil && *req.DetectedAt != "" {
		if t, err := time.Parse(time.RFC3339, *req.DetectedAt); err == nil {
			in.DetectedAt = &t
		}
	}
	inc, err := h.svc.Create(in)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, inc)
}

func (h *IncidentHandler) Update(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	inc, err := h.svc.GetByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "事故不存在: "+err.Error())
		return
	}
	var req struct {
		Title         string `json:"title"`
		Description   string `json:"description"`
		Severity      string `json:"severity"`
		PostmortemURL string `json:"postmortem_url"`
		RootCause     string `json:"root_cause"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if req.Title != "" {
		inc.Title = req.Title
	}
	inc.Description = req.Description
	if req.Severity != "" {
		if !monitoring.IsValidSeverity(req.Severity) {
			response.Error(c, http.StatusBadRequest, "severity 不合法")
			return
		}
		inc.Severity = req.Severity
	}
	inc.PostmortemURL = req.PostmortemURL
	inc.RootCause = req.RootCause
	if err := h.repoUpdate(inc); err != nil {
		response.Error(c, http.StatusInternalServerError, "更新失败: "+err.Error())
		return
	}
	response.Success(c, inc)
}

func (h *IncidentHandler) Delete(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *IncidentHandler) Mitigate(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	userID, _ := middleware.GetUserID(c)
	inc, err := h.svc.Mitigate(id, userID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, inc)
}

func (h *IncidentHandler) Resolve(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	var req struct {
		RootCause     string `json:"root_cause"`
		PostmortemURL string `json:"postmortem_url"`
	}
	_ = c.ShouldBindJSON(&req)
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	inc, err := h.svc.Resolve(id, &incident.ResolveInput{
		RootCause:      req.RootCause,
		PostmortemURL:  req.PostmortemURL,
		ResolvedBy:     userID,
		ResolvedByName: username,
	})
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, inc)
}

// Postmortem 导出事故复盘 Markdown（v2.2）。
//
// 默认 format=md，响应直接以附件形式下载；format=json 则包裹在 JSON 中返回
// （便于前端做预览）。文件名形如 `incident-<id>-postmortem.md`。
func (h *IncidentHandler) Postmortem(c *gin.Context) {
	id, ok := parseIncidentID(c)
	if !ok {
		return
	}
	inc, err := h.svc.GetByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "事故不存在: "+err.Error())
		return
	}
	md := incident.ExportPostmortemMarkdown(inc)

	switch c.DefaultQuery("format", "md") {
	case "json":
		response.Success(c, gin.H{
			"id":       inc.ID,
			"title":    inc.Title,
			"markdown": md,
		})
	default:
		filename := fmt.Sprintf("incident-%d-postmortem.md", inc.ID)
		c.Header("Content-Type", "text/markdown; charset=utf-8")
		c.Header("Content-Disposition", `attachment; filename="`+filename+`"`)
		_, _ = c.Writer.Write([]byte(md))
	}
}

// repoUpdate 通过 service 层间接调用 repo.Update（service 层未直接暴露 Update，
// 这里为 PUT 端点提供专属通道，避免污染 service 接口）。
func (h *IncidentHandler) repoUpdate(inc *monitoring.Incident) error {
	cfg, _ := config.LoadConfig()
	repo := monRepo.NewIncidentRepository(cfg.GetDB())
	return repo.Update(inc)
}

func parseIncidentID(c *gin.Context) (uint, bool) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "无效的 id")
		return 0, false
	}
	return uint(id), true
}
