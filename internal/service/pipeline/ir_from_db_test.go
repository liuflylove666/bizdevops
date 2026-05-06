package pipeline

import (
	"strings"
	"testing"

	"devops/internal/models"
)

const fullConfigJSON = `{
  "stages": [
    {
      "id": "s1",
      "name": "build",
      "depends_on": [],
      "steps": [
        {
          "id": "p1",
          "name": "compile",
          "type": "build",
          "config": { "command": "go build ./...", "workdir": "/src" },
          "timeout": 600
        }
      ]
    },
    {
      "id": "s2",
      "name": "test",
      "depends_on": ["build"],
      "steps": [
        {
          "id": "p2",
          "name": "go-test",
          "type": "test",
          "config": { "commands": ["go test ./...", "go vet ./..."] }
        },
        {
          "id": "p3",
          "name": "lint",
          "type": "scan",
          "config": { "image": "golangci-lint:v1.54" }
        }
      ]
    }
  ],
  "variables": [
    { "name": "GO_VERSION", "value": "1.25", "is_secret": false },
    { "name": "REGISTRY_TOKEN", "value": "<sealed>", "is_secret": true }
  ]
}`

const triggerConfigJSON = `{
  "manual": true,
  "scheduled": { "enabled": true, "cron": "0 2 * * *", "timezone": "UTC" },
  "webhook": { "enabled": true, "branch_filter": ["main", "release/*"] }
}`

func TestIRFromDB_Nil(t *testing.T) {
	if _, err := IRFromDB(nil); err == nil {
		t.Fatal("expected error on nil pipeline")
	}
}

func TestIRFromDB_EmptyName(t *testing.T) {
	if _, err := IRFromDB(&models.Pipeline{Name: ""}); err == nil {
		t.Fatal("expected error on empty name")
	}
}

func TestIRFromDB_NoStages(t *testing.T) {
	p := &models.Pipeline{Name: "x"}
	if _, err := IRFromDB(p); err == nil {
		t.Fatal("expected error: pipeline with no stages")
	}
}

func TestIRFromDB_BadConfigJSON(t *testing.T) {
	p := &models.Pipeline{Name: "x", ConfigJSON: "not-json"}
	_, err := IRFromDB(p)
	if err == nil || !strings.Contains(err.Error(), "parse config_json") {
		t.Fatalf("want parse error, got %v", err)
	}
}

func TestIRFromDB_FullPipeline(t *testing.T) {
	p := &models.Pipeline{
		ID:                42,
		Name:              "order-svc-build",
		ConfigJSON:        fullConfigJSON,
		TriggerConfigJSON: triggerConfigJSON,
	}

	out, err := IRFromDB(p)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}

	// Top-level
	if out.Name != "order-svc-build" {
		t.Errorf("name = %q", out.Name)
	}
	if out.Version == "" {
		t.Errorf("version was not defaulted")
	}

	// Trigger: webhook -> branches; scheduled -> cron
	if out.Trigger == nil {
		t.Fatal("expected trigger populated")
	}
	if len(out.Trigger.Branches) != 2 || out.Trigger.Branches[0] != "main" {
		t.Errorf("branches = %v", out.Trigger.Branches)
	}
	if out.Trigger.Cron != "0 2 * * *" {
		t.Errorf("cron = %q", out.Trigger.Cron)
	}

	// Variables
	if len(out.Variables) != 2 {
		t.Fatalf("variables count = %d", len(out.Variables))
	}
	if !out.Variables[1].Sensitive {
		t.Errorf("REGISTRY_TOKEN should be sensitive")
	}

	// Stages
	if len(out.Stages) != 2 {
		t.Fatalf("stages count = %d", len(out.Stages))
	}
	if out.Stages[0].Name != "build" || len(out.Stages[0].Steps) != 1 {
		t.Errorf("stage[0] shape: %+v", out.Stages[0])
	}
	if got := out.Stages[1].Needs; len(got) != 1 || got[0] != "build" {
		t.Errorf("test stage needs = %v", got)
	}

	// Step: build/compile — type=build → image=alpine; command -> Commands
	build := out.Stages[0].Steps[0]
	if build.Image != "alpine" {
		t.Errorf("build image = %q (want default for type=build)", build.Image)
	}
	if len(build.Commands) != 1 || build.Commands[0] != "go build ./..." {
		t.Errorf("build commands = %v", build.Commands)
	}
	if build.WorkingDir != "/src" {
		t.Errorf("build workdir = %q", build.WorkingDir)
	}
	if build.TimeoutSeconds != 600 {
		t.Errorf("build timeout = %d", build.TimeoutSeconds)
	}

	// Step: test/go-test — commands array preserved verbatim
	gotest := out.Stages[1].Steps[0]
	if len(gotest.Commands) != 2 || gotest.Commands[0] != "go test ./..." {
		t.Errorf("go-test commands = %v", gotest.Commands)
	}

	// Step: lint — explicit image override beats type default
	lint := out.Stages[1].Steps[1]
	if lint.Image != "golangci-lint:v1.54" {
		t.Errorf("lint image = %q (explicit image should override)", lint.Image)
	}
}

func TestIRFromDB_UnknownStepType_PlaceholderImage(t *testing.T) {
	cfg := `{"stages":[{"name":"x","steps":[
		{"name":"weird","type":"made-up","config":{"foo":"bar","baz":"qux"}}
	]}],"variables":[]}`
	p := &models.Pipeline{Name: "demo", ConfigJSON: cfg}

	out, err := IRFromDB(p)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	step := out.Stages[0].Steps[0]
	if step.Image != "placeholder" {
		t.Errorf("unknown type should yield placeholder image, got %q", step.Image)
	}
	// Unknown config keys preserved as Env for visibility.
	if step.Env["foo"] != "bar" || step.Env["baz"] != "qux" {
		t.Errorf("env didn't capture extras: %+v", step.Env)
	}
}

func TestIRFromDB_ProducesValidIR(t *testing.T) {
	// The DB → IR result must pass IR validation; otherwise BE-12 will
	// refuse to MarshalYAML and the export endpoint will 500.
	p := &models.Pipeline{
		ID:         1,
		Name:       "demo",
		ConfigJSON: `{"stages":[{"name":"build","steps":[{"name":"x","type":"build","config":{"command":"echo hi"}}]}]}`,
	}
	out, err := IRFromDB(p)
	if err != nil {
		t.Fatalf("convert failed: %v", err)
	}
	// Lazy validation through MarshalYAML is the actual smoke test;
	// import cycle guards prevent direct ir.Validate call here. The
	// downstream YAML test exercises this path, so just sanity-check
	// the most common landmines:
	if out.Name == "" || len(out.Stages) == 0 || out.Stages[0].Steps[0].Image == "" {
		t.Errorf("converted IR has invariant violations: %+v", out)
	}
}
