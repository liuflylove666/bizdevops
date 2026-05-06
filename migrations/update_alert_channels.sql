-- 更新告警配置渠道示例：邮件 + Telegram
-- 请替换为您真实的 Bot Token 和 Chat ID

UPDATE alert_configs
SET channels = '[
  {
    "type": "email",
    "url": "ops@example.com"
  },
  {
    "type": "telegram",
    "url": "<BOT_TOKEN>",
    "receive_id": "<CHAT_ID>"
  }
]'
WHERE name = 'CPU_HIGH_ALERT';
