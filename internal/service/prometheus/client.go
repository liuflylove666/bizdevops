package prometheus

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// PromClient Prometheus HTTP API 客户端
type PromClient struct {
	baseURL  string
	authType string // none / basic / bearer
	username string
	password string // plaintext after decrypt
	client   *http.Client
}

func NewPromClient(baseURL, authType, username, password string) *PromClient {
	return &PromClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		authType: authType,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *PromClient) doGet(path string, params url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(params) > 0 {
		u += "?" + params.Encode()
	}
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return nil, err
	}
	c.setAuth(req)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *PromClient) setAuth(req *http.Request) {
	switch c.authType {
	case "basic":
		req.SetBasicAuth(c.username, c.password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+c.password)
	}
}

// PromResponse Prometheus API 标准响应
type PromResponse struct {
	Status string          `json:"status"`
	Data   json.RawMessage `json:"data"`
	Error  string          `json:"error,omitempty"`
}

// Query 即时查询 /api/v1/query
func (c *PromClient) Query(query string, ts string) (*PromResponse, error) {
	params := url.Values{"query": {query}}
	if ts != "" {
		params.Set("time", ts)
	}
	return c.apiCall("/api/v1/query", params)
}

// QueryRange 范围查询 /api/v1/query_range
func (c *PromClient) QueryRange(query, start, end, step string) (*PromResponse, error) {
	params := url.Values{
		"query": {query},
		"start": {start},
		"end":   {end},
		"step":  {step},
	}
	return c.apiCall("/api/v1/query_range", params)
}

// Labels 获取标签列表 /api/v1/labels
func (c *PromClient) Labels() (*PromResponse, error) {
	return c.apiCall("/api/v1/labels", nil)
}

// LabelValues 获取标签值 /api/v1/label/{name}/values
func (c *PromClient) LabelValues(name string) (*PromResponse, error) {
	return c.apiCall("/api/v1/label/"+name+"/values", nil)
}

// Series 获取时间序列 /api/v1/series
func (c *PromClient) Series(matchers []string, start, end string) (*PromResponse, error) {
	params := url.Values{"start": {start}, "end": {end}}
	for _, m := range matchers {
		params.Add("match[]", m)
	}
	return c.apiCall("/api/v1/series", params)
}

// Targets 获取目标 /api/v1/targets
func (c *PromClient) Targets() (*PromResponse, error) {
	return c.apiCall("/api/v1/targets", nil)
}

// TestConnection 测试连接
func (c *PromClient) TestConnection() error {
	_, err := c.apiCall("/api/v1/status/buildinfo", nil)
	return err
}

func (c *PromClient) apiCall(path string, params url.Values) (*PromResponse, error) {
	body, err := c.doGet(path, params)
	if err != nil {
		return nil, err
	}
	var resp PromResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if resp.Status == "error" {
		return nil, fmt.Errorf("prometheus error: %s", resp.Error)
	}
	return &resp, nil
}
