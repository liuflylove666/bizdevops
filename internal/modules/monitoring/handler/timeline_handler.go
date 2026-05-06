// Package handler
//
// timeline_handler.go: 事件时间线 HTTP 接入（E4-03 / obs.incident_timeline）。
//
// GET /observability/timeline
//
// Query:
//
//	application_id — 可选，按应用过滤
//	env              — 可选
//	from, to         — RFC3339，可选；默认最近 30 天
//	limit            — 默认 150，最大 300
package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	appRepo "devops/internal/modules/application/repository"
	monRepo "devops/internal/modules/monitoring/repository"
	"devops/internal/service/changelog"
	"devops/internal/service/observability"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ObservabilityTimelineHandler", &ObservabilityTimelineApiHandler{})
}

// ObservabilityTimelineApiHandler IoC 入口。
type ObservabilityTimelineApiHandler struct {
	handler *TimelineHandler
}

// Init 注册路由。
func (h *ObservabilityTimelineApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	svc := observability.NewTimelineService(
		monRepo.NewIncidentRepository(db),
		changelog.NewService(appRepo.NewChangeEventRepository(db)),
		appRepo.NewReleaseRepository(db),
		db,
	)
	h.handler = &TimelineHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("observability")
	root.Use(middleware.AuthMiddleware())
	root.GET("/timeline", h.handler.Get)
	return nil
}

// TimelineHandler HTTP。
type TimelineHandler struct {
	svc *observability.TimelineService
}

// Get 聚合时间线。
func (h *TimelineHandler) Get(c *gin.Context) {
	now := time.Now()
	from := now.AddDate(0, 0, -30)
	to := now

	if v := c.Query("from"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			from = t
		}
	}
	if v := c.Query("to"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			to = t
		}
	}
	if from.After(to) {
		response.BadRequest(c, "from 不能晚于 to")
		return
	}

	var appID *uint
	if v := c.Query("application_id"); v != "" {
		if n, err := strconv.ParseUint(v, 10, 64); err == nil && n > 0 {
			u := uint(n)
			appID = &u
		}
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "150"))

	out, err := h.svc.Aggregate(observability.TimelineQuery{
		ApplicationID: appID,
		Env:           c.Query("env"),
		From:          from,
		To:            to,
		Limit:         limit,
	})
	if err != nil {
		response.InternalError(c, "聚合时间线失败: "+err.Error())
		return
	}
	response.Success(c, out)
}
