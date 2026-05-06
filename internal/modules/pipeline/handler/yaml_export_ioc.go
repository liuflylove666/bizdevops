// Package handler 流水线模块处理器
//
// yaml_export_ioc.go registers YAMLExportHandler with the global IOC
// container (Sprint 2 BE-12). Independent init() per project convention.
package handler

import (
	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("YAMLExportHandler", &YAMLExportApiHandler{})
}

// YAMLExportApiHandler is the IOC wrapper.
type YAMLExportApiHandler struct {
	handler *YAMLExportHandler
}

// Init constructs the handler and registers GET /pipeline/:id/yaml under
// the auth-protected root group.
func (h *YAMLExportApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewYAMLExportHandler(db)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	group := root.Group("")
	group.Use(middleware.AuthMiddleware())
	h.handler.RegisterRoutes(group)

	return nil
}
