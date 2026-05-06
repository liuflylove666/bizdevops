package approval

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"devops/internal/domain/notification/service/telegram"
	"devops/internal/models"
	"devops/internal/repository"
	"devops/internal/service/notification"

	"gorm.io/gorm"
)

// NotificationService 审批通知（仅 Telegram：默认机器人 + 接收方 chat_id）
type NotificationService struct {
	telegramBotRepo *repository.TelegramBotRepository
	templateService *notification.TemplateService
}

// NewNotificationService 创建通知服务
func NewNotificationService(db *gorm.DB) *NotificationService {
	return &NotificationService{
		telegramBotRepo: repository.NewTelegramBotRepository(db),
		templateService: notification.NewTemplateService(repository.NewMessageTemplateRepository(db)),
	}
}

// ApprovalNotifyRequest 审批通知请求
type ApprovalNotifyRequest struct {
	Instance     *models.ApprovalInstance
	NodeInstance *models.ApprovalNodeInstance
	Approvers    []string // Telegram chat_id（逗号拆分后的列表）
	AppName      string
	EnvName      string
	Operator     string
	Description  string
}

func (s *NotificationService) sendTelegramTemplate(ctx context.Context, chatID, templateName string, data map[string]interface{}) error {
	chatID = strings.TrimSpace(chatID)
	if chatID == "" {
		return fmt.Errorf("empty chat_id")
	}

	bot, err := s.telegramBotRepo.GetDefault(ctx)
	if err != nil {
		return fmt.Errorf("load default telegram bot: %w", err)
	}
	if bot == nil {
		return fmt.Errorf("no default telegram bot")
	}
	token := strings.TrimSpace(bot.Token)
	if token == "" {
		return fmt.Errorf("telegram bot token empty")
	}

	text, err := s.templateService.Render(ctx, templateName, data)
	if err != nil {
		return fmt.Errorf("render template: %w", err)
	}

	client := telegram.NewClient(token, bot.APIBaseURL)
	return client.SendMessage(ctx, &telegram.SendMessageRequest{ChatID: chatID, Text: text})
}

// SendApprovalRequest 发送审批请求通知
func (s *NotificationService) SendApprovalRequest(ctx context.Context, req *ApprovalNotifyRequest) error {
	timeoutInfo := ""
	if req.NodeInstance.TimeoutAt != nil {
		timeoutInfo = fmt.Sprintf("⏰ 超时时间: %s", req.NodeInstance.TimeoutAt.Format("2006-01-02 15:04:05"))
	}
	modeText := getModeText(req.NodeInstance.ApproveMode, req.NodeInstance.ApproveCount)

	data := map[string]interface{}{
		"AppName":        req.AppName,
		"EnvName":        req.EnvName,
		"Operator":       req.Operator,
		"ModeText":       modeText,
		"NodeName":       req.NodeInstance.NodeName,
		"NodeOrder":      req.NodeInstance.NodeOrder,
		"Description":    req.Description,
		"TimeoutInfo":    timeoutInfo,
		"NodeInstanceID": req.NodeInstance.ID,
		"InstanceID":     req.Instance.ID,
	}

	for _, approverID := range req.Approvers {
		if err := s.sendTelegramTemplate(ctx, approverID, "APPROVAL_REQUEST", data); err != nil {
			log.Printf("[NotificationService] 发送审批请求 Telegram 失败: approver=%s, err=%v", approverID, err)
		} else {
			log.Printf("[NotificationService] 发送审批请求 Telegram 成功: approver=%s", approverID)
		}
	}

	return nil
}

// SendApprovalResult 发送审批结果通知（当前无发起人 Telegram chat_id，仅记录日志）
func (s *NotificationService) SendApprovalResult(ctx context.Context, instance *models.ApprovalInstance, result string, operator string) error {
	_ = ctx
	log.Printf("[NotificationService] 审批结果: instance=%d, result=%s, operator=%s, chain=%s", instance.ID, result, operator, instance.ChainName)
	return nil
}

// SendTimeoutReminder 发送超时提醒通知
func (s *NotificationService) SendTimeoutReminder(ctx context.Context, nodeInstance *models.ApprovalNodeInstance, approvers []string) error {
	remainingTime := ""
	if nodeInstance.TimeoutAt != nil {
		remaining := time.Until(*nodeInstance.TimeoutAt)
		if remaining > 0 {
			remainingTime = fmt.Sprintf("剩余 %d 分钟", int(remaining.Minutes()))
		}
	}

	data := map[string]interface{}{
		"NodeName":      nodeInstance.NodeName,
		"RemainingTime": remainingTime,
		"InstanceID":    nodeInstance.InstanceID,
	}

	for _, approverID := range approvers {
		if err := s.sendTelegramTemplate(ctx, approverID, "APPROVAL_TIMEOUT_REMINDER", data); err != nil {
			log.Printf("[NotificationService] 发送超时提醒 Telegram 失败: approver=%s, err=%v", approverID, err)
		}
	}

	return nil
}

// SendTimeoutCancelled 发送超时取消通知
func (s *NotificationService) SendTimeoutCancelled(ctx context.Context, instance *models.ApprovalInstance, requesterID string) error {
	data := map[string]interface{}{
		"ChainName": instance.ChainName,
		"Time":      time.Now().Format("2006-01-02 15:04:05"),
	}

	if strings.TrimSpace(requesterID) != "" {
		if err := s.sendTelegramTemplate(ctx, requesterID, "APPROVAL_TIMEOUT_CANCELLED", data); err != nil {
			log.Printf("[NotificationService] 发送超时取消 Telegram 失败: requester=%s, err=%v", requesterID, err)
		} else {
			log.Printf("[NotificationService] 发送超时取消 Telegram 成功: requester=%s", requesterID)
		}
	}

	return nil
}

// getModeText 获取审批模式文本
func getModeText(mode string, count int) string {
	switch mode {
	case "any":
		return "任一人通过"
	case "all":
		return "所有人通过"
	case "count":
		return fmt.Sprintf("%d人通过", count)
	default:
		return mode
	}
}

// ParseApprovers 解析审批人 ID 列表（Telegram 场景下为 chat_id）
func ParseApprovers(approvers string) []string {
	if approvers == "" {
		return nil
	}
	return strings.Split(approvers, ",")
}
