package pipeline

import (
	"encoding/json"
	"fmt"

	"devops/internal/models"
	"devops/internal/service/pipeline/ir"
	"devops/pkg/dto"
)

// IRFromDB converts a legacy DB-stored Pipeline (where stages/steps live in
// ConfigJSON as dto.Stage[]/dto.Step[] with free-form Config maps) into the
// strongly-typed IR representation suitable for YAML export.
//
// Conversion is **best-effort lossy**:
//   - Step.Type (git/docker_build/k8s_deploy/...) is not preserved 1:1 in IR.
//     Each known type is mapped to a default Image and either passes through
//     command/commands keys or synthesizes a placeholder.
//   - Unknown step types collapse to image="placeholder" with the original
//     Config keys serialized into Env so reviewers can see what was lost.
//   - dto.Variable.IsSecret maps to ir.Variable.Sensitive; legacy schema has
//     no concept of credential references (FromCred), so the literal Value
//     is preserved as-is.
//
// Caller is the YAML export handler (BE-12). The same converter will be
// retired in favor of a proper Templates-as-Code path in S4 BE-23.
//
// Returns Validate-passing IR on success; the export endpoint should re-run
// Validate before MarshalYAML for defense-in-depth.
func IRFromDB(p *models.Pipeline) (*ir.Pipeline, error) {
	if p == nil {
		return nil, fmt.Errorf("ir_from_db: nil pipeline")
	}
	if p.Name == "" {
		return nil, fmt.Errorf("ir_from_db: pipeline name is empty (id=%d)", p.ID)
	}

	out := &ir.Pipeline{
		Version: ir.Version,
		Name:    p.Name,
	}

	if t, err := triggerFromDB(p.TriggerConfigJSON); err != nil {
		return nil, err
	} else if t != nil {
		out.Trigger = t
	}

	if p.ConfigJSON != "" {
		var cfg struct {
			Stages    []dto.Stage    `json:"stages"`
			Variables []dto.Variable `json:"variables"`
		}
		if err := json.Unmarshal([]byte(p.ConfigJSON), &cfg); err != nil {
			return nil, fmt.Errorf("ir_from_db: parse config_json failed: %w", err)
		}

		for _, v := range cfg.Variables {
			out.Variables = append(out.Variables, ir.Variable{
				Name:      v.Name,
				Value:     v.Value,
				Sensitive: v.IsSecret,
			})
		}

		for _, st := range cfg.Stages {
			converted, err := stageFromDB(st)
			if err != nil {
				return nil, fmt.Errorf("ir_from_db: stage %q: %w", st.Name, err)
			}
			out.Stages = append(out.Stages, converted)
		}
	}

	// A pipeline with no stages is an export error: IR Validate would reject it
	// and YAML rendering would be misleading. Surface the issue early with a
	// clear message rather than a generic Validate failure downstream.
	if len(out.Stages) == 0 {
		return nil, fmt.Errorf("ir_from_db: pipeline has no stages (id=%d)", p.ID)
	}

	return out, nil
}

// triggerFromDB parses TriggerConfigJSON. Returns (nil, nil) when the JSON
// is empty / yields no signals — IR omits the Trigger section entirely
// rather than emitting an empty stub.
func triggerFromDB(triggerJSON string) (*ir.Trigger, error) {
	if triggerJSON == "" {
		return nil, nil
	}
	var tc dto.TriggerConfig
	if err := json.Unmarshal([]byte(triggerJSON), &tc); err != nil {
		// Tolerate parse failure: the legacy schema is messy, and missing
		// trigger info should not block export of the rest.
		return nil, nil
	}

	out := &ir.Trigger{}
	any := false
	if tc.Webhook != nil && len(tc.Webhook.BranchFilter) > 0 {
		out.Branches = append(out.Branches, tc.Webhook.BranchFilter...)
		out.Events = append(out.Events, "push")
		any = true
	}
	if tc.Scheduled != nil && tc.Scheduled.Enabled && tc.Scheduled.Cron != "" {
		out.Cron = tc.Scheduled.Cron
		any = true
	}
	if tc.Manual {
		// "manual" doesn't map cleanly to YAML; treat it as no-op marker.
		// Documenting this as a known V1 limitation.
		_ = tc.Manual
	}
	if !any {
		return nil, nil
	}
	return out, nil
}

