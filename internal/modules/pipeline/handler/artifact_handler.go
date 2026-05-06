package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/service/pipeline"
	"devops/pkg/dto"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ArtifactHandler", &ArtifactApiHandler{})
}

// ArtifactApiHandler IOC容器注册的处理器
type ArtifactApiHandler struct {
	handler *ArtifactHandler
}

func (h *ArtifactApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	artifactSvc := pipeline.NewArtifactService(db)

	h.handler = NewArtifactHandler(artifactSvc)

	root := cfg.Application.GinRootRouter().Group("artifacts")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)

	return nil
}

func (h *ArtifactApiHandler) Register(r gin.IRouter) {
	r.GET("", h.handler.ListArtifacts)
	r.GET("/:id", h.handler.GetArtifact)
	r.POST("", h.handler.CreateArtifact)
	r.DELETE("/:id", middleware.RequireAdmin(), h.handler.DeleteArtifact)
}

// ArtifactHandler 制品处理器
type ArtifactHandler struct {
	artifactSvc *pipeline.ArtifactService
}

// NewArtifactHandler 创建制品处理器
func NewArtifactHandler(artifactSvc *pipeline.ArtifactService) *ArtifactHandler {
	return &ArtifactHandler{
		artifactSvc: artifactSvc,
	}
}

// ListArtifacts 获取制品列表
func (h *ArtifactHandler) ListArtifacts(c *gin.Context) {
	var req dto.ArtifactListRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	result, err := h.artifactSvc.List(c.Request.Context(), &req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, result)
}

// GetArtifact 获取制品详情
func (h *ArtifactHandler) GetArtifact(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	result, err := h.artifactSvc.Get(c.Request.Context(), uint(id))
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.Success(c, result)
}

// CreateArtifact 创建制品
func (h *ArtifactHandler) CreateArtifact(c *gin.Context) {
	var req dto.ArtifactCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误")
		return
	}

	result, err := h.artifactSvc.Create(c.Request.Context(), &req)
	if err != nil {
		response.FromError(c, err)
		return
	}

	response.SuccessWithMessage(c, "创建成功", result)
}

// DeleteArtifact 删除制品
func (h *ArtifactHandler) DeleteArtifact(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		response.BadRequest(c, "无效的ID")
		return
	}

	if err := h.artifactSvc.Delete(c.Request.Context(), uint(id)); err != nil {
		response.FromError(c, err)
		return
	}

	response.SuccessWithMessage(c, "删除成功", nil)
}
