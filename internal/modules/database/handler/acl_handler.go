package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/service/database"
)

type ACLHandler struct {
	svc *database.ACLService
}

func (h *ACLHandler) Register(r gin.IRouter) {
	r.GET("/instance/:id/acl", h.List)
	r.POST("/instance/:id/acl", h.Bind)
	r.DELETE("/instance/acl/:acl_id", h.Unbind)
	r.GET("/instance/:id/acl/schemas", h.AccessibleSchemas)
}

func (h *ACLHandler) List(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := h.svc.ListByInstance(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

func (h *ACLHandler) Bind(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var in database.ACLBindInput
	if err := c.ShouldBindJSON(&in); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	m, err := h.svc.Bind(c.Request.Context(), uint(id), &in, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": m})
}

func (h *ACLHandler) Unbind(c *gin.Context) {
	aclID, _ := strconv.ParseUint(c.Param("acl_id"), 10, 64)
	if err := h.svc.Unbind(c.Request.Context(), uint(aclID)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *ACLHandler) AccessibleSchemas(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	schemas, isAll := h.svc.AccessibleSchemas(c.Request.Context(), userID, role, nil, uint(id))
	c.JSON(http.StatusOK, gin.H{
		"code": 0, "message": "Success",
		"data": gin.H{"schemas": schemas, "is_all": isAll},
	})
}
