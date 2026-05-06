package handler

import (
	"context"
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models"
	inframodel "devops/internal/models/infrastructure"
	"devops/internal/repository"
	appsvc "devops/internal/service/application"
	"devops/pkg/dto"
	"devops/pkg/ioc"
	"devops/pkg/logger"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

type bindRepoRequest struct {
	GitRepoID uint   `json:"git_repo_id" binding:"required"`
	Role      string `json:"role"`
	IsDefault *bool  `json:"is_default"`
}

var appLog = logger.L().WithField("module", "application")

func init() {
	ioc.Api.RegisterContainer("ApplicationHandler", &ApplicationApiHandler{})
}

type ApplicationApiHandler struct {
	handler *ApplicationHandler
}

func (h *ApplicationApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewApplicationHandler(db)

	root := cfg.Application.GinRootRouter().Group("app")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)

	// 添加 /applications 路由别名，兼容前端请求
	appRoot := cfg.Application.GinRootRouter().Group("applications")
	appRoot.Use(middleware.AuthMiddleware())
	h.Register(appRoot)

	return nil
}

func (h *ApplicationApiHandler) Register(r gin.IRouter) {
	// 查看权限 - 所有登录用户可访问
	r.GET("", h.handler.ListApplications)
	r.GET("/:id", h.handler.GetApplication)
	r.GET("/:id/envs", h.handler.ListAppEnvs)
	r.GET("/:id/readiness", h.handler.GetReadiness)
	r.POST("/:id/readiness/run", h.handler.RefreshReadiness)
	r.GET("/:id/repo-bindings", h.handler.ListRepoBindings)
	r.GET("/:id/delivery-records", h.handler.ListDeliveryRecords)
	r.GET("/delivery-records", h.handler.ListAllDeliveryRecords)
	r.GET("/stats", h.handler.GetStats)
	r.GET("/teams", h.handler.GetTeams)

	// 管理权限 - 需要管理员
	r.POST("/onboarding", middleware.RequireAdmin(), h.handler.SaveOnboarding)
	r.POST("", middleware.RequireAdmin(), h.handler.CreateApplication)
	r.PUT("/:id", middleware.RequireAdmin(), h.handler.UpdateApplication)
	r.DELETE("/:id", middleware.RequireAdmin(), h.handler.DeleteApplication)
	r.POST("/:id/repo-bindings", middleware.RequireAdmin(), h.handler.BindRepo)
	r.PUT("/:id/repo-bindings/:bindingId/default", middleware.RequireAdmin(), h.handler.SetDefaultRepoBinding)
	r.DELETE("/:id/repo-bindings/:bindingId", middleware.RequireAdmin(), h.handler.DeleteRepoBinding)
	r.POST("/:id/envs", middleware.RequireAdmin(), h.handler.CreateAppEnv)
	r.PUT("/:id/envs/:envId", middleware.RequireAdmin(), h.handler.UpdateAppEnv)
	r.DELETE("/:id/envs/:envId", middleware.RequireAdmin(), h.handler.DeleteAppEnv)
}

type ApplicationHandler struct {
	appRepo      *repository.ApplicationRepository
	bindingRepo  *repository.ApplicationRepoBindingRepository
	envRepo      *repository.ApplicationEnvRepository
	deployRepo   *repository.DeployRecordRepository
	readinessSvc *appsvc.ReadinessService
	onboardSvc   *appsvc.OnboardingService
	db           *gorm.DB
}

func NewApplicationHandler(db *gorm.DB) *ApplicationHandler {
	return &ApplicationHandler{
		appRepo:      repository.NewApplicationRepository(db),
		bindingRepo:  repository.NewApplicationRepoBindingRepository(db),
		envRepo:      repository.NewApplicationEnvRepository(db),
		deployRepo:   repository.NewDeployRecordRepository(db),
		readinessSvc: appsvc.NewReadinessService(db),
		onboardSvc:   appsvc.NewOnboardingService(db),
		db:           db,
	}
}

// ListApplications godoc
// @Summary 获取应用列表
// @Description 分页获取应用列表，支持按名称、团队、状态、语言筛选
// @Tags 应用管理
// @Accept json
// @Produce json
// @Param name query string false "应用名称"
// @Param team query string false "团队"
// @Param status query string false "状态"
// @Param language query string false "开发语言"
// @Param page query int false "页码" default(1)
// @Param page_size query int false "每页数量" default(20)
// @Success 200 {object} response.Response{data=response.PageData} "成功"
// @Failure 500 {object} response.Response "服务器错误"
// @Security BearerAuth
// @Router /app [get]
func (h *ApplicationHandler) ListApplications(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	orgID, _ := strconv.ParseUint(c.Query("organization_id"), 10, 64)
	projID, _ := strconv.ParseUint(c.Query("project_id"), 10, 64)
	filter := repository.ApplicationFilter{
		Name:           c.Query("name"),
		Team:           c.Query("team"),
		Status:         c.Query("status"),
		Language:       c.Query("language"),
		OrganizationID: uint(orgID),
		ProjectID:      uint(projID),
	}

	apps, total, err := h.appRepo.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Page(c, apps, total, page, pageSize)
}