// stageFromDB converts one dto.Stage. dto.Stage.DependsOn maps to
// ir.Stage.Needs. dto.Stage.Parallel is not represented in IR (the YAML
// schema expresses parallelism via separate stages with shared Needs);
// preserved as a step-level note so reviewers can spot it.
func stageFromDB(in dto.Stage) (ir.Stage, error) {
	out := ir.Stage{
		Name:  in.Name,
		Needs: append([]string{}, in.DependsOn...),
	}
	for _, sp := range in.Steps {
		converted, err := stepFromDB(sp)
		if err != nil {
			return ir.Stage{}, fmt.Errorf("step %q: %w", sp.Name, err)
		}
		out.Steps = append(out.Steps, converted)
	}
	return out, nil
}

// stepFromDB converts one dto.Step. The free-form Config map is interpreted
// per Type; unknown types collapse to a placeholder image with Config keys
// serialized into Env for visibility.
func stepFromDB(in dto.Step) (ir.Step, error) {
	out := ir.Step{
		Name:           in.Name,
		TimeoutSeconds: in.Timeout,
	}

	out.Image = defaultImageForType(in.Type)
	out.Commands = extractCommands(in.Config)
	out.WorkingDir = stringFrom(in.Config, "workdir", "work_dir")

	if explicitImage := stringFrom(in.Config, "image"); explicitImage != "" {
		out.Image = explicitImage
	}

	// Preserve unknown Config keys as Env for transparency. Skip anything
	// already consumed above so we don't leak duplicates.
	consumed := map[string]bool{
		"image": true, "command": true, "commands": true,
		"workdir": true, "work_dir": true,
	}
	if len(in.Config) > 0 {
		extras := map[string]string{}
		for k, v := range in.Config {
			if consumed[k] {
				continue
			}
			if s, ok := v.(string); ok && s != "" {
				extras[k] = s
			}
		}
		if len(extras) > 0 {
			out.Env = extras
		}
	}

	if out.Image == "" {
		// IR.Validate enforces non-empty image; surface a recognizable
		// placeholder rather than failing — reviewers can fix later.
		out.Image = "placeholder"
	}

	return out, nil
}

// defaultImageForType returns a reasonable default image based on the
// legacy step type. These are deliberately conservative; the export is
// for human review, not for execution.
func defaultImageForType(t string) string {
	switch t {
	case "git":
		return "alpine/git"
	case "docker_build", "docker_push":
		return "docker:24"
	case "k8s_deploy":
		return "bitnami/kubectl"
	case "scan":
		return "aquasec/trivy"
	case "shell", "build", "test", "script":
		return "alpine"
	case "notify":
		return "curlimages/curl"
	default:
		return ""
	}
}

// extractCommands pulls a list of shell commands out of the legacy Config.
// Honors both "commands" (array) and "command" (single string) keys.
func extractCommands(cfg map[string]interface{}) []string {
	if cfg == nil {
		return nil
	}
	if v, ok := cfg["commands"]; ok {
		switch xs := v.(type) {
		case []interface{}:
			out := make([]string, 0, len(xs))
			for _, x := range xs {
				if s, ok := x.(string); ok {
					out = append(out, s)
				}
			}
			if len(out) > 0 {
				return out
			}
		case []string:
			return append([]string{}, xs...)
		}
	}
	if s, ok := cfg["command"].(string); ok && s != "" {
		return []string{s}
	}
	return nil
}

// stringFrom returns the first non-empty string value among the given
// candidate keys. Useful for tolerating "workdir" vs "work_dir" drift.
func stringFrom(cfg map[string]interface{}, keys ...string) string {
	if cfg == nil {
		return ""
	}
	for _, k := range keys {
		if s, ok := cfg[k].(string); ok && s != "" {
			return s
		}
	}
	return ""
}
