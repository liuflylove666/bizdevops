// Package ir defines the canonical, in-memory representation of a CI pipeline.
//
// IR sits between three external forms:
//
//	┌──────────┐   ┌────┐   ┌──────────┐
//	│ Designer │──▶│ IR │──▶│   YAML   │
//	└──────────┘   │    │   └──────────┘
//	               │    │   ┌──────────┐
//	               └────┘──▶│   DB     │
//	                        └──────────┘
//
// IR has zero ORM / HTTP / framework dependencies. It is safe to construct,
// validate, snapshot, hash, and unit-test in isolation.
//
// See ADR-0008 for design rationale and invariants.
package ir

// Version is the IR schema version. Bumped on breaking changes.
const Version = "1.0"

// Pipeline is the canonical representation of a CI pipeline.
//
// It mirrors pkg/dto.PipelineYAMLConfig field-for-field plus an optional
// Layout for round-tripping designer node positions.
type Pipeline struct {
	Version   string     `json:"version" yaml:"version"`
	Name      string     `json:"name" yaml:"name"`
	Trigger   *Trigger   `json:"trigger,omitempty" yaml:"trigger,omitempty"`
	Variables []Variable `json:"variables,omitempty" yaml:"variables,omitempty"`
	Cache     *Cache     `json:"cache,omitempty" yaml:"cache,omitempty"`
	Stages    []Stage    `json:"stages" yaml:"stages"`

	// Layout is designer-only positional metadata. Discarded on YAML export
	// when emit_layout=false; preserved on import for round-trip fidelity.
	Layout *Layout `json:"__layout,omitempty" yaml:"__layout,omitempty"`
}

// Trigger declares how a pipeline gets started.
type Trigger struct {
	Branches []string `json:"branches,omitempty" yaml:"branches,omitempty"`
	Tags     []string `json:"tags,omitempty" yaml:"tags,omitempty"`
	Events   []string `json:"events,omitempty" yaml:"events,omitempty"` // push / pull_request / tag
	Cron     string   `json:"cron,omitempty" yaml:"cron,omitempty"`
}

// Variable is either a literal value or a reference to a stored credential.
//
// FromCred references a named credential resolved by service/pipeline/credential_service.go.
// IR never stores secret material directly.
type Variable struct {
	Name      string `json:"name" yaml:"name"`
	Value     string `json:"value,omitempty" yaml:"value,omitempty"`
	FromCred  string `json:"from_cred,omitempty" yaml:"from_cred,omitempty"`
	Sensitive bool   `json:"sensitive,omitempty" yaml:"sensitive,omitempty"`
}

// Cache controls inter-run dependency / artifact caching.
type Cache struct {
	Key   string   `json:"key" yaml:"key"`
	Paths []string `json:"paths" yaml:"paths"`
}

// Stage is a DAG node grouping ordered steps.
//
// Needs lists upstream stage names; each must appear earlier in Pipeline.Stages.
type Stage struct {
	Name   string   `json:"name" yaml:"name"`
	Needs  []string `json:"needs,omitempty" yaml:"needs,omitempty"`
	Matrix *Matrix  `json:"matrix,omitempty" yaml:"matrix,omitempty"`
	Steps  []Step   `json:"steps" yaml:"steps"`
}

// Matrix expands a stage into a fan-out of variants.
type Matrix struct {
	Include map[string][]string `json:"include" yaml:"include"`
}

// Step is a single container execution within a stage.
type Step struct {
	Name           string            `json:"name" yaml:"name"`
	Image          string            `json:"image" yaml:"image"`
	Commands       []string          `json:"commands,omitempty" yaml:"commands,omitempty"`
	Env            map[string]string `json:"env,omitempty" yaml:"env,omitempty"`
	WorkingDir     string            `json:"working_dir,omitempty" yaml:"working_dir,omitempty"`
	TimeoutSeconds int               `json:"timeout_seconds,omitempty" yaml:"timeout_seconds,omitempty"`
	When           *Condition        `json:"when,omitempty" yaml:"when,omitempty"`
	ContinueOn     *ContinueOn       `json:"continue_on,omitempty" yaml:"continue_on,omitempty"`
}

// Condition gates step execution on simple expression-free predicates.
// Expr is reserved for V2; V1 ignores it.
type Condition struct {
	Branch string `json:"branch,omitempty" yaml:"branch,omitempty"`
	Tag    string `json:"tag,omitempty" yaml:"tag,omitempty"`
	Event  string `json:"event,omitempty" yaml:"event,omitempty"`
	Expr   string `json:"expr,omitempty" yaml:"expr,omitempty"`
}

// ContinueOn allows a step to fail without failing the stage.
type ContinueOn struct {
	Failure bool `json:"failure,omitempty" yaml:"failure,omitempty"`
}

// Layout is designer-only positional metadata.
type Layout struct {
	Nodes []NodeLayout `json:"nodes,omitempty" yaml:"nodes,omitempty"`
}

// NodeLayout pins one designer node to a canvas coordinate.
//
// NodeID is a stable string built by the designer:
//   - "stage:<stage_name>"
//   - "step:<stage_name>:<step_name>"
//
// Designer DB IDs (auto-increment ints) deliberately never enter IR.
type NodeLayout struct {
	NodeID string  `json:"node_id" yaml:"node_id"`
	X      float64 `json:"x" yaml:"x"`
	Y      float64 `json:"y" yaml:"y"`
	Width  float64 `json:"w,omitempty" yaml:"w,omitempty"`
	Height float64 `json:"h,omitempty" yaml:"h,omitempty"`
}

// StageNodeID returns the canonical layout key for a stage.
func StageNodeID(stageName string) string {
	return "stage:" + stageName
}

// StepNodeID returns the canonical layout key for a step within a stage.
func StepNodeID(stageName, stepName string) string {
	return "step:" + stageName + ":" + stepName
}
