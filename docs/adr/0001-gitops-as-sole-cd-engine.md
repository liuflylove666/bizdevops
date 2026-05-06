# ADR-0001: Argo CD 作为唯一 CD 执行引擎

- **状态**: Accepted
- **日期**: 2026-04-21
- **决策人**: 产品总监 + 技术总监
- **涉及 Epic**: Epic 2（GitOps 发布核心）

## 背景

当前 `internal/service/deploy/deploy_service.go` 的 `DeployMethod` 常量定义了两条部署路径：

```go
DeployMethodJenkins = "jenkins"
DeployMethodK8s     = "k8s"
```

再加上已经存在的 Argo CD 集成（`internal/service/argocd/*`），平台实际有 **3 条并存的 CD 执行路径**：

1. **Jenkins 直发**：通过 SSH/脚本登录目标机部署
2. **K8s 直 apply**：平台用 client-go 直接 `Apply` YAML
3. **Argo CD GitOps**：声明式同步

这导致：
- 发布日志割裂（Jenkins console / 平台审计表 / Argo Events）
- 回滚机制不一致（Jenkins 脚本 / K8s 回滚 / Argo CD rollback）
- "真相源"不唯一：线上与 Git 可能漂移且无法感知
- 审批流在 3 条路径中的落地深度不一致，存在审计盲区

## 决策

**v2.0 起，Argo CD（含 Argo Rollouts）是唯一的 CD 执行引擎。** 所有生产环境变更必须：

1. 以 Git commit / PR 为形式进入 `gitops/envs/<env>/<app>/` 目录
2. 由 Argo CD Application 对账生效
3. 平台只负责"生成 PR + 审批 PR + 观测同步状态"，**不再直接写入集群**

Jenkins 降级为 **可选 CI 触发源与构建执行器**，与部署解耦。

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 三路径并存（现状） | 心智负担重、审计割裂、回滚不一致 | ❌ 否 |
| B. 平台自建 Operator 对账 | 重复造 Argo CD 的轮子、团队缺少 SRE 运维深度 | ❌ 否 |
| **C. 全量迁移到 Argo CD** | 业界事实标准、生态成熟、声明式一致性 | ✅ **采纳** |

## 后果

- ✅ 单一真相源（Git）、完整审计链、声明式回滚
- ✅ 与 Argo Rollouts 天然集成，蓝绿/金丝雀由 CRD 驱动（见 ADR-0003）
- ⚠️ 新增 GitOps 仓库管理复杂度（目录约定、CODEOWNERS、分支保护）
- ⚠️ 研发需熟悉 PR-as-Release 心智模型，需培训
- 🔧 旧发布路径在过渡期通过 Feature Flag 控制，现已进入物理下线阶段
- 🔧 历史 `DeployRecord` 数据需要迁移至新 `Release` 聚合根（见 ADR-0002）

## 实施动作

- [x] 定义并清理 Feature Flag：`release.gitops_enabled`（GitOps 主路径已固定启用）
- [ ] GitOps 仓库目录规范：`envs/<env>/<app>/values.yaml` + `Chart.yaml` + `Application.yaml`
- [ ] 扩展 `internal/service/pipeline/gitops_handoff_service.go` 为主发布路径
- [ ] `internal/service/argocd/*` 增加 Application 注册 / Sync / Rollback API
- [x] `deploy_service.go` 的 Jenkins/K8s 直发路径已物理下线（固定拒绝并提示 GitOps）
- [ ] 3 个 Pilot 应用在 Sprint 3 完成切换
- [ ] 旧路径在 Sprint 7 物理下线

## 参考

- 代码：`internal/service/deploy/deploy_service.go`、`internal/service/argocd/*`、`internal/service/pipeline/gitops_handoff_service.go`
- 关联 ADR：0002（Release 聚合根）、0003（Argo Rollouts）
