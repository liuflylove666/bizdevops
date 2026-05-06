// Package incident
//
// postmortem.go: 事故复盘导出（v2.2）。
//
// 输出 Markdown 文本，结构遵循常见 SRE 复盘模板：
//
//   # 事故复盘 - <title>
//   - 元信息（ID、严重、环境、应用、时间线、MTTR）
//   ## 影响
//   ## 根因
//   ## 时间线
//   ## 后续行动（占位）
//
// 调用方：HTTP 层 `GET /incidents/:id/postmortem?format=md` 直接落盘或归档。
package incident

import (
	"bytes"
	"fmt"
	"time"

	"devops/internal/models/monitoring"
)

// ExportPostmortemMarkdown 根据 Incident 生成可下载的 Markdown 复盘文档。
//
// 选择 Markdown 而非 HTML，是因为：
//   - 内容主体（时间线、根因、影响）都是纯文本，Markdown 更轻量；
//   - 可直接贴入文档或 Github Issue，零转换；
//   - 便于 CI 流水线归档（存入 git 仓库做历史回溯）。
func ExportPostmortemMarkdown(inc *monitoring.Incident) string {
	if inc == nil {
		return ""
	}
	var buf bytes.Buffer

	fmt.Fprintf(&buf, "# 事故复盘 - %s\n\n", safe(inc.Title))
	fmt.Fprintf(&buf, "> 事故 ID：`INC-%d`  |  严重等级：**%s**  |  状态：**%s**\n\n",
		inc.ID, inc.Severity, inc.Status)

	// 元信息
	buf.WriteString("## 元信息\n\n")
	buf.WriteString("| 字段 | 值 |\n| --- | --- |\n")
	fmt.Fprintf(&buf, "| 环境 | %s |\n", or(inc.Env, "-"))
	fmt.Fprintf(&buf, "| 应用 | %s |\n", or(inc.ApplicationName, "-"))
	fmt.Fprintf(&buf, "| 来源 | %s |\n", or(inc.Source, "manual"))
	fmt.Fprintf(&buf, "| 发现时间 | %s |\n", fmtTime(&inc.DetectedAt))
	fmt.Fprintf(&buf, "| 止血时间 | %s |\n", fmtTime(inc.MitigatedAt))
	fmt.Fprintf(&buf, "| 解决时间 | %s |\n", fmtTime(inc.ResolvedAt))
	fmt.Fprintf(&buf, "| MTTR | %s |\n", fmtMTTR(inc))
	if inc.ReleaseID != nil {
		fmt.Fprintf(&buf, "| 关联发布 | #%d |\n", *inc.ReleaseID)
	}
	if inc.AlertFingerprint != "" {
		fmt.Fprintf(&buf, "| 告警指纹 | `%s` |\n", inc.AlertFingerprint)
	}
	fmt.Fprintf(&buf, "| 发现人 | %s |\n", or(inc.CreatedByName, "system"))
	fmt.Fprintf(&buf, "| 处理人 | %s |\n", or(inc.ResolvedByName, "-"))
	if inc.PostmortemURL != "" {
		fmt.Fprintf(&buf, "| 原文档 | [链接](%s) |\n", inc.PostmortemURL)
	}
	buf.WriteString("\n")

	// 影响描述
	buf.WriteString("## 影响\n\n")
	if inc.Description != "" {
		buf.WriteString(inc.Description)
	} else {
		buf.WriteString("> （待补充：影响范围、受影响用户数、业务指标波动等）")
	}
	buf.WriteString("\n\n")

	// 根因
	buf.WriteString("## 根因\n\n")
	if inc.RootCause != "" {
		buf.WriteString(inc.RootCause)
	} else {
		buf.WriteString("> （待补充：触发链路、直接原因、根本原因）")
	}
	buf.WriteString("\n\n")

	// 时间线
	buf.WriteString("## 时间线\n\n")
	fmt.Fprintf(&buf, "- `%s` **发现**：%s\n", fmtTime(&inc.DetectedAt), safe(inc.Title))
	if inc.MitigatedAt != nil {
		fmt.Fprintf(&buf, "- `%s` **止血**：影响面已阻断（MTTR 未封闭）\n", fmtTime(inc.MitigatedAt))
	}
	if inc.ResolvedAt != nil {
		fmt.Fprintf(&buf, "- `%s` **解决**：%s\n", fmtTime(inc.ResolvedAt), or(inc.ResolvedByName, "-"))
	}
	buf.WriteString("\n")

	// 后续行动
	buf.WriteString("## 后续行动 (Action Items)\n\n")
	buf.WriteString("- [ ] （负责人 / 截止时间 / 预期效果）\n")
	buf.WriteString("- [ ] （回归验证：监控 / 告警规则补齐）\n")
	buf.WriteString("- [ ] （流程改进：发布/值班/应急预案）\n\n")

	// Footer：导出时间便于区分文档版本
	fmt.Fprintf(&buf, "---\n*本文档由 DevOps 平台自动生成于 %s*\n",
		time.Now().Format("2006-01-02 15:04:05"))
	return buf.String()
}

// ---- helpers ----

func fmtTime(t *time.Time) string {
	if t == nil || t.IsZero() {
		return "-"
	}
	return t.Format("2006-01-02 15:04:05")
}

func fmtMTTR(inc *monitoring.Incident) string {
	m := int(inc.MTTRMinutes())
	if inc.ResolvedAt == nil || m <= 0 {
		return "-"
	}
	if m < 60 {
		return fmt.Sprintf("%d 分钟", m)
	}
	return fmt.Sprintf("%d 小时 %d 分钟", m/60, m%60)
}

func or(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}

func safe(s string) string {
	if s == "" {
		return "(未命名)"
	}
	return s
}
