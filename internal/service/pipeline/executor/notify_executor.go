package executor

import (
	"bytes"
	"context"
	"devops/pkg/logger"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"text/template"
	"time"
)

// PipelineNotifyExecutor 流水线通知执行器
type PipelineNotifyExecutor struct {
	httpClient *http.Client
}

// NewPipelineNotifyExecutor 创建流水线通知执行器
func NewPipelineNotifyExecutor() *PipelineNotifyExecutor {
	return &PipelineNotifyExecutor{
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// NotifyConfig 通知配置
type NotifyConfig struct {
	Type       string            `json:"type"`        // telegram
	WebhookURL string            `json:"webhook_url"` // Telegram: Bot Token
	Secret     string            `json:"secret"`      // 签名密钥（预留，当前未使用）
	Template   string            `json:"template"`    // 消息模板
	AtUsers    []string          `json:"at_users"`    // @用户列表
	AtAll      bool              `json:"at_all"`      // @所有人
	Extra      map[string]string `json:"extra"`       // Telegram: chat_id 放在 extra["chat_id"]
}

// NotifyContext 通知上下文
type NotifyContext struct {
	PipelineName string `json:"pipeline_name"`
	PipelineID   uint   `json:"pipeline_id"`
	RunID        uint   `json:"run_id"`
	Status       string `json:"status"`
	TriggerBy    string `json:"trigger_by"`
	GitBranch    string `json:"git_branch"`
	GitCommit    string `json:"git_commit"`
	GitMessage   string `json:"git_message"`
	Duration     int    `json:"duration"`
	StartedAt    string `json:"started_at"`
	FinishedAt   string `json:"finished_at"`
	URL          string `json:"url"` // 详情页 URL
}

// Execute 执行通知
func (e *PipelineNotifyExecutor) Execute(ctx context.Context, config *NotifyConfig, notifyCtx *NotifyContext) error {
	log := logger.L().WithField("notify_type", config.Type)
	log.Info("发送通知")

	switch config.Type {
	case "telegram":
		return e.sendTelegram(ctx, config, notifyCtx)
	default:
		return fmt.Errorf("不支持的通知类型: %s", config.Type)
	}
}

// sendTelegram 通过 Bot API 发送纯文本（WebhookURL=Token，Extra["chat_id"]=Chat ID）
func (e *PipelineNotifyExecutor) sendTelegram(ctx context.Context, config *NotifyConfig, notifyCtx *NotifyContext) error {
	chatID := ""
	if config.Extra != nil {
		chatID = strings.TrimSpace(config.Extra["chat_id"])
	}
	if chatID == "" {
		return fmt.Errorf("telegram: extra.chat_id is required")
	}
	token := strings.TrimSpace(config.WebhookURL)
	if token == "" {
		return fmt.Errorf("telegram: bot token (webhook_url) is required")
	}
	content := e.renderTemplate(config.Template, notifyCtx)
	if content == "" {
		var sb strings.Builder
		sb.WriteString(fmt.Sprintf("流水线 %s %s\n", notifyCtx.PipelineName, e.statusText(notifyCtx.Status)))
		sb.WriteString(fmt.Sprintf("执行 #%d\n触发人: %s\n", notifyCtx.RunID, notifyCtx.TriggerBy))
		sb.WriteString(notifyCtx.URL)
		content = sb.String()
	}
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", token)
	msg := map[string]string{"chat_id": chatID, "text": content}
	return e.postJSON(ctx, apiURL, msg)
}

// postJSON 发送 JSON POST 请求
func (e *PipelineNotifyExecutor) postJSON(ctx context.Context, url string, data any) error {
	body, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("序列化消息失败: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("创建请求失败: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := e.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("发送请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("请求失败: %d - %s", resp.StatusCode, string(respBody))
	}

	return nil
}

// renderTemplate 渲染模板
func (e *PipelineNotifyExecutor) renderTemplate(tmplStr string, ctx *NotifyContext) string {
	if tmplStr == "" {
		return ""
	}

	tmpl, err := template.New("notify").Parse(tmplStr)
	if err != nil {
		logger.L().WithField("error", err).Warn("解析通知模板失败")
		return ""
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		logger.L().WithField("error", err).Warn("渲染通知模板失败")
		return ""
	}

	return buf.String()
}

// statusText 状态文本
func (e *PipelineNotifyExecutor) statusText(status string) string {
	switch status {
	case "success":
		return "构建成功"
	case "failed":
		return "构建失败"
	case "cancelled":
		return "已取消"
	case "running":
		return "运行中"
	default:
		return status
	}
}

// formatDuration 格式化时长
func (e *PipelineNotifyExecutor) formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%d秒", seconds)
	}
	if seconds < 3600 {
		return fmt.Sprintf("%d分%d秒", seconds/60, seconds%60)
	}
	return fmt.Sprintf("%d时%d分", seconds/3600, (seconds%3600)/60)
}

// PipelineNotifyService 流水线通知服务
type PipelineNotifyService struct {
	executor *PipelineNotifyExecutor
}

// NewPipelineNotifyService 创建流水线通知服务
func NewPipelineNotifyService() *PipelineNotifyService {
	return &PipelineNotifyService{
		executor: NewPipelineNotifyExecutor(),
	}
}

// SendPipelineNotification 发送流水线通知
func (s *PipelineNotifyService) SendPipelineNotification(ctx context.Context, configs []NotifyConfig, notifyCtx *NotifyContext) []error {
	var errors []error

	for _, config := range configs {
		if err := s.executor.Execute(ctx, &config, notifyCtx); err != nil {
			logger.L().WithField("type", config.Type).WithField("error", err).Error("发送通知失败")
			errors = append(errors, err)
		}
	}

	return errors
}

// ParseNotifyConfigs 解析通知配置
func ParseNotifyConfigs(configJSON string) ([]NotifyConfig, error) {
	if configJSON == "" {
		return nil, nil
	}

	var configs []NotifyConfig
	if err := json.Unmarshal([]byte(configJSON), &configs); err != nil {
		return nil, fmt.Errorf("解析通知配置失败: %w", err)
	}

	return configs, nil
}
