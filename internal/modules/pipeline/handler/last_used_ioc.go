// Package handler 流水线模块处理器
//
// last_used_ioc.go registers LastUsedHandler with the global IOC container.
// Standalone file mirrors the diagnosis_ioc.go pattern: independent init()
// keeps blast radius small per Sprint 1 task delivery.
package handler

import (
	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("LastUsedHandler", &LastUsedApiHandler{})
}

// LastUsedApiHandler is the IOC wrapper. init() registers the wrapper;
// Init() (called later by ioc.Api.Init() once config + DB are ready)
// constructs the handler and attaches routes.
type LastUsedApiHandler struct {
	handler *LastUsedHandler
}

// Init constructs the handler and registers GET /pipelines/:id/last-run-config
// under the auth-protected root group.
func (h *LastUsedApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewLastUsedHandler(db)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	group := root.Group("")
	group.Use(middleware.AuthMiddleware())
	h.handler.RegisterRoutes(group)

	return nil
}
