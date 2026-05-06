package deploy

import "time"

// EnvInstance 环境实例：应用在某个环境的具体运行实例
type EnvInstance struct {
	ID              uint       `gorm:"primarykey" json:"id"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	ApplicationID   uint       `gorm:"not null;index" json:"application_id"`
	ApplicationName string     `gorm:"size:100" json:"application_name"`
	Env             string     `gorm:"size:30;not null;index" json:"env"` // dev/test/uat/gray/prod
	ClusterID       *uint      `gorm:"index" json:"cluster_id"`
	ClusterName     string     `gorm:"size:100" json:"cluster_name"`
	Namespace       string     `gorm:"size:100" json:"namespace"`
	DeploymentName  string     `gorm:"size:200" json:"deployment_name"`
	ImageURL        string     `gorm:"size:1000" json:"image_url"`
	ImageTag        string     `gorm:"size:500" json:"image_tag"`
	ImageDigest     string     `gorm:"size:200" json:"image_digest"` // sha256 digest 强制校验
	Replicas        int        `gorm:"default:1" json:"replicas"`
	Status          string     `gorm:"size:20;default:'unknown';index" json:"status"` // running/stopped/deploying/failed/unknown
	LastDeployAt    *time.Time `json:"last_deploy_at"`
	LastDeployBy    string     `gorm:"size:100" json:"last_deploy_by"`
	NacosInstanceID *uint      `json:"nacos_instance_id"`
	NacosTenant     string     `gorm:"size:200" json:"nacos_tenant"`
	NacosGroup      string     `gorm:"size:200" json:"nacos_group"`
	DBInstanceID    *uint      `json:"db_instance_id"`
	DBInstanceName  string     `gorm:"size:100" json:"db_instance_name"`
	ConfigHash      string     `gorm:"size:64" json:"config_hash"` // 运行中配置的哈希
	Metadata        string     `gorm:"type:text" json:"metadata"`  // JSON 扩展
}

func (EnvInstance) TableName() string { return "env_instances" }