// GetApplication godoc
// @Summary 获取应用详情
// @Description 根据ID获取应用详情及环境配置
// @Tags 应用管理
// @Accept json
// @Produce json
// @Param id path int true "应用ID"
// @Success 200 {object} response.Response{data=object{app=models.Application,envs=[]models.ApplicationEnv}} "成功"
// @Failure 400 {object} response.Response "参数错误"
// @Failure 404 {object} response.Response "应用不存在"
// @Security BearerAuth
// @Router /app/{id} [get]
func (h *ApplicationHandler) GetApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	app, err := h.appRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}
	h.enrichApplicationNames(c.Request.Context(), app)

	envs, err := h.envRepo.GetByAppID(c.Request.Context(), uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	bindings, err := h.bindingRepo.List(c.Request.Context(), uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	var defaultBinding *models.ApplicationRepoBinding
	for i := range bindings {
		if bindings[i].IsDefault {
			defaultBinding = &bindings[i]
			break
		}
	}
	if defaultBinding == nil && len(bindings) > 0 {
		defaultBinding = &bindings[0]
	}

	response.Success(c, gin.H{
		"app":                  app,
		"envs":                 envs,
		"repo_bindings":        bindings,
		"default_repo_binding": defaultBinding,
	})
}

func (h *ApplicationHandler) enrichApplicationNames(ctx context.Context, app *models.Application) {
	if app == nil {
		return
	}
	if app.OrganizationID != nil && *app.OrganizationID > 0 {
		var org models.Organization
		if err := h.db.WithContext(ctx).First(&org, *app.OrganizationID).Error; err == nil {
			app.OrgName = org.DisplayName
			if app.OrgName == "" {
				app.OrgName = org.Name
			}
		}
	}
	if app.ProjectID != nil && *app.ProjectID > 0 {
		var proj models.Project
		if err := h.db.WithContext(ctx).First(&proj, *app.ProjectID).Error; err == nil {
			app.ProjectName = proj.DisplayName
			if app.ProjectName == "" {
				app.ProjectName = proj.Name
			}
		}
	}
}

func (h *ApplicationHandler) GetReadiness(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	result, err := h.readinessSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "应用不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ApplicationHandler) RefreshReadiness(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	result, err := h.readinessSvc.Refresh(c.Request.Context(), uint(id))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "应用不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ApplicationHandler) SaveOnboarding(c *gin.Context) {
	var req dto.ApplicationOnboardingRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	result, err := h.onboardSvc.Save(c.Request.Context(), &req, userID)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *ApplicationHandler) CreateApplication(c *gin.Context) {
	var app models.Application
	if err := c.ShouldBindJSON(&app); err != nil {
		appLog.WithError(err).Warn("创建应用参数错误")
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	app.ID = 0
	if userID, ok := middleware.GetUserID(c); ok {
		app.CreatedBy = &userID
	}

	if err := h.appRepo.Create(c.Request.Context(), &app); err != nil {
		appLog.WithError(err).Error("创建应用失败: %s", app.Name)
		response.InternalError(c, "创建失败: "+err.Error())
		return
	}

	response.Success(c, app)
}

func (h *ApplicationHandler) UpdateApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var app models.Application
	if err := c.ShouldBindJSON(&app); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	app.ID = uint(id)
	if err := h.appRepo.Update(c.Request.Context(), &app); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, app)
}

func (h *ApplicationHandler) DeleteApplication(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	if err := h.appRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c)
}

func (h *ApplicationHandler) ListAppEnvs(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	envs, err := h.envRepo.GetByAppID(c.Request.Context(), uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, envs)
}

func (h *ApplicationHandler) ListRepoBindings(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	items, err := h.bindingRepo.List(c.Request.Context(), uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, items)
}

