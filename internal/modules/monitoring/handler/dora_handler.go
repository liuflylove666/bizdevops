// Package handler
//
// dora_handler.go: DORA 四指标 HTTP 接入层（v2.0 / Sprint 4）。
//
// 端点：
//   GET /api/v1/metrics/dora?env=prod&days=7
//   GET /api/v1/metrics/dora?env=prod&from=2026-04-01&to=2026-04-07
package handler

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/service/metrics"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("DORAHandler", &DORAApiHandler{})
}

// DORAApiHandler IoC 入口。
type DORAApiHandler struct {
	handler *DORAHandler
}

func (h *DORAApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	h.handler = &DORAHandler{
		agg: metrics.NewDORAAggregator(db),
	}

	root := cfg.Application.GinRootRouter().Group("metrics")
	root.Use(middleware.AuthMiddleware())
	root.GET("/dora", h.handler.GetDORA)
	return nil
}

// DORAHandler 业务方法。
type DORAHandler struct {
	agg *metrics.DORAAggregator
}

// GetDORA 返回 DORA 四指标快照。
//
// query:
//   env  发布环境（默认 prod）
//   days 周期天数（默认 7，覆盖 from/to）
//   from RFC3339 开始时间
//   to   RFC3339 结束时间
func (h *DORAHandler) GetDORA(c *gin.Context) {
	q := metrics.DORAQuery{
		Env:             c.DefaultQuery("env", "prod"),
		ApplicationName: c.Query("application_name"),
	}
	if v := c.Query("application_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil {
			q.ApplicationID = uint(n)
		}
	}
	if daysStr := c.Query("days"); daysStr != "" {
		if days, err := strconv.Atoi(daysStr); err == nil && days > 0 {
			q.To = time.Now()
			q.From = q.To.Add(-time.Duration(days) * 24 * time.Hour)
		}
	}
	if fromStr := c.Query("from"); fromStr != "" {
		if t, err := time.Parse(time.RFC3339, fromStr); err == nil {
			q.From = t
		}
	}
	if toStr := c.Query("to"); toStr != "" {
		if t, err := time.Parse(time.RFC3339, toStr); err == nil {
			q.To = t
		}
	}

	snap, err := h.agg.Aggregate(c.Request.Context(), q)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, "聚合 DORA 指标失败: "+err.Error())
		return
	}
	response.Success(c, gin.H{
		"enabled":  true,
		"snapshot": snap,
	})
}
