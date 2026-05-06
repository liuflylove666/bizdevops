package database

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
	"devops/internal/models"
	approvalsrv "devops/internal/service/approval"
)

// TicketService SQL 变更工单服务
type TicketService struct {
	ticketR    *dbrepo.SQLChangeTicketRepository
	stmtR      *dbrepo.SQLChangeStatementRepository
	wfR        *dbrepo.SQLChangeWorkflowRepository
	ruleR      *dbrepo.SQLAuditRuleRepository
	rollbackR  *dbrepo.SQLRollbackRepository
	auditor    Auditor
	executor   *Executor
	instanceR  approvalInstanceRepository
	nodeInstR  approvalNodeInstanceRepository
	approvers  *approvalsrv.ApproverResolver
}

type approvalInstanceRepository interface {
	Create(ctx context.Context, instance *models.ApprovalInstance) error
}

type approvalNodeInstanceRepository interface {
	CreateBatch(ctx context.Context, list []models.ApprovalNodeInstance) error
	GetByInstanceIDAndOrder(ctx context.Context, instanceID uint, order int) (*models.ApprovalNodeInstance, error)
	Activate(ctx context.Context, id uint, timeoutAt *time.Time) error
}

const sqlTicketApprovalRecordOffset uint = 2000000000

// SetRollbackRepo 允许外部注入 rollback 仓储（避免 NewTicketService 参数过长）
func (s *TicketService) SetRollbackRepo(r *dbrepo.SQLRollbackRepository) { s.rollbackR = r }

// SetApprovalFlow 配置统一审批中心相关依赖
func (s *TicketService) SetApprovalFlow(
	instanceR approvalInstanceRepository,
	nodeInstR approvalNodeInstanceRepository,
	approvers *approvalsrv.ApproverResolver,
) {
	s.instanceR = instanceR
	s.nodeInstR = nodeInstR
	s.approvers = approvers
}

// Rollbacks 返回工单已生成的反向 SQL
func (s *TicketService) Rollbacks(ctx context.Context, ticketID uint) ([]model.SQLRollbackScript, error) {
	if s.rollbackR == nil {
		return nil, nil
	}
	return s.rollbackR.ListByTicket(ctx, ticketID)
}

func NewTicketService(
	ticketR *dbrepo.SQLChangeTicketRepository,
	stmtR *dbrepo.SQLChangeStatementRepository,
	wfR *dbrepo.SQLChangeWorkflowRepository,
	ruleR *dbrepo.SQLAuditRuleRepository,
	auditor Auditor,
	executor *Executor,
) *TicketService {
	return &TicketService{ticketR: ticketR, stmtR: stmtR, wfR: wfR, ruleR: ruleR, auditor: auditor, executor: executor}
}

// TicketCreateInput 提交工单入参
type TicketCreateInput struct {
	Title       string            `json:"title" binding:"required"`
	Description string            `json:"description"`
	InstanceID  uint              `json:"instance_id" binding:"required"`
	SchemaName  string            `json:"schema_name" binding:"required"`
	ChangeType  int               `json:"change_type"` // 0 DDL 1 DML
	NeedBackup  bool              `json:"need_backup"`
	SQLText     string            `json:"sql_text" binding:"required"`
	AuditSteps  []model.AuditStep `json:"audit_steps"` // 审批流
	DelayMode   string            `json:"delay_mode"`  // none / schedule
	ExecuteTime *time.Time        `json:"execute_time"`
	AllowDrop   bool              `json:"allow_drop"`
	AllowTrunc  bool              `json:"allow_trunc"`
}

// TicketDetail 工单详情聚合
type TicketDetail struct {
	Ticket     *model.SQLChangeTicket          `json:"ticket"`
	Statements []model.SQLChangeStatement      `json:"statements"`
	Workflow   []model.SQLChangeWorkflowDetail `json:"workflow"`
	AuditSteps []model.AuditStep               `json:"audit_steps"`
}

