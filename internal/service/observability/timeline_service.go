// Package observability
//
// timeline_service.go: 事件时间线聚合（E4-03），合并 Incident / ChangeEvent / Release。
package observability

import (
	"sort"
	"strconv"
	"strings"
	"time"

	appmodel "devops/internal/models/application"
	deploymodel "devops/internal/models/deploy"
	inframodel "devops/internal/models/infrastructure"
	monmodel "devops/internal/models/monitoring"
	appRepo "devops/internal/modules/application/repository"
	monRepo "devops/internal/modules/monitoring/repository"
	"devops/internal/service/changelog"
	"devops/pkg/dto"
	"gorm.io/gorm"
)

// TimelineQuery 查询参数。
type TimelineQuery struct {
	ApplicationID *uint
	Env           string
	From          time.Time
	To            time.Time
	Limit         int
}

// TimelineService 聚合服务。
type TimelineService struct {
	incidents *monRepo.IncidentRepository
	changelog *changelog.Service
	releases  *appRepo.ReleaseRepository
	db        *gorm.DB
}

// NewTimelineService 构造。
func NewTimelineService(
	inc *monRepo.IncidentRepository,
	ch *changelog.Service,
	rel *appRepo.ReleaseRepository,
	db *gorm.DB,
) *TimelineService {
	return &TimelineService{incidents: inc, changelog: ch, releases: rel, db: db}
}

// Aggregate 拉取多源事件，按时间降序合并后截断至 Limit。
func (s *TimelineService) Aggregate(q TimelineQuery) (*dto.TimelineResponse, error) {
	if s == nil || s.incidents == nil || s.changelog == nil || s.releases == nil {
		return &dto.TimelineResponse{Items: []dto.TimelineItem{}}, nil
	}
	limit := q.Limit
	if limit < 1 {
		limit = 150
	}
	if limit > 300 {
		limit = 300
	}
	// 每源多取一些，合并后再截断，减少「某一类占满」的偏差。
	perSource := limit
	if perSource < 80 {
		perSource = 80
	}
	if perSource > 120 {
		perSource = 120
	}

	fromStr := q.From.UTC().Format(time.RFC3339)
	toStr := q.To.UTC().Format(time.RFC3339)
	appName := s.lookupApplicationName(q.ApplicationID)

	incFilter := monRepo.IncidentFilter{
		AppID: q.ApplicationID,
		Env:   q.Env,
		From:  &q.From,
		To:    &q.To,
	}
	incs, _, err := s.incidents.List(incFilter, 1, perSource)
	if err != nil {
		return nil, err
	}

	ceFilter := appRepo.ChangeEventFilter{
		ApplicationID: q.ApplicationID,
		Env:           q.Env,
		StartTime:     fromStr,
		EndTime:       toStr,
	}
	events, _, err := s.changelog.List(ceFilter, 1, perSource)
	if err != nil {
		return nil, err
	}

	relFilter := appRepo.ReleaseFilter{
		ApplicationID: q.ApplicationID,
		Env:           q.Env,
		CreatedFrom:   &q.From,
		CreatedTo:     &q.To,
	}
	rels, _, err := s.releases.List(relFilter, 1, perSource)
	if err != nil {
		return nil, err
	}

	items := make([]dto.TimelineItem, 0, len(incs)+len(events)+len(rels))
	for i := range incs {
		items = append(items, incidentToItem(&incs[i]))
	}
	for i := range events {
		items = append(items, changeEventToItem(&events[i]))
	}
	for i := range rels {
		items = append(items, releaseToItem(&rels[i]))
	}
	if s.db != nil {
		alertItems, err := s.listAlertItems(q, appName, perSource)
		if err != nil {
			return nil, err
		}
		items = append(items, alertItems...)

		approvalItems, err := s.listApprovalItems(q, appName, perSource)
		if err != nil {
			return nil, err
		}
		items = append(items, approvalItems...)
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].At.After(items[j].At)
	})

	truncated := len(items) > limit
	if len(items) > limit {
		items = items[:limit]
	}

	return &dto.TimelineResponse{
		Items:     items,
		From:      q.From,
		To:        q.To,
		Truncated: truncated,
	}, nil
}

