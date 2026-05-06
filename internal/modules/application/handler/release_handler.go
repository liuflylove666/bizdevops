// Package handler
//
// release_handler.go: Release 聚合根的 HTTP 接入层（v1 + v2）。
//
// v1 端点（/api/v1/releases/*）保留与现有前端的兼容性。
// v2 端点（/api/v2/releases/*）暴露 GitOps PR、风险评分等 v2 能力。
package handler

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
	"devops/internal/service/changelog"
	releasesvc "devops/internal/service/release"
	"devops/internal/types"
	"devops/pkg/dto"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ReleaseHandler", &ReleaseApiHandler{})
}

// ReleaseApiHandler IoC 入口。
type ReleaseApiHandler struct {
	handler *ReleaseHandler
}

func (h *ReleaseApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	releaseRepo := appRepo.NewReleaseRepository(db)
	itemRepo := appRepo.NewReleaseItemRepository(db)
	nrRepo := appRepo.NewNacosReleaseRepository(db)
	changeEventRepo := appRepo.NewChangeEventRepository(db)
	logSvc := changelog.NewService(changeEventRepo)

	svc := releasesvc.NewService(db, releaseRepo, itemRepo, nrRepo, logSvc).
		WithRiskScorer(releasesvc.NewRiskScorer())
	gitopsPR := releasesvc.NewGitOpsPRService(db, releaseRepo, itemRepo)
	gateSvc := releasesvc.NewGateService(db)
	h.handler = &ReleaseHandler{svc: svc, gitopsPR: gitopsPR, gateSvc: gateSvc}

	// v1 路由：保持向后兼容
	v1 := cfg.Application.GinRootRouter().Group("releases")
	v1.Use(middleware.AuthMiddleware())
	h.RegisterV1(v1)

	delivery := cfg.Application.GinRootRouter().Group("delivery")
	delivery.Use(middleware.AuthMiddleware())
	delivery.POST("/pipeline-runs/:runId/release", h.handler.CreateFromPipelineRun)

	// v2 路由：暴露 GitOps PR / risk score 等 v2 能力
	server := cfg.Application.GinServer()
	v2 := server.Group("/app/api/v2/releases")
	v2.Use(middleware.AuthMiddleware())
	h.RegisterV2(v2)
	return nil
}

// RegisterV1 注册兼容老前端的 CRUD + 状态机端点。
func (h *ReleaseApiHandler) RegisterV1(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/:id/overview", h.handler.GetOverview)
	r.GET("/:id/gates", h.handler.GetGates)
	r.GET("/:id", h.handler.Get)
	r.POST("", h.handler.Create)
	r.PUT("/:id", h.handler.Update)
	r.DELETE("/:id", h.handler.Delete)

	r.GET("/:id/items", h.handler.ListItems)
	r.POST("/:id/items", h.handler.AddItem)
	r.DELETE("/:id/items/:itemId", h.handler.RemoveItem)

	r.POST("/:id/submit", h.handler.Submit)
	r.POST("/:id/approve", h.handler.Approve)
	r.POST("/:id/reject", h.handler.Reject)
	r.POST("/:id/publish", h.handler.Publish)
	r.POST("/:id/gates/refresh", h.handler.RefreshGates)

	// v2.0 能力：同时挂在 v1 路径下，简化前端 baseURL 处理
	r.POST("/:id/gitops-pr", h.handler.OpenGitOpsPR)
	r.POST("/:id/gitops-pr/dry-run", h.handler.DryRunGitOpsPR)
}

// RegisterV2 注册 v2 新增端点。
func (h *ReleaseApiHandler) RegisterV2(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/:id/overview", h.handler.GetOverview)
	r.GET("/:id/gates", h.handler.GetGates)
	r.GET("/:id", h.handler.Get)
	// GitOps PR：dry_run 通过 query 区分
	r.POST("/:id/gitops-pr", h.handler.OpenGitOpsPR)
	r.POST("/:id/gitops-pr/dry-run", h.handler.DryRunGitOpsPR)
}