// Submit 创建并审核工单
func (s *TicketService) Submit(ctx context.Context, applicant, realName string, in *TicketCreateInput) (*model.SQLChangeTicket, error) {
	stmts := SplitStatements(in.SQLText)
	if len(stmts) == 0 {
		return nil, fmt.Errorf("未解析到有效 SQL 语句")
	}
	auditor := s.resolveAuditor(ctx)
	report := auditor.Audit(ctx, stmts, AuditOptions{
		Schema:     in.SchemaName,
		AllowDrop:  in.AllowDrop,
		AllowTrunc: in.AllowTrunc,
	})
	reportJSON, _ := json.Marshal(report)

	// 审批流配置
	steps := in.AuditSteps
	if len(steps) == 0 {
		// 默认单级审批：申请人自审（开发环境兜底）
		steps = []model.AuditStep{{StepName: "default", Approvers: []string{applicant}}}
	}
	cfgJSON, _ := json.Marshal(steps)
	assigned := strings.Join(steps[0].Approvers, ",")

	t := &model.SQLChangeTicket{
		WorkID:      genWorkID(),
		Title:       in.Title,
		Description: in.Description,
		Applicant:   applicant,
		RealName:    realName,
		InstanceID:  in.InstanceID,
		SchemaName:  in.SchemaName,
		ChangeType:  in.ChangeType,
		NeedBackup:  in.NeedBackup,
		Status:      model.TicketStatusPending,
		DelayMode:   firstNonEmpty(in.DelayMode, "none"),
		ExecuteTime: in.ExecuteTime,
		AuditReport: reportJSON,
		AuditConfig: cfgJSON,
		CurrentStep: 0,
		Assigned:    assigned,
	}
	if report.HasError {
		t.Status = model.TicketStatusRejected
	}
	if err := s.ticketR.Create(ctx, t); err != nil {
		return nil, err
	}

	// 语句落库
	list := make([]model.SQLChangeStatement, 0, len(stmts))
	for i, raw := range stmts {
		list = append(list, model.SQLChangeStatement{
			TicketID: t.ID,
			WorkID:   t.WorkID,
			Seq:      i + 1,
			SQLText:  raw,
			State:    "pending",
		})
	}
	if err := s.stmtR.BulkCreate(ctx, list); err != nil {
		return nil, err
	}

	_ = s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
		TicketID: t.ID, WorkID: t.WorkID, Username: applicant, Action: "submit", Step: 0,
		Comment: report.Summary,
	})

	if !report.HasError {
		if err := s.createApprovalFlow(ctx, t, steps); err != nil {
			return nil, err
		}
	}
	return t, nil
}

// Get 工单详情
func (s *TicketService) Get(ctx context.Context, id uint) (*TicketDetail, error) {
	t, err := s.ticketR.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	stmts, err := s.stmtR.ListByTicket(ctx, id)
	if err != nil {
		return nil, err
	}
	wf, err := s.wfR.ListByTicket(ctx, id)
	if err != nil {
		return nil, err
	}
	var steps []model.AuditStep
	if len(t.AuditConfig) > 0 {
		_ = json.Unmarshal(t.AuditConfig, &steps)
	}
	return &TicketDetail{Ticket: t, Statements: stmts, Workflow: wf, AuditSteps: steps}, nil
}

func (s *TicketService) GetByApprovalInstanceID(ctx context.Context, approvalInstanceID uint) (*model.SQLChangeTicket, error) {
	return s.ticketR.GetByApprovalInstanceID(ctx, approvalInstanceID)
}

// List 工单列表
func (s *TicketService) List(ctx context.Context, f dbrepo.TicketFilter, page, pageSize int) ([]model.SQLChangeTicket, int64, error) {
	return s.ticketR.List(ctx, f, page, pageSize)
}

// Agree 审批通过，推进到下一步；若为最后一步则标记为 Ready
func (s *TicketService) Agree(ctx context.Context, id uint, username, comment string) error {
	t, err := s.ticketR.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != model.TicketStatusPending {
		return fmt.Errorf("工单当前状态不允许审批")
	}
	steps, err := decodeSteps(t.AuditConfig)
	if err != nil {
		return err
	}
	if !inSlice(steps[t.CurrentStep].Approvers, username) {
		return fmt.Errorf("当前步骤无该审批人权限")
	}
	next := t.CurrentStep + 1
	fields := map[string]any{"current_step": next}
	if next >= len(steps) {
		fields["status"] = model.TicketStatusReady
		fields["assigned"] = ""
	} else {
		fields["assigned"] = strings.Join(steps[next].Approvers, ",")
	}
	if err := s.ticketR.UpdateFields(ctx, id, fields); err != nil {
		return err
	}
	return s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
		TicketID: id, WorkID: t.WorkID, Username: username, Action: "agree",
		Step: t.CurrentStep, Comment: comment,
	})
}

// Reject 驳回
func (s *TicketService) Reject(ctx context.Context, id uint, username, comment string) error {
	t, err := s.ticketR.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.Status != model.TicketStatusPending {
		return fmt.Errorf("工单当前状态不允许审批")
	}
	if err := s.ticketR.UpdateFields(ctx, id, map[string]any{
		"status":   model.TicketStatusRejected,
		"assigned": "",
	}); err != nil {
		return err
	}
	return s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
		TicketID: id, WorkID: t.WorkID, Username: username, Action: "reject",
		Step: t.CurrentStep, Comment: comment,
	})
}

// Cancel 申请人撤回
func (s *TicketService) Cancel(ctx context.Context, id uint, username string) error {
	t, err := s.ticketR.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if t.Applicant != username {
		return fmt.Errorf("仅申请人可撤回")
	}
	if t.Status != model.TicketStatusPending && t.Status != model.TicketStatusReady {
		return fmt.Errorf("当前状态不允许撤回")
	}
	if err := s.ticketR.UpdateFields(ctx, id, map[string]any{
		"status":   model.TicketStatusCancelled,
		"assigned": "",
	}); err != nil {
		return err
	}
	return s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
		TicketID: id, WorkID: t.WorkID, Username: username, Action: "cancel", Step: t.CurrentStep,
	})
}

