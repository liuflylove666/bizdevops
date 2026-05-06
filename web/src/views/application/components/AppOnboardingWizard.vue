<template>
  <a-modal
    :open="open"
    :title="applicationId ? '补齐应用接入' : '应用接入向导'"
    width="880px"
    :confirm-loading="submitting"
    @ok="submit"
    @cancel="emit('update:open', false)"
  >
    <a-steps :current="currentStep" size="small" class="onboarding-steps">
      <a-step title="应用" />
      <a-step title="仓库" />
      <a-step title="环境" />
      <a-step title="流水线" />
    </a-steps>

    <div v-show="currentStep === 0" class="step-body">
      <a-form :model="form.app" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="应用名称" required>
              <a-input v-model:value="form.app.name" :disabled="!!applicationId" placeholder="demo-api" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="显示名称">
              <a-input v-model:value="form.app.display_name" placeholder="Demo API" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="所属组织">
              <a-select v-model:value="form.app.organization_id" placeholder="选择组织" allow-clear @change="onOrganizationChange">
                <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="所属项目">
              <a-select v-model:value="form.app.project_id" placeholder="选择项目" allow-clear>
                <a-select-option v-for="project in filteredProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="语言">
              <a-select v-model:value="form.app.language" allow-clear>
                <a-select-option value="go">Go</a-select-option>
                <a-select-option value="java">Java</a-select-option>
                <a-select-option value="python">Python</a-select-option>
                <a-select-option value="nodejs">Node.js</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="团队">
              <a-input v-model:value="form.app.team" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="负责人">
              <a-input v-model:value="form.app.owner" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.app.description" :rows="2" />
        </a-form-item>
      </a-form>
    </div>

    <div v-show="currentStep === 1" class="step-body">
      <a-form :model="form.repo" layout="vertical">
        <a-segmented v-model:value="repoMode" :options="repoModeOptions" />
        <template v-if="repoMode === 'existing'">
          <a-form-item label="标准 Git 仓库" required class="field-top">
            <a-select v-model:value="form.repo.git_repo_id" placeholder="选择仓库" show-search allow-clear option-filter-prop="label">
              <a-select-option
                v-for="repo in gitRepos"
                :key="repo.id"
                :value="repo.id"
                :label="`${repo.name} ${repo.url}`"
              >
                {{ repo.name }}<span class="muted">（{{ repo.default_branch || 'main' }}）</span>
              </a-select-option>
            </a-select>
          </a-form-item>
        </template>
        <template v-else>
          <a-row :gutter="16" class="field-top">
            <a-col :span="12">
              <a-form-item label="仓库名称">
                <a-input v-model:value="form.repo.name" placeholder="demo-api" />
              </a-form-item>
            </a-col>
            <a-col :span="12">
              <a-form-item label="Provider">
                <a-select v-model:value="form.repo.provider">
                  <a-select-option value="gitlab">GitLab</a-select-option>
                  <a-select-option value="github">GitHub</a-select-option>
                  <a-select-option value="gitee">Gitee</a-select-option>
                  <a-select-option value="custom">Custom</a-select-option>
                </a-select>
              </a-form-item>
            </a-col>
          </a-row>
          <a-form-item label="仓库 URL" required>
            <a-input v-model:value="form.repo.url" placeholder="https://git.example/group/demo-api.git" />
          </a-form-item>
        </template>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="默认分支">
              <a-input v-model:value="form.repo.default_branch" placeholder="main" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="设为主仓库">
              <a-switch v-model:checked="form.repo.is_default" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </div>

    <div v-show="currentStep === 2" class="step-body">
      <a-form :model="form.env" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="环境" required>
              <a-select v-model:value="form.env.env_name">
                <a-select-option value="dev">dev</a-select-option>
                <a-select-option value="test">test</a-select-option>
                <a-select-option value="staging">staging</a-select-option>
                <a-select-option value="prod">prod</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="代码分支">
              <a-input v-model:value="form.env.branch" placeholder="main" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="副本数">
              <a-input-number v-model:value="form.env.replicas" :min="1" style="width: 100%" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="K8s 集群">
              <a-select v-model:value="form.env.k8s_cluster_id" allow-clear show-search option-filter-prop="label">
                <a-select-option v-for="cluster in k8sClusters" :key="cluster.id" :value="cluster.id" :label="cluster.name">
                  {{ cluster.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="Namespace">
              <a-input v-model:value="form.env.k8s_namespace" placeholder="default" />
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="Deployment">
              <a-input v-model:value="form.env.k8s_deployment" placeholder="demo-api" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="GitOps 仓库">
              <a-select v-model:value="form.env.gitops_repo_id" allow-clear show-search option-filter-prop="label">
                <a-select-option v-for="repo in gitOpsRepos" :key="repo.id" :value="repo.id" :label="`${repo.name} ${repo.repo_url}`">
                  {{ repo.name }}<span class="muted">（{{ repo.branch || 'main' }}）</span>
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="GitOps 目录">
              <a-input v-model:value="form.env.gitops_path" placeholder="apps/demo-api" />
            </a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </div>

    <div v-show="currentStep === 3" class="step-body">
      <a-form :model="form.pipeline" layout="vertical">
        <a-form-item label="创建标准流水线">
          <a-switch v-model:checked="form.pipeline.create" />
        </a-form-item>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="流水线名称">
              <a-input v-model:value="form.pipeline.name" :disabled="!form.pipeline.create" />
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="流水线环境">
              <a-input v-model:value="form.pipeline.env" :disabled="!form.pipeline.create" />
            </a-form-item>
          </a-col>
        </a-row>
        <a-form-item label="说明">
          <a-textarea v-model:value="form.pipeline.description" :rows="2" :disabled="!form.pipeline.create" />
        </a-form-item>
      </a-form>
    </div>

    <template #footer>
      <a-space>
        <a-button @click="emit('update:open', false)">取消</a-button>
        <a-button :disabled="currentStep === 0" @click="currentStep -= 1">上一步</a-button>
        <a-button v-if="currentStep < 3" type="primary" @click="nextStep">下一步</a-button>
        <a-button v-else type="primary" :loading="submitting" @click="submit">完成接入</a-button>
      </a-space>
    </template>
  </a-modal>
</template>

<script setup lang="ts">
import { computed, reactive, ref, watch } from 'vue'
import { message } from 'ant-design-vue'
import { applicationApi, type Application, type ApplicationEnv, type ApplicationOnboardingRequest, type ApplicationOnboardingResponse } from '@/services/application'
import { gitRepoApi } from '@/services/pipeline'
import { argocdApi, type GitOpsRepo } from '@/services/argocd'
import { k8sClusterApi } from '@/services/k8s'
import { catalogApi, type Organization, type Project } from '@/services/catalog'
import type { K8sCluster } from '@/types'

const props = defineProps<{
  open: boolean
  application?: Application | null
}>()

const emit = defineEmits<{
  'update:open': [value: boolean]
  success: [value: ApplicationOnboardingResponse]
}>()

const currentStep = ref(0)
const submitting = ref(false)
const repoMode = ref<'existing' | 'new'>('existing')
const gitRepos = ref<any[]>([])
const gitOpsRepos = ref<GitOpsRepo[]>([])
const k8sClusters = ref<K8sCluster[]>([])
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])

