<!--
  MentionTextarea (Sprint 1 FE-02).

  Drop-in replacement for `<a-textarea>` that adds an `@` mention picker
  for pipeline variables + credentials. Built on top of AntDV's
  `<a-mentions>` so we inherit caret-following popover, arrow navigation,
  and selection insertion for free.

  Usage:
    <MentionTextarea v-model:value="step.command" :pipeline-id="pid" :rows="3" />

  On select the inserted text is `@<name>`. The backend variable
  interpolation (project convention) is responsible for resolving these
  references at execution time; this component is a typing aid only.

  Lazy-loads candidates on first focus to avoid an extra round-trip when
  the user does not need the picker. Once loaded, the result is cached
  for the lifetime of the component instance.
-->
<template>
  <a-mentions
    v-bind="$attrs"
    :value="value"
    :rows="rows"
    :placeholder="placeholderWithHint"
    :prefix="['@']"
    :loading="loading"
    :options="options"
    :filter-option="filterOption"
    :notFoundContent="emptyContent"
    @update:value="onUpdate"
    @focus="onFocus"
  />
</template>

<script setup lang="ts">
import { computed, ref } from 'vue'
import { pipelineApi } from '@/services/pipeline'

interface Props {
  value: string
  pipelineId?: number
  rows?: number
  placeholder?: string
}

const props = withDefaults(defineProps<Props>(), {
  rows: 3,
  placeholder: '',
})

const emit = defineEmits<{
  (e: 'update:value', v: string): void
  (e: 'change', v: string): void
}>()

interface MentionOption {
  value: string
  label: string
  /** Free-form group label rendered inline (AntDV mentions does not group natively). */
  kind: 'variable' | 'credential'
  /** Brief annotation appended to the option label, e.g. "(secret)" or scope. */
  hint?: string
}

const loading = ref(false)
const loaded = ref(false)
const variables = ref<MentionOption[]>([])
const credentials = ref<MentionOption[]>([])

// Combined options handed to <a-mentions>. Variables first (more frequently
// used), credentials second, both alpha-sorted within their group.
const options = computed(() => {
  const opts: Array<{ value: string; label: string }> = []
  for (const v of variables.value) {
    opts.push({ value: v.value, label: `${v.label}${v.hint ? '  ' + v.hint : ''}` })
  }
  for (const c of credentials.value) {
    opts.push({ value: c.value, label: `${c.label}${c.hint ? '  ' + c.hint : ''}` })
  }
  return opts
})

const placeholderWithHint = computed(() => {
  if (props.placeholder) return props.placeholder + '（输入 @ 引用变量/凭证）'
  return '输入 @ 引用变量或凭证'
})

const emptyContent = computed(() => {
  if (loading.value) return '加载中…'
  if (!loaded.value) return '聚焦后加载'
  if (!options.value.length) return '该流水线暂无变量/凭证'
  return '无匹配项'
})

// Loose contains-match; AntDV passes the typed query (without prefix) and
// the option. We match on both value and label so users can type either
// the variable name or any human-readable label fragment.
const filterOption = (input: string, option: { value: string; label: string }) => {
  if (!input) return true
  const q = input.toLowerCase()
  return (
    option.value.toLowerCase().includes(q) ||
    (option.label || '').toLowerCase().includes(q)
  )
}

const onUpdate = (v: string) => {
  emit('update:value', v)
  emit('change', v)
}

// Lazy-load on first focus to avoid hitting the API on every step in the
// designer. A pipeline can have dozens of steps but only a few will edit
// command fields per session.
const onFocus = async () => {
  if (loaded.value || loading.value) return
  loading.value = true
  try {
    const [varsRes, credsRes]: [any, any] = await Promise.all([
      pipelineApi.getVariables(props.pipelineId ? { pipeline_id: props.pipelineId } : undefined),
      pipelineApi.getCredentials(),
    ])

    const rawVars = (varsRes?.data?.items || varsRes?.data || []) as Array<{
      name: string; scope?: string; is_secret?: boolean
    }>
    variables.value = rawVars
      .filter((v) => v && v.name)
      .map((v) => ({
        value: v.name,
        label: v.name,
        kind: 'variable' as const,
        hint: v.is_secret ? '🔒 secret' : v.scope ? `(${v.scope})` : '',
      }))
      .sort((a, b) => a.value.localeCompare(b.value))

    const rawCreds = (credsRes?.data?.items || credsRes?.data || []) as Array<{
      name: string; type?: string
    }>
    credentials.value = rawCreds
      .filter((c) => c && c.name)
      .map((c) => ({
        value: c.name,
        label: c.name,
        kind: 'credential' as const,
        hint: c.type ? `🔑 ${c.type}` : '🔑 cred',
      }))
      .sort((a, b) => a.value.localeCompare(b.value))

    loaded.value = true
  } catch {
    // Silent fail — popover just shows "无匹配项". Don't toast: typing in a
    // textarea triggering an error toast every focus would be annoying.
    variables.value = []
    credentials.value = []
    loaded.value = true
  } finally {
    loading.value = false
  }
}
</script>