// Execute 执行工单（同步）
func (s *TicketService) Execute(ctx context.Context, id uint, username string) error {
	if err := s.executor.Execute(ctx, id); err != nil {
		_ = s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
			TicketID: id, Username: username, Action: "execute_failed", Comment: err.Error(),
		})
		return err
	}
	t, err := s.ticketR.GetByID(ctx, id)
	if err == nil {
		_ = s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
			TicketID: id, WorkID: t.WorkID, Username: username, Action: "execute_success",
		})
	}
	return nil
}

func decodeSteps(raw []byte) ([]model.AuditStep, error) {
	if len(raw) == 0 {
		return nil, fmt.Errorf("审批流未配置")
	}
	var steps []model.AuditStep
	if err := json.Unmarshal(raw, &steps); err != nil {
		return nil, fmt.Errorf("审批流解析失败: %w", err)
	}
	if len(steps) == 0 {
		return nil, fmt.Errorf("审批流为空")
	}
	return steps, nil
}

func inSlice(list []string, v string) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}

func firstNonEmpty(a, b string) string {
	if a != "" {
		return a
	}
	return b
}

func genWorkID() string {
	return fmt.Sprintf("SQL%s", time.Now().Format("20060102150405"))
}

func buildSQLTicketApprovalRecordID(ticketID uint) uint {
	return sqlTicketApprovalRecordOffset + ticketID
}

func (s *TicketService) createApprovalFlow(ctx context.Context, ticket *model.SQLChangeTicket, steps []model.AuditStep) error {
	if ticket == nil || len(steps) == 0 || s.instanceR == nil || s.nodeInstR == nil {
		return nil
	}

	now := time.Now()
	instance := &models.ApprovalInstance{
		RecordID:         buildSQLTicketApprovalRecordID(ticket.ID),
		ChainID:          0,
		ChainName:        fmt.Sprintf("SQL 工单 %s", ticket.WorkID),
		Status:           "pending",
		CurrentNodeOrder: 1,
		StartedAt:        &now,
	}
	if err := s.instanceR.Create(ctx, instance); err != nil {
		return err
	}

	nodeInstances := make([]models.ApprovalNodeInstance, 0, len(steps))
	for i, step := range steps {
		resolvedApprovers := strings.Join(step.Approvers, ",")
		if s.approvers != nil {
			ids, err := s.approvers.ResolveApprovers(ctx, "user", resolvedApprovers, 0)
			if err != nil {
				return err
			}
			resolvedApprovers = ids
		}
		if strings.TrimSpace(resolvedApprovers) == "" {
			return fmt.Errorf("审批步骤 `%s` 未解析到有效审批人", step.StepName)
		}

		nodeName := strings.TrimSpace(step.StepName)
		if nodeName == "" {
			nodeName = fmt.Sprintf("审批节点 %d", i+1)
		}
		nodeInstances = append(nodeInstances, models.ApprovalNodeInstance{
			InstanceID:    instance.ID,
			NodeID:        uint(i + 1),
			NodeName:      nodeName,
			NodeOrder:     i + 1,
			ApproveMode:   "any",
			ApproveCount:  1,
			ApproverType:  "user",
			Approvers:     resolvedApprovers,
			Status:        "pending",
			RejectOnAny:   true,
			TimeoutAction: "auto_reject",
		})
	}
	if err := s.nodeInstR.CreateBatch(ctx, nodeInstances); err != nil {
		return err
	}

	firstNode, err := s.nodeInstR.GetByInstanceIDAndOrder(ctx, instance.ID, 1)
	if err != nil {
		return err
	}
	timeoutAt := now.Add(60 * time.Minute)
	if err := s.nodeInstR.Activate(ctx, firstNode.ID, &timeoutAt); err != nil {
		return err
	}

	if err := s.ticketR.UpdateFields(ctx, ticket.ID, map[string]any{
		"approval_instance_id": instance.ID,
		"assigned":             nodeInstances[0].Approvers,
	}); err != nil {
		return err
	}
	ticket.ApprovalInstanceID = &instance.ID
	ticket.Assigned = nodeInstances[0].Approvers
	_ = s.wfR.Create(ctx, &model.SQLChangeWorkflowDetail{
		TicketID: ticket.ID,
		WorkID:   ticket.WorkID,
		Username: ticket.Applicant,
		Action:   "approval_started",
		Step:     0,
		Comment:  fmt.Sprintf("已进入统一审批中心，审批实例 #%d", instance.ID),
	})
	return nil
}

// resolveAuditor 加载默认规则集；未配置或解析失败则回退到内置默认
func (s *TicketService) resolveAuditor(ctx context.Context) Auditor {
	if s.ruleR == nil {
		return s.auditor
	}
	m, err := s.ruleR.GetDefault(ctx)
	if err != nil || m == nil || len(m.Config) == 0 {
		return s.auditor
	}
	var cfg model.AuditRuleConfig
	if err := json.Unmarshal(m.Config, &cfg); err != nil {
		return s.auditor
	}
	return NewBuiltinAuditorWith(cfg)
}
