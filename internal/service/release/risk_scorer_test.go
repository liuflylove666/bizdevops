package release

import (
	"context"
	"testing"
	"time"

	"devops/internal/models/deploy"
)

func TestRiskScorer_LowEnv_DevOnly(t *testing.T) {
	scorer := NewRiskScorer()
	res := scorer.Score(context.Background(), &RiskInput{
		Release: &deploy.Release{Env: "dev", RolloutStrategy: deploy.RolloutStrategyDirect},
		Now:     time.Date(2026, 4, 21, 14, 0, 0, 0, time.UTC), // 工作日工作时间
	})
	if res.Score != 0 {
		t.Fatalf("dev 环境工作时间无变更项应得 0 分，got %d", res.Score)
	}
	if res.Level != "low" {
		t.Fatalf("low 风险等级，got %s", res.Level)
	}
}

func TestRiskScorer_ProdDirectWithDB_HighScore(t *testing.T) {
	scorer := NewRiskScorer()
	rel := &deploy.Release{
		Env:             "prod",
		RolloutStrategy: deploy.RolloutStrategyDirect,
	}
	items := []deploy.ReleaseItem{
		{ItemType: deploy.ReleaseItemTypeDatabase},
		{ItemType: deploy.ReleaseItemTypeDeployment},
		{ItemType: deploy.ReleaseItemTypeNacosRelease},
	}
	res := scorer.Score(context.Background(), &RiskInput{
		Release: rel, Items: items,
		Now: time.Date(2026, 4, 21, 14, 0, 0, 0, time.UTC),
	})
	// env.production(25) + strategy.direct_in_prod(20) + items.database_change(18) + items.config_with_deployment(10) = 73
	if res.Score < 70 || res.Score > 80 {
		t.Fatalf("期望分数 70~80，got %d", res.Score)
	}
	if res.Level != "high" {
		t.Fatalf("应为 high 风险，got %s", res.Level)
	}
	if len(res.Hits) < 4 {
		t.Fatalf("至少命中 4 条规则，got %d", len(res.Hits))
	}
}

func TestRiskScorer_ScoreCappedAt100(t *testing.T) {
	scorer := NewRiskScorer()
	rel := &deploy.Release{
		Env:             "prod",
		RolloutStrategy: deploy.RolloutStrategyDirect,
		RiskLevel:       "critical",
	}
	items := make([]deploy.ReleaseItem, 0, 8)
	for i := 0; i < 8; i++ {
		items = append(items, deploy.ReleaseItem{ItemType: deploy.ReleaseItemTypeDatabase})
	}
	items = append(items, deploy.ReleaseItem{ItemType: deploy.ReleaseItemTypeNacosRelease})
	items = append(items, deploy.ReleaseItem{ItemType: deploy.ReleaseItemTypeDeployment})
	res := scorer.Score(context.Background(), &RiskInput{
		Release: rel, Items: items,
		Now: time.Date(2026, 4, 25, 23, 0, 0, 0, time.UTC), // 周六晚上
	})
	if res.Score != 100 {
		t.Fatalf("总分应封顶到 100，got %d", res.Score)
	}
	if res.Level != "critical" {
		t.Fatalf("应为 critical，got %s", res.Level)
	}
}

func TestRiskScorer_OffHours_Weekend(t *testing.T) {
	scorer := NewRiskScorer()
	res := scorer.Score(context.Background(), &RiskInput{
		Release: &deploy.Release{Env: "dev"},
		Now:     time.Date(2026, 4, 26, 14, 0, 0, 0, time.UTC), // 周日下午
	})
	hit := false
	for _, h := range res.Hits {
		if h.Key == "time.off_hours" {
			hit = true
			break
		}
	}
	if !hit {
		t.Fatal("周末应命中 time.off_hours 规则")
	}
}

func TestRiskScorer_ScoreAndApply_WritesToRelease(t *testing.T) {
	scorer := NewRiskScorer()
	rel := &deploy.Release{Env: "prod", RolloutStrategy: deploy.RolloutStrategyCanary}
	items := []deploy.ReleaseItem{{ItemType: deploy.ReleaseItemTypeDeployment}}
	res := scorer.ScoreAndApply(context.Background(), rel, items)
	if rel.RiskScore != res.Score {
		t.Fatalf("rel.RiskScore 应被回写，got %d 期望 %d", rel.RiskScore, res.Score)
	}
	if rel.RiskLevel != res.Level {
		t.Fatalf("rel.RiskLevel 应被回写，got %s 期望 %s", rel.RiskLevel, res.Level)
	}
	if rel.RiskFactors == nil {
		t.Fatal("rel.RiskFactors 应被设置")
	}
}

func TestClassifyLevel_Boundaries(t *testing.T) {
	cases := []struct {
		score    int
		expected string
	}{
		{0, "low"}, {20, "low"},
		{21, "medium"}, {50, "medium"},
		{51, "high"}, {80, "high"},
		{81, "critical"}, {100, "critical"},
	}
	for _, c := range cases {
		if got := classifyLevel(c.score); got != c.expected {
			t.Errorf("classifyLevel(%d)=%s, expected %s", c.score, got, c.expected)
		}
	}
}
