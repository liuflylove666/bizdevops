# Notification Domain

消息通知领域模块（DDD 分层）。产品通知唯一保留 **Telegram**（见 `service/telegram`、`model/telegram.go`、`repository/telegram_repo.go`）。

飞书（Feishu）、钉钉（DingTalk）、企业微信（WeChat Work）、通用 Webhook、Slack 等通道均已从代码库中彻底移除，统一通知渠道（`NotificationChannel` / ADR-0005）同步下线。

HTTP Handler 位于 `internal/modules/notification/handler/`（`telegram_handler.go`、`template_handler.go`）。
