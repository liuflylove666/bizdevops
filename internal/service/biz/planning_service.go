package biz

import (
	"context"
	appmodel "devops/internal/models"
	modelbiz "devops/internal/models/biz"
	modeldeploy "devops/internal/models/deploy"
	apprepo "devops/internal/modules/application/repository"
	bizrepo "devops/internal/modules/biz/repository"
	coreRepo "devops/internal/repository"
	"devops/pkg/logger"
	"errors"
	"fmt"
	"strings"

	"gorm.io/gorm"
)

type GoalDetail struct {
	Goal         *modelbiz.BizGoal          `json:"goal"`
	Requirements []modelbiz.BizRequirement  `json:"requirements"`
	Versions     []modelbiz.BizVersion      `json:"versions"`
	Summary      GoalDetailSummary          `json:"summary"`
}

type GoalDetailSummary struct {
	RequirementTotal      int `json:"requirement_total"`
	RequirementBacklog    int `json:"requirement_backlog"`
	RequirementInProgress int `json:"requirement_in_progress"`
	RequirementDone       int `json:"requirement_done"`
	VersionTotal          int `json:"version_total"`
	VersionPlanning       int `json:"version_planning"`
	VersionInProgress     int `json:"version_in_progress"`
	VersionReleased       int `json:"version_released"`
}

type RequirementDetail struct {
	Requirement *modelbiz.BizRequirement `json:"requirement"`
	Goal        *modelbiz.BizGoal        `json:"goal,omitempty"`
	Version     *modelbiz.BizVersion     `json:"version,omitempty"`
	Application *appmodel.Application    `json:"application,omitempty"`
	Pipeline    *modeldeploy.Pipeline    `json:"pipeline,omitempty"`
}

type VersionDetail struct {
	Version      *modelbiz.BizVersion       `json:"version"`
	Goal         *modelbiz.BizGoal          `json:"goal,omitempty"`
	Application  *appmodel.Application      `json:"application,omitempty"`
	Pipeline     *modeldeploy.Pipeline      `json:"pipeline,omitempty"`
	Release      *modeldeploy.Release       `json:"release,omitempty"`
	Requirements []modelbiz.BizRequirement  `json:"requirements"`
	Summary      VersionDetailSummary       `json:"summary"`
}

type VersionDetailSummary struct {
	RequirementTotal      int `json:"requirement_total"`
	RequirementBacklog    int `json:"requirement_backlog"`
	RequirementInProgress int `json:"requirement_in_progress"`
	RequirementDone       int `json:"requirement_done"`
}

type PlanningService struct {
	goals        *bizrepo.BizGoalRepository
	requirements *bizrepo.BizRequirementRepository
	versions     *bizrepo.BizVersionRepository
	jiraMappings *coreRepo.JiraProjectMappingRepository
	jiraSync     jiraSyncClient
	apps         *apprepo.ApplicationRepository
	releases     *apprepo.ReleaseRepository
	db           *gorm.DB
}

type jiraSyncClient interface {
	GetTransitions(instanceID uint, issueKey string) (map[string]interface{}, error)
	TransitionIssue(instanceID uint, issueKey, transitionID string) error
	AddComment(instanceID uint, issueKey, comment string) (map[string]interface{}, error)
}

type JiraWebhookPayload struct {
	WebhookEvent string `json:"webhookEvent"`
	Issue        struct {
		Key    string `json:"key"`
		Fields struct {
			Summary     string `json:"summary"`
			Description any    `json:"description"`
			Labels      []string `json:"labels"`
			Project     struct {
				Key string `json:"key"`
			} `json:"project"`
			Components []struct {
				Name string `json:"name"`
			} `json:"components"`
			FixVersions []struct {
				Name string `json:"name"`
			} `json:"fixVersions"`
			Parent struct {
				Key string `json:"key"`
			} `json:"parent"`
			Epic struct {
				Key string `json:"key"`
			} `json:"epic"`
			Status      struct {
				Name string `json:"name"`
			} `json:"status"`
			Priority struct {
				Name string `json:"name"`
			} `json:"priority"`
			Assignee struct {
				DisplayName string `json:"displayName"`
			} `json:"assignee"`
		} `json:"fields"`
	} `json:"issue"`
}

