package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	tracingSvc "devops/internal/service/tracing"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("TracingHandler", &TracingApiHandler{})
}

type TracingApiHandler struct {
	handler *TracingHandler
}

func (h *TracingApiHandler) Init() error {
	cfg, _ := config.LoadConfig()

	tracerConfig := &tracingSvc.TracerConfig{
		ServiceName:    "devops-platform",
		ServiceVersion: "1.0.0",
		Environment:    "development",
		OTLPEndpoint:   "",
		SampleRate:     1.0,
		Enabled:        false,
	}

	tracer, err := tracingSvc.NewTracer(tracerConfig)
	if err != nil {
		return err
	}

	svc := tracingSvc.NewService(tracer, tracerConfig)
	h.handler = &TracingHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("tracing")
	root.Use(middleware.AuthMiddleware())
	h.RegisterRoutes(root)
	return nil
}

func (h *TracingApiHandler) RegisterRoutes(r gin.IRouter) {
	h.handler.RegisterRoutes(r)
}

type TracingHandler struct {
	svc *tracingSvc.Service
}

func (h *TracingHandler) RegisterRoutes(r gin.IRouter) {
	r.GET("/services", h.ListServices)
	r.GET("/services/:service/operations", h.GetServiceOperations)
	r.GET("/traces", h.QueryTraces)
	r.GET("/traces/:trace_id", h.GetTrace)
	r.GET("/traces/:trace_id/tree", h.GetTraceTree)
	r.GET("/status", h.GetStatus)
}

func (h *TracingHandler) GetStatus(c *gin.Context) {
	enabled := h.svc.IsEnabled()
	endpoint := ""
	if tracer := h.svc.GetTracer(); tracer != nil {
		if cfg := tracer.GetConfig(); cfg != nil {
			endpoint = cfg.OTLPEndpoint
		}
	}
	response.Success(c, gin.H{
		"enabled":  enabled,
		"endpoint": endpoint,
	})
}

func (h *TracingHandler) ListServices(c *gin.Context) {
	services, err := h.svc.ListServices(c.Request.Context())
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"services": services,
	})
}

func (h *TracingHandler) GetServiceOperations(c *gin.Context) {
	serviceName := c.Param("service")
	operations, err := h.svc.GetServiceOperations(c.Request.Context(), serviceName)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, gin.H{
		"service":    serviceName,
		"operations": operations,
	})
}

func (h *TracingHandler) QueryTraces(c *gin.Context) {
	req := &tracingSvc.TraceQuery{}

	if serviceName := c.Query("service"); serviceName != "" {
		req.ServiceName = serviceName
	}
	if operation := c.Query("operation"); operation != "" {
		req.Operation = operation
	}
	if limitStr := c.Query("limit"); limitStr != "" {
		if limit, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = limit
		}
	} else {
		req.Limit = 50
	}
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if offset, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = offset
		}
	}
	if startStr := c.Query("start"); startStr != "" {
		if start, err := strconv.ParseInt(startStr, 10, 64); err == nil {
			req.StartTime = time.Unix(start, 0)
		}
	}
	if endStr := c.Query("end"); endStr != "" {
		if end, err := strconv.ParseInt(endStr, 10, 64); err == nil {
			req.EndTime = time.Unix(end, 0)
		}
	}

	if req.EndTime.IsZero() {
		req.EndTime = time.Now()
	}
	if req.StartTime.IsZero() {
		req.StartTime = req.EndTime.Add(-1 * time.Hour)
	}

	result, err := h.svc.QueryTraces(c.Request.Context(), req)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *TracingHandler) GetTrace(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		response.BadRequest(c, "trace_id is required")
		return
	}

	trace, err := h.svc.GetTraceByID(c.Request.Context(), traceID)
	if err != nil {
		response.NotFound(c, "trace not found")
		return
	}
	response.Success(c, trace)
}

func (h *TracingHandler) GetTraceTree(c *gin.Context) {
	traceID := c.Param("trace_id")
	if traceID == "" {
		response.BadRequest(c, "trace_id is required")
		return
	}

	rootSpan, childSpans, err := h.svc.GetTraceTree(c.Request.Context(), traceID)
	if err != nil {
		response.NotFound(c, "trace not found")
		return
	}
	response.Success(c, gin.H{
		"root": rootSpan,
		"spans": childSpans,
	})
}
