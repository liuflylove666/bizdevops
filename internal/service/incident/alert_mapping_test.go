package incident

import (
	"testing"

	"devops/internal/models/monitoring"
)

func TestMapSeverity(t *testing.T) {
	cases := []struct {
		name   string
		level  string
		labels map[string]string
		want   string
	}{
		{"label 优先 P0", "warning", map[string]string{"severity": "P0"}, monitoring.IncidentSeverityP0},
		{"label 小写也兼容", "warning", map[string]string{"severity": "p1"}, monitoring.IncidentSeverityP1},
		{"无 label level=critical → P0", "critical", nil, monitoring.IncidentSeverityP0},
		{"无 label level=error → P1", "error", nil, monitoring.IncidentSeverityP1},
		{"无 label level=warning → P2", "warning", nil, monitoring.IncidentSeverityP2},
		{"无 label level=info → P3", "info", nil, monitoring.IncidentSeverityP3},
		{"无 label level 为空 → P3", "", nil, monitoring.IncidentSeverityP3},
		{"label 无效 fallback level", "critical", map[string]string{"severity": "XXXX"}, monitoring.IncidentSeverityP0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := mapSeverity(c.level, c.labels); got != c.want {
				t.Errorf("got %s, want %s", got, c.want)
			}
		})
	}
}

func TestExtractEnvFromLabels(t *testing.T) {
	if got := extractEnvFromLabels(nil); got != "prod" {
		t.Errorf("nil labels 应回退 prod, got %s", got)
	}
	if got := extractEnvFromLabels(map[string]string{"env": "staging"}); got != "staging" {
		t.Errorf("应优先 env, got %s", got)
	}
	if got := extractEnvFromLabels(map[string]string{"environment": "test"}); got != "test" {
		t.Errorf("兼容 environment, got %s", got)
	}
	if got := extractEnvFromLabels(map[string]string{"stage": "dev"}); got != "dev" {
		t.Errorf("兼容 stage, got %s", got)
	}
}

func TestExtractAppFromLabels(t *testing.T) {
	if got := extractAppFromLabels(nil); got != "" {
		t.Errorf("nil 应为空, got %s", got)
	}
	labels := map[string]string{"app": "checkout", "service": "payment"}
	// 按 application/app/service/service_name/job 优先级，app 先命中
	if got := extractAppFromLabels(labels); got != "checkout" {
		t.Errorf("应优先 app, got %s", got)
	}
	if got := extractAppFromLabels(map[string]string{"job": "scheduler"}); got != "scheduler" {
		t.Errorf("fallback job, got %s", got)
	}
}

func TestSeverityRank_BumpOnlyUp(t *testing.T) {
	if severityRank(monitoring.IncidentSeverityP0) <= severityRank(monitoring.IncidentSeverityP1) {
		t.Error("P0 必须 > P1")
	}
	if severityRank(monitoring.IncidentSeverityP3) != 1 {
		t.Error("P3 应为 1")
	}
	if severityRank("UNKNOWN") != 0 {
		t.Error("未知等级应为 0，便于新 sev 一律能覆盖")
	}
}