func incidentToItem(i *monmodel.Incident) dto.TimelineItem {
	ref := "/incidents/" + strconv.FormatUint(uint64(i.ID), 10)
	return dto.TimelineItem{
		Kind:     dto.TimelineKindIncident,
		ID:       i.ID,
		At:       i.DetectedAt,
		Title:    i.Title,
		Summary:  trim(i.Description, 240),
		Status:   i.Status,
		Severity: i.Severity,
		Env:      i.Env,
		Ref:      ref,
		Meta: map[string]any{
			"application_id":   i.ApplicationID,
			"application_name": i.ApplicationName,
			"release_id":       i.ReleaseID,
			"source":           i.Source,
		},
	}
}

func changeEventToItem(e *deploymodel.ChangeEvent) dto.TimelineItem {
	ref := "/deploy/timeline"
	return dto.TimelineItem{
		Kind:    dto.TimelineKindChangeEvent,
		ID:      e.ID,
		At:      e.CreatedAt,
		Title:   e.Title,
		Summary: trim(e.Description, 240),
		Status:  e.Status,
		Env:     e.Env,
		Ref:     ref,
		Meta: map[string]any{
			"event_type":       e.EventType,
			"event_id":         e.EventID,
			"application_id":   e.ApplicationID,
			"application_name": e.ApplicationName,
			"operator":         e.Operator,
		},
	}
}

func releaseToItem(r *deploymodel.Release) dto.TimelineItem {
	ref := "/releases/" + strconv.FormatUint(uint64(r.ID), 10)
	return dto.TimelineItem{
		Kind:    dto.TimelineKindRelease,
		ID:      r.ID,
		At:      r.CreatedAt,
		Title:   r.Title,
		Summary: trim(r.Description, 240),
		Status:  r.Status,
		Env:     r.Env,
		Ref:     ref,
		Meta: map[string]any{
			"application_id":   r.ApplicationID,
			"application_name": r.ApplicationName,
			"version":          r.Version,
			"risk_level":       r.RiskLevel,
		},
	}
}

func (s *TimelineService) lookupApplicationName(applicationID *uint) string {
	if s == nil || s.db == nil || applicationID == nil || *applicationID == 0 {
		return ""
	}
	var app appmodel.Application
	if err := s.db.Select("name").First(&app, *applicationID).Error; err != nil {
		return ""
	}
	return strings.TrimSpace(app.Name)
}

func (s *TimelineService) listAlertItems(q TimelineQuery, appName string, limit int) ([]dto.TimelineItem, error) {
	var alerts []monmodel.AlertHistory
	query := s.db.Model(&monmodel.AlertHistory{}).
		Where("created_at >= ? AND created_at <= ?", q.From, q.To)
	if appName != "" {
		like := "%" + appName + "%"
		query = query.Where("(title LIKE ? OR content LIKE ?)", like, like)
	}
	if env := strings.TrimSpace(q.Env); env != "" {
		likeEnv := "%" + env + "%"
		query = query.Where("(title LIKE ? OR content LIKE ? OR source_url LIKE ?)", likeEnv, likeEnv, likeEnv)
	}
	if err := query.Order("created_at DESC").Limit(limit).Find(&alerts).Error; err != nil {
		return nil, err
	}
	items := make([]dto.TimelineItem, 0, len(alerts))
	for i := range alerts {
		items = append(items, alertToItem(&alerts[i], appName))
	}
	return items, nil
}

func (s *TimelineService) listApprovalItems(q TimelineQuery, appName string, limit int) ([]dto.TimelineItem, error) {
	var approvals []inframodel.GitOpsChangeRequest
	query := s.db.Model(&inframodel.GitOpsChangeRequest{}).
		Where("approval_instance_id IS NOT NULL").
		Where(`((approval_finished_at IS NOT NULL AND approval_finished_at >= ? AND approval_finished_at <= ?) OR (approval_finished_at IS NULL AND created_at >= ? AND created_at <= ?))`,
			q.From, q.To, q.From, q.To)
	if appName != "" {
		query = query.Where("application_name = ?", appName)
	}
	if env := strings.TrimSpace(q.Env); env != "" {
		query = query.Where("env = ?", env)
	}
	if err := query.Order("COALESCE(approval_finished_at, created_at) DESC").Limit(limit).Find(&approvals).Error; err != nil {
		return nil, err
	}
	items := make([]dto.TimelineItem, 0, len(approvals))
	for i := range approvals {
		items = append(items, approvalToItem(&approvals[i]))
	}
	return items, nil
}

