// Package metrics
//
// dora_aggregator.go: 聚合 DORA 四指标（v2.0 / Sprint 4）。
//
// 数据源策略：
//   1. Deployment Frequency: 统计 releases.status='published' 的发布数量
//   2. Lead Time for Changes: 统计 published_at - created_at 的中位数
//   3. Change Failure Rate: (failed + rolled_back) / total_terminated
//   4. MTTR: 暂用「published 之前最近一次 failed 的时间差」近似；
//      未来接入 incident 表后改为真实故障恢复时长
//
// 趋势计算：本周期 vs 上一周期 同口径对比，绝对差/相对差均给出。
//
// 性能：默认范围 7 天；超过 90 天加 LIMIT 防爆。
package metrics

import (
	"context"
	"errors"
	"math"
	"sort"
	"time"

	"gorm.io/gorm"

	"devops/internal/models/deploy"
	"devops/internal/models/monitoring"
	monRepo "devops/internal/modules/monitoring/repository"
)

// DORAAggregator DORA 指标聚合器。
type DORAAggregator struct {
	db           *gorm.DB
	incidentRepo *monRepo.IncidentRepository
}

// NewDORAAggregator 构造聚合器。
func NewDORAAggregator(db *gorm.DB) *DORAAggregator {
	return &DORAAggregator{
		db:           db,
		incidentRepo: monRepo.NewIncidentRepository(db),
	}
}

// DORAQuery 查询参数。
type DORAQuery struct {
	From            time.Time
	To              time.Time
	Env             string // 可选：限制环境（默认 prod）
	ApplicationID   uint   // 可选：按 application_id 下钻（0 表示不限）
	ApplicationName string // 可选：按 application_name 下钻（与 ID 二选一即可）
}

// Normalize 把 0 值填充为默认范围（最近 7 天 / env=prod）。
func (q *DORAQuery) Normalize() {
	if q.To.IsZero() {
		q.To = time.Now()
	}
	if q.From.IsZero() {
		q.From = q.To.Add(-7 * 24 * time.Hour)
	}
	if q.Env == "" {
		q.Env = "prod"
	}
	// 上限保护：防止误传超长区间
	maxRange := 90 * 24 * time.Hour
	if q.To.Sub(q.From) > maxRange {
		q.From = q.To.Add(-maxRange)
	}
}

// Window 当前周期长度。
func (q DORAQuery) Window() time.Duration { return q.To.Sub(q.From) }

// PrevQuery 同口径上一周期（保留应用下钻过滤）。
func (q DORAQuery) PrevQuery() DORAQuery {
	w := q.Window()
	return DORAQuery{
		From:            q.From.Add(-w),
		To:              q.From,
		Env:             q.Env,
		ApplicationID:   q.ApplicationID,
		ApplicationName: q.ApplicationName,
	}
}

// SeriesPoint 按天时序点（用于前端 Sparkline）。
type SeriesPoint struct {
	Date  string  `json:"date"`  // YYYY-MM-DD（UTC）
	Value float64 `json:"value"` // 当日指标值（无样本则 0）
}

// MetricValue 单个指标。
type MetricValue struct {
	Key         string        `json:"key"`         // deploy_freq / lead_time / change_fail_rate / mttr
	Title       string        `json:"title"`       // 中文名（前端可不依赖）
	Value       float64       `json:"value"`       // 数值
	Unit        string        `json:"unit"`        // 单位
	Trend       string        `json:"trend"`       // up / down / flat
	Delta       float64       `json:"delta"`       // 与上一周期相对差（百分比，已乘 100）
	DeltaText   string        `json:"delta_text"`  // 易读文本：+12.3% / -5.1% / 持平
	Benchmark   string        `json:"benchmark"`   // elite / high / medium / low
	Description string        `json:"description"` // 说明
	Sample      int           `json:"sample"`      // 当前周期样本数（用于前端判断"数据不足"）
	Series      []SeriesPoint `json:"series"`      // 当前周期按天时序（Sparkline）
	PrevSeries  []SeriesPoint `json:"prev_series"` // 上一周期按天时序（对齐后用于前端叠加对比，v2.1）

	// v2.2: 应用下钻时的"应用 vs 全站"对标；未开启应用下钻时为空。
	FleetValue     float64 `json:"fleet_value"`     // 全站同期该指标值
	FleetBenchmark string  `json:"fleet_benchmark"` // 全站同期 benchmark 档位
	AppVsFleet     string  `json:"app_vs_fleet"`    // better / worse / equal（按 upIsGood 语义）
	AppVsFleetText string  `json:"app_vs_fleet_text"`
}