func NewPlanningService(
	db *gorm.DB,
	goals *bizrepo.BizGoalRepository,
	requirements *bizrepo.BizRequirementRepository,
	versions *bizrepo.BizVersionRepository,
	jiraMappings *coreRepo.JiraProjectMappingRepository,
	jiraSync jiraSyncClient,
) *PlanningService {
	return &PlanningService{
		goals:        goals,
		requirements: requirements,
		versions:     versions,
		jiraMappings: jiraMappings,
		jiraSync:     jiraSync,
		apps:         apprepo.NewApplicationRepository(db),
		releases:     apprepo.NewReleaseRepository(db),
		db:           db,
	}
}

func (s *PlanningService) ListGoals(filter bizrepo.GoalFilter, page, pageSize int) ([]modelbiz.BizGoal, int64, error) {
	return s.goals.List(filter, page, pageSize)
}

func (s *PlanningService) GetGoal(id uint) (*GoalDetail, error) {
	goal, err := s.goals.GetByID(id)
	if err != nil {
		return nil, err
	}
	requirements, err := s.requirements.ListByGoalID(id)
	if err != nil {
		return nil, err
	}
	versions, err := s.versions.ListByGoalID(id)
	if err != nil {
		return nil, err
	}
	if err := s.fillRequirementRelations(requirements); err != nil {
		return nil, err
	}
	if err := s.fillVersionRelations(versions); err != nil {
		return nil, err
	}

	summary := GoalDetailSummary{
		RequirementTotal: len(requirements),
		VersionTotal:     len(versions),
	}
	for _, item := range requirements {
		switch item.Status {
		case "backlog":
			summary.RequirementBacklog++
		case "in_progress":
			summary.RequirementInProgress++
		case "done":
			summary.RequirementDone++
		}
	}
	for _, item := range versions {
		switch item.Status {
		case "planning":
			summary.VersionPlanning++
		case "in_progress":
			summary.VersionInProgress++
		case "released":
			summary.VersionReleased++
		}
	}

	return &GoalDetail{
		Goal:         goal,
		Requirements: requirements,
		Versions:     versions,
		Summary:      summary,
	}, nil
}
func (s *PlanningService) CreateGoal(item *modelbiz.BizGoal) error { return s.goals.Create(item) }
func (s *PlanningService) UpdateGoal(item *modelbiz.BizGoal) error {
	existing, err := s.goals.GetByID(item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = existing.CreatedAt
	return s.goals.Update(item)
}
func (s *PlanningService) DeleteGoal(id uint) error { return s.goals.Delete(id) }

func (s *PlanningService) ListRequirements(filter bizrepo.RequirementFilter, page, pageSize int) ([]modelbiz.BizRequirement, int64, error) {
	list, total, err := s.requirements.List(filter, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := s.fillRequirementRelations(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *PlanningService) GetRequirement(id uint) (*RequirementDetail, error) {
	item, err := s.requirements.GetByID(id)
	if err != nil {
		return nil, err
	}
	var goal *modelbiz.BizGoal
	if item.GoalID != nil {
		goal, err = s.goals.GetByID(*item.GoalID)
		if err != nil {
			return nil, err
		}
		item.GoalName = goal.Name
	}
	var version *modelbiz.BizVersion
	if item.VersionID != nil {
		version, err = s.versions.GetByID(*item.VersionID)
		if err != nil {
			return nil, err
		}
		item.VersionName = version.Name
	}
	var application *appmodel.Application
	if item.ApplicationID != nil {
		application, err = s.apps.GetByID(context.Background(), *item.ApplicationID)
		if err != nil {
			return nil, err
		}
		item.ApplicationName = application.DisplayName
		if item.ApplicationName == "" {
			item.ApplicationName = application.Name
		}
	}
	var pipeline *modeldeploy.Pipeline
	if item.PipelineID != nil {
		pipeline, err = s.getPipelineByID(*item.PipelineID)
		if err != nil {
			return nil, err
		}
		item.PipelineName = pipeline.Name
	}
	return &RequirementDetail{
		Requirement: item,
		Goal:        goal,
		Version:     version,
		Application: application,
		Pipeline:    pipeline,
	}, nil
}

func (s *PlanningService) CreateRequirement(item *modelbiz.BizRequirement) error {
	return s.requirements.Create(item)
}
func (s *PlanningService) UpdateRequirement(item *modelbiz.BizRequirement) error {
	existing, err := s.requirements.GetByID(item.ID)
	if err != nil {
		return err
	}
	oldStatus := existing.Status
	item.CreatedAt = existing.CreatedAt
	item.ExternalKey = existing.ExternalKey
	if item.Source == "" {
		item.Source = existing.Source
	}
	if item.Source == "jira" {
		if item.ApplicationID == nil {
			item.ApplicationID = existing.ApplicationID
		}
		if item.VersionID == nil {
			item.VersionID = existing.VersionID
		}
		if strings.TrimSpace(item.JiraEpicKey) == "" {
			item.JiraEpicKey = existing.JiraEpicKey
		}
		if strings.TrimSpace(item.JiraLabels) == "" {
			item.JiraLabels = existing.JiraLabels
		}
		if strings.TrimSpace(item.JiraComponents) == "" {
			item.JiraComponents = existing.JiraComponents
		}
	}
	if err := s.requirements.Update(item); err != nil {
		return err
	}
	s.syncJiraStatusBestEffort(existing, item, oldStatus)
	return nil
}
func (s *PlanningService) DeleteRequirement(id uint) error { return s.requirements.Delete(id) }

func (s *PlanningService) ListVersions(filter bizrepo.VersionFilter, page, pageSize int) ([]modelbiz.BizVersion, int64, error) {
	list, total, err := s.versions.List(filter, page, pageSize)
	if err != nil {
		return nil, 0, err
	}
	if err := s.fillVersionRelations(list); err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (s *PlanningService) GetVersion(id uint) (*VersionDetail, error) {
	version, err := s.versions.GetByID(id)
	if err != nil {
		return nil, err
	}
	var goal *modelbiz.BizGoal
	if version.GoalID != nil {
		goal, err = s.goals.GetByID(*version.GoalID)
		if err != nil {
			return nil, err
		}
		version.GoalName = goal.Name
	}
	var application *appmodel.Application
	if version.ApplicationID != nil {
		application, err = s.apps.GetByID(context.Background(), *version.ApplicationID)
		if err != nil {
			return nil, err
		}
		version.ApplicationName = application.DisplayName
		if version.ApplicationName == "" {
			version.ApplicationName = application.Name
		}
	}
	var pipeline *modeldeploy.Pipeline
	if version.PipelineID != nil {
		pipeline, err = s.getPipelineByID(*version.PipelineID)
		if err != nil {
			return nil, err
		}
		version.PipelineName = pipeline.Name
	}
	var release *modeldeploy.Release
	if version.ReleaseID != nil {
		release, err = s.releases.GetByID(*version.ReleaseID)
		if err != nil {
			return nil, err
		}
		version.ReleaseTitle = release.Title
	}
	requirements, err := s.requirements.ListByVersionID(id)
	if err != nil {
		return nil, err
	}
	if err := s.fillRequirementRelations(requirements); err != nil {
		return nil, err
	}
	summary := VersionDetailSummary{
		RequirementTotal: len(requirements),
	}
	for _, item := range requirements {
		switch item.Status {
		case "backlog":
			summary.RequirementBacklog++
		case "in_progress":
			summary.RequirementInProgress++
		case "done":
			summary.RequirementDone++
		}
	}
	return &VersionDetail{
		Version:      version,
		Goal:         goal,
		Application:  application,
		Pipeline:     pipeline,
		Release:      release,
		Requirements: requirements,
		Summary:      summary,
	}, nil
}
func (s *PlanningService) CreateVersion(item *modelbiz.BizVersion) error { return s.versions.Create(item) }
func (s *PlanningService) UpdateVersion(item *modelbiz.BizVersion) error {
	existing, err := s.versions.GetByID(item.ID)
	if err != nil {
		return err
	}
	item.CreatedAt = existing.CreatedAt
	return s.versions.Update(item)
}
func (s *PlanningService) DeleteVersion(id uint) error { return s.versions.Delete(id) }

func (s *PlanningService) HandleJiraWebhook(payload *JiraWebhookPayload, jiraInstanceID *uint) (string, error) {
	if payload == nil {
		return "", fmt.Errorf("payload is nil")
	}
	issueKey := strings.TrimSpace(payload.Issue.Key)
	if issueKey == "" {
		return "", fmt.Errorf("issue key is required")
	}

	if strings.EqualFold(payload.WebhookEvent, "jira:issue_deleted") {
		return "deleted", s.db.Where("external_key = ?", issueKey).Delete(&modelbiz.BizRequirement{}).Error
	}

	title := strings.TrimSpace(payload.Issue.Fields.Summary)
	if title == "" {
		title = issueKey
	}
	description := strings.TrimSpace(toPlainText(payload.Issue.Fields.Description))

	newItem := modelbiz.BizRequirement{
		ExternalKey: issueKey,
		Title:       title,
		Source:      "jira",
		Owner:       strings.TrimSpace(payload.Issue.Fields.Assignee.DisplayName),
		Status:      mapJiraStatus(payload.Issue.Fields.Status.Name),
		Priority:    mapJiraPriority(payload.Issue.Fields.Priority.Name),
		Description: description,
		JiraEpicKey: deriveJiraEpicKey(payload.Issue.Fields.Epic.Key, payload.Issue.Fields.Parent.Key),
		JiraLabels:  normalizeCSV(payload.Issue.Fields.Labels),
		JiraComponents: normalizeCSV(extractComponentNames(payload.Issue.Fields.Components)),
	}

	if mapping := s.resolveJiraMapping(jiraInstanceID, payload.Issue.Fields.Project.Key); mapping != nil {
		newItem.ApplicationID = mapping.DevopsAppID
	}
	if len(payload.Issue.Fields.FixVersions) > 0 {
		if versionID := s.resolveBizVersionID(payload.Issue.Fields.FixVersions, newItem.ApplicationID); versionID != nil {
			newItem.VersionID = versionID
		}
	}

	existing, err := s.requirements.GetByExternalKey(issueKey)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "created", s.requirements.Create(&newItem)
		}
		return "", err
	}

	newItem.ID = existing.ID
	newItem.CreatedAt = existing.CreatedAt
	newItem.GoalID = existing.GoalID
	newItem.VersionID = existing.VersionID
	newItem.ApplicationID = existing.ApplicationID
	newItem.PipelineID = existing.PipelineID
	newItem.ValueScore = existing.ValueScore
	if newItem.Owner == "" {
		newItem.Owner = existing.Owner
	}
	if newItem.Description == "" {
		newItem.Description = existing.Description
	}
	return "updated", s.requirements.Update(&newItem)
}

func (s *PlanningService) resolveJiraMapping(jiraInstanceID *uint, projectKey string) *appmodel.JiraProjectMapping {
	if jiraInstanceID == nil || *jiraInstanceID == 0 || s.jiraMappings == nil || strings.TrimSpace(projectKey) == "" {
		return nil
	}
	m, err := s.jiraMappings.GetByInstanceAndProjectKey(*jiraInstanceID, projectKey)
	if err != nil {
		return nil
	}
	return m
}

func (s *PlanningService) syncJiraStatusBestEffort(existing, updated *modelbiz.BizRequirement, oldStatus string) {
	if s.jiraSync == nil || updated == nil || existing == nil {
		return
	}
	if strings.TrimSpace(existing.ExternalKey) == "" || strings.TrimSpace(existing.Source) != "jira" {
		return
	}
	if updated.Status == "" || updated.Status == oldStatus {
		return
	}
	instanceID, issueKey, err := s.resolveJiraInstanceByIssueKey(existing.ExternalKey)
	if err != nil {
		logger.L().WithField("requirement_id", existing.ID).WithField("external_key", existing.ExternalKey).WithField("error", err).Warn("resolve jira mapping for status sync failed")
		return
	}
	transitions, err := s.jiraSync.GetTransitions(instanceID, issueKey)
	if err != nil {
		logger.L().WithField("requirement_id", existing.ID).WithField("issue_key", issueKey).WithField("error", err).Warn("load jira transitions failed")
		return
	}
	transitionID := selectJiraTransitionID(transitions, updated.Status)
	if transitionID == "" {
		logger.L().WithField("requirement_id", existing.ID).WithField("issue_key", issueKey).WithField("target_status", updated.Status).Warn("no jira transition matched target status")
		return
	}
	if err := s.jiraSync.TransitionIssue(instanceID, issueKey, transitionID); err != nil {
		logger.L().WithField("requirement_id", existing.ID).WithField("issue_key", issueKey).WithField("transition_id", transitionID).WithField("error", err).Warn("jira transition failed")
		return
	}
	comment := fmt.Sprintf("状态由 DevOps 同步：%s -> %s", oldStatus, updated.Status)
	if _, err := s.jiraSync.AddComment(instanceID, issueKey, comment); err != nil {
		logger.L().WithField("requirement_id", existing.ID).WithField("issue_key", issueKey).WithField("error", err).Warn("jira sync comment failed")
	}
}

func (s *PlanningService) resolveJiraInstanceByIssueKey(externalKey string) (uint, string, error) {
	issueKey := strings.TrimSpace(externalKey)
	if issueKey == "" {
		return 0, "", fmt.Errorf("issue key is empty")
	}
	projectKey := issueProjectKey(issueKey)
	if projectKey == "" || s.jiraMappings == nil {
		return 0, "", fmt.Errorf("project key or jira mapping repo unavailable")
	}
	mapping, err := s.jiraMappings.FindByProjectKey(projectKey)
	if err != nil {
		return 0, "", err
	}
	if mapping.JiraInstanceID == 0 {
		return 0, "", fmt.Errorf("jira instance id is empty for project %s", projectKey)
	}
	return mapping.JiraInstanceID, issueKey, nil
}

func (s *PlanningService) resolveBizVersionID(fixVersions []struct {
	Name string `json:"name"`
}, applicationID *uint) *uint {
	for _, fv := range fixVersions {
		name := strings.TrimSpace(fv.Name)
		if name == "" {
			continue
		}
		q := s.db.Model(&modelbiz.BizVersion{})
		if applicationID != nil {
			q = q.Where("application_id = ?", *applicationID)
		}
		var version modelbiz.BizVersion
		if err := q.Where("name = ? OR code = ?", name, name).Order("id DESC").First(&version).Error; err == nil {
			return &version.ID
		}
	}
	return nil
}

func (s *PlanningService) fillRequirementRelations(list []modelbiz.BizRequirement) error {
	var goalIDs []uint
	var versionIDs []uint
	var applicationIDs []uint
	var pipelineIDs []uint
	for _, item := range list {
		if item.GoalID != nil {
			goalIDs = append(goalIDs, *item.GoalID)
		}
		if item.VersionID != nil {
			versionIDs = append(versionIDs, *item.VersionID)
		}
		if item.ApplicationID != nil {
			applicationIDs = append(applicationIDs, *item.ApplicationID)
		}
		if item.PipelineID != nil {
			pipelineIDs = append(pipelineIDs, *item.PipelineID)
		}
	}

	goalMap := map[uint]string{}
	if len(goalIDs) > 0 {
		goals, err := s.goals.FindByIDs(goalIDs)
		if err != nil {
			return err
		}
		for _, item := range goals {
			goalMap[item.ID] = item.Name
		}
	}

	versionMap := map[uint]string{}
	if len(versionIDs) > 0 {
		versions, err := s.versions.FindByIDs(versionIDs)
		if err != nil {
			return err
		}
		for _, item := range versions {
			versionMap[item.ID] = item.Name
		}
	}

	applicationMap, err := s.getApplicationNameMap(applicationIDs)
	if err != nil {
		return err
	}

	pipelineMap, err := s.getPipelineNameMap(pipelineIDs)
	if err != nil {
		return err
	}

	for i := range list {
		if list[i].GoalID != nil {
			list[i].GoalName = goalMap[*list[i].GoalID]
		}
		if list[i].VersionID != nil {
			list[i].VersionName = versionMap[*list[i].VersionID]
		}
		if list[i].ApplicationID != nil {
			list[i].ApplicationName = applicationMap[*list[i].ApplicationID]
		}
		if list[i].PipelineID != nil {
			list[i].PipelineName = pipelineMap[*list[i].PipelineID]
		}
	}
	return nil
}

func (s *PlanningService) fillVersionRelations(list []modelbiz.BizVersion) error {
	var goalIDs []uint
	var applicationIDs []uint
	var pipelineIDs []uint
	var releaseIDs []uint
	for _, item := range list {
		if item.GoalID != nil {
			goalIDs = append(goalIDs, *item.GoalID)
		}
		if item.ApplicationID != nil {
			applicationIDs = append(applicationIDs, *item.ApplicationID)
		}
		if item.PipelineID != nil {
			pipelineIDs = append(pipelineIDs, *item.PipelineID)
		}
		if item.ReleaseID != nil {
			releaseIDs = append(releaseIDs, *item.ReleaseID)
		}
	}
	goalMap := map[uint]string{}
	if len(goalIDs) > 0 {
		goals, err := s.goals.FindByIDs(goalIDs)
		if err != nil {
			return err
		}
		for _, item := range goals {
			goalMap[item.ID] = item.Name
		}
	}
	applicationMap, err := s.getApplicationNameMap(applicationIDs)
	if err != nil {
		return err
	}
	pipelineMap, err := s.getPipelineNameMap(pipelineIDs)
	if err != nil {
		return err
	}
	releaseMap, err := s.getReleaseTitleMap(releaseIDs)
	if err != nil {
		return err
	}
	for i := range list {
		if list[i].GoalID != nil {
			list[i].GoalName = goalMap[*list[i].GoalID]
		}
		if list[i].ApplicationID != nil {
			list[i].ApplicationName = applicationMap[*list[i].ApplicationID]
		}
		if list[i].PipelineID != nil {
			list[i].PipelineName = pipelineMap[*list[i].PipelineID]
		}
		if list[i].ReleaseID != nil {
			list[i].ReleaseTitle = releaseMap[*list[i].ReleaseID]
		}
	}
	return nil
}

func (s *PlanningService) getApplicationNameMap(ids []uint) (map[uint]string, error) {
	result := map[uint]string{}
	if len(ids) == 0 {
		return result, nil
	}
	var apps []appmodel.Application
	if err := s.db.Where("id IN ?", uniqueUint(ids)).Find(&apps).Error; err != nil {
		return nil, err
	}
	for _, item := range apps {
		result[item.ID] = item.DisplayName
		if result[item.ID] == "" {
			result[item.ID] = item.Name
		}
	}
	return result, nil
}

func (s *PlanningService) getPipelineNameMap(ids []uint) (map[uint]string, error) {
	result := map[uint]string{}
	if len(ids) == 0 {
		return result, nil
	}
	var pipelines []modeldeploy.Pipeline
	if err := s.db.Where("id IN ?", uniqueUint(ids)).Find(&pipelines).Error; err != nil {
		return nil, err
	}
	for _, item := range pipelines {
		result[item.ID] = item.Name
	}
	return result, nil
}

func (s *PlanningService) getReleaseTitleMap(ids []uint) (map[uint]string, error) {
	result := map[uint]string{}
	if len(ids) == 0 {
		return result, nil
	}
	var releases []modeldeploy.Release
	if err := s.db.Where("id IN ?", uniqueUint(ids)).Find(&releases).Error; err != nil {
		return nil, err
	}
	for _, item := range releases {
		result[item.ID] = item.Title
	}
	return result, nil
}

func (s *PlanningService) getPipelineByID(id uint) (*modeldeploy.Pipeline, error) {
	var item modeldeploy.Pipeline
	return &item, s.db.First(&item, id).Error
}

func uniqueUint(ids []uint) []uint {
	seen := map[uint]struct{}{}
	result := make([]uint, 0, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func toPlainText(value any) string {
	if value == nil {
		return ""
	}
	switch v := value.(type) {
	case string:
		return v
	default:
		return fmt.Sprintf("%v", v)
	}
}

func mapJiraStatus(status string) string {
	s := strings.ToLower(strings.TrimSpace(status))
	switch {
	case s == "", strings.Contains(s, "todo"), strings.Contains(s, "open"), strings.Contains(s, "backlog"):
		return "backlog"
	case strings.Contains(s, "done"), strings.Contains(s, "closed"), strings.Contains(s, "resolved"):
		return "done"
	default:
		return "in_progress"
	}
}

func mapJiraPriority(priority string) string {
	p := strings.ToLower(strings.TrimSpace(priority))
	switch {
	case strings.Contains(p, "highest"), strings.Contains(p, "high"), strings.Contains(p, "critical"), strings.Contains(p, "blocker"):
		return "high"
	case strings.Contains(p, "lowest"), strings.Contains(p, "low"), strings.Contains(p, "minor"), strings.Contains(p, "trivial"):
		return "low"
	default:
		return "medium"
	}
}

func deriveJiraEpicKey(epicKey, parentKey string) string {
	epicKey = strings.TrimSpace(epicKey)
	if epicKey != "" {
		return epicKey
	}
	return strings.TrimSpace(parentKey)
}

func extractComponentNames(components []struct {
	Name string `json:"name"`
}) []string {
	result := make([]string, 0, len(components))
	for _, c := range components {
		if name := strings.TrimSpace(c.Name); name != "" {
			result = append(result, name)
		}
	}
	return result
}

func normalizeCSV(values []string) string {
	if len(values) == 0 {
		return ""
	}
	seen := map[string]struct{}{}
	result := make([]string, 0, len(values))
	for _, value := range values {
		v := strings.TrimSpace(value)
		if v == "" {
			continue
		}
		key := strings.ToLower(v)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, v)
	}
	return strings.Join(result, ",")
}

func issueProjectKey(issueKey string) string {
	parts := strings.Split(strings.TrimSpace(issueKey), "-")
	if len(parts) < 2 {
		return ""
	}
	return strings.TrimSpace(parts[0])
}

func selectJiraTransitionID(payload map[string]interface{}, targetStatus string) string {
	rawTransitions, ok := payload["transitions"].([]interface{})
	if !ok || len(rawTransitions) == 0 {
		return ""
	}
	type transitionItem struct {
		ID     string
		Name   string
		ToName string
	}
	items := make([]transitionItem, 0, len(rawTransitions))
	for _, raw := range rawTransitions {
		m, ok := raw.(map[string]interface{})
		if !ok {
			continue
		}
		item := transitionItem{
			ID:   strings.TrimSpace(fmt.Sprintf("%v", m["id"])),
			Name: strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", m["name"]))),
		}
		if toMap, ok := m["to"].(map[string]interface{}); ok {
			item.ToName = strings.ToLower(strings.TrimSpace(fmt.Sprintf("%v", toMap["name"])))
		}
		if item.ID != "" {
			items = append(items, item)
		}
	}
	if len(items) == 0 {
		return ""
	}
	scoreFn := func(it transitionItem) int {
		text := it.Name + " " + it.ToName
		switch targetStatus {
		case "done":
			switch {
			case strings.Contains(text, "done"), strings.Contains(text, "close"), strings.Contains(text, "resolved"):
				return 3
			case strings.Contains(text, "complete"), strings.Contains(text, "finish"):
				return 2
			}
		case "in_progress":
			switch {
			case strings.Contains(text, "progress"), strings.Contains(text, "start"), strings.Contains(text, "develop"):
				return 3
			case strings.Contains(text, "review"), strings.Contains(text, "doing"):
				return 2
			}
		case "backlog":
			switch {
			case strings.Contains(text, "to do"), strings.Contains(text, "open"), strings.Contains(text, "backlog"):
				return 3
			case strings.Contains(text, "reopen"), strings.Contains(text, "todo"):
				return 2
			}
		}
		return 0
	}
	bestID := ""
	bestScore := -1
	for _, item := range items {
		score := scoreFn(item)
		if score > bestScore {
			bestScore = score
			bestID = item.ID
		}
	}
	if bestScore <= 0 {
		return ""
	}
	return bestID
}
