package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models/deploy"
	approvalRepo "devops/internal/modules/approval/repository"
	appRepo "devops/internal/modules/application/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	approvalsvc "devops/internal/service/approval"
	"devops/internal/service/nacos"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("NacosReleaseHandler", &NacosReleaseApiHandler{})
}

type NacosReleaseApiHandler struct {
	handler *NacosReleaseHandler
}

func (h *NacosReleaseApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	releaseRepo := appRepo.NewNacosReleaseRepository(db)
	instRepo := infraRepo.NewNacosInstanceRepository(db)
	svc := nacos.NewReleaseService(releaseRepo, instRepo)
	chainRepo := approvalRepo.NewApprovalChainRepository(db)
	nodeRepo := approvalRepo.NewApprovalNodeRepository(db)
	instanceRepo := approvalRepo.NewApprovalInstanceRepository(db)
	nodeInstanceRepo := approvalRepo.NewApprovalNodeInstanceRepository(db)
	actionRepo := approvalRepo.NewApprovalActionRepository(db)
	policyRepo := approvalRepo.NewEnvAuditPolicyRepository(db)
	chainService := approvalsvc.NewChainService(chainRepo, nodeRepo)
	nodeExecutor := approvalsvc.NewNodeExecutor(nodeInstanceRepo, actionRepo, instanceRepo)
	approverResolver := approvalsvc.NewApproverResolver(db)
	instanceService := approvalsvc.NewInstanceService(instanceRepo, nodeInstanceRepo, chainService, nodeExecutor, approverResolver)
	svc.SetApprovalFlow(chainService, instanceService)
	svc.SetEnvAuditPolicyRepo(policyRepo)
	h.handler = &NacosReleaseHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("nacos-releases")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *NacosReleaseApiHandler) Register(r gin.IRouter) {
	r.GET("", h.handler.List)
	r.GET("/:id", h.handler.GetByID)
	r.GET("/by-approval/:approvalInstanceId", h.handler.GetByApprovalInstance)
	r.POST("", h.handler.Create)
	r.PUT("/:id", h.handler.Update)
	r.DELETE("/:id", h.handler.Delete)

	// 状态流转
	r.POST("/:id/submit", h.handler.Submit)
	r.POST("/:id/approve", h.handler.Approve)
	r.POST("/:id/reject", h.handler.Reject)
	r.POST("/:id/publish", h.handler.Publish)
	r.POST("/:id/rollback", h.handler.Rollback)

	// 辅助
	r.GET("/fetch-content", h.handler.FetchContent)
	r.GET("/by-service/:serviceId", h.handler.ListByService)
}

type NacosReleaseHandler struct {
	svc *nacos.ReleaseService
}

func (h *NacosReleaseHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var serviceID *uint
	if sid := c.Query("service_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		u := uint(v)
		serviceID = &u
	}
	f := appRepo.NacosReleaseFilter{
		Env:       c.Query("env"),
		Status:    c.Query("status"),
		DataID:    c.Query("data_id"),
		ServiceID: serviceID,
	}
	list, total, err := h.svc.List(f, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *NacosReleaseHandler) GetByID(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	nr, err := h.svc.GetByID(uint(id))
	if err != nil {
		response.NotFound(c, "发布单不存在")
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) GetByApprovalInstance(c *gin.Context) {
	approvalInstanceID, _ := strconv.ParseUint(c.Param("approvalInstanceId"), 10, 64)
	nr, err := h.svc.GetByApprovalInstanceID(uint(approvalInstanceID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Success(c, nil)
			return
		}
		response.NotFound(c, "未找到关联的 Nacos 发布单")
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Create(c *gin.Context) {
	var nr deploy.NacosRelease
	if err := c.ShouldBindJSON(&nr); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	nr.CreatedBy = userID
	nr.CreatedByName = userName
	if err := h.svc.CreateDraft(c.Request.Context(), &nr); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var nr deploy.NacosRelease
	if err := c.ShouldBindJSON(&nr); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	nr.ID = uint(id)
	if err := h.svc.Update(&nr); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *NacosReleaseHandler) Submit(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	nr, err := h.svc.SubmitForApproval(c.Request.Context(), uint(id))
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Approve(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	nr, err := h.svc.Approve(uint(id), userID, userName)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

type rejectDTO struct {
	Reason string `json:"reason" binding:"required"`
}

func (h *NacosReleaseHandler) Reject(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var dto rejectDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		response.BadRequest(c, "请填写驳回原因")
		return
	}
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	nr, err := h.svc.Reject(uint(id), userID, userName, dto.Reason)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Publish(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	nr, err := h.svc.Publish(c.Request.Context(), uint(id), userID, userName)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) Rollback(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID, _ := middleware.GetUserID(c)
	userName, _ := middleware.GetUsername(c)
	nr, err := h.svc.Rollback(c.Request.Context(), uint(id), userID, userName)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, nr)
}

func (h *NacosReleaseHandler) FetchContent(c *gin.Context) {
	instanceID, _ := strconv.ParseUint(c.Query("instance_id"), 10, 64)
	content, err := h.svc.FetchCurrentContent(c.Request.Context(), uint(instanceID), c.Query("tenant"), c.Query("group"), c.Query("data_id"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, content)
}

func (h *NacosReleaseHandler) ListByService(c *gin.Context) {
	serviceID, _ := strconv.ParseUint(c.Param("serviceId"), 10, 64)
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	list, err := h.svc.ListByService(uint(serviceID), limit)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}
