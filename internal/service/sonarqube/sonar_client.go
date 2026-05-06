package sonarqube

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 15 * time.Second},
	}
}

func (c *Client) doRequest(method, path string) ([]byte, error) {
	url := c.baseURL + path
	req, err := http.NewRequest(method, url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.token, "")
	req.Header.Set("Accept", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("SonarQube API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func (c *Client) TestConnection() (map[string]interface{}, error) {
	data, err := c.doRequest("GET", "/api/system/status")
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}
	return result, json.Unmarshal(data, &result)
}

type SonarProject struct {
	Key        string `json:"key"`
	Name       string `json:"name"`
	Qualifier  string `json:"qualifier"`
	Visibility string `json:"visibility"`
}

func (c *Client) ListProjects(page, pageSize int) ([]SonarProject, int, error) {
	path := fmt.Sprintf("/api/projects/search?p=%d&ps=%d", page, pageSize)
	data, err := c.doRequest("GET", path)
	if err != nil {
		return nil, 0, err
	}
	var resp struct {
		Components []SonarProject `json:"components"`
		Paging     struct {
			Total int `json:"total"`
		} `json:"paging"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Components, resp.Paging.Total, nil
}

type QualityGate struct {
	Status     string            `json:"status"`
	Conditions []QualityGateCond `json:"conditions"`
}

type QualityGateCond struct {
	Status         string `json:"status"`
	MetricKey      string `json:"metricKey"`
	Comparator     string `json:"comparator"`
	ErrorThreshold string `json:"errorThreshold"`
	ActualValue    string `json:"actualValue"`
}

func (c *Client) GetQualityGate(projectKey string) (*QualityGate, error) {
	path := fmt.Sprintf("/api/qualitygates/project_status?projectKey=%s", projectKey)
	data, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		ProjectStatus QualityGate `json:"projectStatus"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return &resp.ProjectStatus, nil
}

type Measure struct {
	Metric string `json:"metric"`
	Value  string `json:"value"`
}

func (c *Client) GetMeasures(projectKey string, metrics []string) ([]Measure, error) {
	path := fmt.Sprintf("/api/measures/component?component=%s&metricKeys=%s", projectKey, strings.Join(metrics, ","))
	data, err := c.doRequest("GET", path)
	if err != nil {
		return nil, err
	}
	var resp struct {
		Component struct {
			Measures []Measure `json:"measures"`
		} `json:"component"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, err
	}
	return resp.Component.Measures, nil
}

type Issue struct {
	Key       string `json:"key"`
	Rule      string `json:"rule"`
	Severity  string `json:"severity"`
	Component string `json:"component"`
	Line      int    `json:"line"`
	Message   string `json:"message"`
	Status    string `json:"status"`
	Type      string `json:"type"`
}

func (c *Client) GetIssues(projectKey string, page, pageSize int, severities string) ([]Issue, int, error) {
	path := fmt.Sprintf("/api/issues/search?componentKeys=%s&p=%d&ps=%d", projectKey, page, pageSize)
	if severities != "" {
		path += "&severities=" + severities
	}
	data, err := c.doRequest("GET", path)
	if err != nil {
		return nil, 0, err
	}
	var resp struct {
		Issues []Issue `json:"issues"`
		Total  int     `json:"total"`
	}
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, 0, err
	}
	return resp.Issues, resp.Total, nil
}
