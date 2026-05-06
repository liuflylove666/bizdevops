package pipeline

import (
	"strings"
	"testing"

	"devops/pkg/dto"
)

func TestBuildGitLabCIYAMLGeneratesRunnerDockerJob(t *testing.T) {
	req := &dto.PipelineRequest{
		Name:      "order-api",
		GitBranch: "main",
		Stages: []dto.Stage{
			{
				ID:   "t1f0o2mn",
				Name: "代码检出",
				Steps: []dto.Step{
					{
						ID:   "randomgit",
						Name: "Git Clone",
						Type: "git",
					},
				},
			},
			{
				ID:   "uihmne1f",
				Name: "单元测试",
				Steps: []dto.Step{
					{
						ID:   "ei1iknoe",
						Name: "Go Test",
						Type: "container",
						Config: map[string]interface{}{
							"image":    "golang:1.25-alpine",
							"commands": []interface{}{"go test ./..."},
						},
					},
				},
			},
			{
				ID:   "4vkpnisy",
				Name: "镜像构建与推送",
				Steps: []dto.Step{
					{
						ID:   "vcmc5hxn",
						Name: "Docker Build",
						Type: "docker_build",
						Config: map[string]interface{}{
							"dockerfile": "Dockerfile",
							"context":    ".",
						},
					},
					{
						ID:   "s9r8p7q6",
						Name: "Docker Push",
						Type: "docker_push",
					},
				},
			},
		},
		Variables: []dto.Variable{
			{Name: "APP_LANGUAGE", Value: "go"},
			{Name: "TOKEN", Value: "secret", IsSecret: true},
		},
	}

	got := buildGitLabCIYAML(nil, req)

	for _, want := range []string{
		gitLabProvisioningLabel,
		"docker:26-dind",
		"cat > .jeridevops.Dockerfile <<'JERIDEVOPS_DOCKERFILE'",
		"docker build --pull",
		"docker push $DOCKER_IMAGE",
		"go test ./...",
		"- test",
		"- image",
		"go_test:",
		"build_and_push_image:",
		`IMAGE_NAME: "localhost:5001/jeridevops/order-api"`,
		`GITOPS_IMAGE_REPOSITORY: "localhost:5001/jeridevops/order-api"`,
		`DOCKER_IMAGE: "$IMAGE_NAME:$IMAGE_TAG"`,
		"APP_LANGUAGE: go",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated yaml missing %q:\n%s", want, got)
		}
	}
	if strings.Contains(got, "TOKEN") {
		t.Fatalf("secret variable leaked into generated yaml:\n%s", got)
	}
	if count := strings.Count(got, "docker build --pull"); count != 1 {
		t.Fatalf("expected one docker build job, got %d:\n%s", count, got)
	}
	if strings.Contains(got, "-f Dockerfile") {
		t.Fatalf("generated yaml should not build with repository Dockerfile:\n%s", got)
	}
	for _, randomID := range []string{"t1f0o2mn", "uihmne1f", "4vkpnisy", "ei1iknoe", "vcmc5hxn"} {
		if strings.Contains(got, randomID) {
			t.Fatalf("generated yaml should not expose random client id %q:\n%s", randomID, got)
		}
	}
	if strings.Contains(got, "- checkout") || strings.Contains(got, "git clone") {
		t.Fatalf("generated yaml should not include synthetic checkout stage/job:\n%s", got)
	}
}

func TestBuildGitLabCIYAMLUsesConfiguredImageName(t *testing.T) {
	req := &dto.PipelineRequest{
		Name: "order-api",
		Stages: []dto.Stage{{
			ID: "docker",
			Steps: []dto.Step{{
				ID:   "docker-build",
				Type: "docker_build",
			}},
		}},
		Variables: []dto.Variable{
			{Name: "IMAGE_NAME", Value: "registry.internal/team/order-api"},
		},
	}

	got := buildGitLabCIYAML(nil, req)
	if !containsAny(got, `IMAGE_NAME: "registry.internal/team/order-api"`, "IMAGE_NAME: registry.internal/team/order-api") {
		t.Fatalf("expected configured IMAGE_NAME to be preserved, got:\n%s", got)
	}
	if !containsAny(got, `GITOPS_IMAGE_REPOSITORY: "registry.internal/team/order-api"`, "GITOPS_IMAGE_REPOSITORY: registry.internal/team/order-api") {
		t.Fatalf("expected GitOps image repository to follow IMAGE_NAME, got:\n%s", got)
	}
}

func TestBuildDockerfileReturnsRustTemplate(t *testing.T) {
	req := &dto.PipelineRequest{
		Name: "rust-api",
		Stages: []dto.Stage{
			{
				ID:   "test",
				Name: "单元测试",
				Steps: []dto.Step{
					{
						ID:   "cargo_test",
						Name: "Cargo Test",
						Type: "container",
						Config: map[string]interface{}{
							"image":    "rust:1.87-alpine",
							"commands": []interface{}{"cargo test"},
						},
					},
				},
			},
		},
		Variables: []dto.Variable{
			{Name: "APP_LANGUAGE", Value: "rust"},
			{Name: "BUILD_COMMAND", Value: "cargo build --release --locked"},
			{Name: "APP_PORT", Value: "8081"},
		},
	}

	got := buildDockerfile(req)
	for _, want := range []string{
		"FROM rust:1.87-alpine AS builder",
		"ARG BUILD_COMMAND=\"\"",
		"ARG APP_PORT=\"8080\"",
		"cargo build --release",
		"EXPOSE ${APP_PORT}",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated rust dockerfile missing %q:\n%s", want, got)
		}
	}
}

