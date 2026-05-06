package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/infrastructure"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/internal/service/sonarqube"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("SonarQubeHandler", &SonarQubeApiHandler{})
}

type SonarQubeApiHandler struct {
	handler *SonarQubeHandler
}

func (h *SonarQubeApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	svc := sonarqube.NewService(
		infraRepo.NewSonarQubeInstanceRepository(db),
		infraRepo.NewSonarQubeBindingRepository(db),
	)
	h.handler = &SonarQubeHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("sonarqube")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *SonarQubeApiHandler) Register(r gin.IRouter) {
	r.GET("/instances", h.handler.ListInstances)
	r.GET("/instances/:id", h.handler.GetInstance)
	r.POST("/instances", middleware.RequireAdmin(), h.handler.CreateInstance)
	r.PUT("/instances/:id", middleware.RequireAdmin(), h.handler.UpdateInstance)
	r.DELETE("/instances/:id", middleware.RequireAdmin(), h.handler.DeleteInstance)
	r.POST("/instances/:id/test", h.handler.TestConnection)

	r.GET("/instances/:id/bindings", h.handler.ListBindings)
	r.POST("/instances/:id/bindings", middleware.RequireAdmin(), h.handler.CreateBinding)
	r.PUT("/bindings/:bindingId", middleware.RequireAdmin(), h.handler.UpdateBinding)
	r.DELETE("/bindings/:bindingId", middleware.RequireAdmin(), h.handler.DeleteBinding)

	r.GET("/instances/:id/projects", h.handler.ListSonarProjects)
	r.GET("/instances/:id/quality-gate", h.handler.GetQualityGate)
	r.GET("/instances/:id/measures", h.handler.GetMeasures)
	r.GET("/instances/:id/issues", h.handler.GetIssues)
}

type SonarQubeHandler struct {
	svc *sonarqube.Service
}

func sonarID(c *gin.Context) uint {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(id)
}

func (h *SonarQubeHandler) ListInstances(c *gin.Context) {
	list, err := h.svc.ListInstances()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *SonarQubeHandler) GetInstance(c *gin.Context) {
	inst, err := h.svc.GetInstance(sonarID(c))
	if err != nil {
		response.NotFound(c, "实例不存在")
		return
	}
	response.Success(c, inst)
}

func (h *SonarQubeHandler) CreateInstance(c *gin.Context) {
	var inst infrastructure.SonarQubeInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	inst.ID = 0
	if err := h.svc.CreateInstance(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, inst)
}

func (h *SonarQubeHandler) UpdateInstance(c *gin.Context) {
	var inst infrastructure.SonarQubeInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	inst.ID = sonarID(c)
	if err := h.svc.UpdateInstance(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, inst)
}

func (h *SonarQubeHandler) DeleteInstance(c *gin.Context) {
	if err := h.svc.DeleteInstance(sonarID(c)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *SonarQubeHandler) TestConnection(c *gin.Context) {
	result, err := h.svc.TestConnection(sonarID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *SonarQubeHandler) ListBindings(c *gin.Context) {
	list, err := h.svc.ListBindings(sonarID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *SonarQubeHandler) CreateBinding(c *gin.Context) {
	var b infrastructure.SonarQubeProjectBinding
	if err := c.ShouldBindJSON(&b); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	b.ID = 0
	b.SonarQubeID = sonarID(c)
	if err := h.svc.CreateBinding(&b); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, b)
}

func (h *SonarQubeHandler) UpdateBinding(c *gin.Context) {
	var b infrastructure.SonarQubeProjectBinding
	if err := c.ShouldBindJSON(&b); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	bid, _ := strconv.ParseUint(c.Param("bindingId"), 10, 64)
	b.ID = uint(bid)
	if err := h.svc.UpdateBinding(&b); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, b)
}

func (h *SonarQubeHandler) DeleteBinding(c *gin.Context) {
	bid, _ := strconv.ParseUint(c.Param("bindingId"), 10, 64)
	if err := h.svc.DeleteBinding(uint(bid)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *SonarQubeHandler) ListSonarProjects(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "50"))
	projects, total, err := h.svc.ListProjects(sonarID(c), page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"projects": projects, "total": total})
}

func (h *SonarQubeHandler) GetQualityGate(c *gin.Context) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		response.BadRequest(c, "缺少 projectKey 参数")
		return
	}
	gate, err := h.svc.GetQualityGate(sonarID(c), projectKey)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gate)
}

func (h *SonarQubeHandler) GetMeasures(c *gin.Context) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		response.BadRequest(c, "缺少 projectKey 参数")
		return
	}
	measures, err := h.svc.GetMeasures(sonarID(c), projectKey, nil)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, measures)
}

func (h *SonarQubeHandler) GetIssues(c *gin.Context) {
	projectKey := c.Query("projectKey")
	if projectKey == "" {
		response.BadRequest(c, "缺少 projectKey 参数")
		return
	}
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	severities := c.Query("severities")
	issues, total, err := h.svc.GetIssues(sonarID(c), projectKey, page, pageSize, severities)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{"issues": issues, "total": total})
}