// DORASnapshot 一次聚合输出。
type DORASnapshot struct {
	From    time.Time     `json:"from"`
	To      time.Time     `json:"to"`
	Env     string        `json:"env"`
	Metrics []MetricValue `json:"metrics"`
}

// Aggregate 一次性算出全部 4 个指标 + 同口径环比。
func (a *DORAAggregator) Aggregate(ctx context.Context, q DORAQuery) (*DORASnapshot, error) {
	if a.db == nil {
		return nil, errors.New("db 未初始化")
	}
	q.Normalize()
	prev := q.PrevQuery()

	curRels, err := a.fetchReleases(ctx, q)
	if err != nil {
		return nil, err
	}
	prevRels, err := a.fetchReleases(ctx, prev)
	if err != nil {
		return nil, err
	}

	// v2.1: MTTR 优先用 incident 真实数据；查询失败/无数据时回退到 release 近似
	curIncidents, _ := a.fetchResolvedIncidents(ctx, q)
	prevIncidents, _ := a.fetchResolvedIncidents(ctx, prev)

	curFreq, curLead, curFail, curMTTR, curSample := computeMetricsWithIncidents(curRels, curIncidents, q)
	prevFreq, prevLead, prevFail, prevMTTR, _ := computeMetricsWithIncidents(prevRels, prevIncidents, prev)

	// v2.1: 按天聚合构造 Sparkline 序列（当前 + 上一周期）
	series := buildDailySeries(curRels, curIncidents, q)
	prevSeries := buildDailySeries(prevRels, prevIncidents, prev)

	metrics := []MetricValue{
		buildMetric("deploy_freq", "部署频率", curFreq, prevFreq, "次/天",
			"过去周期内 published 的发布数量除以天数",
			curSample, true, deployFreqBenchmark),
		buildMetric("lead_time", "变更前置时间", curLead, prevLead, "小时",
			"created_at → published_at 的中位时长（小时）",
			curSample, true, leadTimeBenchmark),
		buildMetric("change_fail_rate", "变更失败率", curFail, prevFail, "%",
			"failed + rolled_back 占 terminated 总数的比例",
			curSample, false, changeFailRateBenchmark),
		buildMetric("mttr", "平均恢复时间", curMTTR, prevMTTR, "分钟",
			"incident 真实 resolved_at - detected_at（无 incident 数据时回退到 release 近似）",
			curSample, false, mttrBenchmark),
	}
	for i := range metrics {
		if pts, ok := series[metrics[i].Key]; ok {
			metrics[i].Series = pts
		}
		if pts, ok := prevSeries[metrics[i].Key]; ok {
			metrics[i].PrevSeries = alignPrevToCurrent(pts, len(metrics[i].Series))
		}
	}

	// v2.2: 如果当前查询是"应用下钻"，顺便计算一份无过滤的全站同期值，
	//       把每个指标的 FleetValue / AppVsFleet 填回去。
	if q.ApplicationID > 0 || q.ApplicationName != "" {
		fleetQ := q
		fleetQ.ApplicationID = 0
		fleetQ.ApplicationName = ""
		fleetRels, _ := a.fetchReleases(ctx, fleetQ)
		fleetIncidents, _ := a.fetchResolvedIncidents(ctx, fleetQ)
		fFreq, fLead, fFail, fMTTR, _ := computeMetricsWithIncidents(fleetRels, fleetIncidents, fleetQ)
		attachAppVsFleet(metrics, fFreq, fLead, fFail, fMTTR)
	}

	return &DORASnapshot{
		From:    q.From,
		To:      q.To,
		Env:     q.Env,
		Metrics: metrics,
	}, nil
}

