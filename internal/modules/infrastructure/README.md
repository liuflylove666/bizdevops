# 🔨 基础设施模块 (Infrastructure Module)

## 功能概述
负责 Kubernetes / ArgoCD / Nacos / Jira / SonarQube / Confluence 等基础设施对接。Jenkins 已整体下线。

## 文件结构
```
infrastructure/
├── handler/                           # HTTP处理器
│   ├── k8s_cluster_handler.go        # K8s集群管理接口
│   ├── k8s_deployment_handler.go     # K8s部署管理接口
│   ├── k8s_pod_handler.go            # K8s Pod管理接口
│   ├── k8s_resource_handler.go       # K8s资源管理接口
│   ├── k8s_ops_ioc.go                # K8s操作依赖注入
│   ├── argocd_handler.go             # ArgoCD
│   ├── nacos_handler.go              # Nacos
│   ├── jira_handler.go               # Jira
│   ├── sonarqube_handler.go          # SonarQube
│   └── confluence_handler.go         # Confluence
└── repository/                        # 数据访问层
    ├── k8s_repo.go                   # K8s数据操作
    ├── argocd_repo.go                # ArgoCD
    ├── nacos_repo.go                 # Nacos
    ├── jira_repo.go                  # Jira
    ├── sonarqube_repo.go             # SonarQube
    ├── confluence_repo.go            # Confluence
    └── gitops_repo.go                # GitOps 仓库
```

## 主要功能
- **K8s 集群管理**: 集群配置、连接管理
- **K8s 资源管理**: Deployment、Pod、Service管理
- **K8s 操作**: 重启、扩缩容、日志查看
- **GitOps**: ArgoCD 应用、Nacos 配置、Jira 工单、SonarQube 扫描、Confluence 文档

## API接口
- `GET /k8s/clusters` - 获取 K8s 集群列表
- `POST /k8s/clusters` - 创建 K8s 集群
- `GET /k8s/pods` - 获取 Pod 列表
- `POST /k8s/restart` - 重启应用
- `GET /argocd/*` / `GET /nacos/*` / `GET /jira/*` / `GET /sonarqube/*` / `GET /confluence/*`

## 相关 Service
- `internal/service/kubernetes/` - K8s 业务逻辑
- `internal/service/argocd/`、`internal/service/nacos/`、`internal/service/jira/`、`internal/service/sonarqube/`、`internal/service/confluence/`
