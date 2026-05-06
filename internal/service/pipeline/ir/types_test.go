package ir

import (
	"errors"
	"strings"
	"testing"
)

// minimal returns a syntactically valid pipeline for use as a test fixture.
func minimal() *Pipeline {
	return &Pipeline{
		Version: Version,
		Name:    "demo",
		Stages: []Stage{
			{
				Name:  "build",
				Steps: []Step{{Name: "compile", Image: "golang:1.25"}},
			},
		},
	}
}

func TestValidate_OK(t *testing.T) {
	if err := Validate(minimal()); err != nil {
		t.Fatalf("expected valid pipeline to pass, got: %v", err)
	}
}

func TestValidate_NilPipeline(t *testing.T) {
	if err := Validate(nil); err == nil {
		t.Fatal("expected error on nil pipeline")
	}
}

func TestValidate_EmptyName(t *testing.T) {
	p := minimal()
	p.Name = ""
	if err := Validate(p); !errors.Is(err, ErrEmptyName) {
		t.Fatalf("want ErrEmptyName, got %v", err)
	}
}

func TestValidate_NoStages(t *testing.T) {
	p := minimal()
	p.Stages = nil
	if err := Validate(p); !errors.Is(err, ErrNoStages) {
		t.Fatalf("want ErrNoStages, got %v", err)
	}
}

func TestValidate_DuplicateStage(t *testing.T) {
	p := minimal()
	p.Stages = append(p.Stages, p.Stages[0])
	if err := Validate(p); !errors.Is(err, ErrDuplicateStageName) {
		t.Fatalf("want ErrDuplicateStageName, got %v", err)
	}
}

func TestValidate_EmptyStageName(t *testing.T) {
	p := minimal()
	p.Stages[0].Name = ""
	if err := Validate(p); !errors.Is(err, ErrEmptyStageName) {
		t.Fatalf("want ErrEmptyStageName, got %v", err)
	}
}

func TestValidate_EmptyStepName(t *testing.T) {
	p := minimal()
	p.Stages[0].Steps[0].Name = ""
	if err := Validate(p); !errors.Is(err, ErrEmptyStepName) {
		t.Fatalf("want ErrEmptyStepName, got %v", err)
	}
}

func TestValidate_EmptyStepImage(t *testing.T) {
	p := minimal()
	p.Stages[0].Steps[0].Image = ""
	if err := Validate(p); !errors.Is(err, ErrEmptyStepImage) {
		t.Fatalf("want ErrEmptyStepImage, got %v", err)
	}
}

func TestValidate_DuplicateStep(t *testing.T) {
	p := minimal()
	p.Stages[0].Steps = append(p.Stages[0].Steps, p.Stages[0].Steps[0])
	if err := Validate(p); !errors.Is(err, ErrDuplicateStepName) {
		t.Fatalf("want ErrDuplicateStepName, got %v", err)
	}
}

func TestValidate_StageNeedsUnknown(t *testing.T) {
	p := minimal()
	p.Stages[0].Needs = []string{"nope"}
	if err := Validate(p); !errors.Is(err, ErrNeedsUnknownStage) {
		t.Fatalf("want ErrNeedsUnknownStage, got %v", err)
	}
}

func TestValidate_StageNeedsForwardReference(t *testing.T) {
	p := &Pipeline{
		Version: Version,
		Name:    "demo",
		Stages: []Stage{
			{
				Name:  "build",
				Needs: []string{"deploy"}, // declared later -> forward ref
				Steps: []Step{{Name: "compile", Image: "golang:1.25"}},
			},
			{
				Name:  "deploy",
				Steps: []Step{{Name: "kubectl", Image: "alpine"}},
			},
		},
	}
	err := Validate(p)
	if !errors.Is(err, ErrNeedsForwardReference) {
		t.Fatalf("want ErrNeedsForwardReference, got %v", err)
	}
}

func TestValidate_StageNeedsSelf(t *testing.T) {
	p := minimal()
	p.Stages[0].Needs = []string{p.Stages[0].Name}
	err := Validate(p)
	if err == nil || !strings.Contains(err.Error(), "cannot need itself") {
		t.Fatalf("want self-need error, got %v", err)
	}
}

func TestValidate_DAG_OK(t *testing.T) {
	p := &Pipeline{
		Version: Version,
		Name:    "demo",
		Stages: []Stage{
			{Name: "build", Steps: []Step{{Name: "compile", Image: "golang:1.25"}}},
			{Name: "test", Needs: []string{"build"}, Steps: []Step{{Name: "go-test", Image: "golang:1.25"}}},
			{Name: "deploy", Needs: []string{"test"}, Steps: []Step{{Name: "k8s", Image: "alpine"}}},
		},
	}
	if err := Validate(p); err != nil {
		t.Fatalf("expected valid DAG, got: %v", err)
	}
}

func TestValidate_VariableNoDualSource(t *testing.T) {
	p := minimal()
	p.Variables = []Variable{{Name: "TOKEN", Value: "abc", FromCred: "secret"}}
	if err := Validate(p); !errors.Is(err, ErrVariableValueAndCred) {
		t.Fatalf("want ErrVariableValueAndCred, got %v", err)
	}
}

func TestValidate_VariableDuplicateName(t *testing.T) {
	p := minimal()
	p.Variables = []Variable{
		{Name: "X", Value: "1"},
		{Name: "X", Value: "2"},
	}
	err := Validate(p)
	if err == nil || !strings.Contains(err.Error(), "duplicate name") {
		t.Fatalf("want duplicate name error, got %v", err)
	}
}

func TestValidate_VariableEmptyName(t *testing.T) {
	p := minimal()
	p.Variables = []Variable{{Name: "", Value: "x"}}
	if err := Validate(p); !errors.Is(err, ErrVariableEmptyName) {
		t.Fatalf("want ErrVariableEmptyName, got %v", err)
	}
}

func TestValidate_CacheBothEmpty(t *testing.T) {
	p := minimal()
	p.Cache = &Cache{}
	if err := Validate(p); !errors.Is(err, ErrCacheBothEmpty) {
		t.Fatalf("want ErrCacheBothEmpty, got %v", err)
	}
}

func TestValidate_CacheKeyOnlyOK(t *testing.T) {
	p := minimal()
	p.Cache = &Cache{Key: "go-mod-{{ checksum go.sum }}"}
	if err := Validate(p); err != nil {
		t.Fatalf("expected key-only cache to pass, got: %v", err)
	}
}

func TestValidate_LayoutOrphanRejected(t *testing.T) {
	p := minimal()
	p.Layout = &Layout{Nodes: []NodeLayout{{NodeID: "stage:nonexistent", X: 1, Y: 2}}}
	if err := Validate(p); !errors.Is(err, ErrLayoutOrphanNode) {
		t.Fatalf("want ErrLayoutOrphanNode, got %v", err)
	}
}

func TestValidate_LayoutValid(t *testing.T) {
	p := minimal()
	p.Layout = &Layout{
		Nodes: []NodeLayout{
			{NodeID: StageNodeID("build"), X: 0, Y: 0},
			{NodeID: StepNodeID("build", "compile"), X: 100, Y: 50},
		},
	}
	if err := Validate(p); err != nil {
		t.Fatalf("expected valid layout to pass, got: %v", err)
	}
}

func TestNodeIDHelpers(t *testing.T) {
	if got, want := StageNodeID("build"), "stage:build"; got != want {
		t.Errorf("StageNodeID = %q, want %q", got, want)
	}
	if got, want := StepNodeID("build", "compile"), "step:build:compile"; got != want {
		t.Errorf("StepNodeID = %q, want %q", got, want)
	}
}