func TestBuildGitLabCIYAMLSkipsRedundantGoBuildWhenDockerfileBuildsImage(t *testing.T) {
	req := &dto.PipelineRequest{
		Name:      "order-api",
		GitBranch: "main",
		Stages: []dto.Stage{
			{
				ID:   "build",
				Name: "编译构建",
				Steps: []dto.Step{
					{
						ID:   "go_build",
						Name: "Go Build",
						Type: "container",
						Config: map[string]interface{}{
							"image": "golang:1.25-alpine",
							"commands": []interface{}{
								"go mod download",
								`if [ -d ./cmd/server ]; then CGO_ENABLED=0 go build -o app ./cmd/server; else CGO_ENABLED=0 go build -o app .; fi`,
							},
						},
					},
				},
			},
			{
				ID:   "test",
				Name: "单元测试",
				Steps: []dto.Step{
					{
						ID:   "go_test",
						Name: "Go Test",
						Type: "container",
						Config: map[string]interface{}{
							"image":    "golang:1.25-alpine",
							"commands": []interface{}{"go test ./..."},
						},
					},
				},
			},
			{
				ID:   "docker",
				Name: "镜像构建与推送",
				Steps: []dto.Step{
					{ID: "docker_build", Name: "Docker Build", Type: "docker_build"},
				},
			},
		},
	}

	got := buildGitLabCIYAML(nil, req)
	if strings.Contains(got, "go_build:") || strings.Contains(got, "CGO_ENABLED=0 go build -o app") {
		t.Fatalf("generated yaml should skip standalone Go build when Dockerfile builds final image:\n%s", got)
	}
	if !strings.Contains(got, "go_test:") || !strings.Contains(got, "go test ./...") {
		t.Fatalf("generated yaml should keep Go test job:\n%s", got)
	}
	if !strings.Contains(got, "build_and_push_image:") || !strings.Contains(got, "docker build --pull") {
		t.Fatalf("generated yaml should keep image build job:\n%s", got)
	}
}

func containsAny(s string, candidates ...string) bool {
	for _, candidate := range candidates {
		if strings.Contains(s, candidate) {
			return true
		}
	}
	return false
}

func TestBuildGitLabCIYAMLUsesCustomYAMLVerbatim(t *testing.T) {
	custom := "stages:\n  - custom\ncustom_job:\n  stage: custom\n  script:\n    - echo custom\n"
	req := &dto.PipelineRequest{
		GitLabCIYAML:       custom,
		GitLabCIYAMLCustom: true,
		Stages: []dto.Stage{
			{
				ID: "build",
				Steps: []dto.Step{
					{Type: "docker_build"},
				},
			},
		},
	}

	got := buildGitLabCIYAML(nil, req)
	if got != custom {
		t.Fatalf("expected custom YAML verbatim, got:\n%s", got)
	}
}

func TestBuildDockerfileInfersGoRuntime(t *testing.T) {
	req := &dto.PipelineRequest{
		Stages: []dto.Stage{
			{
				ID: "build",
				Steps: []dto.Step{
					{
						Type: "container",
						Config: map[string]interface{}{
							"image": "golang:1.25-alpine",
						},
					},
				},
			},
		},
	}

	got := buildDockerfile(req)
	if !strings.Contains(got, "FROM golang:1.25-alpine AS builder") {
		t.Fatalf("expected Go builder Dockerfile, got:\n%s", got)
	}
	if !strings.Contains(got, "go build -trimpath") || !strings.Contains(got, "go build -trimpath -ldflags=\"-s -w\" -o /out/app .") {
		t.Fatalf("expected Go Dockerfile to support root-module services, got:\n%s", got)
	}
	if !strings.Contains(got, "ENTRYPOINT [\"/app/app\"]") {
		t.Fatalf("expected runnable Go entrypoint, got:\n%s", got)
	}
}

func TestBuildGitLabCIYAMLInfersNodeImageFromCommands(t *testing.T) {
	req := &dto.PipelineRequest{
		GitBranch: "main",
		Stages: []dto.Stage{
			{
				ID:   "build",
				Name: "Build",
				Steps: []dto.Step{
					{
						ID:   "smoke",
						Name: "Smoke",
						Type: "container",
						Config: map[string]interface{}{
							"commands": []interface{}{"npm run test", "npm run build"},
						},
					},
				},
			},
		},
	}

	got := buildGitLabCIYAML(nil, req)
	if !strings.Contains(got, `image: "node:20-alpine"`) {
		t.Fatalf("expected node image inferred from npm commands, got:\n%s", got)
	}
}
