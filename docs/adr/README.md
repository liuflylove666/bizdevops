# 架构决策记录（ADR）

本目录收录 JeriDevOps v2.0 重构过程中的**不可逆**或**全局性**技术决策。每条决策一旦落地，将在代码、数据、API 层面带来连锁影响。

## 维护约定

- 文件命名：`NNNN-<kebab-case-title>.md`，NNNN 单调递增
- 状态流转：`Proposed` → `Accepted` → `Deprecated` / `Superseded by NNNN`
- 修改已 Accepted 的 ADR 请新建一条 ADR，在新 ADR 中 `Supersedes NNNN`，不直接覆盖历史
- PR 涉及架构级改动时必须引用对应 ADR

## 索引（v2.0）

| ID | 标题 | 状态 | 影响范围 |
|---|---|---|---|
| [0001](./0001-gitops-as-sole-cd-engine.md) | Argo CD 作为唯一 CD 执行引擎 | Accepted | 发布域、基础设施 |
| [0002](./0002-release-as-aggregate-root.md) | Release 聚合根模型（吞并 DeployRecord / Promotion） | Accepted | 数据模型、API |
| [0003](./0003-argo-rollouts-replaces-self-built-progressive-delivery.md) | Argo Rollouts 替代自建金丝雀/蓝绿 | Accepted | 发布、数据模型 |
| [0005](./0005-notification-channel-unification.md) | NotificationChannel 统一抽象 | Accepted | 通知、UI |
| [0006](./0006-global-copilot-no-menu.md) | AI Copilot 全局悬浮、非菜单形态 | Accepted | 前端架构 |
| [0007](./0007-jira-as-authoritative-planning-source.md) | Jira 作为需求规划权威源 | Accepted | 规划域 |

## ADR 模板

新建 ADR 请使用 [`_template.md`](./_template.md)。
