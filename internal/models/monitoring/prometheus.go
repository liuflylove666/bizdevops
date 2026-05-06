package monitoring

import "time"

// PrometheusInstance Prometheus 数据源实例
type PrometheusInstance struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Name      string    `gorm:"size:100;not null;uniqueIndex" json:"name"`
	URL       string    `gorm:"size:500;not null" json:"url"` // e.g. http://prometheus:9090
	AuthType  string    `gorm:"size:20;default:'none'" json:"auth_type"` // none / basic / bearer
	Username  string    `gorm:"size:200" json:"username"`
	Password  string    `gorm:"size:500" json:"password"` // 加密存储(basic auth) or bearer token
	IsDefault bool      `gorm:"default:false" json:"is_default"`
	Status    string    `gorm:"size:20;default:'active'" json:"status"`
	CreatedBy *uint     `json:"created_by"`
}

func (PrometheusInstance) TableName() string { return "prometheus_instances" }
