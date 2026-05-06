package handler

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/models/infrastructure"
	approvalRepo "devops/internal/modules/approval/repository"
	infraRepo "devops/internal/modules/infrastructure/repository"
	approvalsvc "devops/internal/service/approval"
	"devops/internal/service/argocd"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("ArgoCDHandler", &ArgoCDApiHandler{})
}

type ArgoCDApiHandler struct {
	handler *ArgoCDHandler
}

func (h *ArgoCDApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()
	instRepo := infraRepo.NewArgoCDInstanceRepository(db)
	appRepo := infraRepo.NewArgoCDApplicationRepository(db)
	repoRepo := infraRepo.NewGitOpsRepoRepository(db)
	changeRepo := infraRepo.NewGitOpsChangeRequestRepository(db)
	svc := argocd.NewService(instRepo, appRepo, repoRepo, changeRepo)
	chainRepo := approvalRepo.NewApprovalChainRepository(db)
	nodeRepo := approvalRepo.NewApprovalNodeRepository(db)
	instanceRepo := approvalRepo.NewApprovalInstanceRepository(db)
	nodeInstanceRepo := approvalRepo.NewApprovalNodeInstanceRepository(db)
	actionRepo := approvalRepo.NewApprovalActionRepository(db)
	policyRepo := approvalRepo.NewEnvAuditPolicyRepository(db)
	sonarBindingRepo := infraRepo.NewSonarQubeBindingRepository(db)
	chainService := approvalsvc.NewChainService(chainRepo, nodeRepo)
	nodeExecutor := approvalsvc.NewNodeExecutor(nodeInstanceRepo, actionRepo, instanceRepo)
	approverResolver := approvalsvc.NewApproverResolver(db)
	instanceService := approvalsvc.NewInstanceService(instanceRepo, nodeInstanceRepo, chainService, nodeExecutor, approverResolver)
	policyService := approvalsvc.NewEnvAuditPolicyService(policyRepo)
	svc.SetEnvPolicyRepo(policyRepo)
	svc.SetSonarBindingRepo(sonarBindingRepo)
	h.handler = &ArgoCDHandler{svc: svc, chainService: chainService, instanceService: instanceService, policyService: policyService}

	root := cfg.Application.GinRootRouter().Group("argocd")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *ArgoCDApiHandler) Register(r gin.IRouter) {
	// 实例管理
	r.GET("/instances", h.handler.ListInstances)
	r.GET("/instances/:id", h.handler.GetInstance)
	r.POST("/instances", middleware.RequireAdmin(), h.handler.CreateInstance)
	r.PUT("/instances/:id", middleware.RequireAdmin(), h.handler.UpdateInstance)
	r.DELETE("/instances/:id", middleware.RequireAdmin(), h.handler.DeleteInstance)
	r.POST("/instances/:id/test", middleware.RequireAdmin(), h.handler.TestConnection)
	r.POST("/instances/:id/sync-apps", h.handler.SyncFromArgoCD)

	// 应用管理
	r.GET("/apps", h.handler.ListApps)
	r.GET("/apps/:id", h.handler.GetApp)
	r.POST("/apps/:id/sync", h.handler.TriggerSync)
	r.GET("/apps/:id/resources", h.handler.GetResources)

	// GitOps 仓库
	r.GET("/dashboard", h.handler.GetDashboardSummary)
	r.GET("/repos", h.handler.ListRepos)
	r.GET("/repos/:id", h.handler.GetRepo)
	r.POST("/repos", h.handler.CreateRepo)
	r.PUT("/repos/:id", h.handler.UpdateRepo)
	r.DELETE("/repos/:id", h.handler.DeleteRepo)
	r.GET("/change-requests", h.handler.ListChangeRequests)
	r.GET("/change-requests/:id", h.handler.GetChangeRequest)
	r.GET("/change-requests/by-approval/:approvalInstanceId", h.handler.GetChangeRequestByApprovalInstance)
	r.POST("/change-requests/precheck", h.handler.PrecheckChangeRequest)
	r.POST("/change-requests", h.handler.CreateChangeRequest)
}

type ArgoCDHandler struct {
	svc             *argocd.Service
	chainService    *approvalsvc.ChainService
	instanceService *approvalsvc.InstanceService
	policyService   *approvalsvc.EnvAuditPolicyService
}

// --- Instance ---

func (h *ArgoCDHandler) ListInstances(c *gin.Context) {
	list, err := h.svc.ListInstances()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *ArgoCDHandler) GetInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	inst, err := h.svc.GetInstance(uint(id))
	if err != nil {
		response.NotFound(c, "实例不存在")
		return
	}
	response.Success(c, inst)
}

