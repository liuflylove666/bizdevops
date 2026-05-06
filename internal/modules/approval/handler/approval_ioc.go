package handler

import (
	"context"
	"encoding/json"
	"strings"
	"time"
	"devops/internal/config"
	dbmodel "devops/internal/domain/database/model"
	"devops/internal/models"
	deploymodel "devops/internal/models/deploy"
	apprepo "devops/internal/modules/application/repository"
	appHandler "devops/internal/modules/application/handler"
	infraRepo "devops/internal/modules/infrastructure/repository"
	"devops/internal/repository"
	"devops/internal/service/approval"
	"devops/internal/service/argocd"
	"devops/internal/service/deploy"
	"devops/pkg/ioc"
	"devops/pkg/middleware"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func init() {
	ioc.Api.RegisterContainer("ApprovalHandler", &ApprovalIOC{})
}

type ApprovalIOC struct {
	ruleHandler         *ApprovalRuleHandler
	windowHandler       *DeployWindowHandler
	approvalHandler     *ApprovalHandler
	lockHandler         *appHandler.DeployLockHandler
	chainHandler        *ApprovalChainHandler
	timeoutChecker      *approval.TimeoutChecker
	lockCleaner         *deploy.LockCleaner
	chainTimeoutHandler *approval.TimeoutHandler
}

type approvalBusinessResultHandler struct {
	argocdService    *argocd.Service
	ticketRepo       *repository.SQLChangeTicketRepository
	workflowRepo     *repository.SQLChangeWorkflowRepository
	nacosReleaseRepo *apprepo.NacosReleaseRepository
}

func (h *approvalBusinessResultHandler) OnApprovalApproved(ctx context.Context, instance *models.ApprovalInstance) error {
	if instance == nil {
		return nil
	}
	if handled, err := h.syncSQLTicketApproved(ctx, instance); handled || err != nil {
		return err
	}
	if handled, err := h.syncNacosReleaseApproved(ctx, instance); handled || err != nil {
		return err
	}
	if h.argocdService == nil {
		return nil
	}
	return h.argocdService.HandleApprovalApproved(ctx, instance.RecordID)
}

func (h *approvalBusinessResultHandler) OnApprovalRejected(ctx context.Context, instance *models.ApprovalInstance) error {
	if instance == nil {
		return nil
	}
	if handled, err := h.syncSQLTicketRejected(ctx, instance, "审批已拒绝"); handled || err != nil {
		return err
	}
	if handled, err := h.syncNacosReleaseRejected(ctx, instance, "审批已拒绝"); handled || err != nil {
		return err
	}
	if h.argocdService == nil {
		return nil
	}
	return h.argocdService.HandleApprovalRejected(ctx, instance.RecordID, "审批已拒绝")
}

func (h *approvalBusinessResultHandler) OnApprovalCancelled(ctx context.Context, instance *models.ApprovalInstance, reason string) error {
	if instance == nil {
		return nil
	}
	if handled, err := h.syncSQLTicketRejected(ctx, instance, reason); handled || err != nil {
		return err
	}
	if handled, err := h.syncNacosReleaseRejected(ctx, instance, fallbackReason(reason, "审批已取消")); handled || err != nil {
		return err
	}
	if h.argocdService == nil {
		return nil
	}
	return h.argocdService.HandleApprovalCancelled(ctx, instance.RecordID, reason)
}

func (h *approvalBusinessResultHandler) syncSQLTicketApproved(ctx context.Context, instance *models.ApprovalInstance) (bool, error) {
	ticket, err := h.getSQLTicketByApprovalInstance(ctx, instance.ID)
	if err != nil || ticket == nil {
		return false, err
	}

	stepCount := 0
	if len(ticket.AuditConfig) > 0 {
		var steps []dbmodel.AuditStep
		if err := json.Unmarshal(ticket.AuditConfig, &steps); err == nil {
			stepCount = len(steps)
		}
	}
	if err := h.ticketRepo.UpdateFields(ctx, ticket.ID, map[string]any{
		"status":       dbmodel.TicketStatusReady,
		"assigned":     "",
		"current_step": stepCount,
	}); err != nil {
		return true, err
	}
	if h.workflowRepo != nil {
		_ = h.workflowRepo.Create(ctx, &dbmodel.SQLChangeWorkflowDetail{
			TicketID:  ticket.ID,
			WorkID:    ticket.WorkID,
			Username:  "system",
			Action:    "approval_approved",
			Step:      ticket.CurrentStep,
			Comment:   "统一审批中心已通过",
		})
	}
	return true, nil
}

func (h *approvalBusinessResultHandler) syncSQLTicketRejected(ctx context.Context, instance *models.ApprovalInstance, reason string) (bool, error) {
	ticket, err := h.getSQLTicketByApprovalInstance(ctx, instance.ID)
	if err != nil || ticket == nil {
		return false, err
	}

	status := dbmodel.TicketStatusRejected
	action := "approval_rejected"
	if instance.Status == "cancelled" {
		status = dbmodel.TicketStatusCancelled
		action = "approval_cancelled"
	}
	if err := h.ticketRepo.UpdateFields(ctx, ticket.ID, map[string]any{
		"status":   status,
		"assigned": "",
	}); err != nil {
		return true, err
	}
	if h.workflowRepo != nil {
		_ = h.workflowRepo.Create(ctx, &dbmodel.SQLChangeWorkflowDetail{
			TicketID: ticket.ID,
			WorkID:   ticket.WorkID,
			Username: "system",
			Action:   action,
			Step:     ticket.CurrentStep,
			Comment:  reason,
		})
	}
	return true, nil
}

func (h *approvalBusinessResultHandler) getSQLTicketByApprovalInstance(ctx context.Context, approvalInstanceID uint) (*dbmodel.SQLChangeTicket, error) {
	if h.ticketRepo == nil || approvalInstanceID == 0 {
		return nil, nil
	}
	ticket, err := h.ticketRepo.GetByApprovalInstanceID(ctx, approvalInstanceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return ticket, nil
}

func (h *approvalBusinessResultHandler) syncNacosReleaseApproved(ctx context.Context, instance *models.ApprovalInstance) (bool, error) {
	nacosRelease, err := h.getNacosReleaseByApprovalInstance(ctx, instance.ID)
	if err != nil || nacosRelease == nil {
		return nacosRelease != nil, err
	}
	now := time.Now()
	nacosRelease.Status = "approved"
	nacosRelease.RejectReason = ""
	nacosRelease.ApprovedAt = &now
	return true, h.nacosReleaseRepo.Update(nacosRelease)
}

func (h *approvalBusinessResultHandler) syncNacosReleaseRejected(ctx context.Context, instance *models.ApprovalInstance, reason string) (bool, error) {
	nacosRelease, err := h.getNacosReleaseByApprovalInstance(ctx, instance.ID)
	if err != nil || nacosRelease == nil {
		return nacosRelease != nil, err
	}
	now := time.Now()
	nacosRelease.Status = "rejected"
	nacosRelease.RejectReason = fallbackReason(reason, "审批已拒绝")
	nacosRelease.ApprovedAt = &now
	return true, h.nacosReleaseRepo.Update(nacosRelease)
}

func (h *approvalBusinessResultHandler) getNacosReleaseByApprovalInstance(ctx context.Context, approvalInstanceID uint) (*deploymodel.NacosRelease, error) {
	if h.nacosReleaseRepo == nil || approvalInstanceID == 0 {
		return nil, nil
	}
	nacosRelease, err := h.nacosReleaseRepo.GetByApprovalInstanceID(approvalInstanceID)
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, nil
		}
		return nil, err
	}
	return nacosRelease, nil
}

