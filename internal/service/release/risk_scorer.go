// Package release
//
// risk_scorer.go: Release 变更风险评分器（v2.0）。
//
// 设计原则（参考 ADR-0002 / Google SRE 变更风险模型）：
//
//   1. 规则引擎 + 加权求和：每条规则独立判定，命中累加权重，不互斥
//   2. 0~100 评分；阈值映射到 risk_level：
//        score <= 20  -> low
//        score <= 50  -> medium
//        score <= 80  -> high
//        score >  80  -> critical
//   3. 规则可枚举、可解释 —— 命中明细写入 risk_factors 字段，供 UI 展示与审计
//   4. 评分纯计算，不写库；调用方决定何时持久化
//
// 当前内置 7 条规则；未来可扩展为 DSL 配置（数据库化），先把骨架与契约定下来。
package release

import (
	"context"
	"fmt"
	"strings"
	"time"

	"devops/internal/models/deploy"
)

// RiskRule 单条风险规则。
type RiskRule struct {
	Key         string                                // 规则唯一标识（用于 risk_factors 命中明细 + 国际化）
	Name        string                                // 规则中文名（兜底）
	Weight      int                                   // 命中权重（建议 5~30）
	Description string                                // 规则说明
	Match       func(input *RiskInput) (bool, string) // 命中函数；返回 (是否命中, 命中详情)
}

// RiskInput 评分输入。
type RiskInput struct {
	Release *deploy.Release
	Items   []deploy.ReleaseItem
	Now     time.Time
}

// RiskScore 评分输出。
type RiskScore struct {
	Score   int            `json:"score"`   // 0~100
	Level   string         `json:"level"`   // low/medium/high/critical
	Hits    []RiskHit      `json:"hits"`    // 命中规则明细
	Factors map[string]any `json:"factors"` // 序列化到 release.risk_factors 的内容
}

// RiskHit 命中明细。
type RiskHit struct {
	Key     string `json:"key"`
	Name    string `json:"name"`
	Weight  int    `json:"weight"`
	Detail  string `json:"detail,omitempty"`
}

// RiskScorer 风险评分器。
type RiskScorer struct {
	rules   []RiskRule
}

// NewRiskScorer 构造评分器，使用默认规则集。
func NewRiskScorer() *RiskScorer {
	return &RiskScorer{
		rules:   DefaultRiskRules(),
	}
}

// Score 执行评分。
func (s *RiskScorer) Score(ctx context.Context, input *RiskInput) *RiskScore {
	_ = ctx
	if input == nil || input.Release == nil {
		return &RiskScore{Level: levelLow, Factors: map[string]any{}}
	}
	if input.Now.IsZero() {
		input.Now = time.Now()
	}

	hits := make([]RiskHit, 0, len(s.rules))
	total := 0
	for _, rule := range s.rules {
		matched, detail := rule.Match(input)
		if !matched {
			continue
		}
		hits = append(hits, RiskHit{
			Key:    rule.Key,
			Name:   rule.Name,
			Weight: rule.Weight,
			Detail: detail,
		})
		total += rule.Weight
	}
	if total > 100 {
		total = 100
	}

	score := &RiskScore{
		Score:   total,
		Level:   classifyLevel(total),
		Hits:    hits,
		Factors: buildFactors(total, hits),
	}
	return score
}

// ScoreAndApply 评分并把结果写入 release（不入库；调用方自行 Update）。
func (s *RiskScorer) ScoreAndApply(ctx context.Context, rel *deploy.Release, items []deploy.ReleaseItem) *RiskScore {
	res := s.Score(ctx, &RiskInput{Release: rel, Items: items})
	if rel == nil {
		return res
	}
	rel.RiskScore = res.Score
	rel.RiskLevel = res.Level
	rel.RiskFactors = res.Factors
	return res
}

// ---------- 默认规则集 ----------