func (h *ArgoCDHandler) CreateInstance(c *gin.Context) {
	var inst infrastructure.ArgoCDInstance
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
	inst.AuthToken = ""
	response.Success(c, inst)
}

func (h *ArgoCDHandler) UpdateInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var inst infrastructure.ArgoCDInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = uint(id)
	if err := h.svc.UpdateInstance(&inst); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	inst.AuthToken = ""
	response.Success(c, inst)
}

func (h *ArgoCDHandler) DeleteInstance(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteInstance(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *ArgoCDHandler) TestConnection(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.TestConnection(uint(id)); err != nil {
		response.BadRequest(c, "连接失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接成功", nil)
}

func (h *ArgoCDHandler) SyncFromArgoCD(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	count, err := h.svc.SyncFromArgoCD(uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, fmt.Sprintf("已同步 %d 个应用", count), nil)
}

// --- Application ---

func (h *ArgoCDHandler) ListApps(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var instanceID *uint
	var projectID *uint
	if sid := c.Query("instance_id"); sid != "" {
		v, _ := strconv.ParseUint(sid, 10, 64)
		u := uint(v)
		instanceID = &u
	}
	if pid := c.Query("project_id"); pid != "" {
		v, _ := strconv.ParseUint(pid, 10, 64)
		u := uint(v)
		projectID = &u
	}
	f := infraRepo.ArgoCDAppFilter{
		InstanceID:   instanceID,
		ProjectID:    projectID,
		SyncStatus:   c.Query("sync_status"),
		HealthStatus: c.Query("health_status"),
		Env:          c.Query("env"),
		DriftOnly:    c.Query("drift_only") == "true",
	}
	list, total, err := h.svc.ListApplications(f, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *ArgoCDHandler) GetApp(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	app, err := h.svc.GetApplication(uint(id))
	if err != nil {
		response.NotFound(c, "应用不存在")
		return
	}
	response.Success(c, app)
}

func (h *ArgoCDHandler) TriggerSync(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.TriggerSync(uint(id)); err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.SuccessWithMessage(c, "同步已触发", nil)
}

func (h *ArgoCDHandler) GetResources(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	nodes, err := h.svc.GetResourceTree(uint(id))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, nodes)
}

// --- GitOps Repo ---

func (h *ArgoCDHandler) ListRepos(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var projectID *uint
	if pid := c.Query("project_id"); pid != "" {
		v, _ := strconv.ParseUint(pid, 10, 64)
		u := uint(v)
		projectID = &u
	}
	list, total, err := h.svc.ListRepos(projectID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *ArgoCDHandler) GetRepo(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	repo, err := h.svc.GetRepo(uint(id))
	if err != nil {
		response.NotFound(c, "仓库不存在")
		return
	}
	response.Success(c, repo)
}

func (h *ArgoCDHandler) CreateRepo(c *gin.Context) {
	var repo infrastructure.GitOpsRepo
	if err := c.ShouldBindJSON(&repo); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, _ := middleware.GetUserID(c)
	repo.CreatedBy = &userID
	if err := h.svc.CreateRepo(&repo); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	repo.AuthCredential = ""
	response.Success(c, repo)
}

func (h *ArgoCDHandler) UpdateRepo(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var repo infrastructure.GitOpsRepo
	if err := c.ShouldBindJSON(&repo); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	repo.ID = uint(id)
	if err := h.svc.UpdateRepo(&repo); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	repo.AuthCredential = ""
	response.Success(c, repo)
}

func (h *ArgoCDHandler) DeleteRepo(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteRepo(uint(id)); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *ArgoCDHandler) GetDashboardSummary(c *gin.Context) {
	var projectID *uint
	if pid := c.Query("project_id"); pid != "" {
		v, _ := strconv.ParseUint(pid, 10, 64)
		u := uint(v)
		projectID = &u
	}
	data, err := h.svc.DashboardSummary(projectID)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, data)
}

func (h *ArgoCDHandler) ListChangeRequests(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	var projectID *uint
	if pid := c.Query("project_id"); pid != "" {
		v, _ := strconv.ParseUint(pid, 10, 64)
		u := uint(v)
		projectID = &u
	}
	list, total, err := h.svc.ListChangeRequests(projectID, page, pageSize)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Page(c, list, total, page, pageSize)
}

func (h *ArgoCDHandler) GetChangeRequest(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	item, err := h.svc.GetChangeRequest(uint(id))
	if err != nil {
		response.NotFound(c, "变更请求不存在")
		return
	}
	response.Success(c, item)
}

func (h *ArgoCDHandler) GetChangeRequestByApprovalInstance(c *gin.Context) {
	approvalInstanceID, _ := strconv.ParseUint(c.Param("approvalInstanceId"), 10, 64)
	item, err := h.svc.GetChangeRequestByApprovalInstanceID(uint(approvalInstanceID))
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			response.Success(c, nil)
			return
		}
		response.NotFound(c, "未找到关联的 GitOps 变更请求")
		return
	}
	response.Success(c, item)
}

