// Cross-view focus context (Sprint 2 FE-09).
//
// Tracks "what the user is currently working on" — application, environment,
// git branch, and optionally pipeline. Views populate it as the user
// navigates (e.g., PipelineDetail records the pipeline + branch on mount);
// the top-bar ContextPill (FE-10) reads it back so users feel that
// switching pages keeps their context.
//
// Persistence: localStorage, same pattern as recents/favorite stores.
// Sentinel values: `null` for cleared, never empty string — distinguishes
// "explicitly cleared" from "never set" should we need that nuance later.

import { defineStore } from 'pinia'
import { ref, watch, computed } from 'vue'

export interface AppRef {
  id: number
  name: string
}

export interface PipelineRef {
  id: number
  name: string
}

const STORAGE_KEY = 'devops_focus_context'

interface PersistedContext {
  app?: AppRef | null
  env?: string | null
  branch?: string | null
  pipeline?: PipelineRef | null
}

const safeRead = (): PersistedContext => {
  if (typeof window === 'undefined') return {}
  try {
    const raw = localStorage.getItem(STORAGE_KEY)
    return raw ? (JSON.parse(raw) as PersistedContext) : {}
  } catch {
    return {}
  }
}

const safeWrite = (value: PersistedContext) => {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(value))
  } catch {
    /* quota or disabled — ignore */
  }
}

export const useContextStore = defineStore('focusContext', () => {
  const initial = safeRead()
  const app = ref<AppRef | null>(initial.app ?? null)
  const env = ref<string | null>(initial.env ?? null)
  const branch = ref<string | null>(initial.branch ?? null)
  const pipeline = ref<PipelineRef | null>(initial.pipeline ?? null)

  const persist = () =>
    safeWrite({
      app: app.value,
      env: env.value,
      branch: branch.value,
      pipeline: pipeline.value,
    })

  watch([app, env, branch, pipeline], persist, { deep: true })

  /** True when any focus dimension is set; the pill renders only when true. */
  const hasAny = computed(() =>
    !!(app.value || env.value || branch.value || pipeline.value),
  )

  /**
   * Compact human-readable summary, e.g., "order-svc · staging · main".
   * Empty string when nothing is set; suitable for hiding the pill.
   */
  const summary = computed(() => {
    const parts: string[] = []
    if (app.value?.name) parts.push(app.value.name)
    if (pipeline.value?.name && pipeline.value.name !== app.value?.name) {
      parts.push(pipeline.value.name)
    }
    if (env.value) parts.push(env.value)
    if (branch.value) parts.push(branch.value)
    return parts.join(' · ')
  })

  /**
   * Update one or more fields at once. Pass `null` to clear an individual
   * field; omit a field to leave it unchanged. Use clearAll() to reset.
   */
  const update = (
    patch: Partial<{ app: AppRef | null; env: string | null; branch: string | null; pipeline: PipelineRef | null }>,
  ) => {
    if ('app' in patch) app.value = patch.app ?? null
    if ('env' in patch) env.value = patch.env ?? null
    if ('branch' in patch) branch.value = patch.branch ?? null
    if ('pipeline' in patch) pipeline.value = patch.pipeline ?? null
  }

  const clearAll = () => {
    app.value = null
    env.value = null
    branch.value = null
    pipeline.value = null
  }

  return {
    app,
    env,
    branch,
    pipeline,
    hasAny,
    summary,
    update,
    clearAll,
  }
})