// attachAppVsFleet 把全站同期值及好/差判定写回每个指标。
//
// upIsGood 语义：deploy_freq 越大越好；lead_time、change_fail_rate、mttr 越小越好。
func attachAppVsFleet(metrics []MetricValue, fFreq, fLead, fFail, fMTTR float64) {
	setFleet := func(m *MetricValue, fleet float64, benchFn func(float64) string, upIsGood bool) {
		m.FleetValue = fleet
		m.FleetBenchmark = benchFn(fleet)
		m.AppVsFleet, m.AppVsFleetText = judgeAppVsFleet(m.Value, fleet, upIsGood)
	}
	for i := range metrics {
		switch metrics[i].Key {
		case "deploy_freq":
			setFleet(&metrics[i], fFreq, deployFreqBenchmark, true)
		case "lead_time":
			setFleet(&metrics[i], fLead, leadTimeBenchmark, false)
		case "change_fail_rate":
			setFleet(&metrics[i], fFail, changeFailRateBenchmark, false)
		case "mttr":
			setFleet(&metrics[i], fMTTR, mttrBenchmark, false)
		}
	}
}

// judgeAppVsFleet 判定应用值相对全站值的好坏。
//
// 返回 (judgment, text)，判断规则：
//   - 全站为 0 且应用也 0 → equal / 与全站持平
//   - 差异小于 5% → equal / 与全站持平
//   - 否则根据 upIsGood 判定 better/worse
func judgeAppVsFleet(app, fleet float64, upIsGood bool) (string, string) {
	if fleet == 0 && app == 0 {
		return "equal", "与全站持平"
	}
	if fleet == 0 {
		// 全站 0，应用非 0；upIsGood=true 说明越大越好，应用反而有数据是 better
		if upIsGood {
			return "better", "优于全站"
		}
		return "worse", "差于全站"
	}
	delta := (app - fleet) / math.Abs(fleet) * 100
	if math.Abs(delta) < 5 {
		return "equal", "与全站持平"
	}
	if (delta > 0) == upIsGood {
		return "better", "优于全站 " + trimFloat(math.Abs(roundTo(delta, 1))) + "%"
	}
	return "worse", "差于全站 " + trimFloat(math.Abs(roundTo(delta, 1))) + "%"
}

// buildDailySeries 按天聚合 4 个指标的点序列。
//
// 约定：
//   - 以 UTC 日历日为桶（yyyy-MM-dd）
//   - 无样本的日子补 0 点，保证前端 sparkline 横坐标等间距
//   - deploy_freq: 当日 published 计数
//   - lead_time:   当日 published 的前置时间中位（小时）
//   - change_fail_rate: 当日 (failed+rolled_back)/terminated (%)
//   - mttr: 当日 resolved 事故的 resolve-detect 中位（分钟），无事故则回退到当日 release 近似
func buildDailySeries(rels []deploy.Release, incidents []monitoring.Incident, q DORAQuery) map[string][]SeriesPoint {
	type bucket struct {
		published    int
		failed       int
		rolledBack   int
		leadTimes    []float64
		incidentDurs []float64
		failTimes    []time.Time
		pubTimes     []time.Time
	}
	buckets := make(map[string]*bucket)
	getBucket := func(key string) *bucket {
		b, ok := buckets[key]
		if !ok {
			b = &bucket{}
			buckets[key] = b
		}
		return b
	}
	dayKey := func(t time.Time) string { return t.UTC().Format("2006-01-02") }

	for _, r := range rels {
		switch r.Status {
		case deploy.ReleaseStatusPublished:
			if r.PublishedAt == nil {
				continue
			}
			k := dayKey(*r.PublishedAt)
			b := getBucket(k)
			b.published++
			lt := r.PublishedAt.Sub(r.CreatedAt).Hours()
			if lt > 0 {
				b.leadTimes = append(b.leadTimes, lt)
			}
			b.pubTimes = append(b.pubTimes, *r.PublishedAt)
		case deploy.ReleaseStatusFailed:
			k := dayKey(r.UpdatedAt)
			b := getBucket(k)
			b.failed++
			b.failTimes = append(b.failTimes, r.UpdatedAt)
		case deploy.ReleaseStatusRolledBack:
			k := dayKey(r.UpdatedAt)
			b := getBucket(k)
			b.rolledBack++
			b.failTimes = append(b.failTimes, r.UpdatedAt)
		}
	}
	for _, inc := range incidents {
		if inc.ResolvedAt == nil {
			continue
		}
		k := dayKey(*inc.ResolvedAt)
		d := inc.ResolvedAt.Sub(inc.DetectedAt).Minutes()
		if d > 0 {
			getBucket(k).incidentDurs = append(getBucket(k).incidentDurs, d)
		}
	}

	// 枚举整段窗口的每一天（含首尾）
	days := enumerateDays(q.From, q.To)
	freq := make([]SeriesPoint, 0, len(days))
	lead := make([]SeriesPoint, 0, len(days))
	fail := make([]SeriesPoint, 0, len(days))
	mttr := make([]SeriesPoint, 0, len(days))
	for _, d := range days {
		b := buckets[d]
		if b == nil {
			b = &bucket{}
		}
		freq = append(freq, SeriesPoint{Date: d, Value: float64(b.published)})
		lead = append(lead, SeriesPoint{Date: d, Value: roundTo(median(b.leadTimes), 1)})
		terminated := b.published + b.failed + b.rolledBack
		frate := 0.0
		if terminated > 0 {
			frate = float64(b.failed+b.rolledBack) / float64(terminated) * 100
		}
		fail = append(fail, SeriesPoint{Date: d, Value: roundTo(frate, 1)})
		var m float64
		if len(b.incidentDurs) > 0 {
			m = median(b.incidentDurs)
		} else {
			m = computeMTTR(b.failTimes, b.pubTimes)
		}
		mttr = append(mttr, SeriesPoint{Date: d, Value: roundTo(m, 1)})
	}

	return map[string][]SeriesPoint{
		"deploy_freq":      freq,
		"lead_time":        lead,
		"change_fail_rate": fail,
		"mttr":             mttr,
	}
}

