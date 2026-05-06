package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/models/monitoring"
	"devops/internal/repository"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ServiceDetailHandler", &ServiceDetailApiHandler{})
}

type ServiceDetailApiHandler struct {
	handler *ServiceDetailHandler
}

func (h *ServiceDetailApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	h.handler = &ServiceDetailHandler{
		appRepo:    repository.NewApplicationRepository(db),
		envRepo:    repository.NewApplicationEnvRepository(db),
		deployRepo: repository.NewDeployRecordRepository(db),
		db:         db,
	}

	root := cfg.Application.GinRootRouter().Group("service-detail")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *ServiceDetailApiHandler) Register(r gin.IRouter) {
	r.GET("/:id", h.handler.GetServiceOverview)
	r.GET("/:id/alerts", h.handler.GetRecentAlerts)
	r.GET("/:id/health", h.handler.GetHealthStatus)
	r.GET("/:id/resources", h.handler.GetResourceUsage)
}

type ServiceDetailHandler struct {
	appRepo    *repository.ApplicationRepository
	envRepo    *repository.ApplicationEnvRepository
	deployRepo *repository.DeployRecordRepository
	db         *gorm.DB
}

type ServiceOverview struct {
	App                   *models.Application     `json:"app"`
	Envs                  []models.ApplicationEnv `json:"envs"`
	OrgName               string                  `json:"org_name"`
	ProjectName           string                  `json:"project_name"`
	RecentDeliveryRecords []models.DeployRecord   `json:"recent_delivery_records"`
	DeliveryStats         *repository.DeployStats `json:"delivery_stats"`
	AlertCount            int64                   `json:"alert_count"`
	HealthStatus          string                  `json:"health_status"`
}

func (h *ServiceDetailHandler) GetServiceOverview(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	app, err := h.appRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}

	envs, _ := h.envRepo.GetByAppID(c.Request.Context(), uint(id))

	if app.OrganizationID != nil && *app.OrganizationID > 0 {
		var org models.Organization
		if h.db.First(&org, *app.OrganizationID).Error == nil {
			app.OrgName = org.DisplayName
			if app.OrgName == "" {
				app.OrgName = org.Name
			}
		}
	}
	if app.ProjectID != nil && *app.ProjectID > 0 {
		var proj models.Project
		if h.db.First(&proj, *app.ProjectID).Error == nil {
			app.ProjectName = proj.DisplayName
			if app.ProjectName == "" {
				app.ProjectName = proj.Name
			}
		}
	}

	deliveryRecords, _, _ := h.deployRepo.List(c.Request.Context(), repository.DeployRecordFilter{ApplicationID: uint(id)}, 1, 5)

	now := time.Now()
	stats, _ := h.deployRepo.GetStats(c.Request.Context(), repository.DeployStatsFilter{
		ApplicationID: uint(id),
		StartTime:     now.AddDate(0, 0, -30),
		EndTime:       now,
	})

	var alertCount int64
	h.db.Model(&monitoring.AlertHistory{}).Where("title LIKE ? AND created_at > ?", "%"+app.Name+"%", now.AddDate(0, 0, -7)).Count(&alertCount)

	healthStatus := "unknown"
	var hc monitoring.HealthCheckConfig
	if h.db.Where("target_name = ? AND enabled = ?", app.Name, true).Order("last_checked_at DESC").First(&hc).Error == nil {
		healthStatus = hc.LastStatus
	}

	overview := ServiceOverview{
		App:                   app,
		Envs:                  envs,
		OrgName:               app.OrgName,
		ProjectName:           app.ProjectName,
		RecentDeliveryRecords: deliveryRecords,
		DeliveryStats:         stats,
		AlertCount:            alertCount,
		HealthStatus:          healthStatus,
	}
	response.Success(c, overview)
}

func (h *ServiceDetailHandler) GetRecentAlerts(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	app, err := h.appRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	offset := (page - 1) * pageSize

	var alerts []monitoring.AlertHistory
	var total int64
	q := h.db.Model(&monitoring.AlertHistory{}).Where("title LIKE ?", "%"+app.Name+"%")
	q.Count(&total)
	q.Order("created_at DESC").Offset(offset).Limit(pageSize).Find(&alerts)

	response.Page(c, alerts, total, page, pageSize)
}

func (h *ServiceDetailHandler) GetHealthStatus(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	app, err := h.appRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}

	var checks []monitoring.HealthCheckConfig
	h.db.Where("target_name = ?", app.Name).Find(&checks)
	response.Success(c, checks)
}

func (h *ServiceDetailHandler) GetResourceUsage(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	app, err := h.appRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}

	var costs []monitoring.ResourceCost
	h.db.Where("app_name = ?", app.Name).Order("recorded_at DESC").Limit(30).Find(&costs)
	response.Success(c, costs)
}
