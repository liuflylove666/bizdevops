package release

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	modelbiz "devops/internal/models/biz"
	"devops/internal/models/deploy"
	appRepo "devops/internal/modules/application/repository"
)

func newReleaseServiceForTest(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:release_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&deploy.Release{}, &deploy.ReleaseItem{}, &deploy.NacosRelease{}, &modelbiz.BizGoal{}, &modelbiz.BizVersion{}); err != nil {
		t.Fatalf("auto migrate release models failed: %v", err)
	}

	repo := appRepo.NewReleaseRepository(db)
	itemRepo := appRepo.NewReleaseItemRepository(db)
	nrRepo := appRepo.NewNacosReleaseRepository(db)
	svc := NewService(db, repo, itemRepo, nrRepo, nil).WithRiskScorer(NewRiskScorer())
	return svc, db
}

func mustCreateDraftRelease(t *testing.T, svc *Service, env string) *deploy.Release {
	t.Helper()

	rel := &deploy.Release{
		Title:           "release-test",
		ApplicationName: "demo-app",
		Env:             env,
		RolloutStrategy: deploy.RolloutStrategyDirect,
	}
	if err := svc.Create(rel); err != nil {
		t.Fatalf("create release failed: %v", err)
	}
	return rel
}

func TestReleaseServiceSubmitForApprovalAppliesRiskScore(t *testing.T) {
	svc, _ := newReleaseServiceForTest(t)
	rel := mustCreateDraftRelease(t, svc, "prod")

	if err := svc.AddItem(rel.ID, deploy.ReleaseItemTypeDatabase, 1, "db-change"); err != nil {
		t.Fatalf("add release item failed: %v", err)
	}

	updated, err := svc.SubmitForApproval(rel.ID)
	if err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}
	if updated.Status != deploy.ReleaseStatusPendingApproval {
		t.Fatalf("status should be pending_approval, got %s", updated.Status)
	}
	if updated.RiskScore <= 0 {
		t.Fatalf("risk score should be greater than 0, got %d", updated.RiskScore)
	}
	if len(updated.RiskFactors) == 0 {
		t.Fatal("risk factors should be populated")
	}
}

func TestReleaseServiceSubmitForApprovalWithoutItemsFails(t *testing.T) {
	svc, _ := newReleaseServiceForTest(t)
	rel := mustCreateDraftRelease(t, svc, "dev")

	_, err := svc.SubmitForApproval(rel.ID)
	if err == nil {
		t.Fatal("expected error when submit release without items")
	}
}

func TestReleaseServiceApproveAndPublishFlow(t *testing.T) {
	svc, _ := newReleaseServiceForTest(t)
	rel := mustCreateDraftRelease(t, svc, "prod")
	if err := svc.AddItem(rel.ID, deploy.ReleaseItemTypeDeployment, 2, "deploy-item"); err != nil {
		t.Fatalf("add release item failed: %v", err)
	}
	if _, err := svc.SubmitForApproval(rel.ID); err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}

	approved, err := svc.Approve(rel.ID, 1001, "approver")
	if err != nil {
		t.Fatalf("approve failed: %v", err)
	}
	if approved.Status != deploy.ReleaseStatusApproved {
		t.Fatalf("status should be approved, got %s", approved.Status)
	}
	if approved.ApprovedBy == nil || *approved.ApprovedBy != 1001 {
		t.Fatalf("approved_by should be 1001, got %+v", approved.ApprovedBy)
	}

	published, err := svc.Publish(rel.ID, 2002, "publisher")
	if err != nil {
		t.Fatalf("publish failed: %v", err)
	}
	if published.Status != deploy.ReleaseStatusPublished {
		t.Fatalf("status should be published, got %s", published.Status)
	}
	if published.PublishedBy == nil || *published.PublishedBy != 2002 {
		t.Fatalf("published_by should be 2002, got %+v", published.PublishedBy)
	}
}

func TestReleaseServiceRejectFlow(t *testing.T) {
	svc, _ := newReleaseServiceForTest(t)
	rel := mustCreateDraftRelease(t, svc, "prod")
	if err := svc.AddItem(rel.ID, deploy.ReleaseItemTypeNacosRelease, 3, "nacos-item"); err != nil {
		t.Fatalf("add release item failed: %v", err)
	}
	if _, err := svc.SubmitForApproval(rel.ID); err != nil {
		t.Fatalf("submit for approval failed: %v", err)
	}

	rejected, err := svc.Reject(rel.ID, 3003, "reviewer", "need fix")
	if err != nil {
		t.Fatalf("reject failed: %v", err)
	}
	if rejected.Status != deploy.ReleaseStatusRejected {
		t.Fatalf("status should be rejected, got %s", rejected.Status)
	}
	if rejected.RejectReason != "need fix" {
		t.Fatalf("reject reason mismatch, got %s", rejected.RejectReason)
	}
}

