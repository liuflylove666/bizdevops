package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	appRepo "devops/internal/modules/application/repository"
	"devops/internal/service/changelog"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ChangeEventHandler", &ChangeEventApiHandler{})
}

type ChangeEventApiHandler struct {
	handler *ChangeEventHandler
}

func (h *ChangeEventApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	repo := appRepo.NewChangeEventRepository(db)
	svc := changelog.NewService(repo)
	h.handler = &ChangeEventHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("change-events")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *ChangeEventApiHandler) Register(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/stats", h.handler.Stats)
	r.GET("/by-app/:appId", h.handler.ListByApp)
}

type ChangeEventHandler struct {
	svc *changelog.Service
}

func (h *ChangeEventHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var appID *uint
	if sid := c.Query("application_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		u := uint(v)
		appID = &u
	}
	f := appRepo.ChangeEventFilter{
		EventType:     c.Query("event_type"),
		ApplicationID: appID,
		Env:           c.Query("env"),
		Status:        c.Query("status"),
		Operator:      c.Query("operator"),
		StartTime:     c.Query("start_time"),
		EndTime:       c.Query("end_time"),
	}
	list, total, err := h.svc.List(f, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *ChangeEventHandler) Stats(c *gin.Context) {
	stats, err := h.svc.Stats()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, stats)
}

func (h *ChangeEventHandler) ListByApp(c *gin.Context) {
	appID, _ := strconv.ParseUint(c.Param("appId"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	list, err := h.svc.ListByApplication(uint(appID), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}