// ReleaseHandler 业务方法集合。
type ReleaseHandler struct {
	svc      *releasesvc.Service
	gitopsPR *releasesvc.GitOpsPRService
	gateSvc  *releasesvc.GateService
}

// ---------- v1：CRUD + 状态机 ----------

func (h *ReleaseHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(firstQuery(c, "20", "pageSize", "page_size"))

	filter := appRepo.ReleaseFilter{
		Env:    c.Query("env"),
		Status: c.Query("status"),
		Title:  c.Query("title"),
	}
	if projectIDStr := c.Query("project_id"); projectIDStr != "" {
		if v, err := strconv.ParseUint(projectIDStr, 10, 64); err == nil {
			id := uint(v)
			filter.ProjectID = &id
		}
	}
	if appIDStr := c.Query("application_id"); appIDStr != "" {
		if v, err := strconv.ParseUint(appIDStr, 10, 64); err == nil {
			id := uint(v)
			filter.ApplicationID = &id
		}
	}

	list, total, err := h.svc.List(filter, page, pageSize)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "查询发布列表失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"list":      list,
		"items":     list,
		"total":     total,
		"page":      page,
		"pageSize":  pageSize,
		"page_size": pageSize,
	})
}

func (h *ReleaseHandler) Get(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	rel, err := h.svc.GetByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "发布主单不存在: "+err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) GetOverview(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	overview, err := h.svc.GetOverview(c.Request.Context(), id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "发布总览不存在: "+err.Error())
		return
	}
	response.Success(c, overview)
}

func (h *ReleaseHandler) GetGates(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	gates, err := h.gateSvc.Evaluate(c.Request.Context(), id, false)
	if err != nil {
		response.Error(c, http.StatusNotFound, "发布 Gate 不存在: "+err.Error())
		return
	}
	response.Success(c, gates)
}

func (h *ReleaseHandler) Create(c *gin.Context) {
	var req dto.CreateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if !deploy.IsValidRolloutStrategy(req.RolloutStrategy) {
		response.Error(c, http.StatusBadRequest, "rollout_strategy 必须为 direct/canary/blue_green")
		return
	}
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)

	rel := &deploy.Release{
		Title:           req.Title,
		ApplicationID:   req.ApplicationID,
		ApplicationName: req.ApplicationName,
		Env:             req.Env,
		Version:         req.Version,
		Description:     req.Description,
		RiskLevel:       req.RiskLevel,
		RolloutStrategy: defaultStrategy(req.RolloutStrategy),
		RolloutConfig:   types.JSONMap(req.RolloutConfig),
		JiraIssueKeys:   strings.Join(req.JiraIssueKeys, ","),
		CreatedBy:       userID,
		CreatedByName:   username,
	}
	if err := h.svc.Create(rel); err != nil {
		response.Error(c, http.StatusInternalServerError, "创建发布主单失败: "+err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) CreateFromPipelineRun(c *gin.Context) {
	var req dto.CreateReleaseFromPipelineRunRequest
	_ = c.ShouldBindJSON(&req)
	if runIDStr := c.Param("runId"); runIDStr != "" {
		runID, err := strconv.ParseUint(runIDStr, 10, 64)
		if err != nil || runID == 0 {
			response.Error(c, http.StatusBadRequest, "无效的 runId")
			return
		}
		req.PipelineRunID = uint(runID)
	}
	if req.PipelineRunID == 0 {
		response.Error(c, http.StatusBadRequest, "pipeline_run_id 不能为空")
		return
	}
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	rel, err := h.svc.CreateFromPipelineRun(c.Request.Context(), &req, userID, username)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) Update(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req dto.UpdateReleaseRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if req.RolloutStrategy != "" && !deploy.IsValidRolloutStrategy(req.RolloutStrategy) {
		response.Error(c, http.StatusBadRequest, "rollout_strategy 不合法")
		return
	}
	existing, err := h.svc.GetByID(id)
	if err != nil {
		response.Error(c, http.StatusNotFound, "发布主单不存在: "+err.Error())
		return
	}
	if req.Title != "" {
		existing.Title = req.Title
	}
	existing.Description = req.Description
	if req.RiskLevel != "" {
		existing.RiskLevel = req.RiskLevel
	}
	if req.RolloutStrategy != "" {
		existing.RolloutStrategy = req.RolloutStrategy
	}
	if req.RolloutConfig != nil {
		existing.RolloutConfig = types.JSONMap(req.RolloutConfig)
	}
	if req.JiraIssueKeys != nil {
		existing.JiraIssueKeys = strings.Join(req.JiraIssueKeys, ",")
	}
	if err := h.svc.Update(existing); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, existing)
}

