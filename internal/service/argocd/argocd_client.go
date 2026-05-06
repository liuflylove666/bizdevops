package argocd

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

type ArgoCDClient struct {
	baseURL string
	token   string
	client  *http.Client
}

func argoCDHostRequiresInsecureTLS(baseURL string) bool {
	u, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return false
	}
	// compose 内会把 localhost 重写为 host.docker.internal；Argo CD 默认证书 SAN 不含该名，校验必然失败
	h := strings.ToLower(strings.TrimSpace(u.Hostname()))
	return h == "host.docker.internal"
}

func NewArgoCDClient(serverURL, token string, insecure ...bool) *ArgoCDClient {
	base := strings.TrimRight(normalizeArgoCDServerURL(serverURL), "/")
	skipTLSVerify := len(insecure) > 0 && insecure[0]
	if argoCDHostRequiresInsecureTLS(base) {
		skipTLSVerify = true
	}
	return &ArgoCDClient{
		baseURL: base,
		token:   token,
		client: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: skipTLSVerify},
			},
		},
	}
}

func normalizeArgoCDServerURL(serverURL string) string {
	trimmed := strings.TrimSpace(serverURL)
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}

	// Docker compose deployment: localhost in container points to itself.
	if strings.TrimSpace(os.Getenv("MYSQL_HOST")) == "mysql" {
		host := strings.ToLower(parsed.Hostname())
		if host == "localhost" || host == "127.0.0.1" || host == "::1" {
			port := parsed.Port()
			if port != "" {
				parsed.Host = "host.docker.internal:" + port
			} else {
				parsed.Host = "host.docker.internal"
			}
			return parsed.String()
		}
	}
	return trimmed
}

func (c *ArgoCDClient) doRequest(method, path string) ([]byte, error) {
	req, err := http.NewRequest(method, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

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

func (c *ArgoCDClient) doRequestWithBody(method, path, bodyStr string) ([]byte, error) {
	req, err := http.NewRequest(method, c.baseURL+path, strings.NewReader(bodyStr))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")

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

// TestConnection 测试连接
func (c *ArgoCDClient) TestConnection() error {
	_, err := c.doRequest("GET", "/api/v1/session/userinfo")
	return err
}

// ArgoApp Argo CD Application 简化结构
type ArgoApp struct {
	Metadata struct {
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
	} `json:"metadata"`
	Spec struct {
		Project string `json:"project"`
		Source  struct {
			RepoURL        string `json:"repoURL"`
			Path           string `json:"path"`
			TargetRevision string `json:"targetRevision"`
		} `json:"source"`
		Destination struct {
			Server    string `json:"server"`
			Namespace string `json:"namespace"`
		} `json:"destination"`
		SyncPolicy *struct {
			Automated *struct {
				Prune    bool `json:"prune"`
				SelfHeal bool `json:"selfHeal"`
			} `json:"automated"`
		} `json:"syncPolicy"`
	} `json:"spec"`
	Status struct {
		Sync struct {
			Status   string `json:"status"`
			Revision string `json:"revision"`
		} `json:"sync"`
		Health struct {
			Status string `json:"status"`
		} `json:"health"`
		OperationState *struct {
			FinishedAt string `json:"finishedAt"`
		} `json:"operationState"`
	} `json:"status"`
}

// ListApplications 获取所有应用
func (c *ArgoCDClient) ListApplications() ([]ArgoApp, error) {
	body, err := c.doRequest("GET", "/api/v1/applications")
	if err != nil {
		return nil, err
	}
	var result struct {
		Items []ArgoApp `json:"items"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse applications: %w", err)
	}
	return result.Items, nil
}

// GetApplication 获取单个应用
func (c *ArgoCDClient) GetApplication(name string) (*ArgoApp, error) {
	body, err := c.doRequest("GET", "/api/v1/applications/"+name)
	if err != nil {
		return nil, err
	}
	var app ArgoApp
	if err := json.Unmarshal(body, &app); err != nil {
		return nil, fmt.Errorf("parse application: %w", err)
	}
	return &app, nil
}

// SyncApplication 触发同步
func (c *ArgoCDClient) SyncApplication(name string) error {
	_, err := c.doRequestWithBody("POST", "/api/v1/applications/"+name+"/sync", "{}")
	return err
}

// ResourceTree 获取资源树
type ResourceNode struct {
	Group     string `json:"group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Health    *struct {
		Status string `json:"status"`
	} `json:"health"`
}

func (c *ArgoCDClient) GetResourceTree(name string) ([]ResourceNode, error) {
	body, err := c.doRequest("GET", "/api/v1/applications/"+name+"/resource-tree")
	if err != nil {
		return nil, err
	}
	var result struct {
		Nodes []ResourceNode `json:"nodes"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse resource tree: %w", err)
	}
	return result.Nodes, nil
}
