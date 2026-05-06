package biz

import (
	"fmt"
	"testing"
	"time"

	"devops/internal/models"
	modelbiz "devops/internal/models/biz"
	bizrepo "devops/internal/modules/biz/repository"
	coreRepo "devops/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type stubJiraSyncClient struct {
	transitionsResp map[string]interface{}
	transitionCalls int
	commentCalls    int
	lastTransition  string
	lastIssueKey    string
}

func (s *stubJiraSyncClient) GetTransitions(_ uint, _ string) (map[string]interface{}, error) {
	return s.transitionsResp, nil
}

func (s *stubJiraSyncClient) TransitionIssue(_ uint, issueKey, transitionID string) error {
	s.transitionCalls++
	s.lastIssueKey = issueKey
	s.lastTransition = transitionID
	return nil
}

func (s *stubJiraSyncClient) AddComment(_ uint, issueKey, _ string) (map[string]interface{}, error) {
	s.commentCalls++
	s.lastIssueKey = issueKey
	return map[string]interface{}{"ok": true}, nil
}

func newPlanningServiceForTest(t *testing.T) (*PlanningService, *gorm.DB) {
	t.Helper()
	dsn := fmt.Sprintf("file:planning_service_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&modelbiz.BizGoal{}, &modelbiz.BizRequirement{}, &modelbiz.BizVersion{}, &models.JiraProjectMapping{}); err != nil {
		t.Fatalf("auto migrate failed: %v", err)
	}

	goalRepo := bizrepo.NewBizGoalRepository(db)
	reqRepo := bizrepo.NewBizRequirementRepository(db)
	verRepo := bizrepo.NewBizVersionRepository(db)
	mappingRepo := coreRepo.NewJiraProjectMappingRepository(db)
	return NewPlanningService(db, goalRepo, reqRepo, verRepo, mappingRepo, nil), db
}

func TestPlanningServiceHandleJiraWebhook_CreateWithMappingAndVersion(t *testing.T) {
	svc, db := newPlanningServiceForTest(t)

	appID := uint(101)
	if err := db.Create(&models.JiraProjectMapping{
		JiraInstanceID: 1,
		JiraProjectKey: "DEV",
		DevopsAppID:    &appID,
	}).Error; err != nil {
		t.Fatalf("create mapping failed: %v", err)
	}
	if err := db.Create(&modelbiz.BizVersion{
		Name:          "v2.0.0",
		Code:          "v2.0.0",
		ApplicationID: &appID,
		Status:        "planning",
	}).Error; err != nil {
		t.Fatalf("create version failed: %v", err)
	}

	payload := &JiraWebhookPayload{WebhookEvent: "jira:issue_created"}
	payload.Issue.Key = "DEV-12"
	payload.Issue.Fields.Summary = "jira issue summary"
	payload.Issue.Fields.Project.Key = "DEV"
	payload.Issue.Fields.Epic.Key = "DEV-EPIC-2"
	payload.Issue.Fields.Labels = []string{"backend", "release", "backend"}
	payload.Issue.Fields.Components = []struct {
		Name string `json:"name"`
	}{
		{Name: "api"},
		{Name: "release"},
	}
	payload.Issue.Fields.Status.Name = "In Progress"
	payload.Issue.Fields.Priority.Name = "High"
	payload.Issue.Fields.Assignee.DisplayName = "alice"
	payload.Issue.Fields.FixVersions = []struct {
		Name string `json:"name"`
	}{
		{Name: "v2.0.0"},
	}

	instanceID := uint(1)
	action, err := svc.HandleJiraWebhook(payload, &instanceID)
	if err != nil {
		t.Fatalf("handle webhook failed: %v", err)
	}
	if action != "created" {
		t.Fatalf("expected created action, got %s", action)
	}

	item, err := svc.requirements.GetByExternalKey("DEV-12")
	if err != nil {
		t.Fatalf("query requirement failed: %v", err)
	}
	if item.ApplicationID == nil || *item.ApplicationID != appID {
		t.Fatalf("expected application id %d, got %#v", appID, item.ApplicationID)
	}
	if item.VersionID == nil {
		t.Fatalf("expected version to be linked")
	}
	if item.JiraEpicKey != "DEV-EPIC-2" {
		t.Fatalf("expected epic key mapped, got %s", item.JiraEpicKey)
	}
	if item.JiraLabels != "backend,release" {
		t.Fatalf("expected labels normalized, got %s", item.JiraLabels)
	}
	if item.JiraComponents != "api,release" {
		t.Fatalf("expected components normalized, got %s", item.JiraComponents)
	}
	if item.Status != "in_progress" || item.Priority != "high" || item.Source != "jira" {
		t.Fatalf("unexpected mapped fields: status=%s priority=%s source=%s", item.Status, item.Priority, item.Source)
	}
}

