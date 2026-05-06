// Recently-visited pipelines for the top-bar PipelineSwitcher (Sprint 1 FE-01).
//
// Two persisted lists, both backed by localStorage:
//   - recents: FIFO, capped at MAX_RECENTS, deduped by pipeline id
//   - pinned:  user-toggleable, no cap, ordered by pin time (newest first)
//
// Visits are recorded by callers (typically PipelineDetail.vue on mount).
// The store does not fetch — it only remembers what callers feed it.

import { defineStore } from 'pinia'
import { ref, watch } from 'vue'

export interface PipelineRef {
  id: number
  name: string
  description?: string
}

const RECENTS_KEY = 'devops_pipeline_recents'
const PINNED_KEY = 'devops_pipeline_pinned'
const MAX_RECENTS = 5

const safeRead = <T>(key: string): T | null => {
  if (typeof window === 'undefined') return null
  try {
    const raw = localStorage.getItem(key)
    return raw ? (JSON.parse(raw) as T) : null
  } catch {
    return null
  }
}

const safeWrite = (key: string, value: unknown) => {
  if (typeof window === 'undefined') return
  try {
    localStorage.setItem(key, JSON.stringify(value))
  } catch {
    /* quota exceeded or disabled — silently noop */
  }
}

export const useRecentsStore = defineStore('pipelineRecents', () => {
  const recents = ref<PipelineRef[]>(safeRead<PipelineRef[]>(RECENTS_KEY) ?? [])
  const pinned = ref<PipelineRef[]>(safeRead<PipelineRef[]>(PINNED_KEY) ?? [])

  watch(recents, (v) => safeWrite(RECENTS_KEY, v), { deep: true })
  watch(pinned, (v) => safeWrite(PINNED_KEY, v), { deep: true })

  /** Record a pipeline visit. Bubbles to the top, dedupes by id, caps at 5. */
  const addRecent = (p: PipelineRef) => {
    if (!p || !p.id) return
    const existing = recents.value.findIndex((x) => x.id === p.id)
    if (existing !== -1) recents.value.splice(existing, 1)
    recents.value.unshift({ id: p.id, name: p.name, description: p.description })
    if (recents.value.length > MAX_RECENTS) {
      recents.value.length = MAX_RECENTS
    }
  }

  /** Toggle a pipeline's pinned state. Returns the new pinned state. */
  const togglePin = (p: PipelineRef): boolean => {
    const idx = pinned.value.findIndex((x) => x.id === p.id)
    if (idx !== -1) {
      pinned.value.splice(idx, 1)
      return false
    }
    pinned.value.unshift({ id: p.id, name: p.name, description: p.description })
    return true
  }

  const isPinned = (id: number): boolean => pinned.value.some((x) => x.id === id)

  /** Forget a pipeline entirely (e.g., when user deletes it). */
  const forget = (id: number) => {
    recents.value = recents.value.filter((x) => x.id !== id)
    pinned.value = pinned.value.filter((x) => x.id !== id)
  }

  /** Clear all recents (keeps pinned). Used by the dropdown's "清空" link. */
  const clearRecents = () => {
    recents.value = []
  }

  return {
    recents,
    pinned,
    addRecent,
    togglePin,
    isPinned,
    forget,
    clearRecents,
  }
})
