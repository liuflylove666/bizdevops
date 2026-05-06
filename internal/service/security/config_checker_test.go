package security

import (
	"context"
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"devops/internal/models"
)

func newConfigCheckerServiceForTest(t *testing.T) (*ConfigCheckerService, *gorm.DB) {
	t.Helper()

	dsn := fmt.Sprintf("file:config_checker_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite failed: %v", err)
	}
	if err := db.AutoMigrate(&models.ConfigCheck{}); err != nil {
		t.Fatalf("auto migrate config_checks failed: %v", err)
	}
	// GetCheckHistory 会查 k8s_clusters 表，这里创建一个最小结构用于测试。
	if err := db.Exec(`CREATE TABLE k8s_clusters (id INTEGER PRIMARY KEY, name TEXT)`).Error; err != nil {
		t.Fatalf("create k8s_clusters table failed: %v", err)
	}
	if err := db.Exec(`INSERT INTO k8s_clusters (id, name) VALUES (1, 'cluster-1'), (2, 'cluster-2')`).Error; err != nil {
		t.Fatalf("seed k8s_clusters failed: %v", err)
	}
	return &ConfigCheckerService{db: db}, db
}

func boolPtr(v bool) *bool { return &v }

func TestConfigCheckerApplyRuleAndPodFilter(t *testing.T) {
	svc := &ConfigCheckerService{}
	container := corev1.Container{
		Name:  "app",
		Image: "demo:latest",
	}

	runAsNonRootRule := models.ComplianceRule{
		ID:            1,
		Name:          "run-as-non-root",
		Severity:      "high",
		Description:   "must run as non root",
		Remediation:   "set securityContext.runAsNonRoot=true",
		ConditionJSON: `{"field":"securityContext.runAsNonRoot"}`,
	}
	issue := svc.applyRule(container, "prod", "Pod", "pod-1", runAsNonRootRule)
	if issue == nil || issue.RuleID != 1 || issue.ResourceKind != "Pod" {
		t.Fatalf("runAsNonRoot rule should be violated, got %+v", issue)
	}

	privRule := models.ComplianceRule{
		ID:            2,
		Name:          "privileged-disabled",
		Severity:      "critical",
		ConditionJSON: `{"field":"securityContext.privileged"}`,
	}
	container.SecurityContext = &corev1.SecurityContext{Privileged: boolPtr(true)}
	if svc.applyRule(container, "prod", "Pod", "pod-1", privRule) == nil {
		t.Fatal("privileged=true should violate securityContext.privileged rule")
	}

	imageTagRule := models.ComplianceRule{
		ID:            3,
		Name:          "ban-latest",
		Severity:      "medium",
		ConditionJSON: `{"field":"image","operator":"endsWith","value":":latest"}`,
	}
	if svc.applyRule(container, "prod", "Pod", "pod-1", imageTagRule) == nil {
		t.Fatal("image ending with :latest should violate image endsWith rule")
	}

	// ReplicaSet owner 的 Pod 应跳过检查。
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "pod-rs",
			Namespace: "prod",
			OwnerReferences: []metav1.OwnerReference{
				{Kind: "ReplicaSet"},
			},
		},
		Spec: corev1.PodSpec{Containers: []corev1.Container{container}},
	}
	if issues := svc.checkPod(pod, []models.ComplianceRule{runAsNonRootRule, privRule}); len(issues) != 0 {
		t.Fatalf("pod owned by ReplicaSet should be skipped, got issues=%d", len(issues))
	}
}

func TestConfigCheckerApplyRuleResourceLimitsAndResultParsing(t *testing.T) {
	svc, db := newConfigCheckerServiceForTest(t)

	container := corev1.Container{
		Name: "app",
		Resources: corev1.ResourceRequirements{
			Limits: corev1.ResourceList{
				corev1.ResourceCPU: resource.MustParse("500m"),
				// memory 故意不设置，触发规则命中
			},
		},
	}
	memRule := models.ComplianceRule{
		ID:            11,
		Name:          "memory-limit-required",
		Severity:      "high",
		Description:   "memory limit required",
		Remediation:   "set resources.limits.memory",
		ConditionJSON: `{"field":"resources.limits.memory"}`,
	}
	if svc.applyRule(container, "prod", "Deployment", "app", memRule) == nil {
		t.Fatal("missing memory limit should violate rule")
	}

	now := time.Now()
	resultJSON := `[{"rule_id":11,"rule_name":"memory-limit-required","severity":"high","resource_kind":"Deployment","resource_name":"app","namespace":"prod","message":"memory limit required","remediation":"set resources.limits.memory"}]`
	check := &models.ConfigCheck{
		ClusterID:     1,
		Namespace:     "prod",
		Status:        "completed",
		CriticalCount: 0,
		HighCount:     1,
		MediumCount:   0,
		LowCount:      0,
		PassedCount:   2,
		ResultJSON:    resultJSON,
		CheckedAt:     &now,
	}
	if err := db.Create(check).Error; err != nil {
		t.Fatalf("create config check failed: %v", err)
	}

	got, err := svc.GetCheckResult(context.Background(), check.ID)
	if err != nil {
		t.Fatalf("get check result failed: %v", err)
	}
	if got.Summary.High != 1 || len(got.Issues) != 1 {
		t.Fatalf("unexpected parsed check result: %+v", got)
	}

	history, err := svc.GetCheckHistory(context.Background(), 1, 1, 10)
	if err != nil {
		t.Fatalf("get check history failed: %v", err)
	}
	if history.Total != 1 || len(history.Items) != 1 {
		t.Fatalf("unexpected history result: total=%d items=%d", history.Total, len(history.Items))
	}
	if history.Items[0].ClusterName != "cluster-1" {
		t.Fatalf("cluster name should be resolved, got %s", history.Items[0].ClusterName)
	}
}