func TestReleaseServiceGetByIDLoadsNacosAndBizLinks(t *testing.T) {
	svc, db := newReleaseServiceForTest(t)
	rel := mustCreateDraftRelease(t, svc, "prod")

	nr := &deploy.NacosRelease{
		Title:           "nr-1",
		NacosInstanceID: 1,
		Group:           "DEFAULT_GROUP",
		DataID:          "app.yaml",
		Env:             "prod",
		Status:          "draft",
	}
	if err := db.Create(nr).Error; err != nil {
		t.Fatalf("create nacos release failed: %v", err)
	}
	if err := svc.AddItem(rel.ID, deploy.ReleaseItemTypeNacosRelease, nr.ID, "nacos-release-item"); err != nil {
		t.Fatalf("add nacos release item failed: %v", err)
	}

	goal := &modelbiz.BizGoal{Name: "goal-1"}
	if err := db.Create(goal).Error; err != nil {
		t.Fatalf("create biz goal failed: %v", err)
	}
	ver := &modelbiz.BizVersion{
		Name:      "v1.0",
		ReleaseID: &rel.ID,
		GoalID:    &goal.ID,
	}
	if err := db.Create(ver).Error; err != nil {
		t.Fatalf("create biz version failed: %v", err)
	}

	got, err := svc.GetByID(rel.ID)
	if err != nil {
		t.Fatalf("get release by id failed: %v", err)
	}
	if len(got.NacosReleases) != 1 || got.NacosReleases[0].ID != nr.ID {
		t.Fatalf("expected 1 nacos release association, got %+v", got.NacosReleases)
	}
	if got.BizVersionID == nil || *got.BizVersionID != ver.ID {
		t.Fatalf("biz version link missing, got %+v", got.BizVersionID)
	}
	if got.BizGoalName != "goal-1" {
		t.Fatalf("biz goal name mismatch, got %s", got.BizGoalName)
	}
}

func TestReleaseServiceListFillsBizLinks(t *testing.T) {
	svc, db := newReleaseServiceForTest(t)
	relWithBiz := mustCreateDraftRelease(t, svc, "prod")
	relWithoutBiz := mustCreateDraftRelease(t, svc, "dev")

	goal := &modelbiz.BizGoal{Name: "growth-goal"}
	if err := db.Create(goal).Error; err != nil {
		t.Fatalf("create biz goal failed: %v", err)
	}
	ver := &modelbiz.BizVersion{
		Name:      "v2.0",
		ReleaseID: &relWithBiz.ID,
		GoalID:    &goal.ID,
	}
	if err := db.Create(ver).Error; err != nil {
		t.Fatalf("create biz version failed: %v", err)
	}

	list, total, err := svc.List(appRepo.ReleaseFilter{}, 1, 20)
	if err != nil {
		t.Fatalf("list release failed: %v", err)
	}
	if total != 2 {
		t.Fatalf("expected total=2, got %d", total)
	}

	var foundWithBiz, foundWithoutBiz bool
	for _, item := range list {
		switch item.ID {
		case relWithBiz.ID:
			foundWithBiz = true
			if item.BizVersionID == nil || *item.BizVersionID != ver.ID {
				t.Fatalf("release with biz should have BizVersionID=%d, got %+v", ver.ID, item.BizVersionID)
			}
			if item.BizGoalName != "growth-goal" {
				t.Fatalf("release with biz should have BizGoalName, got %s", item.BizGoalName)
			}
		case relWithoutBiz.ID:
			foundWithoutBiz = true
			if item.BizVersionID != nil {
				t.Fatalf("release without biz should not have BizVersionID, got %+v", item.BizVersionID)
			}
		}
	}
	if !foundWithBiz || !foundWithoutBiz {
		t.Fatalf("expected both releases in list, foundWithBiz=%v foundWithoutBiz=%v", foundWithBiz, foundWithoutBiz)
	}
}

func TestReleaseServiceGetByIDNotFound(t *testing.T) {
	svc, _ := newReleaseServiceForTest(t)
	if _, err := svc.GetByID(999999); err == nil {
		t.Fatal("expected not found error for missing release")
	}
}