func (h *ReleaseHandler) Delete(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.Delete(id); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ReleaseHandler) ListItems(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	items, err := h.svc.ListItems(id)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, err.Error())
		return
	}
	response.Success(c, items)
}

func (h *ReleaseHandler) AddItem(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		ItemType  string `json:"item_type" binding:"required"`
		ItemID    uint   `json:"item_id" binding:"required"`
		ItemTitle string `json:"item_title"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}
	if err := h.svc.AddItem(id, req.ItemType, req.ItemID, req.ItemTitle); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ReleaseHandler) RemoveItem(c *gin.Context) {
	itemID, ok := parseIDParam(c, "itemId")
	if !ok {
		return
	}
	if err := h.svc.RemoveItem(itemID); err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, nil)
}

func (h *ReleaseHandler) Submit(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	rel, err := h.svc.SubmitForApproval(id)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) Approve(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	rel, err := h.svc.Approve(id, userID, username)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) Reject(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req struct {
		Reason string `json:"reason"`
	}
	_ = c.ShouldBindJSON(&req)
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	rel, err := h.svc.Reject(id, userID, username, req.Reason)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) Publish(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	gates, err := h.gateSvc.Evaluate(c.Request.Context(), id, true)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "发布 Gate 评估失败: "+err.Error())
		return
	}
	if gates.Blocked {
		response.Error(c, http.StatusBadRequest, "发布 Gate 未通过: "+strings.Join(gates.BlockReasons, "；"))
		return
	}
	userID, _ := middleware.GetUserID(c)
	username, _ := middleware.GetUsername(c)
	rel, err := h.svc.Publish(id, userID, username)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, rel)
}

func (h *ReleaseHandler) RefreshGates(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	gates, err := h.gateSvc.Evaluate(c.Request.Context(), id, true)
	if err != nil {
		response.Error(c, http.StatusBadRequest, "发布 Gate 刷新失败: "+err.Error())
		return
	}
	response.Success(c, gates)
}

// ---------- v2：GitOps PR ----------

func (h *ReleaseHandler) OpenGitOpsPR(c *gin.Context) {
	h.handleGitOpsPR(c, false)
}

func (h *ReleaseHandler) DryRunGitOpsPR(c *gin.Context) {
	h.handleGitOpsPR(c, true)
}

func (h *ReleaseHandler) handleGitOpsPR(c *gin.Context, forceDryRun bool) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}
	var req dto.GitOpsPRRequest
	_ = c.ShouldBindJSON(&req)
	req.ReleaseID = id
	if forceDryRun {
		req.DryRun = true
	}

	userID, exists := middleware.GetUserID(c)
	var operatorID *uint
	if exists {
		operatorID = &userID
	}

	resp, err := h.gitopsPR.OpenPR(c.Request.Context(), &req, operatorID)
	if err != nil {
		response.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	response.Success(c, resp)
}

// ---------- helpers ----------

func parseIDParam(c *gin.Context, name string) (uint, bool) {
	idStr := c.Param(name)
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil || id == 0 {
		response.Error(c, http.StatusBadRequest, "无效的 "+name)
		return 0, false
	}
	return uint(id), true
}

func defaultStrategy(s string) string {
	if strings.TrimSpace(s) == "" {
		return deploy.RolloutStrategyDirect
	}
	return s
}

func firstQuery(c *gin.Context, dft string, names ...string) string {
	for _, name := range names {
		if value := strings.TrimSpace(c.Query(name)); value != "" {
			return value
		}
	}
	return dft
}