func fallbackReason(reason, fallback string) string {
	if strings.TrimSpace(reason) != "" {
		return reason
	}
	return fallback
}

func (h *ApprovalIOC) Init() error {
	cfg, err := config.LoadConfig()
	if err != nil {
		return err
	}

	db := cfg.GetDB()

	// 创建 Repository
	ruleRepo := repository.NewApprovalRuleRepository(db)
	windowRepo := repository.NewDeployWindowRepository(db)
	chainRepo := repository.NewApprovalChainRepository(db)
	nodeRepo := repository.NewApprovalNodeRepository(db)
	instanceRepo := repository.NewApprovalInstanceRepository(db)
	nodeInstanceRepo := repository.NewApprovalNodeInstanceRepository(db)
	actionRepo := repository.NewApprovalActionRepository(db)
	argocdInstRepo := infraRepo.NewArgoCDInstanceRepository(db)
	argocdAppRepo := infraRepo.NewArgoCDApplicationRepository(db)
	argocdRepoRepo := infraRepo.NewGitOpsRepoRepository(db)
	argocdChangeRepo := infraRepo.NewGitOpsChangeRequestRepository(db)
	sqlTicketRepo := repository.NewSQLChangeTicketRepository(db)
	sqlWorkflowRepo := repository.NewSQLChangeWorkflowRepository(db)
	nacosReleaseRepo := apprepo.NewNacosReleaseRepository(db)

	// 创建 Service
	ruleService := approval.NewRuleService(ruleRepo)
	windowService := approval.NewWindowService(windowRepo)

	approvalService := approval.NewApprovalService(db, ruleService)
	lockService := deploy.NewLockService(db)

	// 创建审批链相关 Service
	chainService := approval.NewChainService(chainRepo, nodeRepo)
	nodeExecutor := approval.NewNodeExecutor(nodeInstanceRepo, actionRepo, instanceRepo)
	approverResolver := approval.NewApproverResolver(db)
	instanceService := approval.NewInstanceService(instanceRepo, nodeInstanceRepo, chainService, nodeExecutor, approverResolver)
	argocdService := argocd.NewService(argocdInstRepo, argocdAppRepo, argocdRepoRepo, argocdChangeRepo)
	nodeExecutor.SetResultHandler(&approvalBusinessResultHandler{
		argocdService:    argocdService,
		ticketRepo:       sqlTicketRepo,
		workflowRepo:     sqlWorkflowRepo,
		nacosReleaseRepo: nacosReleaseRepo,
	})

	// 创建 Handler
	h.ruleHandler = NewApprovalRuleHandler(ruleService)
	h.windowHandler = NewDeployWindowHandler(windowService)
	h.approvalHandler = NewApprovalHandler(approvalService)
	h.lockHandler = appHandler.NewDeployLockHandler(lockService)
	h.chainHandler = NewApprovalChainHandler(chainService, instanceService, nodeExecutor)

	// 创建后台任务
	h.timeoutChecker = approval.NewTimeoutChecker(db, ruleService)
	h.lockCleaner = deploy.NewLockCleaner(lockService)
	h.chainTimeoutHandler = approval.NewTimeoutHandler(nodeInstanceRepo, instanceRepo, nodeExecutor)

	// 注册路由
	root := cfg.Application.GinRootRouter().Group("approval")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)

	// 注册发布锁路由
	lockRoot := cfg.Application.GinRootRouter().Group("deploy/locks")
	lockRoot.Use(middleware.AuthMiddleware())
	h.RegisterLockRoutes(lockRoot)

	// 启动后台任务
	go h.timeoutChecker.Start()
	go h.lockCleaner.Start()
	h.chainTimeoutHandler.Start()

	return nil
}

