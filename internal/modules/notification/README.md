# 消息通知模块 (Notification Module)

## 功能概述
产品通知收敛为 **Telegram** 单一渠道，配合消息模板用于流水线、部署、告警等场景。

## 文件结构
```
notification/
├── handler/                        # HTTP 处理器
│   ├── telegram_handler.go        # Telegram Bot / 消息接口
│   └── template_handler.go        # 消息模板 CRUD / 预览
```

## 主要功能
- **Telegram**: Bot 管理、消息发送、消息日志
- **模板渲染**: 消息模板 CRUD、渲染预览

## 历史兼容
飞书（Feishu）、钉钉（DingTalk）、企业微信（WeChat Work）、通用 Webhook、Slack 以及统一通知渠道（ADR-0005）均已彻底移除。存量库升级请执行 `migrations/patch_206_drop_wechatwork_and_unified_channels.sql`。