// alignPrevToCurrent 把上一周期点序列按"x 轴索引"对齐到当前周期，长度与当前周期相同。
//
// 前端叠加对比时，"昨天 vs 一周前的昨天"按索引一一对齐；
// 长度不一致时按当前周期长度取尾部（对齐最后 N 天）。Date 字段保留为上一周期的真实日期，
// 前端 tooltip 可看到"对比日"。
func alignPrevToCurrent(prev []SeriesPoint, wantLen int) []SeriesPoint {
	if wantLen <= 0 {
		return nil
	}
	if len(prev) == wantLen {
		return prev
	}
	if len(prev) > wantLen {
		return prev[len(prev)-wantLen:]
	}
	// prev 不足则在前面补零，保证前端 x 轴对齐
	pad := make([]SeriesPoint, wantLen-len(prev))
	for i := range pad {
		pad[i] = SeriesPoint{Date: "", Value: 0}
	}
	return append(pad, prev...)
}

// enumerateDays 枚举 [from, to] 的每一天 UTC 日历日（包含首尾）。
func enumerateDays(from, to time.Time) []string {
	from = time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	to = time.Date(to.UTC().Year(), to.UTC().Month(), to.UTC().Day(), 0, 0, 0, 0, time.UTC)
	days := []string{}
	for !from.After(to) {
		days = append(days, from.Format("2006-01-02"))
		from = from.Add(24 * time.Hour)
	}
	// 防止极端情况（窗口 < 1 天）空数组
	if len(days) == 0 {
		days = []string{to.Format("2006-01-02")}
	}
	return days
}

// fetchReleases 拉取目标窗口内的发布主单（仅含已 terminated 的状态，用于评分）。
func (a *DORAAggregator) fetchReleases(ctx context.Context, q DORAQuery) ([]deploy.Release, error) {
	var list []deploy.Release
	tx := a.db.WithContext(ctx).
		Where("env = ?", q.Env).
		Where("created_at >= ? AND created_at <= ?", q.From, q.To)
	if q.ApplicationID > 0 {
		tx = tx.Where("application_id = ?", q.ApplicationID)
	} else if q.ApplicationName != "" {
		tx = tx.Where("application_name = ?", q.ApplicationName)
	}
	err := tx.Order("created_at ASC").Find(&list).Error
	return list, err
}

