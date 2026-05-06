package security

import (
	"testing"

	"devops/internal/models"
	"devops/pkg/dto"
)

func TestSplitImageNameAndTag(t *testing.T) {
	cases := []struct {
		image    string
		name     string
		tag      string
	}{
		{"", "", ""},
		{"nginx", "nginx", ""},
		{"nginx:1.25", "nginx", "1.25"},
		{"registry.example.com/app:v2", "registry.example.com/app", "v2"},
		{"registry.example.com:5000/app:v2", "registry.example.com:5000/app", "v2"},
	}
	for _, c := range cases {
		name, tag := splitImageNameAndTag(c.image)
		if name != c.name || tag != c.tag {
			t.Fatalf("splitImageNameAndTag(%q) got (%q,%q), want (%q,%q)", c.image, name, tag, c.name, c.tag)
		}
	}
}

func TestDeriveAssociationSource(t *testing.T) {
	req := &dto.ScanHistoryRequest{PipelineRunID: 10}
	appID := uint(99)
	scanWithAppID := &models.ImageScan{ApplicationID: &appID, Image: "repo/app:v1"}
	if got := deriveAssociationSource(scanWithAppID, req, appID, "", nil); got != "application_id" {
		t.Fatalf("expected application_id source, got %s", got)
	}

	scanWithName := &models.ImageScan{ApplicationName: "demo", Image: "repo/demo:v1"}
	if got := deriveAssociationSource(scanWithName, req, 0, "demo", nil); got != "application_name" {
		t.Fatalf("expected application_name source, got %s", got)
	}

	pipelineID := uint(10)
	scanWithPipeline := &models.ImageScan{PipelineRunID: &pipelineID, Image: "repo/other:v1"}
	if got := deriveAssociationSource(scanWithPipeline, req, 0, "", nil); got != "pipeline_run" {
		t.Fatalf("expected pipeline_run source, got %s", got)
	}

	scanLegacy := &models.ImageScan{Image: "registry/team-service:v2"}
	if got := deriveAssociationSource(scanLegacy, req, 0, "", []string{"team-service"}); got != "legacy_image_keyword" {
		t.Fatalf("expected legacy_image_keyword source, got %s", got)
	}
}
