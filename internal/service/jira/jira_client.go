package jira

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type JiraClient struct {
	baseURL  string
	username string
	token    string
	authType string
	client   *http.Client
}

func NewJiraClient(baseURL, username, token, authType string) *JiraClient {
	base := strings.TrimRight(baseURL, "/")
	if !strings.Contains(base, "://") {
		base = "https://" + base
	}
	return &JiraClient{
		baseURL:  base,
		username: username,
		token:    token,
		authType: authType,
		client:   &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *JiraClient) doRequest(method, path string, query url.Values) ([]byte, error) {
	u := c.baseURL + path
	if len(query) > 0 {
		u += "?" + query.Encode()
	}
	req, err := http.NewRequest(method, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	if c.authType == "basic" {
		req.SetBasicAuth(c.username, c.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jira API %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *JiraClient) doPost(path string, payload interface{}) ([]byte, error) {
	var bodyReader io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		bodyReader = strings.NewReader(string(data))
	}
	u := c.baseURL + path
	req, err := http.NewRequest("POST", u, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	if c.authType == "basic" {
		req.SetBasicAuth(c.username, c.token)
	} else {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("jira request failed: %w", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("jira API %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

// TestConnection 测试连接
func (c *JiraClient) TestConnection() error {
	_, err := c.doRequest("GET", "/rest/api/2/myself", nil)
	return err
}

// GetCurrentUser 获取当前用户信息
func (c *JiraClient) GetCurrentUser() (map[string]interface{}, error) {
	body, err := c.doRequest("GET", "/rest/api/2/myself", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// ListProjects 获取项目列表
func (c *JiraClient) ListProjects() ([]map[string]interface{}, error) {
	body, err := c.doRequest("GET", "/rest/api/2/project", nil)
	if err != nil {
		return nil, err
	}
	var result []map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// SearchIssues JQL 搜索
func (c *JiraClient) SearchIssues(jql string, startAt, maxResults int, fields []string) (map[string]interface{}, error) {
	params := url.Values{
		"jql":        {jql},
		"startAt":    {fmt.Sprintf("%d", startAt)},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
	}
	if len(fields) > 0 {
		params.Set("fields", strings.Join(fields, ","))
	}
	body, err := c.doRequest("GET", "/rest/api/2/search", params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetIssue 获取单个 Issue 详情
func (c *JiraClient) GetIssue(issueKey string) (map[string]interface{}, error) {
	body, err := c.doRequest("GET", "/rest/api/2/issue/"+issueKey, nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetBoards 获取看板列表
func (c *JiraClient) GetBoards(projectKey string) (map[string]interface{}, error) {
	params := url.Values{}
	if projectKey != "" {
		params.Set("projectKeyOrId", projectKey)
	}
	body, err := c.doRequest("GET", "/rest/agile/1.0/board", params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetSprints 获取 Sprint 列表
func (c *JiraClient) GetSprints(boardID int, state string) (map[string]interface{}, error) {
	params := url.Values{}
	if state != "" {
		params.Set("state", state)
	}
	body, err := c.doRequest("GET", fmt.Sprintf("/rest/agile/1.0/board/%d/sprint", boardID), params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// GetSprintIssues 获取 Sprint 中的 Issues
func (c *JiraClient) GetSprintIssues(sprintID int, startAt, maxResults int) (map[string]interface{}, error) {
	params := url.Values{
		"startAt":    {fmt.Sprintf("%d", startAt)},
		"maxResults": {fmt.Sprintf("%d", maxResults)},
	}
	body, err := c.doRequest("GET", fmt.Sprintf("/rest/agile/1.0/sprint/%d/issue", sprintID), params)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// AddIssueComment 添加评论
func (c *JiraClient) AddIssueComment(issueKey, comment string) (map[string]interface{}, error) {
	payload := map[string]string{"body": comment}
	body, err := c.doPost("/rest/api/2/issue/"+issueKey+"/comment", payload)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}

// TransitionIssue 流转 Issue 状态
func (c *JiraClient) TransitionIssue(issueKey string, transitionID string) error {
	payload := map[string]interface{}{
		"transition": map[string]string{"id": transitionID},
	}
	_, err := c.doPost("/rest/api/2/issue/"+issueKey+"/transitions", payload)
	return err
}

// GetTransitions 获取可用的状态流转
func (c *JiraClient) GetTransitions(issueKey string) (map[string]interface{}, error) {
	body, err := c.doRequest("GET", "/rest/api/2/issue/"+issueKey+"/transitions", nil)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	json.Unmarshal(body, &result)
	return result, nil
}
