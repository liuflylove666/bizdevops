package application

import (
	"context"
	"testing"

	appmodel "devops/internal/models/application"
	"devops/internal/models/deploy"
	"devops/pkg/dto"
)

func TestOnboardingServiceCreatesDeliveryChainAndReadinessSnapshot(t *testing.T) {
	_, db := newReadinessServiceForTest(t)
	if err := db.AutoMigrate(&deploy.GitRepository{}); err != nil {
		t.Fatalf("auto migrate git repository failed: %v", err)
	}
	svc := NewOnboardingService(db)
	clusterID := uint(1)

	result, err := svc.Save(context.Background(), &dto.ApplicationOnboardingRequest{
		App: dto.ApplicationOnboardingAppInput{
			Name:        "demo-api",
			DisplayName: "Demo API",
			Language:    "go",
			Team:        "platform",
			Owner:       "alice",
		},
		Repo: &dto.ApplicationOnboardingRepoInput{
			URL:           "https://git.example/demo-api.git",
			Provider:      "gitlab",
			DefaultBranch: "main",
		},
		Env: &dto.ApplicationOnboardingEnvInput{
			EnvName:       "test",
			K8sClusterID:  &clusterID,
			K8sNamespace:  "demo",
			K8sDeployment: "demo-api",
			Replicas:      2,
		},
		Pipeline: &dto.ApplicationOnboardingPipelineInput{
			Create: true,
		},
	}, 7)
	if err != nil {
		t.Fatalf("save onboarding failed: %v", err)
	}
	if !result.Created || result.ApplicationID == 0 {
		t.Fatalf("expected created application, got created=%v id=%d", result.Created, result.ApplicationID)
	}
	if result.GitRepoID == nil || result.RepoBindingID == nil || result.EnvID == nil || result.PipelineID == nil {
		t.Fatalf("expected full chain ids, got repo=%v binding=%v env=%v pipeline=%v", result.GitRepoID, result.RepoBindingID, result.EnvID, result.PipelineID)
	}
	if result.Readiness == nil || result.Readiness.Score < 70 {
		t.Fatalf("expected refreshed readiness score >= 70, got %#v", result.Readiness)
	}

	var snapshotCount int64
	if err := db.Model(&appmodel.ApplicationReadinessCheck{}).Where("application_id = ?", result.ApplicationID).Count(&snapshotCount).Error; err != nil {
		t.Fatalf("count readiness snapshots failed: %v", err)
	}
	if snapshotCount != int64(result.Readiness.Total) {
		t.Fatalf("expected %d readiness snapshots, got %d", result.Readiness.Total, snapshotCount)
	}
}

func TestOnboardingServiceUpsertsExistingDeliveryChain(t *testing.T) {
	_, db := newReadinessServiceForTest(t)
	if err := db.AutoMigrate(&deploy.GitRepository{}); err != nil {
		t.Fatalf("auto migrate git repository failed: %v", err)
	}
	svc := NewOnboardingService(db)

	req := &dto.ApplicationOnboardingRequest{
		App: dto.ApplicationOnboardingAppInput{
			Name:  "demo-api",
			Team:  "platform",
			Owner: "alice",
		},
		Repo: &dto.ApplicationOnboardingRepoInput{
			URL:           "https://git.example/demo-api.git",
			DefaultBranch: "main",
		},
		Env: &dto.ApplicationOnboardingEnvInput{
			EnvName:       "test",
			K8sNamespace:  "demo",
			K8sDeployment: "demo-api",
			Replicas:      1,
		},
		Pipeline: &dto.ApplicationOnboardingPipelineInput{Create: true},
	}
	first, err := svc.Save(context.Background(), req, 7)
	if err != nil {
		t.Fatalf("first onboarding save failed: %v", err)
	}

	req.ApplicationID = &first.ApplicationID
	req.Env.Replicas = 3
	second, err := svc.Save(context.Background(), req, 7)
	if err != nil {
		t.Fatalf("second onboarding save failed: %v", err)
	}
	if second.Created {
		t.Fatal("second onboarding save should update existing app")
	}

	var repoBindings, envs, pipelines int64
	if err := db.Model(&appmodel.ApplicationRepoBinding{}).Where("application_id = ?", first.ApplicationID).Count(&repoBindings).Error; err != nil {
		t.Fatalf("count repo bindings failed: %v", err)
	}
	if err := db.Model(&appmodel.ApplicationEnv{}).Where("app_id = ?", first.ApplicationID).Count(&envs).Error; err != nil {
		t.Fatalf("count envs failed: %v", err)
	}
	if err := db.Model(&deploy.Pipeline{}).Where("application_id = ?", first.ApplicationID).Count(&pipelines).Error; err != nil {
		t.Fatalf("count pipelines failed: %v", err)
	}
	if repoBindings != 1 || envs != 1 || pipelines != 1 {
		t.Fatalf("expected upserted rows, got bindings=%d envs=%d pipelines=%d", repoBindings, envs, pipelines)
	}

	var env appmodel.ApplicationEnv
	if err := db.Where("app_id = ? AND env_name = ?", first.ApplicationID, "test").First(&env).Error; err != nil {
		t.Fatalf("load env failed: %v", err)
	}
	if env.Replicas != 3 {
		t.Fatalf("expected env replicas updated to 3, got %d", env.Replicas)
	}
}