func (h *ArgoCDHandler) PrecheckChangeRequest(c *gin.Context) {
	var req argocd.CreateChangeRequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	precheck, err := h.runPrecheck(c, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	response.Success(c, precheck)
}

func (h *ArgoCDHandler) CreateChangeRequest(c *gin.Context) {
	var req argocd.CreateChangeRequestInput
	if err := c.ShouldBindJSON(&req); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	userID, hasUser := middleware.GetUserID(c)
	var createdBy *uint
	if hasUser {
		createdBy = &userID
	}
	precheck, err := h.runPrecheck(c, &req)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if precheck != nil && !precheck.CanCreate {
		response.BadRequest(c, summarizePrecheckErrors(precheck))
		return
	}
	item, err := h.svc.CreateChangeRequest(c.Request.Context(), &req, createdBy)
	if err != nil {
		response.BadRequest(c, err.Error())
		return
	}
	if item != nil {
		if approvalErr := h.bindApprovalChain(c, item); approvalErr != nil {
			item.ErrorMessage = fmt.Sprintf("%s; 审批链创建失败: %v", item.ErrorMessage, approvalErr)
		}
	}
	response.Success(c, item)
}

func (h *ArgoCDHandler) bindApprovalChain(c *gin.Context, item *infrastructure.GitOpsChangeRequest) error {
	if item == nil || h.chainService == nil || h.instanceService == nil {
		return nil
	}
	chain, err := h.matchApprovalChain(c, item.Env)
	if err != nil || chain == nil {
		return err
	}

	recordID := argocd.BuildChangeRequestApprovalRecordID(item.ID)
	instance, err := h.instanceService.Create(c.Request.Context(), recordID, chain)
	if err != nil {
		return err
	}
	if err := h.svc.AttachApproval(item.ID, instance.ID, chain.ID, chain.Name); err != nil {
		return err
	}
	if err := h.svc.NotifyApprovalPending(c.Request.Context(), item.ID); err != nil {
		item.ErrorMessage = fmt.Sprintf("%s; MR 审批提示回写失败: %v", item.ErrorMessage, err)
	}
	item.ApprovalInstanceID = &instance.ID
	item.ApprovalChainID = &chain.ID
	item.ApprovalChainName = chain.Name
	item.ApprovalStatus = "pending"
	return nil
}

func (h *ArgoCDHandler) runPrecheck(c *gin.Context, req *argocd.CreateChangeRequestInput) (*argocd.ChangeRequestPrecheck, error) {
	precheck, err := h.svc.PrecheckChangeRequest(req)
	if err != nil {
		return nil, err
	}
	if precheck == nil || precheck.Policy == nil || !precheck.Policy.RequireChain {
		return precheck, nil
	}
	envName := req.Env
	if envName == "" && precheck.Policy != nil {
		envName = precheck.Policy.EnvName
	}
	chain, err := h.matchApprovalChain(c, envName)
	passed := err == nil && chain != nil
	check := argocd.ChangeRequestPrecheckItem{
		Key:      "approval_chain",
		Name:     "审批链门禁",
		Required: true,
		Passed:   passed,
	}
	if passed {
		check.Message = fmt.Sprintf("已匹配审批链 `%s`", chain.Name)
	} else {
		check.Message = "环境策略要求审批链，但当前环境未匹配到可用审批链"
		precheck.CanCreate = false
	}
	precheck.Checks = append(precheck.Checks, check)
	return precheck, nil
}

func summarizePrecheckErrors(precheck *argocd.ChangeRequestPrecheck) string {
	if precheck == nil || len(precheck.Checks) == 0 {
		return "GitOps 变更预检未通过"
	}
	messages := make([]string, 0, len(precheck.Checks))
	for _, check := range precheck.Checks {
		if check.Passed {
			continue
		}
		messages = append(messages, check.Message)
	}
	if len(messages) == 0 {
		return "GitOps 变更预检未通过"
	}
	return strings.Join(messages, "; ")
}

func (h *ArgoCDHandler) matchApprovalChain(c *gin.Context, env string) (*models.ApprovalChain, error) {
	if h.policyService != nil && env != "" {
		policy, err := h.policyService.GetByEnvName(env)
		if err == nil && policy != nil && policy.RequireChain && policy.DefaultChainID != nil {
			return h.chainService.GetWithNodes(c.Request.Context(), *policy.DefaultChainID)
		}
	}
	return h.chainService.Match(c.Request.Context(), 0, env)
}
