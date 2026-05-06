package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models"
	appModel "devops/internal/models/application"
	deployModel "devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("CatalogHandler", &CatalogApiHandler{})
}

type CatalogApiHandler struct {
	handler *CatalogHandler
}

func (h *CatalogApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	h.handler = &CatalogHandler{
		orgRepo:  appRepo.NewOrganizationRepository(db),
		projRepo: appRepo.NewProjectRepository(db),
		envRepo:  appRepo.NewEnvDefinitionRepository(db),
		db:       db,
	}

	root := cfg.Application.GinRootRouter().Group("catalog")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *CatalogApiHandler) Register(r gin.IRouter) {
	r.GET("/orgs", h.handler.ListOrgs)
	r.POST("/orgs", middleware.RequireAdmin(), h.handler.CreateOrg)
	r.PUT("/orgs/:id", middleware.RequireAdmin(), h.handler.UpdateOrg)
	r.DELETE("/orgs/:id", middleware.RequireAdmin(), h.handler.DeleteOrg)

	r.GET("/projects", h.handler.ListProjects)
	r.GET("/projects/:id/overview", h.handler.GetProjectOverview)
	r.POST("/projects", middleware.RequireAdmin(), h.handler.CreateProject)
	r.PUT("/projects/:id", middleware.RequireAdmin(), h.handler.UpdateProject)
	r.DELETE("/projects/:id", middleware.RequireAdmin(), h.handler.DeleteProject)

	r.GET("/envs", h.handler.ListEnvs)
	r.POST("/envs", middleware.RequireAdmin(), h.handler.CreateEnv)
	r.PUT("/envs/:id", middleware.RequireAdmin(), h.handler.UpdateEnv)
	r.DELETE("/envs/:id", middleware.RequireAdmin(), h.handler.DeleteEnv)
}

type CatalogHandler struct {
	orgRepo  *appRepo.OrganizationRepository
	projRepo *appRepo.ProjectRepository
	envRepo  *appRepo.EnvDefinitionRepository
	db       *gorm.DB
}

type projectOverviewApplication struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	Owner          string `json:"owner"`
	Team           string `json:"team"`
	Status         string `json:"status"`
	ReadinessScore int    `json:"readiness_score"`
	ReadinessLevel string `json:"readiness_level"`
	PipelineCount  int64  `json:"pipeline_count"`
	EnvCount       int64  `json:"env_count"`
}

type projectOverviewPipeline struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	ApplicationName string     `json:"application_name"`
	Env             string     `json:"env"`
	Status          string     `json:"status"`
	LastRunStatus   string     `json:"last_run_status"`
	LastRunAt       *time.Time `json:"last_run_at"`
}

type projectOverviewRelease struct {
	ID              uint       `json:"id"`
	Title           string     `json:"title"`
	ApplicationName string     `json:"application_name"`
	Env             string     `json:"env"`
	Status          string     `json:"status"`
	CreatedAt       *time.Time `json:"created_at"`
	RiskLevel       string     `json:"risk_level"`
	RiskScore       int        `json:"risk_score"`
}

