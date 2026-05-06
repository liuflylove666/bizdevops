# ADR-0006: AI Copilot 全局悬浮、非菜单形态

- **状态**: Accepted
- **日期**: 2026-04-21
- **决策人**: 产品总监
- **涉及 Epic**: Epic 5（AI Copilot 全局化）

## 背景

当前 AI 能力已经实现得相当完整（`internal/service/ai/*`），功能包括会话、知识库、工具执行（查询类 + 操作类）、LLM 配置。但用户触达路径是：

- 菜单 `AI 知识库 / AI 配置` 进入独立页面
- 对话页面独立渲染，脱离业务上下文

**真正高价值的交互** —— "为什么这个 Pod 重启了 3 次"、"帮我回滚这个发布单"、"告警原因分析" —— **都发生在业务页面里**，而不是独立的 AI 页面。

好消息：前端已在 `web/src/components/ai/CopilotDock.vue`（原 `AIChatWidget`）实现了悬浮窗雏形（已挂载在 `MainLayout.vue` 中）。

## 决策

**AI Copilot 只有一个形态：全局悬浮 + 页面上下文感知。**

- 悬浮按钮常驻右下角，所有业务页面可见
- 抽屉打开时**自动采集 `PageContext`**：当前路由、当前应用 ID、选中实体（Pod/Release/Alert ID）、时间范围等
- Copilot 根据 Context 提供定向 Prompt 建议（如在告警详情页 → "分析此告警根因"）
- **取消"AI 对话"独立菜单**；仅保留管理员入口："AI 知识库"、"LLM 配置"

## 方案对比

| 方案 | 评价 | 结论 |
|---|---|---|
| A. 菜单 + 悬浮并存 | 入口冗余，用户困惑 | ❌ 否 |
| B. 纯菜单（回归传统聊天） | 脱离上下文，价值不足 | ❌ 否 |
| **C. 全局悬浮 + 上下文** | 差异化竞争力 | ✅ **采纳** |

## 后果

- ✅ AI 使用频率从"尝鲜型"升级为"日常工具"
- ✅ 危险操作（回滚、重启）必须在 Context 下有二次确认 UI 约束（沿用 `IsDangerousAction`）
- ⚠️ 每个业务页面都需要实现 `PageContext` 采集（通过 `useAIContext()` composable）
- ⚠️ 依赖稳定的 LLM 上游，需要 Fallback 策略

## 实施动作

- [ ] 完善 `web/src/composables/useAIContext.ts`（路由 → context 自动映射）
- [ ] 扩充意图识别：发布 / 回滚 / 查告警 / 生成 Release 4 类主流程
- [ ] 危险动作 UI 规范：确认弹窗 + 审计日志
- [ ] 删除 AI 对话独立菜单，仅保留"知识库"与"LLM 配置"
- [ ] 文档：开发者如何为自己的页面接入 Context

## 参考

- 代码：`internal/service/ai/*`、`web/src/components/ai/CopilotDock.vue`
