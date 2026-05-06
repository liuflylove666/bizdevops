# Models 数据模型包

本包包含 DevOps 平台的所有数据库模型定义，按功能领域拆分为多个子包。

## 目录结构

```
models/
├── application/     # 应用管理模型
│   ├── doc.go
│   └── application.go
├── deploy/          # 部署流程模型
│   ├── doc.go
│   ├── deploy.go    # 部署记录、部署锁、部署窗口
│   ├── approval.go  # 审批链、审批节点、审批记录
│   └── pipeline.go  # 流水线、构建任务、制品
├── infrastructure/  # 基础设施模型
│   ├── doc.go
│   ├── common.go    # 共享类型定义
│   ├── k8s.go       # K8s 集群、关联配置
│   └── cron_hpa.go  # 定时 HPA 配置
├── monitoring/      # 监控告警模型
│   ├── doc.go
│   ├── alert.go     # 告警配置、告警历史、静默规则
│   ├── healthcheck.go # 健康检查配置、历史
│   ├── log.go       # 日志告警、解析模板、数据源
│   └── cost.go      # 成本记录、预算、优化建议
├── notification/    # 消息通知模型
│   ├── doc.go
│   └── telegram.go  # Telegram Bot、消息日志
├── system/          # 系统管理模型
│   ├── doc.go
│   ├── user.go      # 用户模型
│   ├── rbac.go      # 角色、权限、关联表
│   ├── permission.go # 权限常量、检查函数
│   ├── audit.go     # 审计日志
│   ├── oa.go        # OA 数据、地址、通知配置
│   └── security.go  # 镜像仓库、扫描、合规规则
├── common.go        # 共享类型（JSONMap、EncryptionKey）
└── models.go        # 类型别名（向后兼容）
```

## 使用方式

### 方式1: 使用类型别名（向后兼容）

```go
import "devops/internal/models"

user := &models.User{Username: "admin"}
app := &models.Application{Name: "my-app"}
```

### 方式2: 直接使用子包（推荐）

```go
import (
    "devops/internal/models/system"
    "devops/internal/models/application"
)

user := &system.User{Username: "admin"}
app := &application.Application{Name: "my-app"}
```

## 子包说明

| 子包 | 说明 | 主要模型 |
|------|------|----------|
| `notification` | 消息通知 | TelegramBot, TelegramMessageLog |
| `infrastructure` | 基础设施 | K8sCluster, CronHPA, ArgoCDInstance, NacosInstance |
| `deploy` | 部署流程 | DeployRecord, ApprovalChain, Pipeline |
| `monitoring` | 监控告警 | AlertConfig, HealthCheckConfig, CostBudget |
| `system` | 系统管理 | User, Role, Permission, AuditLog |
| `application` | 应用管理 | Application, ApplicationEnv |

## 注意事项

1. 新代码建议直接导入子包使用，更清晰明确
2. `models.go` 提供类型别名，确保现有代码无需修改
3. 权限常量和检查函数也通过别名导出，可继续使用 `models.HasPermission()` 等
4. 共享类型（如 `JSONMap`、`EncryptionKey`）保留在根包的 `common.go` 中
