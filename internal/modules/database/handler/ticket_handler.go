package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/repository"
	"devops/internal/service/database"
)

type TicketHandler struct {
	svc *database.TicketService
}

func (h *TicketHandler) Register(r gin.IRouter) {
	g := r.Group("/ticket")
	g.POST("", h.Create)
	g.GET("", h.List)
	g.GET("/by-approval/:approvalInstanceId", h.GetByApprovalInstance)
	g.GET("/:id", h.Get)
	g.POST("/:id/agree", h.Agree)
	g.POST("/:id/reject", h.Reject)
	g.POST("/:id/cancel", h.Cancel)
	g.POST("/:id/execute", h.Execute)
	g.GET("/:id/rollback", h.Rollback)
}

func (h *TicketHandler) Create(c *gin.Context) {
	var in database.TicketCreateInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	applicant := c.GetString("username")
	if applicant == "" {
		applicant = "unknown"
	}
	realName := c.GetString("real_name")
	t, err := h.svc.Submit(c.Request.Context(), applicant, realName, &in)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": t})
}

func (h *TicketHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	instanceID, _ := strconv.ParseUint(c.Query("instance_id"), 10, 64)
	f := repository.TicketFilter{
		Applicant:  c.Query("applicant"),
		Assignee:   c.Query("assignee"),
		InstanceID: uint(instanceID),
		Keyword:    c.Query("keyword"),
	}
	if s := c.Query("status"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			f.Status = &v
		}
	}
	if s := c.Query("change_type"); s != "" {
		if v, err := strconv.Atoi(s); err == nil {
			f.ChangeType = &v
		}
	}
	list, total, err := h.svc.List(c.Request.Context(), f, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}

func (h *TicketHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	d, err := h.svc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": d})
}

func (h *TicketHandler) GetByApprovalInstance(c *gin.Context) {
	approvalInstanceID, _ := strconv.ParseUint(c.Param("approvalInstanceId"), 10, 64)
	ticket, err := h.svc.GetByApprovalInstanceID(c.Request.Context(), uint(approvalInstanceID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": nil})
			return
		}
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": ticket})
}

type ticketActionReq struct {
	Comment string `json:"comment"`
}

func (h *TicketHandler) Agree(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req ticketActionReq
	_ = c.ShouldBindJSON(&req)
	user := c.GetString("username")
	if err := h.svc.Agree(c.Request.Context(), uint(id), user, req.Comment); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *TicketHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req ticketActionReq
	_ = c.ShouldBindJSON(&req)
	user := c.GetString("username")
	if err := h.svc.Reject(c.Request.Context(), uint(id), user, req.Comment); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *TicketHandler) Cancel(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	user := c.GetString("username")
	if err := h.svc.Cancel(c.Request.Context(), uint(id), user); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *TicketHandler) Execute(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	user := c.GetString("username")
	if err := h.svc.Execute(c.Request.Context(), uint(id), user); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *TicketHandler) Rollback(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := h.svc.Rollbacks(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}