// DefaultRiskRules 内置规则集。可被替换/扩展。
func DefaultRiskRules() []RiskRule {
	return []RiskRule{
		{
			Key: "env.production", Name: "生产环境发布", Weight: 25,
			Description: "目标环境为 prod 时风险显著上升",
			Match: func(in *RiskInput) (bool, string) {
				env := strings.ToLower(strings.TrimSpace(in.Release.Env))
				if env == "prod" || env == "production" {
					return true, "env=" + env
				}
				return false, ""
			},
		},
		{
			Key: "strategy.direct_in_prod", Name: "生产直发（无渐进发布）", Weight: 20,
			Description: "生产环境且 RolloutStrategy=direct，缺少灰度兜底",
			Match: func(in *RiskInput) (bool, string) {
				env := strings.ToLower(strings.TrimSpace(in.Release.Env))
				strategy := strings.TrimSpace(in.Release.RolloutStrategy)
				if (env == "prod" || env == "production") && (strategy == "" || strategy == deploy.RolloutStrategyDirect) {
					return true, "strategy=direct"
				}
				return false, ""
			},
		},
		{
			Key: "items.database_change", Name: "包含数据库变更", Weight: 18,
			Description: "数据库变更（DDL/DML）属高风险变更类型",
			Match: func(in *RiskInput) (bool, string) {
				count := 0
				for _, item := range in.Items {
					if item.ItemType == deploy.ReleaseItemTypeDatabase || item.ItemType == deploy.ReleaseItemTypeSQLTicket {
						count++
					}
				}
				if count > 0 {
					return true, fmt.Sprintf("database_items=%d", count)
				}
				return false, ""
			},
		},
		{
			Key: "items.large_batch", Name: "变更项过多", Weight: 12,
			Description: "单次发布关联子项数量过多（>5），故障爆炸半径大",
			Match: func(in *RiskInput) (bool, string) {
				if len(in.Items) > 5 {
					return true, fmt.Sprintf("items=%d", len(in.Items))
				}
				return false, ""
			},
		},
		{
			Key: "items.config_with_deployment", Name: "镜像 + 配置同时变更", Weight: 10,
			Description: "镜像与 Nacos 配置同时变更，故障定位更复杂",
			Match: func(in *RiskInput) (bool, string) {
				hasDeploy, hasConfig := false, false
				for _, item := range in.Items {
					switch item.ItemType {
					case deploy.ReleaseItemTypeDeployment, deploy.ReleaseItemTypePipelineRun:
						hasDeploy = true
					case deploy.ReleaseItemTypeNacosRelease:
						hasConfig = true
					}
				}
				if hasDeploy && hasConfig {
					return true, "deployment+nacos"
				}
				return false, ""
			},
		},
		{
			Key: "time.off_hours", Name: "非工作时间发布", Weight: 8,
			Description: "工作日 21:00 ~ 次日 08:00、周末全天，On-call 响应能力下降",
			Match: func(in *RiskInput) (bool, string) {
				now := in.Now
				weekday := now.Weekday()
				hour := now.Hour()
				if weekday == time.Saturday || weekday == time.Sunday {
					return true, "weekend"
				}
				if hour >= 21 || hour < 8 {
					return true, fmt.Sprintf("hour=%02d", hour)
				}
				return false, ""
			},
		},
		{
			Key: "manual.high_risk", Name: "手工标记高风险", Weight: 15,
			Description: "Release.RiskLevel 已被发起人手工标记为 high/critical",
			Match: func(in *RiskInput) (bool, string) {
				lvl := strings.ToLower(strings.TrimSpace(in.Release.RiskLevel))
				if lvl == levelHigh || lvl == levelCritical {
					return true, "manual_level=" + lvl
				}
				return false, ""
			},
		},
	}
}

// classifyLevel 把分数映射到风险等级。
func classifyLevel(score int) string {
	switch {
	case score <= 20:
		return levelLow
	case score <= 50:
		return levelMedium
	case score <= 80:
		return levelHigh
	default:
		return levelCritical
	}
}

// buildFactors 把命中明细转换为 JSON 友好的 map 结构。
func buildFactors(score int, hits []RiskHit) map[string]any {
	hitList := make([]map[string]any, 0, len(hits))
	for _, h := range hits {
		hitList = append(hitList, map[string]any{
			"key":    h.Key,
			"name":   h.Name,
			"weight": h.Weight,
			"detail": h.Detail,
		})
	}
	return map[string]any{
		"score":      score,
		"level":      classifyLevel(score),
		"hits":       hitList,
		"calculated": time.Now().Format(time.RFC3339),
		"version":    "v1",
	}
}

const (
	levelLow      = "low"
	levelMedium   = "medium"
	levelHigh     = "high"
	levelCritical = "critical"
)