func (h *ApprovalIOC) Register(r gin.IRouter) {
	// 审批规则 - 管理员权限
	rules := r.Group("/rules")
	{
		rules.GET("", h.ruleHandler.List)
		rules.GET("/:id", h.ruleHandler.GetByID)
		rules.POST("", middleware.RequireAdmin(), h.ruleHandler.Create)
		rules.PUT("/:id", middleware.RequireAdmin(), h.ruleHandler.Update)
		rules.DELETE("/:id", middleware.RequireAdmin(), h.ruleHandler.Delete)
	}

	// 发布窗口 - 管理员权限
	windows := r.Group("/windows")
	{
		windows.GET("", h.windowHandler.List)
		windows.GET("/:id", h.windowHandler.GetByID)
		windows.GET("/check", h.windowHandler.CheckWindow)
		windows.POST("", middleware.RequireAdmin(), h.windowHandler.Create)
		windows.PUT("/:id", middleware.RequireAdmin(), h.windowHandler.Update)
		windows.DELETE("/:id", middleware.RequireAdmin(), h.windowHandler.Delete)
	}

	// 审批链管理 - 管理员权限
	chains := r.Group("/chains")
	{
		chains.GET("", h.chainHandler.ListChains)
		chains.GET("/:id", h.chainHandler.GetChain)
		chains.POST("", middleware.RequireAdmin(), h.chainHandler.CreateChain)
		chains.PUT("/:id", middleware.RequireAdmin(), h.chainHandler.UpdateChain)
		chains.DELETE("/:id", middleware.RequireAdmin(), h.chainHandler.DeleteChain)
		chains.POST("/:id/nodes", middleware.RequireAdmin(), h.chainHandler.AddNode)
		chains.PUT("/:id/nodes/:nodeId", middleware.RequireAdmin(), h.chainHandler.UpdateNode)
		chains.DELETE("/:id/nodes/:nodeId", middleware.RequireAdmin(), h.chainHandler.DeleteNode)
		chains.PUT("/:id/nodes/reorder", middleware.RequireAdmin(), h.chainHandler.ReorderNodes)
		chains.POST("/:id/test", middleware.RequireAdmin(), h.chainHandler.TestChain)
	}

	// 审批实例 - 查看所有人可访问，取消需要权限
	instances := r.Group("/instances")
	{
		instances.GET("", h.chainHandler.ListInstances)
		instances.GET("/:id", h.chainHandler.GetInstance)
		instances.POST("/:id/cancel", h.chainHandler.CancelInstance)
	}

	// 审批节点操作 - 审批人可操作
	nodes := r.Group("/nodes")
	{
		nodes.POST("/:nodeInstanceId/approve", h.chainHandler.ApproveNode)
		nodes.POST("/:nodeInstanceId/reject", h.chainHandler.RejectNode)
		nodes.POST("/:nodeInstanceId/transfer", h.chainHandler.TransferNode)
	}

	// 审批链待审批列表
	r.GET("/chain/pending", h.chainHandler.GetPendingApprovals)

	// 审批统计
	r.GET("/stats", h.chainHandler.GetStats)

	// 审批规则检查与历史查询
	r.GET("/check", h.approvalHandler.CheckApprovalRequired)
	r.GET("/history", h.approvalHandler.GetHistory)
	r.GET("/history/export", h.approvalHandler.ExportHistory)
	r.GET("/:id/records", h.approvalHandler.GetApprovalRecords)
}

func (h *ApprovalIOC) RegisterLockRoutes(r gin.IRouter) {
	r.GET("", h.lockHandler.List)
	r.GET("/check", h.lockHandler.CheckLock)
	r.POST("/release", middleware.RequireAdmin(), h.lockHandler.ForceRelease)
}