func alertToItem(a *monmodel.AlertHistory, appName string) dto.TimelineItem {
	title := strings.TrimSpace(a.Title)
	if title == "" {
		title = "告警事件 #" + strconv.FormatUint(uint64(a.ID), 10)
	}
	status := firstNonEmpty(strings.TrimSpace(a.AckStatus), strings.TrimSpace(a.Status))
	summary := firstNonEmpty(strings.TrimSpace(a.Content), strings.TrimSpace(a.ErrorMsg))
	meta := map[string]any{
		"alert_config_id": a.AlertConfigID,
		"alert_type":      a.Type,
		"dispatch_status": a.Status,
		"source_id":       a.SourceID,
		"source_url":      a.SourceURL,
	}
	if appName != "" {
		meta["application_name"] = appName
	}
	return dto.TimelineItem{
		Kind:     dto.TimelineKindAlert,
		ID:       a.ID,
		At:       a.CreatedAt,
		Title:    title,
		Summary:  trim(summary, 240),
		Status:   status,
		Severity: a.Level,
		Ref:      "/alert/history",
		Meta:     meta,
	}
}

func approvalToItem(a *inframodel.GitOpsChangeRequest) dto.TimelineItem {
	at := a.CreatedAt
	if a.ApprovalFinishedAt != nil && !a.ApprovalFinishedAt.IsZero() {
		at = *a.ApprovalFinishedAt
	}
	title := firstNonEmpty(strings.TrimSpace(a.ApplicationName), "GitOps 变更") + " 审批"
	summaryParts := make([]string, 0, 4)
	if chain := strings.TrimSpace(a.ApprovalChainName); chain != "" {
		summaryParts = append(summaryParts, "审批链："+chain)
	}
	if repo := strings.TrimSpace(a.ImageRepository); repo != "" {
		tag := strings.TrimSpace(a.ImageTag)
		if tag != "" {
			summaryParts = append(summaryParts, "镜像："+repo+":"+tag)
		} else {
			summaryParts = append(summaryParts, "镜像："+repo)
		}
	}
	if mr := strings.TrimSpace(a.MergeRequestIID); mr != "" {
		summaryParts = append(summaryParts, "MR !"+mr)
	}
	if msg := strings.TrimSpace(a.ErrorMessage); msg != "" {
		summaryParts = append(summaryParts, msg)
	}
	ref := "/approval/history"
	if a.ApprovalInstanceID != nil && *a.ApprovalInstanceID > 0 {
		ref = "/approval/instances/" + strconv.FormatUint(uint64(*a.ApprovalInstanceID), 10)
	}
	return dto.TimelineItem{
		Kind:    dto.TimelineKindApproval,
		ID:      a.ID,
		At:      at,
		Title:   title,
		Summary: trim(strings.Join(summaryParts, " · "), 240),
		Status:  firstNonEmpty(strings.TrimSpace(a.ApprovalStatus), strings.TrimSpace(a.Status)),
		Env:     a.Env,
		Ref:     ref,
		Meta: map[string]any{
			"change_request_id":    a.ID,
			"application_name":     a.ApplicationName,
			"approval_chain_id":    a.ApprovalChainID,
			"approval_chain_name":  a.ApprovalChainName,
			"approval_instance_id": a.ApprovalInstanceID,
			"merge_request_iid":    a.MergeRequestIID,
			"merge_request_url":    a.MergeRequestURL,
			"change_status":        a.Status,
			"auto_merge_status":    a.AutoMergeStatus,
		},
	}
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	if n < 3 {
		return s[:n]
	}
	return s[:n-3] + "..."
}
