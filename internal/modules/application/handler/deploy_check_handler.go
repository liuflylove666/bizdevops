package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/service/deploy"
	"devops/internal/service/kubernetes"
	"devops/pkg/dto"
	apperrors "devops/pkg/errors"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("DeployCheckHandler", &DeployCheckApiHandler{})
}

type DeployCheckApiHandler struct {
	handler *DeployCheckHandler
}

func (h *DeployCheckApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	clientMgr := kubernetes.NewK8sClientManager(db)
	checkSvc := deploy.NewDeployCheckService(db, clientMgr)
	h.handler = NewDeployCheckHandler(checkSvc)

	root := cfg.Application.GinRootRouter().Group("deploy")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)

	return nil
}

func (h *DeployCheckApiHandler) Register(r gin.IRouter) {
	// 部署前置检查
	r.POST("/pre-check", h.handler.PreCheck)
}

type DeployCheckHandler struct {
	checkSvc *deploy.DeployCheckService
}

func NewDeployCheckHandler(checkSvc *deploy.DeployCheckService) *DeployCheckHandler {
	return &DeployCheckHandler{
		checkSvc: checkSvc,
	}
}

// PreCheck 部署前置检查
func (h *DeployCheckHandler) PreCheck(c *gin.Context) {
	var req dto.DeployPreCheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": apperrors.ErrCodeInvalidParams, "message": "请填写完整的检查配置信息"})
		return
	}

	result, err := h.checkSvc.PreCheck(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": apperrors.ErrCodeK8sDeploy, "message": "部署前置检查失败，请检查集群连接和资源配置"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": apperrors.Success, "message": "success", "data": result})
}
