package notification

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/models/system"
	"devops/internal/modules/system/repository"
)

// InitDefaultTemplates 初始化默认消息模板
func InitDefaultTemplates(db *gorm.DB) error {
	repo := repository.NewMessageTemplateRepository(db)
	svc := NewTemplateService(repo)

	defaults := []system.MessageTemplate{
		{
			Name:        "SSL_CERT_ALERT",
			Type:        "card",
			Description: "SSL证书过期告警卡片",
			IsActive:    true,
			Content: `{
  "config": { "wide_screen_mode": true },
  "header": {
    "title": { "tag": "plain_text", "content": "{{.Title}}" },
    "template": "{{.HeaderColor}}"
  },
  "elements": [
    {
      "tag": "div",
      "text": {
        "tag": "lark_md",
        "content": "**域名**: {{.Domain}}\n**告警级别**: {{.AlertLevel}}\n**剩余天数**: {{.DaysRemaining}}天\n**过期时间**: {{.ExpiryDate}}\n**颁发者**: {{.Issuer}}"
      }
    }
  ]
}`,
		},
		{
			Name:        "HEALTH_CHECK_ALERT",
			Type:        "card",
			Description: "健康检查失败告警卡片",
			IsActive:    true,
			Content: `{
  "config": { "wide_screen_mode": true },
  "header": {
    "title": { "tag": "plain_text", "content": "{{.Title}}" },
    "template": "red"
  },
  "elements": [
    {
      "tag": "div",
      "text": {
        "tag": "lark_md",
        "content": "**名称**: {{.Name}}\n**类型**: {{.Type}}\n**状态**: Unhealthy\n**错误信息**: {{.ErrorMsg}}\n**时间**: {{.Time}}"
      }
    }
  ]
}`,
		},
		{
			Name:        "COST_ANOMALY",
			Type:        "text",
			Description: "成本异常告警（Telegram 纯文本）",
			IsActive:    true,
			Content: `⚠️ 成本异常告警
日期: {{.Date}}
实际成本: ¥{{.ActualCost}}
预期成本: ¥{{.ExpectedCost}}
偏差: {{.Deviation}}%
{{.Message}}`,
		},
		{
			Name:        "COST_WASTE",
			Type:        "text",
			Description: "资源浪费提示（Telegram 纯文本）",
			IsActive:    true,
			Content: `💡 资源浪费提示
浪费成本: ¥{{.WastedCost}}
闲置资源: {{.IdleCount}} 个
超配资源: {{.OverCount}} 个
{{.Message}}`,
		},
		{
			Name:        "COST_BUDGET_EXCEEDED",
			Type:        "text",
			Description: "成本预算超支（Telegram 纯文本）",
			IsActive:    true,
			Content: `🔴 成本预算超支
项目: {{.Project}}
当前花费: ¥{{.CurrentCost}}
预算: ¥{{.Budget}}
超支: ¥{{.Overrun}}
使用率: {{.UsageRate}}%
{{.Message}}`,
		},
		{
			Name:        "COST_BUDGET_WARNING",
			Type:        "text",
			Description: "成本预算预警（Telegram 纯文本）",
			IsActive:    true,
			Content: `💰 成本预算预警
项目: {{.Project}}
当前花费: ¥{{.CurrentCost}}
预算: ¥{{.Budget}}
使用率: {{.UsageRate}}%
{{.Message}}`,
		},
		{
			Name:        "APPROVAL_REQUEST",
			Type:        "text",
			Description: "发布审批请求（Telegram 纯文本）",
			IsActive:    true,
			Content: `🔔 发布审批请求
应用: {{.AppName}}
环境: {{.EnvName}}
申请人: {{.Operator}}
审批模式: {{.ModeText}}
当前节点: {{.NodeName}}（第 {{.NodeOrder}} 步）
说明: {{.Description}}
{{if .TimeoutInfo}}{{.TimeoutInfo}}
{{end}}
请在平台处理审批实例 #{{.InstanceID}}（节点实例 #{{.NodeInstanceID}}）。`,
		},
		{
			Name:        "APPROVAL_RESULT",
			Type:        "text",
			Description: "审批结果通知（Telegram 纯文本）",
			IsActive:    true,
			Content: `{{.Title}}
{{.ResultText}}
审批链: {{.ChainName}}
操作人: {{.Operator}}
时间: {{.Time}}`,
		},
		{
			Name:        "APPROVAL_TIMEOUT_REMINDER",
			Type:        "text",
			Description: "审批超时提醒（Telegram 纯文本）",
			IsActive:    true,
			Content: `⏰ 审批即将超时
节点: {{.NodeName}}
{{.RemainingTime}}
请尽快处理实例 #{{.InstanceID}}。`,
		},
		{
			Name:        "APPROVAL_TIMEOUT_CANCELLED",
			Type:        "text",
			Description: "审批超时取消（Telegram 纯文本）",
			IsActive:    true,
			Content: `⏰ 审批已超时取消
审批链: {{.ChainName}}
取消时间: {{.Time}}
如需重新发布，请重新提交发布申请。`,
		},
	}
	
	return svc.EnsureDefaultTemplates(context.Background(), defaults)
}
