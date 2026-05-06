package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
)

func newSecurityServiceForTest(t *testing.T) (*SecurityService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:security_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.ImageScan{}, &models.ConfigCheck{}); err != nil {
		t.Fatalf("auto migrate security models failed: %v", err)
	}
	return NewSecurityService(db), db
}

func TestSecurityServiceGetOverviewClusterFilter(t *testing.T) {
	svc, db := newSecurityServiceForTest(t)
	now := time.Now()
	recent := now.Add(-24 * time.Hour)
	old := now.AddDate(0, 0, -40)

	// 仅 recent + completed 参与统计
	seedScans := []models.ImageScan{
		{Image: "repo/app:v1", Status: "completed", CriticalCount: 1, HighCount: 2, MediumCount: 3, LowCount: 4, ScannedAt: &recent},
		{Image: "repo/app:v2", Status: "failed", CriticalCount: 9, ScannedAt: &recent},
		{Image: "repo/app:v0", Status: "completed", CriticalCount: 9, ScannedAt: &old},
	}
	for i := range seedScans {
		if err := db.Create(&seedScans[i]).Error; err != nil {
			t.Fatalf("seed image_scan failed: %v", err)
		}
	}

	seedChecks := []models.ConfigCheck{
		{ClusterID: 1, Status: "completed", CriticalCount: 1, HighCount: 1, MediumCount: 1, LowCount: 1, PassedCount: 2, CheckedAt: &recent},
		{ClusterID: 2, Status: "completed", CriticalCount: 2, HighCount: 0, MediumCount: 0, LowCount: 0, PassedCount: 1, CheckedAt: &recent},
		{ClusterID: 1, Status: "running", CriticalCount: 9, CheckedAt: &recent},
		{ClusterID: 1, Status: "completed", CriticalCount: 9, CheckedAt: &old},
	}
	for i := range seedChecks {
		if err := db.Create(&seedChecks[i]).Error; err != nil {
			t.Fatalf("seed config_check failed: %v", err)
		}
	}

	overview, err := svc.GetOverview(context.Background(), 1)
	if err != nil {
		t.Fatalf("get overview failed: %v", err)
	}
	if overview.VulnSummary.Total != 10 {
		t.Fatalf("vuln summary total mismatch, got %d", overview.VulnSummary.Total)
	}
	if overview.ConfigSummary.Critical != 1 || overview.ConfigSummary.Total != 4 {
		t.Fatalf("config summary should only include cluster 1 recent completed, got %+v", overview.ConfigSummary)
	}
	if len(overview.TrendData) != 7 {
		t.Fatalf("trend data should include 7 days, got %d", len(overview.TrendData))
	}
}

func TestSecurityServiceScoreAndRiskLevelBoundaries(t *testing.T) {
	svc, _ := newSecurityServiceForTest(t)

	score := svc.calculateSecurityScore(
		dto.VulnSummary{Critical: 10, High: 10, Medium: 10, Low: 10},
		dto.ConfigCheckSummary{Critical: 10, High: 10, Medium: 10, Low: 10},
	)
	if score != 0 {
		t.Fatalf("score should floor at 0, got %d", score)
	}

	cases := []struct {
		score int
		level string
	}{
		{95, "low"},
		{80, "medium"},
		{60, "high"},
		{30, "critical"},
	}
	for _, c := range cases {
		if got := svc.getRiskLevel(c.score); got != c.level {
			t.Fatalf("getRiskLevel(%d)=%s, want %s", c.score, got, c.level)
		}
	}
}
