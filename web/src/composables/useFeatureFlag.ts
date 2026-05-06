import { computed, ref, type ComputedRef, type Ref } from 'vue'
import { featureFlagApi, type FeatureFlag } from '@/services/featureFlag'

/**
 * 前端 Feature Flag 消费入口。
 *
 * 单一真相源：docs/roadmap/v2.0-feature-flags.md
 * 对应后端常量：internal/service/feature/v2_flags.go
 *
 * 使用方式：
 *   const enabled = useFeatureFlag('some.feature_flag')
 *   if (enabled.value) { ... }
 *
 * 初始化（在 App.vue 或 main.ts 中）：
 *   await loadFeatureFlags()
 */

// ---- v2.0 Flag 常量（与后端保持一致） ------------------------------------
export const FEATURE_FLAGS = {
  // E7-06 后 release 域 Flag 已清理，保留对象用于后续扩展。
} as const

export type FeatureFlagName = (typeof FEATURE_FLAGS)[keyof typeof FEATURE_FLAGS]

// ---- 模块级 Store ---------------------------------------------------------
// 首次 loadFeatureFlags 之前，全部查询返回 false（保守路径）。
const flagMap: Ref<Map<string, FeatureFlag>> = ref(new Map())
const isLoaded = ref(false)
const isLoading = ref(false)

interface DevOverride {
  value: boolean
  reason: string
}

const devOverrides: Map<string, DevOverride> = new Map()

// 本地开发 URL query 支持：?ff_some.feature_flag=1
function applyQueryStringOverrides() {
  if (typeof window === 'undefined') return
  try {
    const params = new URLSearchParams(window.location.search)
    params.forEach((value, key) => {
      if (!key.startsWith('ff_')) return
      const flagName = key.slice(3)
      const bool = value === '1' || value === 'true'
      devOverrides.set(flagName, { value: bool, reason: 'URL query' })
    })
  } catch {
    // ignore
  }
}

/**
 * 检查某个 Flag 是否开启。
 *
 * 优先级：
 *   1. 开发覆盖（URL query / localStorage）
 *   2. 后端 Rollout 结果（rollout_percentage == 100 或 is_enabled 且落在灰度桶内）
 *   3. 未加载时返回 false（保守策略）
 */
export function isFlagEnabled(name: string): boolean {
  const override = devOverrides.get(name)
  if (override) return override.value

  const flag = flagMap.value.get(name)
  if (!flag) return false
  if (!flag.is_enabled) return false
  if (flag.rollout_percentage >= 100) return true
  // 对于白名单与用户级灰度，前端无法准确判断（需后端 check 接口）。
  // 这里只做"全量开启"的粗粒度判断。精细灰度请改用 checkFlag()。
  return false
}

/**
 * 以响应式 ComputedRef 形式读取 Flag。适用于 <template> 和 setup 作用域。
 */
export function useFeatureFlag(name: string): ComputedRef<boolean> {
  return computed(() => {
    // 依赖 flagMap.value（ref）以保持响应性
    void flagMap.value
    return isFlagEnabled(name)
  })
}

/**
 * 从后端加载全部 Feature Flag。
 * - 幂等，可重复调用（用于登录后刷新）
 * - 失败时保留既有状态，并在 console 报 warn
 */
export async function loadFeatureFlags(force = false): Promise<void> {
  if (isLoaded.value && !force) return
  if (isLoading.value) return
  isLoading.value = true
  applyQueryStringOverrides()
  try {
    const resp = await featureFlagApi.list({ silent: true })
    const items = resp?.data?.items || []
    const next = new Map<string, FeatureFlag>()
    for (const item of items) next.set(item.name, item)
    flagMap.value = next
    isLoaded.value = true
  } catch (err) {
    // eslint-disable-next-line no-console
    console.warn('[feature-flag] 加载失败，所有 Flag 视为关闭：', err)
  } finally {
    isLoading.value = false
  }
}

/**
 * 开发调试：强制覆盖某个 Flag。
 * 仅影响前端判断，不调用后端。
 */
export function overrideFlag(name: string, value: boolean, reason = 'manual'): void {
  devOverrides.set(name, { value, reason })
  // 触发响应式更新
  flagMap.value = new Map(flagMap.value)
}

/**
 * 清除开发覆盖。
 */
export function clearOverrides(): void {
  devOverrides.clear()
  flagMap.value = new Map(flagMap.value)
}

/**
 * 对外暴露已加载状态（用于启动页 loading 守卫）。
 */
export function useFeatureFlagReady(): ComputedRef<boolean> {
  return computed(() => isLoaded.value)
}
