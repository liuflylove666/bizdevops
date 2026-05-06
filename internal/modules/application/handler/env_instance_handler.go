package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
	"devops/internal/service/envinstance"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("EnvInstanceHandler", &EnvInstanceApiHandler{})
}

type EnvInstanceApiHandler struct {
	handler *EnvInstanceHandler
}

func (h *EnvInstanceApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	repo := appRepo.NewEnvInstanceRepository(db)
	svc := envinstance.NewService(repo)
	h.handler = &EnvInstanceHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("env-instances")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *EnvInstanceApiHandler) Register(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/:id", h.handler.GetByID)
	r.POST("", h.handler.Create)
	r.PUT("/:id", h.handler.Update)
	r.DELETE("/:id", h.handler.Delete)
	r.GET("/by-app/:appId", h.handler.ListByApp)
	r.GET("/matrix", h.handler.EnvMatrix)
}

type EnvInstanceHandler struct {
	svc *envinstance.Service
}

func (h *EnvInstanceHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var appID *uint
	if sid := c.Query("application_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		u := uint(v)
		appID = &u
	}
	var clusterID *uint
	if sid := c.Query("cluster_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		u := uint(v)
		clusterID = &u
	}
	f := appRepo.EnvInstanceFilter{
		ApplicationID: appID,
		Env:           c.Query("env"),
		ClusterID:     clusterID,
		Status:        c.Query("status"),
	}
	list, total, err := h.svc.List(f, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *EnvInstanceHandler) GetByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	inst, err := h.svc.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "环境实例不存在")
		return
	}
	response.Success(c, inst)
}

func (h *EnvInstanceHandler) Create(c *gin.Context) {
	var inst deploy.EnvInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	if err := h.svc.Create(&inst); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, inst)
}

func (h *EnvInstanceHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var inst deploy.EnvInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = uint(id)
	if err := h.svc.Update(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, inst)
}

func (h *EnvInstanceHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *EnvInstanceHandler) ListByApp(c *gin.Context) {
	appID, _ := strconv.ParseUint(c.Param("appId"), 10, 64)
	list, err := h.svc.ListByApp(uint(appID))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *EnvInstanceHandler) EnvMatrix(c *gin.Context) {
	envsStr := c.Query("envs")
	var envs []string
	if envsStr != "" {
		envs = strings.Split(envsStr, ",")
	}
	list, err := h.svc.EnvMatrix(envs)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}