// fetchResolvedIncidents 拉取目标窗口内已 resolved 的事故（v2.1 MTTR 真实数据源）。
//
// 当应用下钻过滤启用时，使用 repo 的 List（支持 app 过滤）而非窗口方法，统一口径。
func (a *DORAAggregator) fetchResolvedIncidents(ctx context.Context, q DORAQuery) ([]monitoring.Incident, error) {
	if a.incidentRepo == nil {
		return nil, nil
	}
	if q.ApplicationID == 0 && q.ApplicationName == "" {
		return a.incidentRepo.ListResolvedInWindow(ctx, q.Env, q.From, q.To)
	}
	// 使用通用 List + detected_at 过滤实现 app 下钻
	f := monRepo.IncidentFilter{
		Env:    q.Env,
		Status: monitoring.IncidentStatusResolved,
		From:   &q.From,
		To:     &q.To,
	}
	if q.ApplicationID > 0 {
		id := q.ApplicationID
		f.AppID = &id
	}
	// 一次性取全部；上限保护：Window 最多 90 天，app 粒度事故通常 << 1000
	list, _, err := a.incidentRepo.List(f, 1, 1000)
	if err != nil {
		return nil, err
	}
	// 手工过滤 application_name（repo 目前无此条件）
	if q.ApplicationName != "" && q.ApplicationID == 0 {
		filtered := list[:0]
		for _, inc := range list {
			if inc.ApplicationName == q.ApplicationName {
				filtered = append(filtered, inc)
			}
		}
		list = filtered
	}
	return list, nil
}

// computeMetricsWithIncidents 综合 release + incident 计算指标。
//
// MTTR 优先取 incident 真实数据（resolved_at - detected_at 中位分钟）；
// 当 incident 为空时回退到 release 近似算法（failed → 下次 published）。
func computeMetricsWithIncidents(rels []deploy.Release, incidents []monitoring.Incident, q DORAQuery) (float64, float64, float64, float64, int) {
	freq, lead, fail, mttrApprox, sample := computeMetrics(rels, q)
	if len(incidents) == 0 {
		return freq, lead, fail, mttrApprox, sample
	}
	durations := make([]float64, 0, len(incidents))
	for _, inc := range incidents {
		if inc.ResolvedAt == nil {
			continue
		}
		d := inc.ResolvedAt.Sub(inc.DetectedAt).Minutes()
		if d > 0 {
			durations = append(durations, d)
		}
	}
	if len(durations) == 0 {
		return freq, lead, fail, mttrApprox, sample
	}
	return freq, lead, fail, roundTo(median(durations), 1), sample
}

// computeMetrics 从发布列表计算 4 个原始指标（基础口径，MTTR 为近似值）。
//
// 返回值依次：deploy_freq、lead_time（小时）、change_fail_rate（百分比）、mttr（分钟）、样本数
func computeMetrics(rels []deploy.Release, q DORAQuery) (float64, float64, float64, float64, int) {
	if len(rels) == 0 {
		return 0, 0, 0, 0, 0
	}

	// Deployment Frequency
	publishedCount := 0
	failedCount := 0
	rolledBackCount := 0
	terminated := 0
	leadTimes := make([]float64, 0, len(rels))
	publishedAt := make([]time.Time, 0)
	failedAt := make([]time.Time, 0)

	for _, r := range rels {
		switch r.Status {
		case deploy.ReleaseStatusPublished:
			publishedCount++
			terminated++
			if r.PublishedAt != nil {
				lt := r.PublishedAt.Sub(r.CreatedAt).Hours()
				if lt > 0 {
					leadTimes = append(leadTimes, lt)
				}
				publishedAt = append(publishedAt, *r.PublishedAt)
			}
		case deploy.ReleaseStatusFailed:
			failedCount++
			terminated++
			failedAt = append(failedAt, r.UpdatedAt)
		case deploy.ReleaseStatusRolledBack:
			rolledBackCount++
			terminated++
			failedAt = append(failedAt, r.UpdatedAt)
		}
	}

	days := q.Window().Hours() / 24
	if days <= 0 {
		days = 1
	}
	freq := float64(publishedCount) / days

	leadTime := median(leadTimes)

	failRate := 0.0
	if terminated > 0 {
		failRate = float64(failedCount+rolledBackCount) / float64(terminated) * 100
	}

	mttr := computeMTTR(failedAt, publishedAt)

	return roundTo(freq, 2), roundTo(leadTime, 1), roundTo(failRate, 1), roundTo(mttr, 1), terminated
}

// computeMTTR 近似：每次 failed 找其后第一个 published，取时间差中位数（分钟）。
func computeMTTR(failures, publishedTimes []time.Time) float64 {
	if len(failures) == 0 || len(publishedTimes) == 0 {
		return 0
	}
	sort.Slice(failures, func(i, j int) bool { return failures[i].Before(failures[j]) })
	sort.Slice(publishedTimes, func(i, j int) bool { return publishedTimes[i].Before(publishedTimes[j]) })

	durations := make([]float64, 0, len(failures))
	for _, f := range failures {
		idx := sort.Search(len(publishedTimes), func(i int) bool {
			return publishedTimes[i].After(f)
		})
		if idx < len(publishedTimes) {
			durations = append(durations, publishedTimes[idx].Sub(f).Minutes())
		}
	}
	return median(durations)
}

