package metrics

import (
	"math"
	"testing"
	"time"

	"devops/internal/models/deploy"
	"devops/internal/models/monitoring"
)

func TestComputeMetrics_Empty(t *testing.T) {
	q := DORAQuery{From: time.Now().Add(-24 * time.Hour), To: time.Now(), Env: "prod"}
	freq, lead, fail, mttr, sample := computeMetrics(nil, q)
	if freq != 0 || lead != 0 || fail != 0 || mttr != 0 || sample != 0 {
		t.Fatalf("空输入应全 0，got freq=%v lead=%v fail=%v mttr=%v sample=%v", freq, lead, fail, mttr, sample)
	}
}

func TestComputeMetrics_BasicFlow(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	q := DORAQuery{From: now.Add(-24 * 7 * time.Hour), To: now, Env: "prod"}
	pubAt1 := now.Add(-2 * time.Hour)
	pubAt2 := now.Add(-1 * time.Hour)
	rels := []deploy.Release{
		{Status: deploy.ReleaseStatusPublished, CreatedAt: now.Add(-5 * time.Hour), PublishedAt: &pubAt1},
		{Status: deploy.ReleaseStatusPublished, CreatedAt: now.Add(-3 * time.Hour), PublishedAt: &pubAt2},
		{Status: deploy.ReleaseStatusFailed, CreatedAt: now.Add(-6 * time.Hour), UpdatedAt: now.Add(-4 * time.Hour)},
		{Status: deploy.ReleaseStatusRolledBack, CreatedAt: now.Add(-8 * time.Hour), UpdatedAt: now.Add(-7 * time.Hour)},
	}
	freq, lead, fail, _, sample := computeMetrics(rels, q)
	if sample != 4 {
		t.Fatalf("sample 应为 4，got %d", sample)
	}
	expectedFreq := roundTo(2.0/7, 2)
	if freq != expectedFreq {
		t.Errorf("freq 应为 %v，got %v", expectedFreq, freq)
	}
	if lead != 3 { // 中位数为 (3+2)/2=2.5? lead=PublishedAt-CreatedAt：2-(-5)=3，1-(-3)=2 → median=2.5
		// 实际：median([3, 2]) = 2.5
		if lead != 2.5 {
			t.Errorf("lead 应为 2.5，got %v", lead)
		}
	}
	// fail rate = (1+1)/4 = 50%
	if fail != 50 {
		t.Errorf("fail rate 应为 50，got %v", fail)
	}
}

func TestComputeTrend(t *testing.T) {
	cases := []struct {
		cur, prev      float64
		expectedTrend  string
		expectedDelta  float64
		expectedTextOK func(s string) bool
	}{
		{0, 0, "flat", 0, func(s string) bool { return s == "持平" }},
		{10, 0, "up", 100, func(s string) bool { return s == "+100%" }},
		{20, 10, "up", 100, func(s string) bool { return s == "+100%" }},
		{5, 10, "down", -50, func(s string) bool { return s == "-50%" }},
		{10.05, 10, "flat", 0.5, func(s string) bool { return s == "持平" }},
	}
	for _, c := range cases {
		trend, delta, text := computeTrend(c.cur, c.prev)
		if trend != c.expectedTrend {
			t.Errorf("trend(%v,%v): got %s, want %s", c.cur, c.prev, trend, c.expectedTrend)
		}
		if math.Abs(delta-c.expectedDelta) > 0.1 {
			t.Errorf("delta(%v,%v): got %v, want %v", c.cur, c.prev, delta, c.expectedDelta)
		}
		if !c.expectedTextOK(text) {
			t.Errorf("text(%v,%v): got %s", c.cur, c.prev, text)
		}
	}
}

func TestBenchmarks(t *testing.T) {
	if deployFreqBenchmark(2) != "elite" {
		t.Error("deploy_freq 2/day 应为 elite")
	}
	if deployFreqBenchmark(0.2) != "high" {
		t.Error("deploy_freq 0.2/day (≥1/week) 应为 high")
	}
	if leadTimeBenchmark(0.5) != "elite" {
		t.Error("lead_time 0.5h 应为 elite")
	}
	if changeFailRateBenchmark(3) != "elite" {
		t.Error("fail_rate 3% 应为 elite")
	}
	if changeFailRateBenchmark(20) != "low" {
		t.Error("fail_rate 20% 应为 low")
	}
	if mttrBenchmark(30) != "elite" {
		t.Error("mttr 30min 应为 elite")
	}
}

func TestQueryNormalize(t *testing.T) {
	q := DORAQuery{}
	q.Normalize()
	if q.Env != "prod" {
		t.Errorf("env default 应为 prod，got %s", q.Env)
	}
	if q.To.IsZero() || q.From.IsZero() {
		t.Error("from/to 应被填充")
	}
	if q.Window() > 8*24*time.Hour {
		t.Error("默认窗口应为 7 天")
	}

	// 上限保护
	q2 := DORAQuery{From: time.Now().Add(-200 * 24 * time.Hour), To: time.Now()}
	q2.Normalize()
	if q2.Window() > 91*24*time.Hour {
		t.Errorf("超长窗口应被裁剪到 90 天，got %v", q2.Window())
	}
}