func (h *ApplicationHandler) BindRepo(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	if _, err := h.appRepo.GetByID(c.Request.Context(), uint(id)); err != nil {
		response.NotFound(c, "应用不存在")
		return
	}

	var req bindRepoRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	role := strings.TrimSpace(req.Role)
	if role == "" {
		role = "primary"
	}
	binding := &models.ApplicationRepoBinding{
		ApplicationID: uint(id),
		GitRepoID:     req.GitRepoID,
		Role:          role,
	}
	if req.IsDefault != nil {
		binding.IsDefault = *req.IsDefault
	}
	if userID, ok := middleware.GetUserID(c); ok {
		binding.CreatedBy = &userID
	}

	if err := h.bindingRepo.Bind(c.Request.Context(), binding); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, binding)
}

func (h *ApplicationHandler) SetDefaultRepoBinding(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	bindingID, err := strconv.ParseUint(c.Param("bindingId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "绑定ID格式错误")
		return
	}

	if err := h.bindingRepo.SetDefault(c.Request.Context(), uint(appID), uint(bindingID)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "仓库绑定不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c)
}

func (h *ApplicationHandler) DeleteRepoBinding(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	bindingID, err := strconv.ParseUint(c.Param("bindingId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "绑定ID格式错误")
		return
	}

	if err := h.bindingRepo.Delete(c.Request.Context(), uint(appID), uint(bindingID)); err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "仓库绑定不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c)
}

func (h *ApplicationHandler) CreateAppEnv(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	var env models.ApplicationEnv
	if err := c.ShouldBindJSON(&env); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	env.ID = 0
	env.ApplicationID = uint(id)
	if err := h.normalizeAppEnv(c.Request.Context(), &env); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	if err := h.envRepo.Create(c.Request.Context(), &env); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, env)
}

func (h *ApplicationHandler) UpdateAppEnv(c *gin.Context) {
	appID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	envId, err := strconv.ParseUint(c.Param("envId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "环境ID格式错误")
		return
	}

	existing, err := h.envRepo.GetByID(c.Request.Context(), uint(envId))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.NotFound(c, "环境不存在")
			return
		}
		response.InternalError(c, err.Error())
		return
	}
	if existing.ApplicationID != uint(appID) {
		response.NotFound(c, "环境不存在")
		return
	}

	var env models.ApplicationEnv
	if err := c.ShouldBindJSON(&env); err != nil {
		response.BadRequest(c, err.Error())
		return
	}

	env.ID = uint(envId)
	env.ApplicationID = uint(appID)
	if err := h.normalizeAppEnv(c.Request.Context(), &env); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if err := h.envRepo.Update(c.Request.Context(), &env); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, env)
}

func (h *ApplicationHandler) normalizeAppEnv(ctx context.Context, env *models.ApplicationEnv) error {
	if env == nil {
		return fmt.Errorf("环境配置不能为空")
	}
	env.EnvName = strings.TrimSpace(env.EnvName)
	if env.EnvName == "" {
		return fmt.Errorf("请选择环境")
	}
	var app models.Application
	if err := h.db.WithContext(ctx).First(&app, env.ApplicationID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return fmt.Errorf("应用不存在")
		}
		return err
	}
	env.Branch = strings.TrimSpace(env.Branch)
	env.GitOpsBranch = strings.TrimSpace(env.GitOpsBranch)
	env.GitOpsPath = cleanRelativePath(env.GitOpsPath)
	env.HelmChartPath = cleanRelativePath(env.HelmChartPath)
	env.HelmValuesPath = cleanRelativePath(env.HelmValuesPath)
	env.HelmReleaseName = strings.TrimSpace(env.HelmReleaseName)
	env.K8sNamespace = strings.TrimSpace(env.K8sNamespace)
	env.K8sDeployment = strings.TrimSpace(env.K8sDeployment)
	env.CPURequest = strings.TrimSpace(env.CPURequest)
	env.CPULimit = strings.TrimSpace(env.CPULimit)
	env.MemoryRequest = strings.TrimSpace(env.MemoryRequest)
	env.MemoryLimit = strings.TrimSpace(env.MemoryLimit)
	env.Config = strings.TrimSpace(env.Config)
	if env.Replicas <= 0 {
		env.Replicas = 1
	}
	if env.GitOpsRepoID != nil && *env.GitOpsRepoID == 0 {
		env.GitOpsRepoID = nil
	}
	if env.ArgoCDApplicationID != nil && *env.ArgoCDApplicationID == 0 {
		env.ArgoCDApplicationID = nil
	}

	if env.GitOpsRepoID != nil {
		var found inframodel.GitOpsRepo
		if err := h.db.WithContext(ctx).First(&found, *env.GitOpsRepoID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("GitOps 部署仓库不存在")
			}
			return err
		}
		if env.GitOpsBranch == "" {
			env.GitOpsBranch = strings.TrimSpace(found.Branch)
		}
		if env.GitOpsPath == "" {
			env.GitOpsPath = cleanRelativePath(found.Path)
		}
	}
	if env.ArgoCDApplicationID != nil {
		var argoApp inframodel.ArgoCDApplication
		if err := h.db.WithContext(ctx).First(&argoApp, *env.ArgoCDApplicationID).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return fmt.Errorf("ArgoCD 应用不存在")
			}
			return err
		}
		if env.K8sNamespace == "" {
			env.K8sNamespace = strings.TrimSpace(argoApp.DestNamespace)
		}
	}

	env.GitOpsBranch = firstNonEmptyAppEnv(env.GitOpsBranch, env.Branch, "main")
	if env.HelmChartPath == "" {
		env.HelmChartPath = env.GitOpsPath
	}
	if env.HelmValuesPath == "" {
		env.HelmValuesPath = defaultHelmValuesPath(env.GitOpsPath, env.EnvName)
	}
	if env.HelmReleaseName == "" {
		env.HelmReleaseName = firstNonEmptyAppEnv(env.K8sDeployment, app.Name, env.EnvName)
	}
	return nil
}

