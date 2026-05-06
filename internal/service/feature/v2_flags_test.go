package feature

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// 纯逻辑测试。依赖数据库的 SeedV2Flags 完整行为由集成测试覆盖（需要 MySQL）。

func TestV2Flags_AllNamesUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for _, spec := range V2Flags {
		_, dup := seen[spec.Name]
		assert.Falsef(t, dup, "v2.0 flag 名称重复: %s", spec.Name)
		seen[spec.Name] = struct{}{}
	}
}

func TestV2Flags_NamingConvention(t *testing.T) {
	// 全部小写 + snake_case + 至少包含一个域前缀点
	for _, spec := range V2Flags {
		assert.Containsf(t, spec.Name, ".", "flag 名缺少域前缀: %s", spec.Name)
		assert.NotEmptyf(t, spec.DisplayName, "flag DisplayName 不可为空: %s", spec.Name)
		assert.NotEmptyf(t, spec.Description, "flag Description 不可为空: %s", spec.Name)
	}
}

func TestV2Flags_ConstantsCoverAllSpecs(t *testing.T) {
	// 清单中必须包含所有 v2 常量，避免漏声明
	constants := []string{}

	specNames := make(map[string]struct{}, len(V2Flags))
	for _, s := range V2Flags {
		specNames[s.Name] = struct{}{}
	}

	for _, name := range constants {
		_, ok := specNames[name]
		assert.Truef(t, ok, "常量 %q 未出现在 V2Flags 清单中", name)
	}
	assert.Equalf(t, len(constants), len(V2Flags), "V2Flags 清单数量应与常量数量一致（当前 spec=%d，constants=%d）", len(V2Flags), len(constants))
}

func TestSeedV2Flags_NilDB(t *testing.T) {
	err := SeedV2Flags(context.TODO(), nil)
	assert.Error(t, err, "传 nil DB 必须返回错误而不是 panic")
}
