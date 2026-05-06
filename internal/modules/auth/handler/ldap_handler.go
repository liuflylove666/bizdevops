package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/modules/auth/repository"
	"devops/internal/service/auth"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("LDAPHandler", &LDAPApiHandler{})
}

type LDAPApiHandler struct {
	handler *LDAPHandler
}

func (h *LDAPApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	configRepo := repository.NewSystemConfigRepository(db)
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	urRepo := repository.NewUserRoleRepository(db)

	svc := auth.NewLDAPService(db, configRepo, userRepo, roleRepo, urRepo)
	h.handler = &LDAPHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("system/ldap")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *LDAPApiHandler) Register(r gin.IRouter) {
	r.GET("/config", h.handler.GetConfig)
	r.POST("/config", h.handler.SaveConfig)
	r.POST("/test-connection", h.handler.TestConnection)

	r.GET("/group-mappings", h.handler.ListGroupMappings)
	r.POST("/group-mappings", h.handler.CreateGroupMapping)
	r.PUT("/group-mappings/:id", h.handler.UpdateGroupMapping)
	r.DELETE("/group-mappings/:id", h.handler.DeleteGroupMapping)
}

type LDAPHandler struct {
	svc *auth.LDAPService
}

func (h *LDAPHandler) GetConfig(c *gin.Context) {
	cfg, err := h.svc.GetConfig(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	cfg.BindPassword = ""
	response.Success(c, cfg)
}

func (h *LDAPHandler) SaveConfig(c *gin.Context) {
	var cfg auth.LDAPConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if cfg.BindPassword == "" {
		old, _ := h.svc.GetConfig(c.Request.Context())
		if old != nil {
			cfg.BindPassword = old.BindPassword
		}
	}
	if err := h.svc.SaveConfig(c.Request.Context(), &cfg); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *LDAPHandler) TestConnection(c *gin.Context) {
	var cfg auth.LDAPConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if cfg.BindPassword == "" {
		old, _ := h.svc.GetConfig(c.Request.Context())
		if old != nil {
			cfg.BindPassword = old.BindPassword
		}
	}
	if err := h.svc.TestConnection(c.Request.Context(), &cfg); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接成功", nil)
}

func (h *LDAPHandler) ListGroupMappings(c *gin.Context) {
	list, err := h.svc.ListGroupMappings(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *LDAPHandler) CreateGroupMapping(c *gin.Context) {
	var m auth.LDAPGroupMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.CreateGroupMapping(c.Request.Context(), &m); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, m)
}

func (h *LDAPHandler) UpdateGroupMapping(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var m auth.LDAPGroupMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m.ID = uint(id)
	if err := h.svc.UpdateGroupMapping(c.Request.Context(), &m); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, m)
}

func (h *LDAPHandler) DeleteGroupMapping(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteGroupMapping(c.Request.Context(), uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}
