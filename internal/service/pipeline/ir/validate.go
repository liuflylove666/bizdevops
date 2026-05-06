package ir

import (
	"errors"
	"fmt"
)

// Sentinel errors for programmatic checks. Wrapped messages add context.
var (
	ErrEmptyName              = errors.New("ir: pipeline name is empty")
	ErrNoStages               = errors.New("ir: pipeline must declare at least one stage")
	ErrDuplicateStageName     = errors.New("ir: duplicate stage name")
	ErrEmptyStageName         = errors.New("ir: stage name is empty")
	ErrNoSteps                = errors.New("ir: stage must declare at least one step")
	ErrEmptyStepName          = errors.New("ir: step name is empty")
	ErrDuplicateStepName      = errors.New("ir: duplicate step name within stage")
	ErrEmptyStepImage         = errors.New("ir: step image is empty")
	ErrNeedsForwardReference  = errors.New("ir: stage needs references a stage that has not yet been declared")
	ErrNeedsUnknownStage      = errors.New("ir: stage needs references an unknown stage")
	ErrCacheBothEmpty         = errors.New("ir: cache key and paths are both empty")
	ErrLayoutOrphanNode       = errors.New("ir: layout references a node that does not exist in pipeline")
	ErrVariableEmptyName      = errors.New("ir: variable name is empty")
	ErrVariableValueAndCred   = errors.New("ir: variable cannot set both value and from_cred")
)

// Validate checks the structural invariants stated in ADR-0008.
//
// It does NOT validate:
//   - whether referenced credentials actually exist (handled at execution time)
//   - whether referenced images can be pulled
//   - matrix expansion correctness beyond shape
//
// On the first violation Validate returns; callers wanting all errors should
// invoke per-section helpers manually.
func Validate(p *Pipeline) error {
	if p == nil {
		return errors.New("ir: pipeline is nil")
	}
	if p.Name == "" {
		return ErrEmptyName
	}
	if len(p.Stages) == 0 {
		return ErrNoStages
	}

	if err := validateVariables(p.Variables); err != nil {
		return err
	}
	if err := validateCache(p.Cache); err != nil {
		return err
	}
	if err := validateStages(p.Stages); err != nil {
		return err
	}
	if err := validateLayout(p); err != nil {
		return err
	}
	return nil
}

func validateVariables(vars []Variable) error {
	seen := make(map[string]struct{}, len(vars))
	for i, v := range vars {
		if v.Name == "" {
			return fmt.Errorf("variable[%d]: %w", i, ErrVariableEmptyName)
		}
		if _, dup := seen[v.Name]; dup {
			return fmt.Errorf("variable[%d] %q: duplicate name", i, v.Name)
		}
		seen[v.Name] = struct{}{}

		if v.Value != "" && v.FromCred != "" {
			return fmt.Errorf("variable %q: %w", v.Name, ErrVariableValueAndCred)
		}
	}
	return nil
}

func validateCache(c *Cache) error {
	if c == nil {
		return nil
	}
	if c.Key == "" && len(c.Paths) == 0 {
		return ErrCacheBothEmpty
	}
	return nil
}

func validateStages(stages []Stage) error {
	declared := make(map[string]int, len(stages))

	for i, st := range stages {
		if st.Name == "" {
			return fmt.Errorf("stage[%d]: %w", i, ErrEmptyStageName)
		}
		if _, dup := declared[st.Name]; dup {
			return fmt.Errorf("stage[%d] %q: %w", i, st.Name, ErrDuplicateStageName)
		}

		// Resolve `needs` BEFORE inserting current stage to enforce
		// "must appear earlier" (forward references rejected).
		for _, need := range st.Needs {
			if need == st.Name {
				return fmt.Errorf("stage %q: cannot need itself", st.Name)
			}
			if _, ok := declared[need]; !ok {
				if stageExistsLater(stages, i, need) {
					return fmt.Errorf("stage %q needs %q: %w", st.Name, need, ErrNeedsForwardReference)
				}
				return fmt.Errorf("stage %q needs %q: %w", st.Name, need, ErrNeedsUnknownStage)
			}
		}
		declared[st.Name] = i

		if len(st.Steps) == 0 {
			return fmt.Errorf("stage %q: %w", st.Name, ErrNoSteps)
		}
		if err := validateSteps(st.Name, st.Steps); err != nil {
			return err
		}
	}
	return nil
}

func stageExistsLater(stages []Stage, fromIdx int, name string) bool {
	for j := fromIdx + 1; j < len(stages); j++ {
		if stages[j].Name == name {
			return true
		}
	}
	return false
}

func validateSteps(stageName string, steps []Step) error {
	seen := make(map[string]struct{}, len(steps))
	for i, s := range steps {
		if s.Name == "" {
			return fmt.Errorf("stage %q step[%d]: %w", stageName, i, ErrEmptyStepName)
		}
		if _, dup := seen[s.Name]; dup {
			return fmt.Errorf("stage %q step[%d] %q: %w", stageName, i, s.Name, ErrDuplicateStepName)
		}
		seen[s.Name] = struct{}{}

		if s.Image == "" {
			return fmt.Errorf("stage %q step %q: %w", stageName, s.Name, ErrEmptyStepImage)
		}
	}
	return nil
}

func validateLayout(p *Pipeline) error {
	if p.Layout == nil {
		return nil
	}
	known := make(map[string]struct{})
	for _, st := range p.Stages {
		known[StageNodeID(st.Name)] = struct{}{}
		for _, sp := range st.Steps {
			known[StepNodeID(st.Name, sp.Name)] = struct{}{}
		}
	}
	for _, n := range p.Layout.Nodes {
		if _, ok := known[n.NodeID]; !ok {
			return fmt.Errorf("layout node %q: %w", n.NodeID, ErrLayoutOrphanNode)
		}
	}
	return nil
}
