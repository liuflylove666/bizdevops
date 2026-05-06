// Package handler 流水线模块处理器
//
// diagnosis_ioc.go registers DiagnosisHandler with the global IOC container.
// Kept in its own file (not folded into ioc.go) to keep blast radius small
// when this Sprint 1 feature ships independently.
package handler

import (
	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("DiagnosisHandler", &DiagnosisApiHandler{})
}

// DiagnosisApiHandler is the IOC wrapper for DiagnosisHandler. Per the
// project convention, init() registers the wrapper; Init() (called later by
// ioc.Api.Init() once config + DB are ready) constructs the actual handler
// and attaches routes.
type DiagnosisApiHandler struct {
	handler *DiagnosisHandler
}

// Init constructs the handler and registers GET /pipeline/runs/:id/diagnosis
// under the auth-protected root group.
func (h *DiagnosisApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewDiagnosisHandler(db)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	group := root.Group("")
	group.Use(middleware.AuthMiddleware())
	h.handler.RegisterRoutes(group)

	return nil
}