const applicationId = ref<number | undefined>()
const filteredProjects = computed(() => form.app.organization_id
  ? projects.value.filter(project => project.organization_id === form.app.organization_id)
  : projects.value)
const repoModeOptions = [
  { label: '选择已有仓库', value: 'existing' },
  { label: '登记新仓库', value: 'new' },
]

const form = reactive<{
  app: Partial<Application>
  repo: {
    git_repo_id?: number
    name: string
    url: string
    provider: string
    default_branch: string
    role: string
    is_default: boolean
  }
  env: Partial<ApplicationEnv>
  pipeline: {
    create: boolean
    name: string
    description: string
    env: string
  }
}>({
  app: {},
  repo: {
    git_repo_id: undefined,
    name: '',
    url: '',
    provider: 'gitlab',
    default_branch: 'main',
    role: 'primary',
    is_default: true,
  },
  env: {
    env_name: 'test',
    branch: 'main',
    replicas: 1,
  },
  pipeline: {
    create: true,
    name: '',
    description: '',
    env: 'test',
  },
})

watch(() => props.open, async (open) => {
  if (!open) return
  resetForm()
  await loadOptions()
}, { immediate: true })

watch(() => form.app.name, (name) => {
  const appName = String(name || '').trim()
  if (!applicationId.value && appName && !form.app.display_name) {
    form.app.display_name = appName
  }
  if (appName) {
    form.repo.name = form.repo.name || appName
    form.env.k8s_deployment = form.env.k8s_deployment || appName
    form.env.gitops_path = form.env.gitops_path || `apps/${appName}`
    form.pipeline.name = form.pipeline.name || `${appName}-test-ci`
  }
})

watch(() => form.env.env_name, (env) => {
  const envName = String(env || 'test')
  form.pipeline.env = envName
  if (form.app.name) {
    form.pipeline.name = `${form.app.name}-${envName}-ci`
  }
})

