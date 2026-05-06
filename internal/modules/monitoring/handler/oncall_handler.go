package handler

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/repository"
	"devops/internal/service/oncall"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("OncallHandler", &OncallApiHandler{})
}

type OncallApiHandler struct {
	handler *OncallHandler
}

func (h *OncallApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	svc := oncall.NewService(
		repository.NewOncallScheduleRepository(db),
		repository.NewOncallShiftRepository(db),
		repository.NewOncallOverrideRepository(db),
		repository.NewAlertAssignmentRepository(db),
	)
	h.handler = &OncallHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("oncall")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *OncallApiHandler) Register(r gin.IRouter) {
	// 排班表
	r.GET("/schedules", h.handler.ListSchedules)
	r.GET("/schedules/:id", h.handler.GetSchedule)
	r.POST("/schedules", middleware.RequireAdmin(), h.handler.CreateSchedule)
	r.PUT("/schedules/:id", middleware.RequireAdmin(), h.handler.UpdateSchedule)
	r.DELETE("/schedules/:id", middleware.RequireAdmin(), h.handler.DeleteSchedule)

	// 班次
	r.GET("/schedules/:id/shifts", h.handler.ListShifts)
	r.POST("/schedules/:id/shifts", middleware.RequireAdmin(), h.handler.CreateShift)
	r.DELETE("/shifts/:shiftId", middleware.RequireAdmin(), h.handler.DeleteShift)
	r.POST("/schedules/:id/generate", middleware.RequireAdmin(), h.handler.GenerateShifts)

	// 当前值班
	r.GET("/schedules/:id/current", h.handler.GetCurrentOnCall)

	// 临时替换
	r.GET("/schedules/:id/overrides", h.handler.ListOverrides)
	r.POST("/schedules/:id/overrides", h.handler.CreateOverride)
	r.DELETE("/overrides/:overrideId", h.handler.DeleteOverride)

	// 告警分配/认领
	r.POST("/alerts/:alertId/assign", h.handler.AssignAlert)
	r.POST("/alerts/:alertId/claim", h.handler.ClaimAlert)
	r.POST("/alerts/:alertId/resolve", h.handler.ResolveAssignment)
	r.GET("/alerts/:alertId/assignment", h.handler.GetAssignment)
	r.GET("/my-assignments", h.handler.ListMyAssignments)
}

type OncallHandler struct {
	svc *oncall.Service
}

func pid(c *gin.Context) uint {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(id)
}

// --- Schedule ---

func (h *OncallHandler) ListSchedules(c *gin.Context) {
	list, err := h.svc.ListSchedules()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *OncallHandler) GetSchedule(c *gin.Context) {
	s, err := h.svc.GetSchedule(pid(c))
	if err != nil {
		response.NotFound(c, "排班表不存在")
		return
	}
	response.Success(c, s)
}

func (h *OncallHandler) CreateSchedule(c *gin.Context) {
	var s models.OncallSchedule
	if err := c.ShouldBindJSON(&s); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	s.ID = 0
	if uid, ok := middleware.GetUserID(c); ok {
		s.CreatedBy = uid
	}
	if err := h.svc.CreateSchedule(&s); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, s)
}

func (h *OncallHandler) UpdateSchedule(c *gin.Context) {
	var s models.OncallSchedule
	if err := c.ShouldBindJSON(&s); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	s.ID = pid(c)
	if err := h.svc.UpdateSchedule(&s); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, s)
}

func (h *OncallHandler) DeleteSchedule(c *gin.Context) {
	if err := h.svc.DeleteSchedule(pid(c)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

// --- Shifts ---

func (h *OncallHandler) ListShifts(c *gin.Context) {
	var start, end time.Time
	if s := c.Query("start"); s != "" {
		start, _ = time.Parse("2006-01-02", s)
	}
	if e := c.Query("end"); e != "" {
		end, _ = time.Parse("2006-01-02", e)
	}
	list, err := h.svc.ListShifts(pid(c), start, end)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *OncallHandler) CreateShift(c *gin.Context) {
	var shift models.OncallShift
	if err := c.ShouldBindJSON(&shift); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	shift.ID = 0
	shift.ScheduleID = pid(c)
	if err := h.svc.CreateShift(&shift); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, shift)
}

func (h *OncallHandler) DeleteShift(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("shiftId"), 10, 64)
	if err := h.svc.DeleteShift(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *OncallHandler) GenerateShifts(c *gin.Context) {
	var body struct {
		UserIDs   []uint    `json:"user_ids"`
		UserNames []string  `json:"user_names"`
		StartDate string    `json:"start_date"`
		Count     int       `json:"count"`
		Type      string    `json:"type"` // weekly / daily
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	startDate, err := time.Parse("2006-01-02", body.StartDate)
	if err != nil {
		response.BadRequest(c, "日期格式错误，请使用 YYYY-MM-DD")
		return
	}
	if body.Count <= 0 {
		body.Count = 4
	}

	scheduleID := pid(c)
	if body.Type == "daily" {
		err = h.svc.GenerateDailyShifts(scheduleID, body.UserIDs, body.UserNames, startDate, body.Count)
	} else {
		err = h.svc.GenerateWeeklyShifts(scheduleID, body.UserIDs, body.UserNames, startDate, body.Count)
	}
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "排班生成成功", nil)
}

func (h *OncallHandler) GetCurrentOnCall(c *gin.Context) {
	shifts, err := h.svc.GetCurrentOnCall(pid(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, shifts)
}

// --- Override ---

func (h *OncallHandler) ListOverrides(c *gin.Context) {
	list, err := h.svc.ListOverrides(pid(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *OncallHandler) CreateOverride(c *gin.Context) {
	var o models.OncallOverride
	if err := c.ShouldBindJSON(&o); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	o.ID = 0
	o.ScheduleID = pid(c)
	if uid, ok := middleware.GetUserID(c); ok {
		o.CreatedBy = uid
	}
	if err := h.svc.CreateOverride(&o); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, o)
}

func (h *OncallHandler) DeleteOverride(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("overrideId"), 10, 64)
	if err := h.svc.DeleteOverride(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

// --- Alert Assignment ---

func (h *OncallHandler) AssignAlert(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alertId"), 10, 64)
	var body struct {
		ScheduleID uint `json:"schedule_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.ScheduleID == 0 {
		response.BadRequest(c, "请指定排班表 schedule_id")
		return
	}
	a, err := h.svc.AssignAlert(uint(alertID), body.ScheduleID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, a)
}

func (h *OncallHandler) ClaimAlert(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alertId"), 10, 64)
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	if err := h.svc.ClaimAlert(uint(alertID), userID, userName); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "已认领", nil)
}

func (h *OncallHandler) ResolveAssignment(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alertId"), 10, 64)
	var body struct {
		Comment string `json:"comment"`
	}
	c.ShouldBindJSON(&body)
	userID, _ := middleware.GetUserID(c)
	if err := h.svc.ResolveAlert(uint(alertID), userID, body.Comment); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "已解决", nil)
}

func (h *OncallHandler) GetAssignment(c *gin.Context) {
	alertID, _ := strconv.ParseUint(c.Param("alertId"), 10, 64)
	a, err := h.svc.GetAlertAssignment(uint(alertID))
	if err != nil {
		response.NotFound(c, "无分配记录")
		return
	}
	response.Success(c, a)
}

func (h *OncallHandler) ListMyAssignments(c *gin.Context) {
	userID, _ := middleware.GetUserID(c)
	status := c.Query("status")
	list, err := h.svc.ListMyAssignments(userID, status)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}
