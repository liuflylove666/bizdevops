# ADR-0002: Release 聚合根模型（吞并 DeployRecord / Promotion）

- **状态**: Accepted
- **日期**: 2026-04-21
- **决策人**: 产品总监 + 技术总监
- **涉及 Epic**: Epic 2（GitOps 发布核心）、Epic 7（遗留清理）

## 背景

当前"发布"相关的数据模型分散为 5 张核心表（见 `internal/models/deploy/*`）：

- `DeployRecord`：应用部署记录（含审批、回滚、执行方式）
- `Release` + `ReleaseItem`：统一发布主单 + 子项
- `EnvPromotionRecord` + `EnvPromotionStep`：环境晋级
- `NacosRelease`：Nacos 配置发布单
- `ChangeEvent`：统一变更事件

问题：
- 同一次"上线"可能跨多张表，查询需 JOIN 3~4 张表
- 审批 / 风险评分 / 观测事件无法在同一实体上聚合
- API 数量多且语义重叠（`/deploys` / `/releases` / `/promotions` 三套增删改查）

## 决策

**以 `Release` 作为发布域的聚合根**，其他表降级为其构成部分：

```
Release (aggregate root)
├── metadata: id, title, description, risk_score, rollout_strategy, created_by, ...
├── items: []ReleaseItem
│     └── item_type ∈ { deployment, nacos, database }
│     └── ref_id → 对应子表
├── approval_instance_id → ApprovalInstance
└── change_events: []ChangeEvent  (投影，非主存储)
```

- `DeployRecord` → `ReleaseItem{item_type: "deployment"}` + `deployment_items` 明细子表
- `EnvPromotion*` → `ReleaseItem{item_type: "promotion"}`（promotion 被视为"环境复制型发布"）
- `NacosRelease` → `ReleaseItem{item_type: "nacos"}`
- `SQLChangeTicket` → `ReleaseItem{item_type: "database"}`（通过 ref_id 关联）
- `ChangeEvent` 降级为**事件流投影**，消费 Release 状态变化写入

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 保持 5 张表并列 | 查询复杂、API 泛滥 | ❌ 否 |
| B. 所有类型合并为一张大表（EAV） | 查询高效但破坏类型安全 | ❌ 否 |
| **C. Release 聚合根 + 子表 polymorphic 关联** | 保持类型清晰，API 统一 | ✅ **采纳** |

## 后果

- ✅ 一个 Release ID 穿透全链路：PR / 审批 / Argo CD Sync / 告警关联
- ✅ API 收敛：`/api/v2/releases/*` 取代 3 套旧 API
- ✅ 变更风险评分、DORA 指标、事件追溯均以 Release 为基础
- ⚠️ 现有前端 `DeployHistory.vue` 等页面需要改造数据源
- ⚠️ 多处引用 `DeployRecord` 的服务需要兼容适配器
- 🔧 数据迁移：`migrations/patch_200_release_aggregate.sql`

## 实施动作

- [ ] 扩展 `internal/models/deploy/release.go`：新增 `risk_score`、`rollout_strategy`、`approval_instance_id`、`argo_app_name` 等字段
- [ ] `ReleaseItem.item_type` 扩展枚举：`deployment | promotion | nacos | database`
- [ ] 数据迁移脚本：将历史 `DeployRecord`、`EnvPromotionRecord` 映射入 `release_items`
- [ ] 旧 API 保留，添加 `Deprecated` Header，返回体增加 `release_id` 字段
- [ ] 新 API `/app/api/v2/releases` 落地
- [ ] 前端 `web/src/services/release.ts` 重构
- [ ] Grace Period: v1 API 保留 3 个月

## 参考

- 代码：`internal/models/deploy/{release.go,deploy.go,promotion.go,nacos_release.go}`、`internal/service/release/release_service.go`
- 关联 ADR：0001、0003
