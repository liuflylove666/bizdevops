package security

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
)

func newAuditLoggerServiceForTest(t *testing.T) (*AuditLoggerService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:audit_logger_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.SecurityAuditLog{}); err != nil {
		t.Fatalf("auto migrate security_audit_logs failed: %v", err)
	}
	svc := NewAuditLoggerService(db)
	t.Cleanup(func() { svc.Stop() })
	return svc, db
}

func seedAuditLogs(t *testing.T, db *gorm.DB) {
	t.Helper()
	uid1 := uint(101)
	uid2 := uint(202)
	cid1 := uint(1)
	cid2 := uint(2)
	now := time.Now()
	logs := []models.SecurityAuditLog{
		{
			UserID: &uid1, Username: "alice", Action: "scan",
			ResourceType: "image", ResourceName: "repo/app:v1", ClusterID: &cid1, ClusterName: "cluster-a",
			Result: "success", ClientIP: "10.0.0.1", CreatedAt: now.Add(-2 * time.Hour),
		},
		{
			UserID: &uid2, Username: "bob", Action: "deploy",
			ResourceType: "config", ResourceName: "cm-prod", ClusterID: &cid2, ClusterName: "cluster-b",
			Result: "failed", ClientIP: "10.0.0.2", CreatedAt: now.Add(-1 * time.Hour),
		},
	}
	for i := range logs {
		if err := db.Create(&logs[i]).Error; err != nil {
			t.Fatalf("seed audit log failed: %v", err)
		}
	}
}

func TestAuditLoggerListWithFilters(t *testing.T) {
	svc, db := newAuditLoggerServiceForTest(t)
	seedAuditLogs(t, db)

	resp, err := svc.List(context.Background(), &dto.AuditLogRequest{
		Action:   "scan",
		ClusterID: 1,
		Page:     1,
		PageSize: 10,
	})
	if err != nil {
		t.Fatalf("list audit logs failed: %v", err)
	}
	if resp.Total != 1 || len(resp.Items) != 1 {
		t.Fatalf("expected exactly one log item, got total=%d items=%d", resp.Total, len(resp.Items))
	}
	if resp.Items[0].Action != "scan" || resp.Items[0].ClusterName != "cluster-a" {
		t.Fatalf("unexpected log item: %+v", resp.Items[0])
	}
}

func TestAuditLoggerExportJSONAndCSV(t *testing.T) {
	svc, db := newAuditLoggerServiceForTest(t)
	seedAuditLogs(t, db)

	jsonData, jsonType, err := svc.Export(context.Background(), &dto.AuditLogRequest{}, "json")
	if err != nil {
		t.Fatalf("export json failed: %v", err)
	}
	if jsonType != "application/json" {
		t.Fatalf("json content type mismatch, got %s", jsonType)
	}
	if !strings.Contains(string(jsonData), `"username": "alice"`) {
		t.Fatalf("json export should contain seeded data, got %s", string(jsonData))
	}

	csvData, csvType, err := svc.Export(context.Background(), &dto.AuditLogRequest{}, "csv")
	if err != nil {
		t.Fatalf("export csv failed: %v", err)
	}
	if csvType != "text/csv" {
		t.Fatalf("csv content type mismatch, got %s", csvType)
	}
	if !strings.Contains(string(csvData), "用户,操作,资源类型") {
		t.Fatalf("csv export should contain header, got %s", string(csvData))
	}
}

func TestAuditLoggerExportUnsupportedFormat(t *testing.T) {
	svc, _ := newAuditLoggerServiceForTest(t)
	if _, _, err := svc.Export(context.Background(), &dto.AuditLogRequest{}, "xml"); err == nil {
		t.Fatal("unsupported format should return error")
	}
}
