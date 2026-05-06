package dto

// ==================== 部署前置检查 DTO ====================

// DeployPreCheckRequest 部署前置检查请求
type DeployPreCheckRequest struct {
	ApplicationID uint   `json:"application_id" binding:"required"`
	EnvName       string `json:"env_name" binding:"required"`
	ImageTag      string `json:"image_tag"`
}

// DeployPreCheckResponse 部署前置检查响应
type DeployPreCheckResponse struct {
	CanDeploy bool           `json:"can_deploy"`
	Checks    []PreCheckItem `json:"checks"`
	Warnings  []string       `json:"warnings"`
	Errors    []string       `json:"errors"`
}

// PreCheckItem 检查项
type PreCheckItem struct {
	Name    string `json:"name"`
	Status  string `json:"status"` // passed, warning, failed, skipped
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}
