import { ref, watch } from 'vue'

export interface RequirementColumnOption {
  label: string
  value: string
}

export function useBizRequirementColumnPrefs(
  storageKey: string,
  options: RequirementColumnOption[],
  defaultValues: string[]
) {
  const allowed = new Set(options.map(item => item.value))

  const load = (): string[] => {
    const fallback = [...defaultValues]
    if (typeof window === 'undefined') return fallback
    try {
      const raw = window.localStorage.getItem(storageKey)
      if (!raw) return fallback
      const parsed = JSON.parse(raw)
      if (!Array.isArray(parsed)) return fallback
      const filtered = parsed.filter((item: unknown): item is string => typeof item === 'string' && allowed.has(item))
      return filtered.length > 0 ? filtered : fallback
    } catch {
      return fallback
    }
  }

  const selectedOptionalColumns = ref<string[]>(load())

  watch(selectedOptionalColumns, (values) => {
    if (typeof window === 'undefined') return
    window.localStorage.setItem(storageKey, JSON.stringify(values))
  })

  const resetOptionalColumns = () => {
    selectedOptionalColumns.value = [...defaultValues]
  }

  return {
    selectedOptionalColumns,
    resetOptionalColumns,
  }
}
