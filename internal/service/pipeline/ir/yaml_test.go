package ir

import (
	"reflect"
	"strings"
	"testing"
)

// fullPipeline returns a feature-complete Pipeline used to exercise every
// IR field at least once. Round-trip tests below assert that nothing in
// here gets dropped through Marshal → Unmarshal.
func fullPipeline() *Pipeline {
	return &Pipeline{
		Version: Version,
		Name:    "order-svc-build",
		Trigger: &Trigger{
			Branches: []string{"main", "release/*"},
			Tags:     []string{"v*"},
			Events:   []string{"push", "pull_request"},
			Cron:     "0 2 * * *",
		},
		Variables: []Variable{
			{Name: "GO_VERSION", Value: "1.25"},
			{Name: "REGISTRY_TOKEN", FromCred: "prod-registry", Sensitive: true},
		},
		Cache: &Cache{
			Key:   "go-mod-{{ checksum go.sum }}",
			Paths: []string{"~/.cache/go-build", "~/go/pkg/mod"},
		},
		Stages: []Stage{
			{
				Name: "build",
				Steps: []Step{
					{
						Name:           "compile",
						Image:          "golang:1.25",
						Commands:       []string{"go build ./..."},
						Env:            map[string]string{"CGO_ENABLED": "0"},
						WorkingDir:     "/src",
						TimeoutSeconds: 600,
					},
				},
			},
			{
				Name:  "test",
				Needs: []string{"build"},
				Matrix: &Matrix{
					Include: map[string][]string{
						"go-version": {"1.24", "1.25"},
					},
				},
				Steps: []Step{
					{
						Name:     "unit-test",
						Image:    "golang:1.25",
						Commands: []string{"go test ./..."},
						When: &Condition{
							Branch: "main",
							Event:  "push",
						},
					},
					{
						Name:       "lint",
						Image:      "golangci-lint:v1.54",
						ContinueOn: &ContinueOn{Failure: true},
					},
				},
			},
		},
		Layout: &Layout{
			Nodes: []NodeLayout{
				{NodeID: StageNodeID("build"), X: 100, Y: 50, Width: 200, Height: 80},
				{NodeID: StepNodeID("build", "compile"), X: 110, Y: 60},
				{NodeID: StageNodeID("test"), X: 100, Y: 200},
				{NodeID: StepNodeID("test", "unit-test"), X: 110, Y: 210},
				{NodeID: StepNodeID("test", "lint"), X: 110, Y: 280},
			},
		},
	}
}

func TestMarshalYAML_RejectsNil(t *testing.T) {
	if _, err := MarshalYAML(nil, MarshalOptions{}); err == nil {
		t.Fatal("expected error on nil pipeline")
	}
}

func TestMarshalYAML_RejectsInvalid(t *testing.T) {
	bad := minimal()
	bad.Name = "" // trips ErrEmptyName

	_, err := MarshalYAML(bad, MarshalOptions{})
	if err == nil {
		t.Fatal("expected validation failure to bubble up")
	}
	if !strings.Contains(err.Error(), "refuse to marshal invalid pipeline") {
		t.Errorf("error should hint refuse: %v", err)
	}
}

func TestMarshalYAML_DefaultsVersion(t *testing.T) {
	p := minimal()
	p.Version = "" // user forgot
	data, err := MarshalYAML(p, MarshalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "version: \""+Version+"\"") {
		t.Errorf("expected version default applied, got:\n%s", data)
	}
}

func TestMarshalYAML_LayoutOmittedByDefault(t *testing.T) {
	p := fullPipeline()
	data, err := MarshalYAML(p, MarshalOptions{}) // IncludeLayout=false
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "__layout") {
		t.Errorf("expected __layout absent when IncludeLayout=false:\n%s", data)
	}
}

func TestMarshalYAML_LayoutPreservedWhenRequested(t *testing.T) {
	p := fullPipeline()
	data, err := MarshalYAML(p, MarshalOptions{IncludeLayout: true})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "__layout") {
		t.Errorf("expected __layout present when IncludeLayout=true:\n%s", data)
	}
	// One representative coordinate from fullPipeline().
	if !strings.Contains(string(data), "node_id: stage:build") {
		t.Errorf("expected layout node_id key in output:\n%s", data)
	}
}

func TestMarshalYAML_DoesNotMutateInput(t *testing.T) {
	// Important: caller must not see Layout disappear when IncludeLayout=false.
	p := fullPipeline()
	originalLayoutLen := len(p.Layout.Nodes)
	if _, err := MarshalYAML(p, MarshalOptions{}); err != nil {
		t.Fatal(err)
	}
	if p.Layout == nil || len(p.Layout.Nodes) != originalLayoutLen {
		t.Errorf("MarshalYAML mutated input; original Layout was clobbered")
	}
}

