# ADR-0003: Argo Rollouts 替代自建金丝雀 / 蓝绿

- **状态**: Accepted
- **日期**: 2026-04-21
- **决策人**: 产品总监 + 技术总监
- **涉及 Epic**: Epic 2、Epic 7

## 背景

当前代码中存在自建的渐进式发布实现：

- 表：`CanaryRelease`、`BlueGreenDeployment`（历史实现，相关代码已移除）
- 服务：`internal/service/deploy/canary_service.go`
- API：`/applications/:id/release/canary/*`、`/deploy/bluegreen/*`
- 前端：独立页面 `CanaryList.vue`、`BlueGreenList.vue`（已标记 deleted，见 git status）

问题：
- 与 Argo Rollouts 能力严重重叠，维护两套流量切换逻辑
- 自建版本缺少 Analysis / Experiment 等高级能力
- 蓝绿 / 金丝雀本应是 Release 的**策略选项**，而非独立模块

## 决策

**v2.0 删除自建渐进式发布，改为使用 Argo Rollouts CRD。**

- 金丝雀 / 蓝绿作为 `Release.rollout_strategy` 枚举：`direct | canary | blue_green`
- 平台根据策略生成 Argo Rollouts CRD YAML 并写入 GitOps 仓库
- 运行时状态通过 Argo Rollouts API / kubectl 查询，不再自建状态机

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 保留自建 + 同时支持 Argo Rollouts | 两套并存，心智分裂 | ❌ 否 |
| B. 自建路径增强（加 Analysis） | 再造轮子 | ❌ 否 |
| **C. 全量迁移 Argo Rollouts** | 社区事实标准，生态完整 | ✅ **采纳** |

## 后果

- ✅ 获得 Argo Rollouts 的 Analysis、Experiment 与渐进式发布能力
- ✅ 策略与 Release 聚合，UI 心智一致
- ⚠️ 集群需部署 Argo Rollouts Controller
- ⚠️ 历史 Canary / BlueGreen 数据需归档
- 🔧 前端金丝雀页面已在 git status 中标记删除，需要确认清理

## 实施动作

- [ ] `Release` 增加字段 `rollout_strategy` + `rollout_config JSON`
- [ ] GitOps PR 生成时根据策略渲染 Argo Rollouts CRD
- [ ] 删除 `internal/service/deploy/canary_service.go`、`canary_handler.go`
- [ ] 删除 `CanaryRelease` / `BlueGreenDeployment` 表（数据归档后）
- [ ] 删除 `web/src/services/canary.ts`、`web/src/services/bluegreen.ts`（已在 git status 中）
- [ ] 集群运维：安装 Argo Rollouts Controller（Pilot 环境先行）
- [ ] 路由重定向：`/canary/list`、`/bluegreen/list` → `/argocd`（已在 router/index.ts 中完成）

## 参考

- 代码：`internal/service/deploy/canary_service.go`
- 上游：[Argo Rollouts](https://argoproj.github.io/rollouts/)
- 关联 ADR：0001、0002
