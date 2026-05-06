package incident

import (
	"strings"
	"testing"
	"time"

	"devops/internal/models/monitoring"
)

func TestExportPostmortemMarkdown_FullFields(t *testing.T) {
	detected := time.Date(2026, 4, 21, 10, 0, 0, 0, time.UTC)
	mitigated := detected.Add(15 * time.Minute)
	resolved := detected.Add(90 * time.Minute)
	rid := uint(42)
	inc := &monitoring.Incident{
		ID:               7,
		Title:            "支付回调超时",
		Description:      "下单转化率下降 8%",
		ApplicationName:  "checkout",
		Env:              "prod",
		Severity:         monitoring.IncidentSeverityP1,
		Status:           monitoring.IncidentStatusResolved,
		DetectedAt:       detected,
		MitigatedAt:      &mitigated,
		ResolvedAt:       &resolved,
		Source:           monitoring.IncidentSourceAlert,
		ReleaseID:        &rid,
		AlertFingerprint: "abc123",
		PostmortemURL:    "https://wiki/incident-7",
		RootCause:        "上游 gateway 超时阈值从 3s 改为 1s",
		CreatedByName:    "alice",
		ResolvedByName:   "bob",
	}
	md := ExportPostmortemMarkdown(inc)

	mustContain(t, md, "# 事故复盘 - 支付回调超时")
	mustContain(t, md, "INC-7")
	mustContain(t, md, "**P1**")
	mustContain(t, md, "**resolved**")
	mustContain(t, md, "| 应用 | checkout |")
	mustContain(t, md, "| 关联发布 | #42 |")
	mustContain(t, md, "| 告警指纹 | `abc123` |")
	mustContain(t, md, "1 小时 30 分钟") // MTTR
	mustContain(t, md, "上游 gateway 超时阈值")
	mustContain(t, md, "下单转化率下降")
	mustContain(t, md, "## 后续行动")
	// 未设置的字段不该误展示为 resolved
	if strings.Contains(md, "| 解决时间 | - |") {
		t.Error("已 resolved 的事故不应显示 '-'")
	}
}

func TestExportPostmortemMarkdown_OpenFallback(t *testing.T) {
	inc := &monitoring.Incident{
		ID:         1,
		Title:      "",
		Severity:   monitoring.IncidentSeverityP3,
		Status:     monitoring.IncidentStatusOpen,
		DetectedAt: time.Now(),
	}
	md := ExportPostmortemMarkdown(inc)
	mustContain(t, md, "(未命名)")
	mustContain(t, md, "待补充：影响范围")
	mustContain(t, md, "待补充：触发链路")
	mustContain(t, md, "| MTTR | - |")
}

func TestExportPostmortemMarkdown_Nil(t *testing.T) {
	if got := ExportPostmortemMarkdown(nil); got != "" {
		t.Errorf("nil 应返回空串, got %q", got)
	}
}

func mustContain(t *testing.T, body, sub string) {
	t.Helper()
	if !strings.Contains(body, sub) {
		t.Errorf("输出缺少 %q\n---\n%s", sub, body)
	}
}
