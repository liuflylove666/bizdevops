# ADR-0007: Jira 作为需求规划权威源

- **状态**: Accepted
- **日期**: 2026-04-21
- **决策人**: 产品总监
- **涉及 Epic**: Epic 7.5（Biz 改造）

## 背景

平台当前同时存在两套"需求/规划"体系：

- **自建**：`BizGoal` / `BizRequirement` / `BizVersion`（`internal/models/biz/*`）
- **外部**：Jira 集成（`internal/service/jira/*`）

二者字段高度重叠（标题、负责人、状态、优先级、截止时间），但**没有双向同步**，导致：
- 有 Jira 的团队要维护两套数据
- 没有 Jira 的团队只能用简版
- 平台内"需求"无法与真实迭代对齐

## 决策

**Jira 为需求规划的权威来源**。自建 Biz 模块改造为两态：

1. **已接 Jira** — Biz 模块变为**只读门面视图**：
   - 从 Jira 拉取 Issue + 聚合到应用 / 版本
   - 平台只保留"应用关联" + "版本关联"两类扩展元数据
   - 写操作全部回到 Jira
2. **未接 Jira** — 保留简版（OKR + 简单需求池），但明确标注"内置简版，建议接入 Jira"

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 保持双轨（现状） | 数据割裂 | ❌ 否 |
| B. 删除 Biz 模块 | 无 Jira 用户失去能力 | ❌ 否 |
| **C. Jira 为权威，Biz 作视图** | 两态平衡 | ✅ **采纳** |

## 后果

- ✅ 接 Jira 的团队不再双写
- ✅ Release 聚合根可直接关联 Jira Issue Key，实现端到端追溯
- ⚠️ Jira Webhook / 定时同步需要可靠性保证
- ⚠️ 前端规划页面需要根据是否接 Jira 渲染不同 UI

## 实施动作

- [ ] 探针接口：`GET /api/v2/planning/source` 返回 `jira | builtin`
- [ ] Jira 同步 Job：定时拉 + Webhook 推的双保险
- [ ] 前端规划页面：根据探针结果渲染 `PlanningJiraView` 或 `PlanningBuiltinView`
- [ ] `Release.jira_issue_keys[]` 字段
- [ ] 旧表 `biz_goals` / `biz_requirements` / `biz_versions` 保留，仅作为 builtin 模式使用

## 参考

- 代码：`internal/service/biz/planning_service.go`、`internal/service/jira/*`