// median 中位数（数据为空时返回 0）。
func median(values []float64) float64 {
	n := len(values)
	if n == 0 {
		return 0
	}
	sorted := append([]float64(nil), values...)
	sort.Float64s(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}

// roundTo 保留 N 位小数。
func roundTo(v float64, n int) float64 {
	p := math.Pow(10, float64(n))
	return math.Round(v*p) / p
}

// buildMetric 组装单个指标 + 趋势 + 基准。
//
// upIsGood: true 表示数值越大越好（部署频率、前置时间小越好取反）。这里:
//   - deploy_freq: 越大越好 (upIsGood=true)
//   - lead_time:   越小越好 → upIsGood=false  ⚠️ 上面调用时传错了，这里语义改为 "数值升 = 趋势 up"
//
// 简化：trend 仅描述数值变化方向（升/降/平），由前端结合 upIsGood 判断颜色。
func buildMetric(key, title string, cur, prev float64, unit, desc string, sample int, _upIsGood bool, bench func(float64) string) MetricValue {
	trend, delta, txt := computeTrend(cur, prev)
	return MetricValue{
		Key:         key,
		Title:       title,
		Value:       cur,
		Unit:        unit,
		Trend:       trend,
		Delta:       delta,
		DeltaText:   txt,
		Benchmark:   bench(cur),
		Description: desc,
		Sample:      sample,
	}
}

// computeTrend 比较当前值与上一周期；返回 (trend, deltaPercent, deltaText)。
func computeTrend(cur, prev float64) (string, float64, string) {
	if prev == 0 && cur == 0 {
		return "flat", 0, "持平"
	}
	if prev == 0 {
		return "up", 100, "+100%"
	}
	delta := (cur - prev) / prev * 100
	delta = roundTo(delta, 1)
	if math.Abs(delta) < 1 {
		return "flat", delta, "持平"
	}
	if delta > 0 {
		return "up", delta, formatDelta(delta)
	}
	return "down", delta, formatDelta(delta)
}

func formatDelta(d float64) string {
	if d > 0 {
		return "+" + trimFloat(d) + "%"
	}
	return trimFloat(d) + "%"
}

func trimFloat(f float64) string {
	// 去掉无意义的尾部 0
	s := []byte{}
	s = appendFloat(s, f)
	return string(s)
}

func appendFloat(dst []byte, v float64) []byte {
	// 保留 1 位小数
	if v == float64(int64(v)) {
		return append(dst, []byte(itoa(int64(v)))...)
	}
	return append(dst, []byte(ftoa(v))...)
}

func itoa(n int64) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	buf := [20]byte{}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

func ftoa(v float64) string {
	rounded := roundTo(v, 1)
	intPart := int64(rounded)
	frac := int64(math.Abs(rounded-float64(intPart))*10 + 0.5)
	return itoa(intPart) + "." + itoa(frac)
}

// ---------- 行业基准（DORA 2024 报告分档） ----------

func deployFreqBenchmark(v float64) string {
	switch {
	case v >= 1: // 每天 ≥1 次
		return "elite"
	case v >= 1.0/7: // 每周 ≥1 次
		return "high"
	case v >= 1.0/30: // 每月 ≥1 次
		return "medium"
	default:
		return "low"
	}
}

func leadTimeBenchmark(hours float64) string {
	switch {
	case hours <= 1:
		return "elite"
	case hours <= 24:
		return "high"
	case hours <= 24*7:
		return "medium"
	default:
		return "low"
	}
}

func changeFailRateBenchmark(pct float64) string {
	switch {
	case pct <= 5:
		return "elite"
	case pct <= 10:
		return "high"
	case pct <= 15:
		return "medium"
	default:
		return "low"
	}
}

func mttrBenchmark(minutes float64) string {
	switch {
	case minutes <= 60:
		return "elite"
	case minutes <= 24*60:
		return "high"
	case minutes <= 7*24*60:
		return "medium"
	default:
		return "low"
	}
}
