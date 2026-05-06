package kubernetes

import (
	"net"
	"net/url"
	"os"
	"strings"

	"k8s.io/client-go/rest"
)

// applyAPIServerHostRewrite 将 API server 的 127.0.0.1 / localhost / ::1 替换为 K8S_API_SERVER_HOST_REWRITE。
// 典型场景：DevOps 跑在 Docker 内，导入本机 kind/minikube 的 kubeconfig（server 指向宿主机上的随机端口），
// 容器内访问 127.0.0.1 无法到达宿主机 API，需改为 host.docker.internal（配合 compose extra_hosts）。
func applyAPIServerHostRewrite(cfg *rest.Config) {
	rewrite := strings.TrimSpace(os.Getenv("K8S_API_SERVER_HOST_REWRITE"))
	if rewrite == "" || cfg == nil {
		return
	}

	hostStr := strings.TrimSpace(cfg.Host)
	if hostStr == "" {
		return
	}

	raw := hostStr
	if !strings.Contains(raw, "://") {
		raw = "https://" + raw
	}
	u, err := url.Parse(raw)
	if err != nil {
		return
	}

	hn := u.Hostname()
	if hn != "127.0.0.1" && hn != "localhost" && hn != "::1" {
		return
	}

	port := u.Port()
	if port == "" {
		switch strings.ToLower(u.Scheme) {
		case "http":
			port = "80"
		default:
			port = "443"
		}
	}

	u.Host = net.JoinHostPort(rewrite, port)
	if strings.Contains(hostStr, "://") {
		cfg.Host = u.String()
	} else {
		cfg.Host = u.Host
	}

	// 仍用原 API 主机名校验服务端证书（kind 等证书 SAN 多为 127.0.0.1，与物理连接目标 host.docker.internal 不一致）
	if cfg.TLSClientConfig.ServerName == "" {
		cfg.TLSClientConfig.ServerName = hn
	}
}
