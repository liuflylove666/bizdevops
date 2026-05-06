package types

import (
	"database/sql/driver"
	"encoding/json"
)

// JSONMap 通用 JSON Map 类型（供 model 层使用，避免与 internal/models 的聚合 import 形成环）
type JSONMap map[string]any

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = make(map[string]any)
		return nil
	}
	bytes, ok := value.([]byte)
	if !ok {
		return nil
	}
	return json.Unmarshal(bytes, m)
}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return json.Marshal(map[string]any{})
	}
	return json.Marshal(m)
}
