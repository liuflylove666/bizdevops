package application

import (
	"context"
	"fmt"
	"testing"

	appmodel "devops/internal/models/application"
	"devops/internal/models/deploy"
	"devops/internal/models/infrastructure"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newReadinessServiceForTest(t *testing.T) (*ReadinessService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(
		&appmodel.Application{},
		&appmodel.ApplicationRepoBinding{},
		&appmodel.ApplicationEnv{},
		&appmodel.ApplicationReadinessCheck{},
		&deploy.Pipeline{},
		&deploy.ApprovalRule{},
		&deploy.EnvAuditPolicy{},
		&deploy.DeployWindow{},
		&infrastructure.GitOpsRepo{},
		&infrastructure.ArgoCDApplication{},
	); err != nil {
		t.Fatalf("auto migrate readiness models failed: %v", err)
	}
	return NewReadinessService(db), db
}

func TestReadinessServiceReportsMissingChecksForEmptyApp(t *testing.T) {
	svc, db := newReadinessServiceForTest(t)
	app := appmodel.Application{Name: "demo"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app failed: %v", err)
	}

	result, err := svc.Get(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("get readiness failed: %v", err)
	}
	if result.Score >= 40 {
		t.Fatalf("empty app should have low readiness score, got %d", result.Score)
	}
	if result.Completed != 0 || len(result.NextActions) == 0 {
		t.Fatalf("empty app should have missing actions, completed=%d actions=%d", result.Completed, len(result.NextActions))
	}
}

func TestReadinessServiceReportsReadyForCompleteApp(t *testing.T) {
	svc, db := newReadinessServiceForTest(t)
	app := appmodel.Application{Name: "demo", DisplayName: "Demo", Team: "platform", Owner: "alice", GitRepo: "https://git.example/demo.git"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app failed: %v", err)
	}
	clusterID := uint(1)
	appID := app.ID
	records := []any{
		&appmodel.ApplicationEnv{ApplicationID: app.ID, EnvName: "prod", K8sClusterID: &clusterID, K8sNamespace: "demo", K8sDeployment: "demo-api"},
		&deploy.Pipeline{Name: "demo-ci", ApplicationID: &appID, ApplicationName: app.Name, ConfigJSON: "{}"},
		&infrastructure.GitOpsRepo{Name: "demo-gitops", RepoURL: "https://git.example/gitops.git", ApplicationID: &appID, ApplicationName: app.Name},
		&infrastructure.ArgoCDApplication{ArgoCDInstanceID: 1, Name: "demo-prod", ApplicationID: &appID, ApplicationName: app.Name},
		&deploy.ApprovalRule{AppID: app.ID, Env: "prod", Enabled: true, NeedApproval: true},
		&deploy.EnvAuditPolicy{EnvName: "prod", Enabled: true, RequireApproval: true},
	}
	for _, record := range records {
		if err := db.Create(record).Error; err != nil {
			t.Fatalf("create readiness dependency failed: %v", err)
		}
	}

	result, err := svc.Get(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("get readiness failed: %v", err)
	}
	if result.Score != 100 || result.Level != "ready" {
		t.Fatalf("complete app should be ready, got score=%d level=%s", result.Score, result.Level)
	}
	if result.Completed != result.Total {
		t.Fatalf("all checks should pass, completed=%d total=%d", result.Completed, result.Total)
	}
}

func TestReadinessServiceRefreshPersistsLatestSnapshots(t *testing.T) {
	svc, db := newReadinessServiceForTest(t)
	app := appmodel.Application{Name: "demo"}
	if err := db.Create(&app).Error; err != nil {
		t.Fatalf("create app failed: %v", err)
	}

	first, err := svc.Refresh(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("refresh readiness failed: %v", err)
	}
	var count int64
	if err := db.Model(&appmodel.ApplicationReadinessCheck{}).Where("application_id = ?", app.ID).Count(&count).Error; err != nil {
		t.Fatalf("count readiness snapshots failed: %v", err)
	}
	if count != int64(first.Total) {
		t.Fatalf("expected %d persisted snapshots, got %d", first.Total, count)
	}

	if err := db.Model(&appmodel.Application{}).Where("id = ?", app.ID).Updates(map[string]any{"team": "platform", "owner": "alice"}).Error; err != nil {
		t.Fatalf("update app failed: %v", err)
	}
	second, err := svc.Refresh(context.Background(), app.ID)
	if err != nil {
		t.Fatalf("second refresh readiness failed: %v", err)
	}
	if err := db.Model(&appmodel.ApplicationReadinessCheck{}).Where("application_id = ?", app.ID).Count(&count).Error; err != nil {
		t.Fatalf("count readiness snapshots after refresh failed: %v", err)
	}
	if count != int64(second.Total) {
		t.Fatalf("refresh should upsert snapshots, expected %d rows got %d", second.Total, count)
	}

	var profile appmodel.ApplicationReadinessCheck
	if err := db.Where("application_id = ? AND check_key = ?", app.ID, "profile").First(&profile).Error; err != nil {
		t.Fatalf("load profile snapshot failed: %v", err)
	}
	if profile.Status != "pass" || profile.Score != second.Score {
		t.Fatalf("profile snapshot was not updated, status=%s score=%d want score=%d", profile.Status, profile.Score, second.Score)
	}
}
