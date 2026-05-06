package handler

import (
	"strconv"

	"devops/internal/config"
	modelbiz "devops/internal/models/biz"
	bizrepo "devops/internal/modules/biz/repository"
	coreRepo "devops/internal/repository"
	bizsvc "devops/internal/service/biz"
	"devops/internal/service/jira"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"

	"github.com/gin-gonic/gin"
)

func init() {
	ioc.Api.RegisterContainer("BizPlanningHandler", &PlanningApiHandler{})
}

type PlanningApiHandler struct {
	handler *PlanningHandler
}

type PlanningHandler struct {
	svc *bizsvc.PlanningService
}

func (h *PlanningApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}
	db := cfg.GetDB()
	goalRepo := bizrepo.NewBizGoalRepository(db)
	reqRepo := bizrepo.NewBizRequirementRepository(db)
	verRepo := bizrepo.NewBizVersionRepository(db)
	jiraInstanceRepo := coreRepo.NewJiraInstanceRepository(db)
	jiraMappingRepo := coreRepo.NewJiraProjectMappingRepository(db)
	jiraSvc := jira.NewService(jiraInstanceRepo, jiraMappingRepo)
	h.handler = &PlanningHandler{
		svc: bizsvc.NewPlanningService(db, goalRepo, reqRepo, verRepo, jiraMappingRepo, jiraSvc),
	}

	root := cfg.Application.GinRootRouter().Group("biz")
	root.Use(middleware.AuthMiddleware())
	h.handler.Register(root)
	return nil
}

func (h *PlanningHandler) Register(r gin.IRouter) {
	r.GET("/planning/source", h.GetPlanningSource)
	r.POST("/planning/jira/webhook", h.JiraWebhook)

	goals := r.Group("/goals")
	{
		goals.GET("", h.ListGoals)
		goals.GET("/:id", h.GetGoal)
		goals.POST("", h.CreateGoal)
		goals.PUT("/:id", h.UpdateGoal)
		goals.DELETE("/:id", h.DeleteGoal)
	}

	requirements := r.Group("/requirements")
	{
		requirements.GET("", h.ListRequirements)
		requirements.GET("/:id", h.GetRequirement)
		requirements.POST("", h.CreateRequirement)
		requirements.PUT("/:id", h.UpdateRequirement)
		requirements.DELETE("/:id", h.DeleteRequirement)
	}

	versions := r.Group("/versions")
	{
		versions.GET("", h.ListVersions)
		versions.GET("/:id", h.GetVersion)
		versions.POST("", h.CreateVersion)
		versions.PUT("/:id", h.UpdateVersion)
		versions.DELETE("/:id", h.DeleteVersion)
	}
}

func parseUintParam(c *gin.Context, name string) (uint, bool) {
	id, err := strconv.ParseUint(c.Param(name), 10, 64)
	if err != nil {
		response.ParamIDError(c, name)
		return 0, false
	}
	return uint(id), true
}

func (h *PlanningHandler) GetPlanningSource(c *gin.Context) {
	response.Success(c, gin.H{"source": "jira"})
}

func (h *PlanningHandler) JiraWebhook(c *gin.Context) {
	var payload bizsvc.JiraWebhookPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	instanceID := parseOptionalUintQuery(c, "instance_id")
	action, err := h.svc.HandleJiraWebhook(&payload, instanceID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"action":    action,
		"issue_key": payload.Issue.Key,
	})
}

func (h *PlanningHandler) allowBuiltinPlanningWrite(c *gin.Context) bool {
	response.Forbidden(c, "规划已切换为 Jira 权威源，请在 Jira 中编辑；本模块接口只读")
	return false
}

func parsePage(c *gin.Context) (int, int) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	return page, pageSize
}

func parseOptionalUintQuery(c *gin.Context, name string) *uint {
	val := c.Query(name)
	if val == "" {
		return nil
	}
	id, err := strconv.ParseUint(val, 10, 64)
	if err != nil {
		return nil
	}
	result := uint(id)
	return &result
}

func (h *PlanningHandler) ListGoals(c *gin.Context) {
	page, pageSize := parsePage(c)
	filter := bizrepo.GoalFilter{
		Status:  c.Query("status"),
		Keyword: c.Query("keyword"),
	}
	list, total, err := h.svc.ListGoals(filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *PlanningHandler) GetGoal(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetGoal(id)
	if err != nil {
		response.NotFound(c, "业务目标不存在")
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) CreateGoal(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	var item modelbiz.BizGoal
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.CreateGoal(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) UpdateGoal(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var item modelbiz.BizGoal
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	item.ID = id
	if err := h.svc.UpdateGoal(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) DeleteGoal(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteGoal(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *PlanningHandler) ListRequirements(c *gin.Context) {
	page, pageSize := parsePage(c)
	filter := bizrepo.RequirementFilter{
		Status:        c.Query("status"),
		Priority:      c.Query("priority"),
		Source:        c.Query("source"),
		ExternalKey:   c.Query("external_key"),
		JiraEpicKey:   c.Query("jira_epic_key"),
		JiraLabel:     c.Query("jira_label"),
		JiraComponent: c.Query("jira_component"),
		GoalID:        parseOptionalUintQuery(c, "goal_id"),
		VersionID:     parseOptionalUintQuery(c, "version_id"),
		Keyword:       c.Query("keyword"),
	}
	list, total, err := h.svc.ListRequirements(filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *PlanningHandler) GetRequirement(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetRequirement(id)
	if err != nil {
		response.NotFound(c, "需求不存在")
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) CreateRequirement(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	var item modelbiz.BizRequirement
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.CreateRequirement(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) UpdateRequirement(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var item modelbiz.BizRequirement
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	item.ID = id
	if err := h.svc.UpdateRequirement(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) DeleteRequirement(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteRequirement(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *PlanningHandler) ListVersions(c *gin.Context) {
	page, pageSize := parsePage(c)
	filter := bizrepo.VersionFilter{
		Status:  c.Query("status"),
		GoalID:  parseOptionalUintQuery(c, "goal_id"),
		Keyword: c.Query("keyword"),
	}
	list, total, err := h.svc.ListVersions(filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *PlanningHandler) GetVersion(c *gin.Context) {
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	item, err := h.svc.GetVersion(id)
	if err != nil {
		response.NotFound(c, "版本计划不存在")
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) CreateVersion(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	var item modelbiz.BizVersion
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.CreateVersion(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) UpdateVersion(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	var item modelbiz.BizVersion
	if err := c.ShouldBindJSON(&item); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	item.ID = id
	if err := h.svc.UpdateVersion(&item); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

func (h *PlanningHandler) DeleteVersion(c *gin.Context) {
	if !h.allowBuiltinPlanningWrite(c) {
		return
	}
	id, ok := parseUintParam(c, "id")
	if !ok {
		return
	}
	if err := h.svc.DeleteVersion(id); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}