func TestJudgeAppVsFleet(t *testing.T) {
	// deploy_freq: 越大越好
	if j, _ := judgeAppVsFleet(2, 1, true); j != "better" {
		t.Error("应用 2 > fleet 1 (upIsGood) 应为 better")
	}
	if j, _ := judgeAppVsFleet(0.5, 1, true); j != "worse" {
		t.Error("应用 0.5 < fleet 1 (upIsGood) 应为 worse")
	}
	// mttr: 越小越好
	if j, _ := judgeAppVsFleet(30, 60, false); j != "better" {
		t.Error("app mttr 30 < fleet 60 应为 better")
	}
	if j, _ := judgeAppVsFleet(120, 60, false); j != "worse" {
		t.Error("app mttr 120 > fleet 60 应为 worse")
	}
	// 5% 内视为持平
	if j, _ := judgeAppVsFleet(1.02, 1, true); j != "equal" {
		t.Error("差异 2% 应视为 equal")
	}
	// 双 0
	if j, _ := judgeAppVsFleet(0, 0, true); j != "equal" {
		t.Error("双 0 应为 equal")
	}
}

func TestEnumerateDays(t *testing.T) {
	from := time.Date(2026, 4, 18, 10, 0, 0, 0, time.UTC)
	to := time.Date(2026, 4, 21, 9, 0, 0, 0, time.UTC)
	days := enumerateDays(from, to)
	if len(days) != 4 {
		t.Fatalf("期望 4 天，got %d: %v", len(days), days)
	}
	if days[0] != "2026-04-18" || days[3] != "2026-04-21" {
		t.Errorf("首尾日期不正确: %v", days)
	}
}

func TestBuildDailySeries_Shape(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	q := DORAQuery{From: now.Add(-3 * 24 * time.Hour), To: now, Env: "prod"}
	pub := now.Add(-12 * time.Hour)
	rels := []deploy.Release{
		{Status: deploy.ReleaseStatusPublished, CreatedAt: now.Add(-14 * time.Hour), PublishedAt: &pub},
	}
	series := buildDailySeries(rels, nil, q)
	for _, key := range []string{"deploy_freq", "lead_time", "change_fail_rate", "mttr"} {
		pts, ok := series[key]
		if !ok || len(pts) != 4 {
			t.Errorf("%s 期望 4 个点（含首尾），got %v", key, len(pts))
		}
	}
	// 最后一天应有 1 次发布
	if series["deploy_freq"][3].Value != 1 {
		t.Errorf("deploy_freq 末日应=1，got %v", series["deploy_freq"][3].Value)
	}
}

func TestComputeMetricsWithIncidents_PrefersIncidentData(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	q := DORAQuery{From: now.Add(-7 * 24 * time.Hour), To: now, Env: "prod"}

	// 1 次 published，给 release 近似算法没有 MTTR 数据
	pubAt := now.Add(-1 * time.Hour)
	rels := []deploy.Release{
		{Status: deploy.ReleaseStatusPublished, CreatedAt: now.Add(-3 * time.Hour), PublishedAt: &pubAt},
	}

	// 2 个 incident，分别 MTTR=30min 和 90min，中位 60min
	resolved1 := now.Add(-2*time.Hour + 30*time.Minute)
	resolved2 := now.Add(-1*time.Hour + 30*time.Minute)
	incidents := []monitoring.Incident{
		{DetectedAt: now.Add(-3 * time.Hour), ResolvedAt: &resolved1},
		{DetectedAt: now.Add(-3 * time.Hour), ResolvedAt: &resolved2},
	}

	_, _, _, mttr, _ := computeMetricsWithIncidents(rels, incidents, q)
	if mttr != 60 {
		t.Errorf("incident 真实 MTTR 应为 60min（中位数），got %v", mttr)
	}
}

func TestComputeMetricsWithIncidents_FallbackWhenNoIncidents(t *testing.T) {
	now := time.Date(2026, 4, 21, 12, 0, 0, 0, time.UTC)
	q := DORAQuery{From: now.Add(-7 * 24 * time.Hour), To: now, Env: "prod"}
	pubAt := now.Add(-1 * time.Hour)
	rels := []deploy.Release{
		{Status: deploy.ReleaseStatusFailed, CreatedAt: now.Add(-4 * time.Hour), UpdatedAt: now.Add(-3 * time.Hour)},
		{Status: deploy.ReleaseStatusPublished, CreatedAt: now.Add(-2 * time.Hour), PublishedAt: &pubAt},
	}
	_, _, _, mttr, _ := computeMetricsWithIncidents(rels, nil, q)
	// failed at -3h, published at -1h → 120min
	if mttr != 120 {
		t.Errorf("回退 MTTR 应为 120min，got %v", mttr)
	}
}

func TestComputeMTTR(t *testing.T) {
	base := time.Date(2026, 4, 21, 0, 0, 0, 0, time.UTC)
	failures := []time.Time{
		base.Add(1 * time.Hour),
		base.Add(5 * time.Hour),
	}
	pubs := []time.Time{
		base.Add(2 * time.Hour), // failure1 → 1h 恢复 = 60min
		base.Add(6 * time.Hour), // failure2 → 1h 恢复 = 60min
	}
	mttr := computeMTTR(failures, pubs)
	if mttr != 60 {
		t.Errorf("mttr 应为 60min，got %v", mttr)
	}
}
