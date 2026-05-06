package nacos

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type NacosClient struct {
	baseURL  string
	username string
	password string
	token    string
	client   *http.Client
}

func NewNacosClient(addr, username, password string) *NacosClient {
	base := strings.TrimRight(addr, "/")
	if !strings.Contains(base, "://") {
		base = "http://" + base
	}
	return &NacosClient{
		baseURL:  base,
		username: username,
		password: password,
		client:   &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *NacosClient) login() error {
	if c.username == "" {
		return nil
	}
	data := url.Values{
		"username": {c.username},
		"password": {c.password},
	}
	resp, err := c.client.PostForm(c.baseURL+"/nacos/v1/auth/login", data)
	if err != nil {
		return fmt.Errorf("nacos login: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("nacos login response: %s", string(body))
	}
	if token, ok := result["accessToken"].(string); ok && token != "" {
		c.token = token
		return nil
	}
	return fmt.Errorf("nacos login failed: %s", string(body))
}

func (c *NacosClient) doRequest(method, path string, params url.Values) ([]byte, error) {
	if c.token == "" && c.username != "" {
		if err := c.login(); err != nil {
			return nil, err
		}
	}
	if c.token != "" {
		params.Set("accessToken", c.token)
	}

	var req *http.Request
	var err error
	fullURL := c.baseURL + path

	if method == http.MethodGet || method == http.MethodDelete {
		fullURL += "?" + params.Encode()
		req, err = http.NewRequest(method, fullURL, nil)
	} else {
		req, err = http.NewRequest(method, fullURL, strings.NewReader(params.Encode()))
		if req != nil {
			req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		}
	}
	if err != nil {
		return nil, err
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	if resp.StatusCode == http.StatusForbidden && c.username != "" {
		if err := c.login(); err != nil {
			return nil, err
		}
		return c.doRequest(method, path, params)
	}

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("nacos API %s %s: status %d, body: %s", method, path, resp.StatusCode, string(body))
	}
	return body, nil
}

// --- Namespace ---

type NacosNamespace struct {
	Namespace         string `json:"namespace"`
	NamespaceShowName string `json:"namespaceShowName"`
	ConfigCount       int    `json:"configCount"`
}

func (c *NacosClient) ListNamespaces() ([]NacosNamespace, error) {
	body, err := c.doRequest(http.MethodGet, "/nacos/v1/console/namespaces", url.Values{})
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Code int              `json:"code"`
		Data []NacosNamespace `json:"data"`
	}
	if err := json.Unmarshal(body, &wrap); err != nil {
		var list []NacosNamespace
		if err2 := json.Unmarshal(body, &list); err2 == nil {
			return list, nil
		}
		return nil, fmt.Errorf("parse namespaces: %w, body: %s", err, string(body))
	}
	return wrap.Data, nil
}

// --- Config ---

type NacosConfigItem struct {
	ID      string `json:"id"`
	DataID  string `json:"dataId"`
	Group   string `json:"group"`
	Content string `json:"content,omitempty"`
	Type    string `json:"type"`
	Tenant  string `json:"tenant"`
	AppName string `json:"appName"`
	MD5     string `json:"md5"`
}

type ConfigListResult struct {
	TotalCount     int               `json:"totalCount"`
	PageNumber     int               `json:"pageNumber"`
	PagesAvailable int               `json:"pagesAvailable"`
	PageItems      []NacosConfigItem `json:"pageItems"`
}

func (c *NacosClient) ListConfigs(tenant, group, dataID string, page, pageSize int) (*ConfigListResult, error) {
	params := url.Values{
		"tenant":   {tenant},
		"pageNo":   {fmt.Sprintf("%d", page)},
		"pageSize": {fmt.Sprintf("%d", pageSize)},
		"search":   {"blur"},
		// Nacos 2.3.x requires dataId param to be present for /cs/configs list API.
		// If omitted, it returns "Required request parameter 'dataId' is not present".
		"dataId": {dataID},
	}
	if group != "" {
		params.Set("group", group)
	}
	body, err := c.doRequest(http.MethodGet, "/nacos/v1/cs/configs", params)
	if err != nil {
		return nil, err
	}
	var result ConfigListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse config list: %w, body: %s", err, string(body))
	}
	return &result, nil
}

func (c *NacosClient) GetConfig(tenant, group, dataID string) (string, error) {
	params := url.Values{
		"tenant": {tenant},
		"group":  {group},
		"dataId": {dataID},
	}
	body, err := c.doRequest(http.MethodGet, "/nacos/v1/cs/configs", params)
	if err != nil {
		return "", err
	}
	return string(body), nil
}

func (c *NacosClient) PublishConfig(tenant, group, dataID, content, configType string) error {
	params := url.Values{
		"tenant":  {tenant},
		"group":   {group},
		"dataId":  {dataID},
		"content": {content},
	}
	if configType != "" {
		params.Set("type", configType)
	}
	_, err := c.doRequest(http.MethodPost, "/nacos/v1/cs/configs", params)
	return err
}

func (c *NacosClient) DeleteConfig(tenant, group, dataID string) error {
	params := url.Values{
		"tenant": {tenant},
		"group":  {group},
		"dataId": {dataID},
	}
	_, err := c.doRequest(http.MethodDelete, "/nacos/v1/cs/configs", params)
	return err
}

// --- History ---

type ConfigHistoryItem struct {
	ID               string `json:"id"`
	Nid              int64  `json:"nid"`
	DataID           string `json:"dataId"`
	Group            string `json:"group"`
	Tenant           string `json:"tenant"`
	Content          string `json:"content,omitempty"`
	OpType           string `json:"opType"`
	CreatedTime      string `json:"createdTime"`
	LastModifiedTime string `json:"lastModifiedTime"`
}

type HistoryListResult struct {
	TotalCount     int                 `json:"totalCount"`
	PageNumber     int                 `json:"pageNumber"`
	PagesAvailable int                 `json:"pagesAvailable"`
	PageItems      []ConfigHistoryItem `json:"pageItems"`
}

func (c *NacosClient) ListConfigHistory(tenant, group, dataID string, page, pageSize int) (*HistoryListResult, error) {
	params := url.Values{
		"tenant":   {tenant},
		"group":    {group},
		"dataId":   {dataID},
		"pageNo":   {fmt.Sprintf("%d", page)},
		"pageSize": {fmt.Sprintf("%d", pageSize)},
		"search":   {"accurate"},
	}
	body, err := c.doRequest(http.MethodGet, "/nacos/v1/cs/history", params)
	if err != nil {
		return nil, err
	}
	var result HistoryListResult
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse history: %w, body: %s", err, string(body))
	}
	return &result, nil
}

func (c *NacosClient) GetConfigHistoryDetail(tenant, group, dataID string, nid int64) (*ConfigHistoryItem, error) {
	params := url.Values{
		"tenant": {tenant},
		"group":  {group},
		"dataId": {dataID},
		"nid":    {fmt.Sprintf("%d", nid)},
	}
	body, err := c.doRequest(http.MethodGet, "/nacos/v1/cs/history", params)
	if err != nil {
		return nil, err
	}
	var item ConfigHistoryItem
	if err := json.Unmarshal(body, &item); err != nil {
		return nil, fmt.Errorf("parse history detail: %w, body: %s", err, string(body))
	}
	return &item, nil
}

// --- Test Connection ---

func (c *NacosClient) TestConnection() error {
	if c.username != "" {
		if err := c.login(); err != nil {
			return err
		}
	}
	_, err := c.ListNamespaces()
	return err
}
