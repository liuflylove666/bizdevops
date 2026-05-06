package handler

import (
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models/monitoring"
	monitoringRepo "devops/internal/modules/monitoring/repository"
	promSvc "devops/internal/service/prometheus"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("PrometheusHandler", &PrometheusApiHandler{})
}

type PrometheusApiHandler struct {
	handler *PrometheusHandler
}

func (h *PrometheusApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	repo := monitoringRepo.NewPrometheusInstanceRepository(db)
	svc := promSvc.NewService(repo)
	h.handler = &PrometheusHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("prometheus")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *PrometheusApiHandler) Register(r gin.IRouter) {
	// 实例管理
	r.GET("/instances", h.handler.ListInstances)
	r.GET("/instances/:id", h.handler.GetInstance)
	r.POST("/instances", middleware.RequireAdmin(), h.handler.CreateInstance)
	r.PUT("/instances/:id", middleware.RequireAdmin(), h.handler.UpdateInstance)
	r.DELETE("/instances/:id", middleware.RequireAdmin(), h.handler.DeleteInstance)
	r.POST("/instances/:id/test", middleware.RequireAdmin(), h.handler.TestConnection)

	// 查询代理 BFF
	r.GET("/query", h.handler.Query)
	r.GET("/query_range", h.handler.QueryRange)
	r.GET("/labels", h.handler.Labels)
	r.GET("/label/:name/values", h.handler.LabelValues)
	r.GET("/series", h.handler.Series)
	r.GET("/targets", h.handler.Targets)
}

type PrometheusHandler struct {
	svc *promSvc.Service
}

// --- Instance CRUD ---

func (h *PrometheusHandler) ListInstances(c *gin.Context) {
	list, err := h.svc.ListInstances()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *PrometheusHandler) GetInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	inst, err := h.svc.GetInstance(uint(id))
	if err != nil {
		response.NotFound(c, "实例不存在")
		return
	}
	response.Success(c, inst)
}

func (h *PrometheusHandler) CreateInstance(c *gin.Context) {
	var inst monitoring.PrometheusInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	inst.CreatedBy = &userID
	if err := h.svc.CreateInstance(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	inst.Password = ""
	response.Success(c, inst)
}

func (h *PrometheusHandler) UpdateInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var inst monitoring.PrometheusInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = uint(id)
	if err := h.svc.UpdateInstance(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	inst.Password = ""
	response.Success(c, inst)
}

func (h *PrometheusHandler) DeleteInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteInstance(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *PrometheusHandler) TestConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.TestConnection(uint(id)); err != nil {
		response.BadRequest(c, "连接失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接成功", nil)
}

// --- Query Proxy BFF ---

func (h *PrometheusHandler) getInstanceID(c *gin.Context) uint {
	if sid := c.Query("instance_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		return uint(v)
	}
	return 0 // use default
}

func (h *PrometheusHandler) Query(c *gin.Context) {
	query := c.Query("query")
	if query == "" {
		response.BadRequest(c, "query 参数必填")
		return
	}
	data, err := h.svc.Query(h.getInstanceID(c), query, c.Query("time"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *PrometheusHandler) QueryRange(c *gin.Context) {
	query := c.Query("query")
	start := c.Query("start")
	end := c.Query("end")
	step := c.DefaultQuery("step", "60s")
	if query == "" || start == "" || end == "" {
		response.BadRequest(c, "query/start/end 参数必填")
		return
	}
	data, err := h.svc.QueryRange(h.getInstanceID(c), query, start, end, step)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *PrometheusHandler) Labels(c *gin.Context) {
	data, err := h.svc.Labels(h.getInstanceID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *PrometheusHandler) LabelValues(c *gin.Context) {
	name := c.Param("name")
	data, err := h.svc.LabelValues(h.getInstanceID(c), name)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *PrometheusHandler) Series(c *gin.Context) {
	matchParam := c.Query("match[]")
	if matchParam == "" {
		matchParam = c.Query("match")
	}
	matchers := strings.Split(matchParam, ",")
	start := c.Query("start")
	end := c.Query("end")
	data, err := h.svc.Series(h.getInstanceID(c), matchers, start, end)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *PrometheusHandler) Targets(c *gin.Context) {
	data, err := h.svc.Targets(h.getInstanceID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}
