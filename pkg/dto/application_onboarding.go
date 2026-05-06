package dto

type ApplicationOnboardingRequest struct {
	ApplicationID *uint                               `json:"application_id,omitempty"`
	App           ApplicationOnboardingAppInput       `json:"app"`
	Repo          *ApplicationOnboardingRepoInput     `json:"repo,omitempty"`
	Env           *ApplicationOnboardingEnvInput      `json:"env,omitempty"`
	Pipeline      *ApplicationOnboardingPipelineInput `json:"pipeline,omitempty"`
}

type ApplicationOnboardingAppInput struct {
	Name           string `json:"name"`
	DisplayName    string `json:"display_name"`
	Description    string `json:"description"`
	OrganizationID *uint  `json:"organization_id,omitempty"`
	ProjectID      *uint  `json:"project_id,omitempty"`
	GitRepo        string `json:"git_repo"`
	Language       string `json:"language"`
	Framework      string `json:"framework"`
	Team           string `json:"team"`
	Owner          string `json:"owner"`
	Status         string `json:"status"`
}

type ApplicationOnboardingRepoInput struct {
	GitRepoID     *uint  `json:"git_repo_id,omitempty"`
	Name          string `json:"name"`
	URL           string `json:"url"`
	Provider      string `json:"provider"`
	DefaultBranch string `json:"default_branch"`
	Role          string `json:"role"`
	IsDefault     *bool  `json:"is_default,omitempty"`
}

type ApplicationOnboardingEnvInput struct {
	EnvName             string `json:"env_name"`
	Branch              string `json:"branch"`
	GitOpsRepoID        *uint  `json:"gitops_repo_id,omitempty"`
	ArgoCDApplicationID *uint  `json:"argocd_application_id,omitempty"`
	GitOpsBranch        string `json:"gitops_branch"`
	GitOpsPath          string `json:"gitops_path"`
	HelmChartPath       string `json:"helm_chart_path"`
	HelmValuesPath      string `json:"helm_values_path"`
	HelmReleaseName     string `json:"helm_release_name"`
	K8sClusterID        *uint  `json:"k8s_cluster_id,omitempty"`
	K8sNamespace        string `json:"k8s_namespace"`
	K8sDeployment       string `json:"k8s_deployment"`
	Replicas            int    `json:"replicas"`
	CPURequest          string `json:"cpu_request"`
	CPULimit            string `json:"cpu_limit"`
	MemoryRequest       string `json:"memory_request"`
	MemoryLimit         string `json:"memory_limit"`
	Config              string `json:"config"`
}

type ApplicationOnboardingPipelineInput struct {
	PipelineID       *uint  `json:"pipeline_id,omitempty"`
	Create           bool   `json:"create"`
	Name             string `json:"name"`
	Description      string `json:"description"`
	Env              string `json:"env"`
	SourceTemplateID *uint  `json:"source_template_id,omitempty"`
	GitRepoID        *uint  `json:"git_repo_id,omitempty"`
	GitBranch        string `json:"git_branch"`
}

type ApplicationOnboardingResponse struct {
	ApplicationID   uint                          `json:"application_id"`
	ApplicationName string                        `json:"application_name"`
	Created         bool                          `json:"created"`
	RepoBindingID   *uint                         `json:"repo_binding_id,omitempty"`
	GitRepoID       *uint                         `json:"git_repo_id,omitempty"`
	EnvID           *uint                         `json:"env_id,omitempty"`
	PipelineID      *uint                         `json:"pipeline_id,omitempty"`
	UpdatedSections []string                      `json:"updated_sections"`
	Readiness       *ApplicationReadinessResponse `json:"readiness,omitempty"`
}
