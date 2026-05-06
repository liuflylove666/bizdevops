package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/repository"
	"devops/internal/service/approval"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("EnvAuditPolicyHandler", &EnvAuditPolicyApiHandler{})
}

type EnvAuditPolicyApiHandler struct {
	handler *EnvAuditPolicyHandler
}

func (h *EnvAuditPolicyApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	repo := repository.NewEnvAuditPolicyRepository(db)
	svc := approval.NewEnvAuditPolicyService(repo)

	h.handler = &EnvAuditPolicyHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("approval/env-policies")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *EnvAuditPolicyApiHandler) Register(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/:id", h.handler.GetByID)
	r.POST("", middleware.RequireAdmin(), h.handler.Create)
	r.PUT("/:id", middleware.RequireAdmin(), h.handler.Update)
	r.DELETE("/:id", middleware.RequireAdmin(), h.handler.Delete)
	r.POST("/:id/preset", middleware.RequireAdmin(), h.handler.ApplyPreset)
}

type EnvAuditPolicyHandler struct {
	svc *approval.EnvAuditPolicyService
}

func (h *EnvAuditPolicyHandler) List(c *gin.Context) {
	policies, err := h.svc.List()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, policies)
}

func (h *EnvAuditPolicyHandler) GetByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	p, err := h.svc.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "策略不存在")
		return
	}
	response.Success(c, p)
}

func (h *EnvAuditPolicyHandler) Create(c *gin.Context) {
	var p models.EnvAuditPolicy
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	p.ID = 0
	if userID, ok := middleware.GetUserID(c); ok {
		p.CreatedBy = userID
	}
	if err := h.svc.Create(&p); err != nil {
		response.InternalError(c, "创建失败: "+err.Error())
		return
	}
	response.Success(c, p)
}

func (h *EnvAuditPolicyHandler) Update(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	var p models.EnvAuditPolicy
	if err := c.ShouldBindJSON(&p); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	p.ID = uint(id)
	if err := h.svc.Update(&p); err != nil {
		response.InternalError(c, "更新失败: "+err.Error())
		return
	}
	response.Success(c, p)
}

func (h *EnvAuditPolicyHandler) Delete(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, "删除失败: "+err.Error())
		return
	}
	response.OK(c)
}

func (h *EnvAuditPolicyHandler) ApplyPreset(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "ID格式错误")
		return
	}
	var body struct {
		Preset string `json:"preset"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Preset == "" {
		response.BadRequest(c, "请指定预设: loose/moderate/strict/critical")
		return
	}
	p, err := h.svc.ApplyPreset(uint(id), body.Preset)
	if err != nil {
		response.InternalError(c, "应用预设失败: "+err.Error())
		return
	}
	response.Success(c, p)
}
