package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	dbrepo "devops/internal/domain/database/repository"
)

type StatementHandler struct {
	repo *dbrepo.SQLChangeStatementRepository
}

func (h *StatementHandler) Register(r gin.IRouter) {
	r.GET("/statements", h.List)
}

func (h *StatementHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	instanceID, _ := strconv.ParseUint(c.Query("instance_id"), 10, 64)
	ticketID, _ := strconv.ParseUint(c.Query("ticket_id"), 10, 64)

	f := dbrepo.StatementFilter{
		TicketID:   uint(ticketID),
		WorkID:     c.Query("work_id"),
		InstanceID: uint(instanceID),
		State:      c.Query("state"),
		Applicant:  c.Query("applicant"),
	}
	list, total, err := h.repo.List(c.Request.Context(), f, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}
