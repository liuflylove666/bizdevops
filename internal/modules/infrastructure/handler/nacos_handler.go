package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/infrastructure"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/internal/service/nacos"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("NacosHandler", &NacosApiHandler{})
}

type NacosApiHandler struct {
	handler *NacosHandler
}

func (h *NacosApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	repo := infraRepo.NewNacosInstanceRepository(db)
	svc := nacos.NewService(repo)
	h.handler = &NacosHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("nacos")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *NacosApiHandler) Register(r gin.IRouter) {
	// 实例管理
	r.GET("/instances", h.handler.ListInstances)
	r.GET("/instances/:id", h.handler.GetInstance)
	r.POST("/instances", middleware.RequireAdmin(), h.handler.CreateInstance)
	r.PUT("/instances/:id", middleware.RequireAdmin(), h.handler.UpdateInstance)
	r.DELETE("/instances/:id", middleware.RequireAdmin(), h.handler.DeleteInstance)
	r.POST("/instances/:id/test-connection", middleware.RequireAdmin(), h.handler.TestConnection)

	// 命名空间
	r.GET("/instances/:id/namespaces", h.handler.ListNamespaces)

	// 配置管理
	r.GET("/instances/:id/configs", h.handler.ListConfigs)
	r.GET("/instances/:id/config", h.handler.GetConfig)
	r.POST("/instances/:id/config", h.handler.PublishConfig)
	r.DELETE("/instances/:id/config", h.handler.DeleteConfig)

	// 历史
	r.GET("/instances/:id/config/history", h.handler.ListConfigHistory)
	r.GET("/instances/:id/config/history/detail", h.handler.GetConfigHistoryDetail)

	// 跨环境对比与同步
	r.POST("/compare", h.handler.CompareConfigs)
	r.POST("/sync", middleware.RequireAdmin(), h.handler.SyncConfig)
}

type NacosHandler struct {
	svc *nacos.Service
}

func (h *NacosHandler) ListInstances(c *gin.Context) {
	list, err := h.svc.ListInstances(c.Request.Context(), c.Query("env"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *NacosHandler) GetInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	inst, err := h.svc.GetInstance(c.Request.Context(), uint(id))
	if err != nil {
		response.NotFound(c, "实例不存在")
		return
	}
	response.Success(c, inst)
}

func (h *NacosHandler) CreateInstance(c *gin.Context) {
	var inst infrastructure.NacosInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	inst.CreatedBy = &userID
	if err := h.svc.CreateInstance(c.Request.Context(), &inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	inst.Password = ""
	response.Success(c, inst)
}

func (h *NacosHandler) UpdateInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var inst infrastructure.NacosInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = uint(id)
	if err := h.svc.UpdateInstance(c.Request.Context(), &inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	inst.Password = ""
	response.Success(c, inst)
}

func (h *NacosHandler) DeleteInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteInstance(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *NacosHandler) TestConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.TestConnection(c.Request.Context(), uint(id)); err != nil {
		response.BadRequest(c, "连接失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接成功", nil)
}

func (h *NacosHandler) ListNamespaces(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := h.svc.ListNamespaces(c.Request.Context(), uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *NacosHandler) ListConfigs(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.svc.ListConfigs(c.Request.Context(), uint(id), c.Query("tenant"), c.Query("group"), c.Query("data_id"), page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *NacosHandler) GetConfig(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	content, err := h.svc.GetConfig(c.Request.Context(), uint(id), c.Query("tenant"), c.Query("group"), c.Query("data_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, content)
}

type publishConfigDTO struct {
	Tenant     string `json:"tenant"`
	Group      string `json:"group" binding:"required"`
	DataID     string `json:"data_id" binding:"required"`
	Content    string `json:"content" binding:"required"`
	ConfigType string `json:"config_type"`
}

func (h *NacosHandler) PublishConfig(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var dto publishConfigDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.PublishConfig(c.Request.Context(), uint(id), dto.Tenant, dto.Group, dto.DataID, dto.Content, dto.ConfigType); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *NacosHandler) DeleteConfig(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteConfig(c.Request.Context(), uint(id), c.Query("tenant"), c.Query("group"), c.Query("data_id")); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *NacosHandler) ListConfigHistory(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	result, err := h.svc.ListConfigHistory(c.Request.Context(), uint(id), c.Query("tenant"), c.Query("group"), c.Query("data_id"), page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *NacosHandler) GetConfigHistoryDetail(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	nid, _ := strconv.ParseInt(c.Query("nid"), 10, 64)
	item, err := h.svc.GetConfigHistoryDetail(c.Request.Context(), uint(id), c.Query("tenant"), c.Query("group"), c.Query("data_id"), nid)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, item)
}

type compareDTO struct {
	SourceInstanceID uint   `json:"source_instance_id" binding:"required"`
	TargetInstanceID uint   `json:"target_instance_id" binding:"required"`
	SourceTenant     string `json:"source_tenant"`
	TargetTenant     string `json:"target_tenant"`
	Group            string `json:"group"`
}

func (h *NacosHandler) CompareConfigs(c *gin.Context) {
	var dto compareDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	items, err := h.svc.CompareConfigs(c.Request.Context(), dto.SourceInstanceID, dto.TargetInstanceID, dto.SourceTenant, dto.TargetTenant, dto.Group)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, items)
}

type syncDTO struct {
	SourceInstanceID uint   `json:"source_instance_id" binding:"required"`
	TargetInstanceID uint   `json:"target_instance_id" binding:"required"`
	SourceTenant     string `json:"source_tenant"`
	TargetTenant     string `json:"target_tenant"`
	Group            string `json:"group" binding:"required"`
	DataID           string `json:"data_id" binding:"required"`
}

func (h *NacosHandler) SyncConfig(c *gin.Context) {
	var dto syncDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.SyncConfig(c.Request.Context(), dto.SourceInstanceID, dto.TargetInstanceID, dto.SourceTenant, dto.TargetTenant, dto.Group, dto.DataID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "同步成功", nil)
}
