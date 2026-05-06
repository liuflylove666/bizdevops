// Package infrastructure 定义基础设施相关的数据模型
// 本文件包含 Kubernetes 集群相关的模型定义
package infrastructure

import (
	"gorm.io/gorm"
)

// ==================== K8s 集群模型 ====================

// K8sCluster K8s集群模型
// 存储 Kubernetes 集群的连接配置
type K8sCluster struct {
	gorm.Model
	Name            string `gorm:"size:100;not null" json:"name"`                        // 集群名称
	Kubeconfig      string `gorm:"type:text;not null" json:"kubeconfig"`                 // kubeconfig 配置内容
	Namespace       string `gorm:"size:100;default:'default';not null" json:"namespace"` // 默认命名空间
	Registry        string `gorm:"size:500" json:"registry"`                             // 镜像仓库地址
	Repository      string `gorm:"size:200" json:"repository"`                           // 镜像仓库名称
	Description     string `gorm:"type:text" json:"description"`                         // 描述
	Status          string `gorm:"size:20;default:'active';not null" json:"status"`      // 状态
	IsDefault       bool   `gorm:"default:false" json:"is_default"`                      // 是否默认集群
	InsecureSkipTLS bool   `gorm:"default:false" json:"insecure_skip_tls"`               // 跳过 TLS 证书验证
	CheckTimeout    int    `gorm:"default:180;not null" json:"check_timeout"`            // 健康检查超时时间(秒)
	CreatedBy       *uint  `gorm:"index" json:"created_by"`
	UpdatedBy       *uint  `gorm:"index" json:"updated_by"`
}

// TableName 指定表名
func (K8sCluster) TableName() string {
	return "k8s_clusters"
}
