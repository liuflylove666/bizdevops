package release

import (
	"context"
	"testing"
	"time"

	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"
	"devops/internal/models/system"
	"devops/pkg/dto"
)

func newGateServiceForTest(t *testing.T) (*GateService, func(*deploy.Release, ...deploy.ReleaseItem)) {
	t.Helper()

	_, db := newReleaseServiceForTest(t)
	if err := db.AutoMigrate(
		&deploy.ReleaseGateResult{},
		&deploy.ApprovalInstance{},
		&deploy.EnvAuditPolicy{},
		&deploy.DeployWindow{},
		&deploy.DeployLock{},
		&system.ImageScan{},
		&infrastructure.GitOpsChangeRequest{},
		&infrastructure.ArgoCDApplication{},
	); err != nil {
		t.Fatalf("auto migrate gate models failed: %v", err)
	}

	createRelease := func(rel *deploy.Release, items ...deploy.ReleaseItem) {
		t.Helper()
		if err := db.Create(rel).Error; err != nil {
			t.Fatalf("create release failed: %v", err)
		}
		for i := range items {
			items[i].ReleaseID = rel.ID
			if err := db.Create(&items[i]).Error; err != nil {
				t.Fatalf("create release item failed: %v", err)
			}
		}
	}

	return NewGateService(db), createRelease
}

func TestGateServiceBlocksDraftWithoutItems(t *testing.T) {
	svc, createRelease := newGateServiceForTest(t)
	rel := &deploy.Release{
		Title:           "draft-release",
		ApplicationName: "demo",
		Env:             "prod",
		Status:          deploy.ReleaseStatusDraft,
		RiskLevel:       "low",
	}
	createRelease(rel)

	summary, err := svc.Evaluate(context.Background(), rel.ID, false)
	if err != nil {
		t.Fatalf("evaluate gates failed: %v", err)
	}
	if !summary.Blocked || summary.CanPublish {
		t.Fatalf("draft without items should be blocked, got blocked=%v canPublish=%v", summary.Blocked, summary.CanPublish)
	}
	if gateStatus(summary.Items, "change_items") != "block" {
		t.Fatalf("change_items should block, got %s", gateStatus(summary.Items, "change_items"))
	}
	if gateStatus(summary.Items, "approval") != "block" {
		t.Fatalf("approval should block, got %s", gateStatus(summary.Items, "approval"))
	}
}

func TestGateServiceBlocksApprovedReleaseWithoutGitOpsPRButPassesCleanScan(t *testing.T) {
	svc, createRelease := newGateServiceForTest(t)
	appID := uint(1001)
	now := time.Now()
	rel := &deploy.Release{
		Title:           "approved-release",
		ApplicationID:   &appID,
		ApplicationName: "demo",
		Env:             "test",
		Status:          deploy.ReleaseStatusApproved,
		RiskLevel:       "low",
		ApprovedAt:      &now,
	}
	createRelease(rel, deploy.ReleaseItem{ItemType: deploy.ReleaseItemTypePipelineRun, ItemID: 2001, ItemTitle: "pipeline"})
	if err := svc.db.Create(&system.ImageScan{
		ApplicationID:   &appID,
		ApplicationName: "demo",
		Image:           "demo:v1",
		Status:          "completed",
		RiskLevel:       "low",
		CreatedAt:       now,
	}).Error; err != nil {
		t.Fatalf("create image scan failed: %v", err)
	}

	summary, err := svc.Evaluate(context.Background(), rel.ID, false)
	if err != nil {
		t.Fatalf("evaluate gates failed: %v", err)
	}
	if !summary.Blocked {
		t.Fatal("approved release without GitOps PR should block publish")
	}
	if got := gateStatus(summary.Items, "security_scan"); got != "pass" {
		t.Fatalf("clean security scan should pass, got %s", got)
	}
	if got := gateStatus(summary.Items, "gitops_pr"); got != "block" {
		t.Fatalf("approved release without GitOps PR should block before publish, got %s", got)
	}
}

func TestGateServiceBlocksMergedPRUntilArgoSyncedHealthy(t *testing.T) {
	svc, createRelease := newGateServiceForTest(t)
	appID := uint(1002)
	change := &infrastructure.GitOpsChangeRequest{
		GitOpsRepoID:    1,
		ApplicationID:   &appID,
		ApplicationName: "demo",
		Env:             "prod",
		Title:           "release demo",
		FilePath:        "apps/demo/deploy.yaml",
		ImageRepository: "registry/demo",
		ImageTag:        "v1",
		Status:          "merged",
		AutoMergeStatus: "success",
	}
	if err := svc.db.Create(change).Error; err != nil {
		t.Fatalf("create gitops change failed: %v", err)
	}
	now := time.Now()
	rel := &deploy.Release{
		Title:                 "pr-merged-release",
		ApplicationID:         &appID,
		ApplicationName:       "demo",
		Env:                   "prod",
		Status:                deploy.ReleaseStatusPRMerged,
		RiskLevel:             "low",
		ApprovedAt:            &now,
		GitOpsChangeRequestID: &change.ID,
	}
	createRelease(rel, deploy.ReleaseItem{ItemType: deploy.ReleaseItemTypePipelineRun, ItemID: 2002, ItemTitle: "pipeline"})
	if err := svc.db.Create(&infrastructure.ArgoCDApplication{
		ArgoCDInstanceID: 1,
		Name:             "demo-prod",
		ApplicationID:    &appID,
		ApplicationName:  "demo",
		Env:              "prod",
		SyncStatus:       "OutOfSync",
		HealthStatus:     "Healthy",
	}).Error; err != nil {
		t.Fatalf("create argocd application failed: %v", err)
	}

	summary, err := svc.Evaluate(context.Background(), rel.ID, false)
	if err != nil {
		t.Fatalf("evaluate gates failed: %v", err)
	}
	if !summary.Blocked {
		t.Fatalf("merged PR with OutOfSync Argo app should block")
	}
	if got := gateStatus(summary.Items, "argocd_sync"); got != "block" {
		t.Fatalf("argocd_sync should block, got %s", got)
	}
}

func TestGateServicePersistsGateSnapshots(t *testing.T) {
	svc, createRelease := newGateServiceForTest(t)
	rel := &deploy.Release{
		Title:           "persist-release",
		ApplicationName: "demo",
		Env:             "dev",
		Status:          deploy.ReleaseStatusDraft,
		RiskLevel:       "low",
	}
	createRelease(rel)

	if _, err := svc.Evaluate(context.Background(), rel.ID, true); err != nil {
		t.Fatalf("evaluate gates failed: %v", err)
	}
	var count int64
	if err := svc.db.Model(&deploy.ReleaseGateResult{}).Where("release_id = ?", rel.ID).Count(&count).Error; err != nil {
		t.Fatalf("count gate snapshots failed: %v", err)
	}
	if count == 0 {
		t.Fatal("expected persisted gate snapshots")
	}
}

func gateStatus(items []dto.ReleaseGateResultDTO, key string) string {
	for _, item := range items {
		if item.Key == key {
			return item.Status
		}
	}
	return ""
}