func TestPlanningServiceHandleJiraWebhook_UpdatePreserveExistingRelations(t *testing.T) {
	svc, _ := newPlanningServiceForTest(t)

	goalID := uint(88)
	item := &modelbiz.BizRequirement{
		ExternalKey: "DEV-99",
		Title:       "old",
		Source:      "jira",
		Status:      "backlog",
		Priority:    "medium",
		GoalID:      &goalID,
	}
	if err := svc.requirements.Create(item); err != nil {
		t.Fatalf("create requirement failed: %v", err)
	}

	payload := &JiraWebhookPayload{WebhookEvent: "jira:issue_updated"}
	payload.Issue.Key = "DEV-99"
	payload.Issue.Fields.Summary = "new title"
	payload.Issue.Fields.Status.Name = "Done"
	payload.Issue.Fields.Priority.Name = "Low"

	action, err := svc.HandleJiraWebhook(payload, nil)
	if err != nil {
		t.Fatalf("handle webhook failed: %v", err)
	}
	if action != "updated" {
		t.Fatalf("expected updated action, got %s", action)
	}

	updated, err := svc.requirements.GetByExternalKey("DEV-99")
	if err != nil {
		t.Fatalf("query requirement failed: %v", err)
	}
	if updated.GoalID == nil || *updated.GoalID != goalID {
		t.Fatalf("expected goal id preserved, got %#v", updated.GoalID)
	}
	if updated.Title != "new title" || updated.Status != "done" || updated.Priority != "low" {
		t.Fatalf("unexpected updated fields: %#v", updated)
	}
}

func TestPlanningServiceHandleJiraWebhook_Delete(t *testing.T) {
	svc, _ := newPlanningServiceForTest(t)
	if err := svc.requirements.Create(&modelbiz.BizRequirement{
		ExternalKey: "DEV-7",
		Title:       "to delete",
		Source:      "jira",
		Status:      "backlog",
		Priority:    "medium",
	}); err != nil {
		t.Fatalf("create requirement failed: %v", err)
	}

	payload := &JiraWebhookPayload{WebhookEvent: "jira:issue_deleted"}
	payload.Issue.Key = "DEV-7"
	action, err := svc.HandleJiraWebhook(payload, nil)
	if err != nil {
		t.Fatalf("delete webhook failed: %v", err)
	}
	if action != "deleted" {
		t.Fatalf("expected deleted action, got %s", action)
	}
	if _, err := svc.requirements.GetByExternalKey("DEV-7"); err == nil {
		t.Fatalf("expected record deleted")
	}
}

func TestPlanningServiceUpdateRequirement_SyncJiraTransition(t *testing.T) {
	svc, db := newPlanningServiceForTest(t)
	stub := &stubJiraSyncClient{
		transitionsResp: map[string]interface{}{
			"transitions": []interface{}{
				map[string]interface{}{
					"id":   "31",
					"name": "Done",
					"to": map[string]interface{}{
						"name": "Done",
					},
				},
			},
		},
	}
	svc.jiraSync = stub

	if err := db.Create(&models.JiraProjectMapping{
		JiraInstanceID: 2,
		JiraProjectKey: "DEV",
	}).Error; err != nil {
		t.Fatalf("create mapping failed: %v", err)
	}
	req := &modelbiz.BizRequirement{
		ExternalKey: "DEV-123",
		Title:       "need sync",
		Source:      "jira",
		Status:      "backlog",
		Priority:    "medium",
	}
	if err := svc.requirements.Create(req); err != nil {
		t.Fatalf("create requirement failed: %v", err)
	}

	updated := *req
	updated.Status = "done"
	if err := svc.UpdateRequirement(&updated); err != nil {
		t.Fatalf("update requirement failed: %v", err)
	}
	if stub.transitionCalls != 1 {
		t.Fatalf("expected one transition call, got %d", stub.transitionCalls)
	}
	if stub.commentCalls != 1 {
		t.Fatalf("expected one comment call, got %d", stub.commentCalls)
	}
	if stub.lastIssueKey != "DEV-123" || stub.lastTransition != "31" {
		t.Fatalf("unexpected sync payload: issue=%s transition=%s", stub.lastIssueKey, stub.lastTransition)
	}
}

func TestSelectJiraTransitionID(t *testing.T) {
	payload := map[string]interface{}{
		"transitions": []interface{}{
			map[string]interface{}{"id": "11", "name": "To Do", "to": map[string]interface{}{"name": "To Do"}},
			map[string]interface{}{"id": "22", "name": "Start Progress", "to": map[string]interface{}{"name": "In Progress"}},
			map[string]interface{}{"id": "33", "name": "Resolve", "to": map[string]interface{}{"name": "Done"}},
		},
	}
	if got := selectJiraTransitionID(payload, "backlog"); got != "11" {
		t.Fatalf("expected backlog transition 11, got %s", got)
	}
	if got := selectJiraTransitionID(payload, "in_progress"); got != "22" {
		t.Fatalf("expected in_progress transition 22, got %s", got)
	}
	if got := selectJiraTransitionID(payload, "done"); got != "33" {
		t.Fatalf("expected done transition 33, got %s", got)
	}
}

func TestNormalizeCSV(t *testing.T) {
	got := normalizeCSV([]string{"  backend ", "release", "BACKEND", "", "release"})
	if got != "backend,release" {
		t.Fatalf("expected normalized csv, got %s", got)
	}
}