function resetForm() {
  const app = props.application || null
  applicationId.value = app?.id
  currentStep.value = 0
  repoMode.value = 'existing'
  Object.assign(form.app, {
    name: app?.name || '',
    display_name: app?.display_name || '',
    description: app?.description || '',
    organization_id: app?.organization_id,
    project_id: app?.project_id,
    git_repo: app?.git_repo || '',
    language: app?.language || 'go',
    framework: app?.framework || '',
    team: app?.team || '',
    owner: app?.owner || '',
    status: app?.status || 'active',
  })
  Object.assign(form.repo, {
    git_repo_id: undefined,
    name: app?.name || '',
    url: app?.git_repo || '',
    provider: 'gitlab',
    default_branch: 'main',
    role: 'primary',
    is_default: true,
  })
  Object.assign(form.env, {
    env_name: 'test',
    branch: 'main',
    gitops_repo_id: undefined,
    k8s_cluster_id: undefined,
    k8s_namespace: app?.name || '',
    k8s_deployment: app?.name || '',
    gitops_path: app?.name ? `apps/${app.name}` : '',
    replicas: 1,
  })
  Object.assign(form.pipeline, {
    create: true,
    name: app?.name ? `${app.name}-test-ci` : '',
    description: '',
    env: 'test',
  })
}

async function loadOptions() {
  const [gitRepoRes, gitOpsRepoRes, clusterRes, orgRes, projectRes] = await Promise.allSettled([
    gitRepoApi.list({ page: 1, page_size: 200 }),
    argocdApi.listRepos({ page: 1, page_size: 200 }) as any,
    k8sClusterApi.list(),
    catalogApi.listOrgs(),
    catalogApi.listProjects(),
  ])
  if (gitRepoRes.status === 'fulfilled') {
    const data = gitRepoRes.value?.data || {}
    gitRepos.value = data.items || data.list || []
  }
  if (gitOpsRepoRes.status === 'fulfilled') {
    const data = gitOpsRepoRes.value?.data || {}
    gitOpsRepos.value = data.items || data.list || []
  }
  if (clusterRes.status === 'fulfilled') {
    k8sClusters.value = clusterRes.value?.data?.items || []
  }
  if (orgRes.status === 'fulfilled') {
    organizations.value = orgRes.value?.data || []
  }
  if (projectRes.status === 'fulfilled') {
    projects.value = projectRes.value?.data || []
  }
}

function onOrganizationChange() {
  const currentProjectId = form.app.project_id
  if (!currentProjectId) return
  const matched = projects.value.find(project => project.id === currentProjectId)
  if (!matched || matched.organization_id !== form.app.organization_id) {
    form.app.project_id = undefined
  }
}

function nextStep() {
  if (currentStep.value === 0 && !String(form.app.name || '').trim()) {
    message.error('请填写应用名称')
    return
  }
  if (currentStep.value === 1) {
    if (repoMode.value === 'existing' && !form.repo.git_repo_id) {
      message.error('请选择 Git 仓库')
      return
    }
    if (repoMode.value === 'new' && !String(form.repo.url || '').trim()) {
      message.error('请填写仓库 URL')
      return
    }
  }
  currentStep.value += 1
}

async function submit() {
  if (!String(form.app.name || '').trim()) {
    currentStep.value = 0
    message.error('请填写应用名称')
    return
  }

  const repo = repoMode.value === 'existing'
    ? { git_repo_id: form.repo.git_repo_id, role: form.repo.role, is_default: form.repo.is_default, default_branch: form.repo.default_branch }
    : { name: form.repo.name, url: form.repo.url, provider: form.repo.provider, default_branch: form.repo.default_branch, role: form.repo.role, is_default: form.repo.is_default }

  const payload: ApplicationOnboardingRequest = {
    application_id: applicationId.value,
    app: form.app,
    repo,
    env: form.env,
    pipeline: form.pipeline,
  }

  submitting.value = true
  try {
    const res = await applicationApi.saveOnboarding(payload)
    if (res.code === 0 && res.data) {
      message.success(applicationId.value ? '接入信息已补齐' : '应用接入完成')
      emit('success', res.data)
      emit('update:open', false)
    } else {
      message.error(res.message || '接入保存失败')
    }
  } catch (e: any) {
    message.error(e.message || '接入保存失败')
  } finally {
    submitting.value = false
  }
}
</script>

<style scoped>
.onboarding-steps {
  margin-bottom: 20px;
}
.step-body {
  min-height: 340px;
}
.field-top {
  margin-top: 16px;
}
.muted {
  color: #8c8c8c;
  margin-left: 6px;
}
</style>
