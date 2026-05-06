# ADR-0005: NotificationChannel 统一抽象

- **状态**: Superseded（v2.3 起废弃）
- **日期**: 2026-04-21
- **决策人**: 技术总监
- **涉及 Epic**: Epic 6（治理压缩）

> **更新（2026-04-23）**：产品通知收敛到 Telegram 单一渠道后，统一抽象层已无必要。`NotificationChannel` 模型、仓储、Handler、前端统一页均已于 v2.3 删除。飞书 / 钉钉 / 企业微信 / Slack / 通用 Webhook 一并下线，仅保留 `internal/domain/notification/service/telegram` 专属实现。本 ADR 仅作历史记录保留。

## 背景

通知能力分散在 4 套独立实现：

- 飞书：`internal/domain/notification/model/*` + `internal/service/notification/feishu/*` + 前端 4 个页面
- 钉钉：同上结构，独立一套
- 企业微信：同上结构，独立一套
- Telegram：同上结构，独立一套

问题：
- 4 套"应用管理 / 机器人管理 / 消息发送 / 消息日志"重复菜单（16 页）
- 绑定关系（Jenkins / K8s Cluster / 审批流）分散在 `JenkinsFeishuApp` / `JenkinsDingtalkApp` / `JenkinsWechatWorkApp` 等 N×M 表
- 新增一个通道（如 Slack / Lark Suite）需要改动 8+ 处

## 决策

**引入 `NotificationChannel` 统一抽象层**：

- Channel 为一等公民，类型字段 `provider` ∈ `{ feishu, dingtalk, wechatwork, telegram, slack, webhook }`
- 底层保留 provider 专属配置（credentials、app_secret），但对外以统一接口暴露
- 绑定关系表合并：`entity_notification_bindings(entity_type, entity_id, channel_id)`
- 前端：一个"通知通道"页管所有 provider，一个"消息模板"页

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 保持 4 套独立 | 扩展成本高 | ❌ 否 |
| B. 彻底删 provider 特有能力（如飞书卡片） | 丢失差异化能力 | ❌ 否 |
| **C. 统一接口 + provider 扩展点** | 扩展性 + 能力保留 | ✅ **采纳** |

## 后果

- ✅ UI 菜单 16 → 2（通道 + 模板）
- ✅ 新 Provider 扩展只需实现 `ChannelProvider` 接口
- ⚠️ 现有 4 套表需要迁移到 `notification_channels` 单表
- ⚠️ 模板字段需兼容富文本 / Markdown / 卡片三种形式

## 实施动作

- [ ] 领域接口：`pkg/notification/channel.go`（`type ChannelProvider interface`）
- [ ] 统一表：`notification_channels`（迁移脚本 `patch_201_notification_channels.sql`）
- [ ] 绑定表：`entity_notification_bindings` 取代 N×M 专用表
- [ ] 前端：合并 `FeishuApp` / `DingtalkApp` / `WechatWorkApp` / `TelegramApp` 页为单页
- [ ] 删除"手动发消息"页，仅保留通道配置页内的 Test 按钮

## 参考

- 代码：`internal/domain/notification/*`、`internal/service/notification/*`
- 关联：Epic 6
