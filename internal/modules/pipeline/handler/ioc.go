// Package handler 流水线模块处理器
// 本文件实现流水线模板、构建资源和 Webhook 的 IOC 注册
package handler

import (
	"devops/internal/config"
	"devops/internal/service/pipeline"
	"devops/pkg/ioc"
	"devops/pkg/middleware"

	"github.com/gin-gonic/gin"
)

func init() {
	ioc.Api.RegisterContainer("PipelineTemplateHandler", &PipelineTemplateApiHandler{})
	ioc.Api.RegisterContainer("WebhookHandler", &WebhookApiHandler{})
}

// PipelineTemplateApiHandler 流水线模板 API Handler IOC 包装器
type PipelineTemplateApiHandler struct {
	handler *TemplateHandler
}

// Init 初始化 Handler
func (h *PipelineTemplateApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	h.handler = NewTemplateHandler(db)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	templateGroup := root.Group("")
	templateGroup.Use(middleware.AuthMiddleware())
	h.handler.RegisterRoutes(templateGroup)

	return nil
}

// WebhookApiHandler Git Webhook API Handler IOC 包装器
type WebhookApiHandler struct {
	handler *WebhookHandler
}

// Init 初始化 Handler
func (h *WebhookApiHandler) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()
	runService := pipeline.NewRunService(db)
	h.handler = NewWebhookHandler(db, runService)

	root := cfg.Application.GinRootRouter().(*gin.RouterGroup)
	webhookGroup := root.Group("")
	h.handler.RegisterRoutes(webhookGroup)

	return nil
}