func TestUnmarshalYAML_RejectsBadYAML(t *testing.T) {
	_, err := UnmarshalYAML([]byte("name: x\nstages: [not a list of objects"))
	if err == nil {
		t.Fatal("expected yaml parse error")
	}
}

func TestUnmarshalYAML_RejectsInvariantViolation(t *testing.T) {
	// Decodes fine but trips Validate (duplicate stage names).
	bad := []byte(`
version: "1.0"
name: bad
stages:
  - name: build
    steps:
      - name: a
        image: alpine
  - name: build
    steps:
      - name: b
        image: alpine
`)
	if _, err := UnmarshalYAML(bad); err == nil {
		t.Fatal("expected duplicate-stage error")
	}
}

func TestRoundTrip_FullPipelineWithoutLayout(t *testing.T) {
	in := fullPipeline()
	out, err := RoundTrip(in, MarshalOptions{})
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}

	// Without layout, the only expected difference is Layout itself.
	if out.Layout != nil {
		t.Errorf("expected Layout stripped, got %+v", out.Layout)
	}
	// All other top-level fields should match.
	in2 := *in
	in2.Layout = nil
	if !reflect.DeepEqual(&in2, out) {
		t.Errorf("round-trip diverged.\n want=%+v\n got =%+v", &in2, out)
	}
}

func TestRoundTrip_FullPipelineWithLayout(t *testing.T) {
	in := fullPipeline()
	out, err := RoundTrip(in, MarshalOptions{IncludeLayout: true})
	if err != nil {
		t.Fatalf("round-trip failed: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round-trip diverged with layout.\n want=%+v\n got =%+v", in, out)
	}
}

func TestRoundTrip_IsIdempotent(t *testing.T) {
	// Marshal → Unmarshal → Marshal should produce byte-identical output.
	in := fullPipeline()

	first, err := MarshalYAML(in, MarshalOptions{IncludeLayout: true})
	if err != nil {
		t.Fatal(err)
	}
	mid, err := UnmarshalYAML(first)
	if err != nil {
		t.Fatal(err)
	}
	second, err := MarshalYAML(mid, MarshalOptions{IncludeLayout: true})
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("marshal not idempotent.\n first=\n%s\n second=\n%s", first, second)
	}
}

func TestMarshalYAML_OmitsEmptyOptionalFields(t *testing.T) {
	// Minimal pipeline should not emit empty trigger/variables/cache/layout sections.
	p := minimal()
	data, err := MarshalYAML(p, MarshalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	yamlStr := string(data)
	for _, banned := range []string{"trigger:", "variables:", "cache:", "__layout:"} {
		if strings.Contains(yamlStr, banned) {
			t.Errorf("expected %q absent from minimal pipeline yaml:\n%s", banned, yamlStr)
		}
	}
	// But required fields must be there.
	for _, required := range []string{"version:", "name: demo", "stages:"} {
		if !strings.Contains(yamlStr, required) {
			t.Errorf("expected %q present in yaml:\n%s", required, yamlStr)
		}
	}
}

func TestRoundTrip_PreservesCommitConditionsAndContinueOn(t *testing.T) {
	// Tighter regression: When and ContinueOn nested structs survive.
	in := &Pipeline{
		Version: Version,
		Name:    "demo",
		Stages: []Stage{
			{
				Name: "build",
				Steps: []Step{
					{
						Name:       "step",
						Image:      "alpine",
						When:       &Condition{Branch: "main", Tag: "v*", Event: "push", Expr: "true"},
						ContinueOn: &ContinueOn{Failure: true},
					},
				},
			},
		},
	}
	out, err := RoundTrip(in, MarshalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	got := out.Stages[0].Steps[0]
	if got.When == nil || got.When.Branch != "main" {
		t.Errorf("When.Branch lost: %+v", got.When)
	}
	if got.When.Tag != "v*" || got.When.Event != "push" || got.When.Expr != "true" {
		t.Errorf("When fields lost: %+v", got.When)
	}
	if got.ContinueOn == nil || !got.ContinueOn.Failure {
		t.Errorf("ContinueOn.Failure lost: %+v", got.ContinueOn)
	}
}

func TestRoundTrip_PreservesMatrix(t *testing.T) {
	in := &Pipeline{
		Version: Version,
		Name:    "demo",
		Stages: []Stage{
			{
				Name: "test",
				Matrix: &Matrix{
					Include: map[string][]string{
						"os":         {"linux", "darwin"},
						"go-version": {"1.24", "1.25"},
					},
				},
				Steps: []Step{{Name: "go-test", Image: "golang:1.25"}},
			},
		},
	}
	out, err := RoundTrip(in, MarshalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(in.Stages[0].Matrix, out.Stages[0].Matrix) {
		t.Errorf("matrix lost.\n want=%+v\n got =%+v", in.Stages[0].Matrix, out.Stages[0].Matrix)
	}
}