func cleanRelativePath(value string) string {
	value = strings.Trim(strings.TrimSpace(value), "/")
	if value == "" || value == "." {
		return ""
	}
	return path.Clean(value)
}

func defaultHelmValuesPath(basePath, envName string) string {
	envName = strings.TrimSpace(envName)
	if envName == "" {
		envName = "values"
	}
	basePath = cleanRelativePath(basePath)
	if basePath == "" {
		return path.Join("values", envName+".yaml")
	}
	return path.Join(basePath, "values", envName+".yaml")
}

func firstNonEmptyAppEnv(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (h *ApplicationHandler) DeleteAppEnv(c *gin.Context) {
	envId, err := strconv.ParseUint(c.Param("envId"), 10, 64)
	if err != nil {
		response.BadRequest(c, "环境ID格式错误")
		return
	}

	if err := h.envRepo.Delete(c.Request.Context(), uint(envId)); err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.OK(c)
}

func (h *ApplicationHandler) ListDeliveryRecords(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.DeployRecordFilter{
		ApplicationID: uint(id),
		EnvName:       c.Query("env"),
		Status:        c.Query("status"),
	}

	records, total, err := h.deployRepo.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Page(c, records, total, page, pageSize)
}

func (h *ApplicationHandler) ListAllDeliveryRecords(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	filter := repository.DeployRecordFilter{
		AppName: c.Query("app_name"),
		EnvName: c.Query("env"),
		Status:  c.Query("status"),
	}

	records, total, err := h.deployRepo.List(c.Request.Context(), filter, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Page(c, records, total, page, pageSize)
}

func (h *ApplicationHandler) GetStats(c *gin.Context) {
	type StatItem struct {
		Name  string `json:"name"`
		Count int64  `json:"count"`
	}

	// 应用总数
	var appCount int64
	h.db.Model(&models.Application{}).Count(&appCount)

	// 按团队统计
	var teamStats []StatItem
	h.db.Raw(`SELECT team as name, COUNT(*) as count FROM applications WHERE team != '' GROUP BY team ORDER BY count DESC`).Scan(&teamStats)

	// 按语言统计
	var langStats []StatItem
	h.db.Raw(`SELECT language as name, COUNT(*) as count FROM applications WHERE language != '' GROUP BY language ORDER BY count DESC`).Scan(&langStats)

	// 今日交付数
	var todayDeliveries int64
	h.db.Raw(`SELECT COUNT(*) FROM deploy_records WHERE DATE(created_at) = CURDATE()`).Scan(&todayDeliveries)

	// 本周交付数
	var weekDeliveries int64
	h.db.Raw(`SELECT COUNT(*) FROM deploy_records WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)`).Scan(&weekDeliveries)

	// 交付成功率
	var successCount, totalCount int64
	h.db.Raw(`SELECT COUNT(*) FROM deploy_records WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY)`).Scan(&totalCount)
	h.db.Raw(`SELECT COUNT(*) FROM deploy_records WHERE created_at >= DATE_SUB(NOW(), INTERVAL 7 DAY) AND status = 'success'`).Scan(&successCount)

	successRate := float64(0)
	if totalCount > 0 {
		successRate = float64(successCount) / float64(totalCount) * 100
	}

	response.Success(c, gin.H{
		"app_count":        appCount,
		"team_stats":       teamStats,
		"lang_stats":       langStats,
		"today_deliveries": todayDeliveries,
		"week_deliveries":  weekDeliveries,
		"success_rate":     successRate,
	})
}

func (h *ApplicationHandler) GetTeams(c *gin.Context) {
	teams, err := h.appRepo.GetAllTeams(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}

	response.Success(c, teams)
}