type projectOverviewFocusItem struct {
	Key         string `json:"key"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Severity    string `json:"severity"`
	Path        string `json:"path"`
}

type projectOverviewArgoCDApp struct {
	ID              uint       `json:"id"`
	Name            string     `json:"name"`
	ApplicationName string     `json:"application_name"`
	Env             string     `json:"env"`
	SyncStatus      string     `json:"sync_status"`
	HealthStatus    string     `json:"health_status"`
	DriftDetected   bool       `json:"drift_detected"`
	LastSyncAt      *time.Time `json:"last_sync_at"`
}

// --- Organization ---

func (h *CatalogHandler) ListOrgs(c *gin.Context) {
	list, err := h.orgRepo.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *CatalogHandler) CreateOrg(c *gin.Context) {
	var org appModel.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.orgRepo.Create(c.Request.Context(), &org); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, org)
}

func (h *CatalogHandler) UpdateOrg(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var org appModel.Organization
	if err := c.ShouldBindJSON(&org); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	org.ID = uint(id)
	if err := h.orgRepo.Update(c.Request.Context(), &org); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, org)
}

func (h *CatalogHandler) DeleteOrg(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.orgRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

// --- Project ---

func (h *CatalogHandler) ListProjects(c *gin.Context) {
	orgID, _ := strconv.ParseUint(c.Query("organization_id"), 10, 64)
	list, err := h.projRepo.List(c.Request.Context(), uint(orgID))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	orgs, _ := h.orgRepo.List(c.Request.Context())
	orgMap := make(map[uint]string)
	for _, o := range orgs {
		orgMap[o.ID] = o.DisplayName
		if orgMap[o.ID] == "" {
			orgMap[o.ID] = o.Name
		}
	}
	for i := range list {
		list[i].OrgName = orgMap[list[i].OrganizationID]
	}
	response.Success(c, list)
}

func (h *CatalogHandler) GetProjectOverview(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "项目ID格式错误")
		return
	}

	project, err := h.projRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "项目不存在")
		return
	}

	if project.OrganizationID > 0 {
		if org, orgErr := h.orgRepo.GetByID(c.Request.Context(), project.OrganizationID); orgErr == nil {
			project.OrgName = org.DisplayName
			if project.OrgName == "" {
				project.OrgName = org.Name
			}
		}
	}

	var apps []models.Application
	if err := h.db.WithContext(c.Request.Context()).
		Where("project_id = ?", project.ID).
		Order("created_at DESC").
		Find(&apps).Error; err != nil {
		response.InternalError(c, err.Error())
		return
	}

	appIDs := make([]uint, 0, len(apps))
	for _, app := range apps {
		appIDs = append(appIDs, app.ID)
	}

	readinessByApp := map[uint]models.ApplicationReadinessCheck{}
	if len(appIDs) > 0 {
		var rows []models.ApplicationReadinessCheck
		if err := h.db.WithContext(c.Request.Context()).
			Where("application_id IN ?", appIDs).
			Where("check_key = ?", "profile").
			Find(&rows).Error; err == nil {
			for _, row := range rows {
				readinessByApp[row.ApplicationID] = row
			}
		}
	}

	pipelineCountByApp := map[uint]int64{}
	envCountByApp := map[uint]int64{}
	if len(appIDs) > 0 {
		type countRow struct {
			ApplicationID uint
			Count         int64
		}
		var pipelineRows []countRow
		if err := h.db.WithContext(c.Request.Context()).
			Model(&deployModel.Pipeline{}).
			Select("application_id, COUNT(*) AS count").
			Where("application_id IN ?", appIDs).
			Group("application_id").
			Scan(&pipelineRows).Error; err == nil {
			for _, row := range pipelineRows {
				pipelineCountByApp[row.ApplicationID] = row.Count
			}
		}
		var envRows []countRow
		if err := h.db.WithContext(c.Request.Context()).
			Model(&models.ApplicationEnv{}).
			Select("app_id AS application_id, COUNT(*) AS count").
			Where("app_id IN ?", appIDs).
			Group("app_id").
			Scan(&envRows).Error; err == nil {
			for _, row := range envRows {
				envCountByApp[row.ApplicationID] = row.Count
			}
		}
	}

	appItems := make([]projectOverviewApplication, 0, len(apps))
	activeAppCount := 0
	readyAppCount := 0
	totalReadiness := 0
	for _, app := range apps {
		ready := readinessByApp[app.ID]
		if app.Status == "active" {
			activeAppCount++
		}
		if ready.Score >= 80 {
			readyAppCount++
		}
		totalReadiness += ready.Score
		appItems = append(appItems, projectOverviewApplication{
			ID:             app.ID,
			Name:           app.Name,
			DisplayName:    app.DisplayName,
			Owner:          app.Owner,
			Team:           app.Team,
			Status:         app.Status,
			ReadinessScore: ready.Score,
			ReadinessLevel: ready.Level,
			PipelineCount:  pipelineCountByApp[app.ID],
			EnvCount:       envCountByApp[app.ID],
		})
	}

	var avgReadiness float64
	if len(apps) > 0 {
		avgReadiness = float64(totalReadiness) / float64(len(apps))
	}

	var recentPipelines []projectOverviewPipeline
	if len(appIDs) > 0 {
		_ = h.db.WithContext(c.Request.Context()).
			Model(&deployModel.Pipeline{}).
			Select("id, name, application_name, env, status, last_run_status, last_run_at").
			Where("project_id = ?", project.ID).
			Order("updated_at DESC").
			Limit(8).
			Scan(&recentPipelines).Error
	}

	var recentReleases []projectOverviewRelease
	if len(appIDs) > 0 {
		_ = h.db.WithContext(c.Request.Context()).
			Model(&deployModel.Release{}).
			Select("releases.id, releases.title, releases.application_name, releases.env, releases.status, releases.created_at, releases.risk_level, releases.risk_score").
			Joins("JOIN applications ON applications.id = releases.application_id").
			Where("applications.project_id = ?", project.ID).
			Order("releases.created_at DESC").
			Limit(8).
			Scan(&recentReleases).Error
	}

	var pipelineCount int64
	_ = h.db.WithContext(c.Request.Context()).Model(&deployModel.Pipeline{}).Where("project_id = ?", project.ID).Count(&pipelineCount).Error

	var releaseCount int64
	_ = h.db.WithContext(c.Request.Context()).
		Model(&deployModel.Release{}).
		Joins("JOIN applications ON applications.id = releases.application_id").
		Where("applications.project_id = ?", project.ID).
		Count(&releaseCount).Error

	var failedPipelineCount int64
	_ = h.db.WithContext(c.Request.Context()).
		Model(&deployModel.Pipeline{}).
		Where("project_id = ? AND last_run_status = ?", project.ID, "failed").
		Count(&failedPipelineCount).Error

	var pendingReleaseCount int64
	_ = h.db.WithContext(c.Request.Context()).
		Model(&deployModel.Release{}).
		Joins("JOIN applications ON applications.id = releases.application_id").
		Where("applications.project_id = ? AND releases.status IN ?", project.ID, []string{"draft", "pending_approval", "approved", "pr_opened", "pr_merged"}).
		Count(&pendingReleaseCount).Error

	var openIncidentCount int64
	_ = h.db.WithContext(c.Request.Context()).
		Model(&models.Incident{}).
		Joins("JOIN applications ON applications.id = incidents.application_id").
		Where("applications.project_id = ? AND incidents.status IN ?", project.ID, []string{"open", "mitigated"}).
		Count(&openIncidentCount).Error

	var argocdAppCount int64
	var driftAppCount int64
	var outOfSyncAppCount int64
	var degradedAppCount int64
	var recentArgoApps []projectOverviewArgoCDApp
	if len(appIDs) > 0 {
		_ = h.db.WithContext(c.Request.Context()).
			Model(&models.ArgoCDApplication{}).
			Where("application_id IN ?", appIDs).
			Count(&argocdAppCount).Error
		_ = h.db.WithContext(c.Request.Context()).
			Model(&models.ArgoCDApplication{}).
			Where("application_id IN ? AND drift_detected = ?", appIDs, true).
			Count(&driftAppCount).Error
		_ = h.db.WithContext(c.Request.Context()).
			Model(&models.ArgoCDApplication{}).
			Where("application_id IN ? AND sync_status = ?", appIDs, "OutOfSync").
			Count(&outOfSyncAppCount).Error
		_ = h.db.WithContext(c.Request.Context()).
			Model(&models.ArgoCDApplication{}).
			Where("application_id IN ? AND health_status IN ?", appIDs, []string{"Degraded", "Missing"}).
			Count(&degradedAppCount).Error
		_ = h.db.WithContext(c.Request.Context()).
			Model(&models.ArgoCDApplication{}).
			Select("id, name, application_name, env, sync_status, health_status, drift_detected, last_sync_at").
			Where("application_id IN ?", appIDs).
			Order("updated_at DESC").
			Limit(8).
			Scan(&recentArgoApps).Error
	}

	focusItems := make([]projectOverviewFocusItem, 0)
	if openIncidentCount > 0 {
		focusItems = append(focusItems, projectOverviewFocusItem{
			Key:         "open_incidents",
			Title:       "存在未关闭事故",
			Description: "当前项目仍有生产事故处于 open 或 mitigated 状态，需要优先处置。",
			Severity:    "high",
			Path:        "/incidents?project_id=" + strconv.FormatUint(uint64(project.ID), 10),
		})
	}
	if failedPipelineCount > 0 {
		focusItems = append(focusItems, projectOverviewFocusItem{
			Key:         "failed_pipelines",
			Title:       "存在失败流水线",
			Description: "最近有流水线执行失败，建议优先检查交付链路稳定性。",
			Severity:    "medium",
			Path:        "/pipeline/list?project_id=" + strconv.FormatUint(uint64(project.ID), 10),
		})
	}
	if pendingReleaseCount > 0 {
		focusItems = append(focusItems, projectOverviewFocusItem{
			Key:         "pending_releases",
			Title:       "存在未完成发布主单",
			Description: "项目下仍有待审批、待合并或待发布的变更，需要确认卡点。",
			Severity:    "medium",
			Path:        "/releases?project_id=" + strconv.FormatUint(uint64(project.ID), 10),
		})
	}
	if driftAppCount > 0 || outOfSyncAppCount > 0 || degradedAppCount > 0 {
		focusItems = append(focusItems, projectOverviewFocusItem{
			Key:         "gitops_risk",
			Title:       "存在 GitOps 漂移或同步异常",
			Description: "项目下 ArgoCD 应用存在 Drift、OutOfSync 或 Degraded，需要优先核查运行态一致性。",
			Severity:    "high",
			Path:        "/argocd?project_id=" + strconv.FormatUint(uint64(project.ID), 10),
		})
	}
	if len(apps) > 0 && readyAppCount < len(apps) {
		focusItems = append(focusItems, projectOverviewFocusItem{
			Key:         "readiness_gap",
			Title:       "部分应用未完成接入",
			Description: "项目下仍有应用接入完整度未达标，建议补齐仓库、环境、流水线和 GitOps 配置。",
			Severity:    "low",
			Path:        "/applications?project_id=" + strconv.FormatUint(uint64(project.ID), 10),
		})
	}

	response.Success(c, gin.H{
		"project":               project,
		"app_count":             len(apps),
		"active_app_count":      activeAppCount,
		"pipeline_count":        pipelineCount,
		"release_count":         releaseCount,
		"avg_readiness_score":   avgReadiness,
		"ready_app_count":       readyAppCount,
		"open_incident_count":   openIncidentCount,
		"failed_pipeline_count": failedPipelineCount,
		"pending_release_count": pendingReleaseCount,
		"argocd_app_count":      argocdAppCount,
		"drift_app_count":       driftAppCount,
		"out_of_sync_app_count": outOfSyncAppCount,
		"degraded_app_count":    degradedAppCount,
		"apps":                  appItems,
		"recent_pipelines":      recentPipelines,
		"recent_releases":       recentReleases,
		"recent_argocd_apps":    recentArgoApps,
		"focus_items":           focusItems,
	})
}

func (h *CatalogHandler) CreateProject(c *gin.Context) {
	var proj appModel.Project
	if err := c.ShouldBindJSON(&proj); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.projRepo.Create(c.Request.Context(), &proj); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, proj)
}

func (h *CatalogHandler) UpdateProject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var proj appModel.Project
	if err := c.ShouldBindJSON(&proj); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	proj.ID = uint(id)
	if err := h.projRepo.Update(c.Request.Context(), &proj); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, proj)
}

func (h *CatalogHandler) DeleteProject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.projRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

// --- Env Definition ---

func (h *CatalogHandler) ListEnvs(c *gin.Context) {
	list, err := h.envRepo.List(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *CatalogHandler) CreateEnv(c *gin.Context) {
	var env appModel.EnvDefinition
	if err := c.ShouldBindJSON(&env); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.envRepo.Create(c.Request.Context(), &env); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, env)
}

func (h *CatalogHandler) UpdateEnv(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var env appModel.EnvDefinition
	if err := c.ShouldBindJSON(&env); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	env.ID = uint(id)
	if err := h.envRepo.Update(c.Request.Context(), &env); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, env)
}

func (h *CatalogHandler) DeleteEnv(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.envRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}
