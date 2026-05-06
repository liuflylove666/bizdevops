package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/service/workspace"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("WorkspaceHandler", &WorkspaceApiHandler{})
}

type WorkspaceApiHandler struct {
	handler *WorkspaceHandler
}

func (h *WorkspaceApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	h.handler = NewWorkspaceHandler(workspace.NewActionService(cfg.GetDB()))

	root := cfg.Application.GinRootRouter().Group("workspace")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)

	return nil
}

func (h *WorkspaceApiHandler) Register(r gin.IRouter) {
	r.GET("/actions", h.handler.ListActions)
}

type WorkspaceHandler struct {
	actionSvc *workspace.ActionService
}

func NewWorkspaceHandler(actionSvc *workspace.ActionService) *WorkspaceHandler {
	return &WorkspaceHandler{actionSvc: actionSvc}
}

func (h *WorkspaceHandler) ListActions(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "40"))
	var projectID *uint
	if pid := c.Query("project_id"); pid != "" {
		if v, err := strconv.ParseUint(pid, 10, 64); err == nil {
			id := uint(v)
			projectID = &id
		}
	}
	result, err := h.actionSvc.List(c.Request.Context(), limit, projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}
