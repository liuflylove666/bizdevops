package ir

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// MarshalOptions tunes how a Pipeline is rendered to YAML.
//
// IncludeLayout controls whether designer-only positional metadata is kept.
// Default (false) is correct for human-edited / Git-committed YAML; the
// designer round-trip flow flips it to true so it can restore canvas state.
type MarshalOptions struct {
	IncludeLayout bool
}

// MarshalYAML renders an IR Pipeline into YAML bytes.
//
// The struct's `yaml:"..."` tags are authoritative; this function only
// adds the optional Layout-stripping policy. Output is deterministic for
// a given input + options because Pipeline fields are declared in a fixed
// order and yaml.v3 emits them as declared.
//
// Returns ErrEmptyName if validation pre-flight fails — exporting an
// invalid pipeline would produce YAML that cannot be round-tripped.
func MarshalYAML(p *Pipeline, opts MarshalOptions) ([]byte, error) {
	if p == nil {
		return nil, fmt.Errorf("ir: cannot marshal nil pipeline")
	}
	if err := Validate(p); err != nil {
		return nil, fmt.Errorf("ir: refuse to marshal invalid pipeline: %w", err)
	}

	// Default the version on output so YAML always carries it, even when
	// the caller forgot to set it. Unmarshal reads it back faithfully.
	out := *p
	if out.Version == "" {
		out.Version = Version
	}
	if !opts.IncludeLayout {
		out.Layout = nil
	}

	data, err := yaml.Marshal(&out)
	if err != nil {
		return nil, fmt.Errorf("ir: yaml marshal failed: %w", err)
	}
	return data, nil
}

// UnmarshalYAML parses YAML bytes back into an IR Pipeline.
//
// Validation is applied after parse: a YAML document that decodes
// successfully but violates an IR invariant (e.g., duplicate stage name)
// is rejected here — better to fail at import than silently produce a
// half-broken in-memory tree.
//
// Pairs with MarshalYAML for round-trip equality (BE-14 守门).
func UnmarshalYAML(data []byte) (*Pipeline, error) {
	var p Pipeline
	if err := yaml.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("ir: yaml unmarshal failed: %w", err)
	}
	if err := Validate(&p); err != nil {
		return nil, fmt.Errorf("ir: parsed yaml violates invariants: %w", err)
	}
	return &p, nil
}

// RoundTrip is a test/debug helper: marshal then unmarshal. The returned
// Pipeline should be field-equal to the input modulo Layout when
// IncludeLayout=false.
//
// Production code should not call this — it has no operational use beyond
// validation tooling. Exported because BE-12 / BE-14 / S3 BE-18 each
// benefit from it as a regression check.
func RoundTrip(p *Pipeline, opts MarshalOptions) (*Pipeline, error) {
	data, err := MarshalYAML(p, opts)
	if err != nil {
		return nil, err
	}
	return UnmarshalYAML(data)
}
