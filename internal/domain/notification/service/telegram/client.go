// Package telegram 提供 Telegram Bot API 客户端
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"devops/pkg/logger"
)

const defaultAPIBaseURL = "https://api.telegram.org"

// Client Telegram 客户端
type Client struct {
	token      string
	apiBaseURL string
	logger     *logger.Logger
	httpClient *http.Client
}

// NewClient 创建 Telegram 客户端。apiBaseURL 为空时使用官方地址。
func NewClient(token, apiBaseURL string) *Client {
	base := strings.TrimRight(apiBaseURL, "/")
	if base == "" {
		base = defaultAPIBaseURL
	}
	return &Client{
		token:      token,
		apiBaseURL: base,
		logger:     logger.NewLogger("INFO"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// SendMessage 调用 Bot API 的 sendMessage 方法
func (c *Client) SendMessage(ctx context.Context, req *SendMessageRequest) error {
	if c.token == "" {
		return fmt.Errorf("telegram bot token is empty")
	}
	if req.ChatID == "" {
		return fmt.Errorf("chat_id is required")
	}

	data, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal message failed: %w", err)
	}

	url := fmt.Sprintf("%s/bot%s/sendMessage", c.apiBaseURL, c.token)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(data))
	if err != nil {
		return fmt.Errorf("create request failed: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	var result APIResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode response failed: %w", err)
	}

	if !result.OK {
		return fmt.Errorf("telegram API error: %d - %s", result.ErrorCode, result.Description)
	}

	c.logger.Info("Telegram message sent to chat %s", req.ChatID)
	return nil
}

// GetMe 调用 getMe 验证 Token 有效性
func (c *Client) GetMe(ctx context.Context) (map[string]any, error) {
	if c.token == "" {
		return nil, fmt.Errorf("telegram bot token is empty")
	}
	url := fmt.Sprintf("%s/bot%s/getMe", c.apiBaseURL, c.token)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		OK          bool           `json:"ok"`
		ErrorCode   int            `json:"error_code"`
		Description string         `json:"description"`
		Result      map[string]any `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if !result.OK {
		return nil, fmt.Errorf("telegram API error: %d - %s", result.ErrorCode, result.Description)
	}
	return result.Result, nil
}
