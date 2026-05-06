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

func newRuleServiceForTest(t *testing.T) (*RuleService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:rule_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.ComplianceRule{}); err != nil {
		t.Fatalf("auto migrate compliance_rule failed: %v", err)
	}
	return NewRuleService(db), db
}

func TestRuleServiceUpdateBuiltinOnlyTogglesEnabled(t *testing.T) {
	svc, db := newRuleServiceForTest(t)
	ctx := context.Background()

	builtin := &models.ComplianceRule{
		Name:          "builtin-rule",
		Severity:      "high",
		Category:      "security",
		CheckType:     "builtin",
		Enabled:       true,
		ConditionJSON: `{"field":"livenessProbe"}`,
	}
	if err := db.Create(builtin).Error; err != nil {
		t.Fatalf("create builtin rule failed: %v", err)
	}

	err := svc.Update(ctx, &dto.ComplianceRuleRequest{
		ID:            builtin.ID,
		Name:          "should-not-change",
		Severity:      "low",
		Category:      "other",
		Enabled:       false,
		ConditionJSON: `{"field":"image"}`,
	})
	if err != nil {
		t.Fatalf("update builtin rule failed: %v", err)
	}

	var saved models.ComplianceRule
	if err := db.First(&saved, builtin.ID).Error; err != nil {
		t.Fatalf("query updated rule failed: %v", err)
	}
	if saved.Name != "builtin-rule" {
		t.Fatalf("builtin rule name should not change, got %s", saved.Name)
	}
	if saved.Enabled {
		t.Fatal("builtin rule enabled should be toggled to false")
	}
}

func TestRuleServiceDeleteBuiltinRejectedAndToggleEnabled(t *testing.T) {
	svc, db := newRuleServiceForTest(t)
	ctx := context.Background()

	builtin := &models.ComplianceRule{
		Name:          "builtin-rule",
		Severity:      "medium",
		Category:      "security",
		CheckType:     "builtin",
		Enabled:       true,
		ConditionJSON: `{"field":"livenessProbe"}`,
	}
	custom := &models.ComplianceRule{
		Name:          "custom-rule",
		Severity:      "low",
		Category:      "resource",
		CheckType:     "custom",
		Enabled:       false,
		ConditionJSON: `{"field":"resources.limits.cpu"}`,
	}
	if err := db.Create(builtin).Error; err != nil {
		t.Fatalf("create builtin rule failed: %v", err)
	}
	if err := db.Create(custom).Error; err != nil {
		t.Fatalf("create custom rule failed: %v", err)
	}
	var before models.ComplianceRule
	if err := db.First(&before, custom.ID).Error; err != nil {
		t.Fatalf("query custom rule before toggle failed: %v", err)
	}

	if err := svc.Delete(ctx, builtin.ID); err == nil {
		t.Fatal("delete builtin rule should fail")
	}

	if err := svc.ToggleEnabled(ctx, custom.ID); err != nil {
		t.Fatalf("toggle enabled failed: %v", err)
	}
	var saved models.ComplianceRule
	if err := db.First(&saved, custom.ID).Error; err != nil {
		t.Fatalf("query toggled rule failed: %v", err)
	}
	if saved.Enabled == before.Enabled {
		t.Fatalf("custom rule enabled should be flipped, before=%v after=%v", before.Enabled, saved.Enabled)
	}
}

func TestRuleServiceListWithFilters(t *testing.T) {
	svc, db := newRuleServiceForTest(t)
	ctx := context.Background()

	seed := []models.ComplianceRule{
		{Name: "r1", Severity: "high", Category: "security", CheckType: "custom", Enabled: true, ConditionJSON: "{}"},
		{Name: "r2", Severity: "low", Category: "resource", CheckType: "custom", Enabled: false, ConditionJSON: "{}"},
		{Name: "r3", Severity: "medium", Category: "security", CheckType: "custom", Enabled: true, ConditionJSON: "{}"},
	}
	for i := range seed {
		if err := db.Create(&seed[i]).Error; err != nil {
			t.Fatalf("seed rule failed: %v", err)
		}
	}

	enabled := true
	items, err := svc.List(ctx, "security", &enabled)
	if err != nil {
		t.Fatalf("list rules failed: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("expected 2 enabled security rules, got %d", len(items))
	}
}
