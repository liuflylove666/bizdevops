// Package handler 流水线模块处理器
//
// log_tail_ioc.go registers LogTailHandler with the global IOC container
// (Sprint 2 BE-13). Independent init() per project convention.
package handler

import (
	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("LogTailHandler", &LogTailApiHandler{})
}

// LogTailApiHandler is the IOC wrapper.
type LogTailApiHandler struct {
	handler *LogTailHandler
}

// Init constructs the handler and registers GET /pipeline/runs/:id/log/tail.
func (h *LogTailApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewLogTailHandler(db)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	group := root.Group("")
	group.Use(middleware.AuthMiddleware())
	h.handler.RegisterRoutes(group)

	return nil
}
