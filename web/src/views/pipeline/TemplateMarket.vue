<template>
  <div class="template-market">
    <div class="market-layout">
      <!-- 左侧：分类树 -->
      <div class="category-sidebar">
        <a-card title="分类" :bordered="false" size="small">
          <a-tree
            v-model:selectedKeys="selectedCategories"
            :tree-data="categoryTree"
            :show-icon="true"
            @select="onCategorySelect"
          >
            <template #icon="{ data }">
              <FolderOutlined v-if="data?.children" />
              <FileOutlined v-else />
            </template>
          </a-tree>
        </a-card>

        <!-- 标签筛选 -->
        <a-card title="标签" :bordered="false" size="small" style="margin-top: 16px">
          <a-checkbox-group v-model:value="selectedTags" @change="onTagChange">
            <div v-for="tag in availableTags" :key="tag" style="margin-bottom: 8px">
              <a-checkbox :value="tag">{{ tag }}</a-checkbox>
            </div>
          </a-checkbox-group>
        </a-card>

        <!-- 收藏夹 -->
        <a-card title="我的收藏" :bordered="false" size="small" style="margin-top: 16px">
          <a-button type="link" block @click="showFavorites">
            <StarFilled style="color: #faad14" /> 查看收藏 ({{ favoriteCount }})
          </a-button>
        </a-card>
      </div>

      <!-- 右侧：模板列表 -->
      <div class="template-content">
        <a-card :bordered="false">
          <template #title>
            <a-space>
              <span>{{ showFavoritesOnly ? '我的收藏' : (showMyTemplates ? '我的模板' : '模板市场') }}</span>
              <a-tag color="blue">{{ visibleTotal }} 个模板</a-tag>
            </a-space>
          </template>
          <template #extra>
            <a-space>
              <a-button @click="showMyTemplates = !showMyTemplates">
                {{ showMyTemplates ? '返回市场' : '我的模板' }}
              </a-button>
              <a-dropdown>
                <a-button type="primary">
                  <PlusOutlined /> 创建模板 <DownOutlined />
                </a-button>
                <template #overlay>
                  <a-menu @click="handleCreateMenu">
                    <a-menu-item key="blank">
                      <FileAddOutlined /> 空白模板
                    </a-menu-item>
                    <a-menu-item key="from-pipeline">
                      <CopyOutlined /> 从现有流水线创建
                    </a-menu-item>
                  </a-menu>
                </template>
              </a-dropdown>
            </a-space>
          </template>

          <!-- 搜索和筛选栏 -->
          <div style="margin-bottom: 16px">
            <a-space wrap style="width: 100%">
              <a-input-search
                v-model:value="filter.keyword"
                placeholder="搜索模板名称、描述、标签..."
                style="width: 300px"
                @search="fetchTemplates"
                allowClear
              >
                <template #prefix>
                  <SearchOutlined />
                </template>
              </a-input-search>
              <a-select v-model:value="filter.order_by" style="width: 140px" @change="fetchTemplates">
                <a-select-option value="">默认排序</a-select-option>
                <a-select-option value="usage_count">最多使用</a-select-option>
                <a-select-option value="rating">最高评分</a-select-option>
                <a-select-option value="created_at">最新创建</a-select-option>
                <a-select-option value="updated_at">最近更新</a-select-option>
              </a-select>
              <a-button @click="resetFilters">
                <ClearOutlined /> 重置筛选
              </a-button>
            </a-space>

            <!-- 已选标签 -->
            <div v-if="selectedTags.length > 0" style="margin-top: 12px">
              <a-space wrap>
                <span style="color: #999">已选标签:</span>
                <a-tag
                  v-for="tag in selectedTags"
                  :key="tag"
                  closable
                  @close="removeTag(tag)"
                >
                  {{ tag }}
                </a-tag>
              </a-space>
            </div>
          </div>

          <a-alert
            show-icon
            type="info"
            style="margin-bottom: 16px"
            message="标准交付建议：填写 `REGISTRY_ID` 可启用镜像扫描门禁；填写 `AUTO_GITOPS_HANDOFF=true`、`GITOPS_REPO_ID`、`APP_NAME`、`DEPLOY_ENV`、`GITOPS_FILE_PATH` 可在流水线成功后自动发起 GitOps 变更。"
          />

          <!-- 模板卡片列表 -->
          <a-spin :spinning="loading">
            <div class="template-grid">
              <div v-for="tpl in displayedTemplates" :key="tpl.id" class="template-grid-item">
                <a-card hoverable class="template-card">
                  <template #cover>
                    <div class="template-cover" @click="showDetail(tpl)">
                      <div class="template-icon">
                        <CodeOutlined v-if="tpl.category === 'build'" />
                        <RocketOutlined v-else-if="tpl.category === 'deploy'" />
                        <BugOutlined v-else-if="tpl.category === 'test'" />
                        <SafetyOutlined v-else-if="tpl.category === 'security'" />
                        <AppstoreOutlined v-else />
                      </div>
                      <div class="template-badges">
                        <a-tag v-if="tpl.is_official" color="gold" size="small">官方</a-tag>
                        <a-tag v-else-if="tpl.is_public" color="blue" size="small">公开</a-tag>
                        <a-tag v-else color="default" size="small">私有</a-tag>
                      </div>
                      <div class="favorite-btn" @click.stop="toggleFavorite(tpl)">
                        <StarFilled v-if="isFavorite(tpl.id)" style="color: #faad14" />
                        <StarOutlined v-else style="color: #fff" />
                      </div>
                    </div>
                  </template>
                  <div @click="showDetail(tpl)" style="cursor: pointer">
                    <a-card-meta :title="tpl.name" :description="tpl.description || '暂无描述'">
                      <template #avatar>
                        <a-tag v-if="tpl.category" size="small">{{ getCategoryLabel(tpl.category) }}</a-tag>
                      </template>
                    </a-card-meta>
                    
                    <!-- 标签 -->
                    <div v-if="tpl.tags && tpl.tags.length > 0" style="margin-top: 8px">
                      <a-tag v-for="tag in tpl.tags.slice(0, 3)" :key="tag" size="small" color="blue">
                        {{ tag }}
                      </a-tag>
                      <a-tag v-if="tpl.tags.length > 3" size="small">+{{ tpl.tags.length - 3 }}</a-tag>
                    </div>

                    <div class="template-summary">
                      <span>{{ summarizeStages(tpl) }}</span>
                      <span>{{ summarizeVariables(tpl) }}</span>
                      <span>{{ summarizeCI(tpl) }}</span>
                    </div>

                    <div class="template-footer">
                      <a-space>
                        <span><StarFilled style="color: #faad14" /> {{ tpl.rating?.toFixed(1) || '-' }}</span>
                        <span style="color: #999">({{ tpl.rating_count || 0 }})</span>
                      </a-space>
                      <span style="color: #999">使用 {{ tpl.usage_count || 0 }}</span>
                    </div>
                  </div>
                </a-card>
              </div>
            </div>

            <a-empty v-if="!loading && displayedTemplates.length === 0" description="暂无模板" />
          </a-spin>

          <!-- 分页 -->
          <div style="margin-top: 16px; text-align: right" v-if="total > filter.page_size">
            <a-pagination
              v-model:current="filter.page"
              :total="total"
              :page-size="filter.page_size"
              @change="fetchTemplates"
              show-quick-jumper
              show-size-changer
            />
          </div>
        </a-card>
      </div>
    </div>

    <!-- 模板详情弹窗 - 增强版 -->
    <a-modal v-model:open="detailVisible" :title="currentTemplate?.name" width="900px" :footer="null">
      <template v-if="currentTemplate">
        <a-tabs>
          <a-tab-pane key="info" tab="基本信息">
            <a-descriptions :column="2" bordered size="small">
              <a-descriptions-item label="模板名称">{{ currentTemplate.name }}</a-descriptions-item>
              <a-descriptions-item label="标识">{{ currentTemplate.slug }}</a-descriptions-item>
              <a-descriptions-item label="分类">{{ getCategoryLabel(currentTemplate.category) }}</a-descriptions-item>
              <a-descriptions-item label="版本">{{ currentTemplate.version }}</a-descriptions-item>
              <a-descriptions-item label="评分">
                <a-rate :value="currentTemplate.rating" disabled allow-half />
                <span style="margin-left: 8px">{{ currentTemplate.rating?.toFixed(1) }} ({{ currentTemplate.rating_count }} 人评价)</span>
              </a-descriptions-item>
              <a-descriptions-item label="使用次数">{{ currentTemplate.usage_count }}</a-descriptions-item>
              <a-descriptions-item label="创建时间">{{ currentTemplate.created_at }}</a-descriptions-item>
              <a-descriptions-item label="更新时间">{{ currentTemplate.updated_at }}</a-descriptions-item>
              <a-descriptions-item label="描述" :span="2">{{ currentTemplate.description || '-' }}</a-descriptions-item>
              <a-descriptions-item label="标签" :span="2">
                <a-space wrap v-if="currentTemplate.tags && currentTemplate.tags.length > 0">
                  <a-tag v-for="tag in currentTemplate.tags" :key="tag" color="blue">{{ tag }}</a-tag>
                </a-space>
                <span v-else style="color: #999">无</span>
              </a-descriptions-item>
            </a-descriptions>
          </a-tab-pane>

          <a-tab-pane key="stages" tab="阶段说明">
            <a-timeline v-if="currentTemplate.config_json?.stages">
              <a-timeline-item
                v-for="(stage, index) in currentTemplate.config_json.stages"
                :key="index"
                color="blue"
              >
                <template #dot>
                  <span style="font-weight: bold">{{ index + 1 }}</span>
                </template>
                <div>
                  <h4>{{ stage.name }}</h4>
                  <p style="color: #666">{{ stage.description || '暂无描述' }}</p>
                  <a-descriptions size="small" :column="1" bordered>
                    <a-descriptions-item label="步骤数">
                      {{ stage.steps?.length || 0 }}
                    </a-descriptions-item>
                    <a-descriptions-item label="步骤列表">
                      <ul style="margin: 0; padding-left: 20px">
                        <li v-for="(step, si) in stage.steps" :key="si">
                          {{ step.name }} ({{ step.type }})
                        </li>
                      </ul>
                    </a-descriptions-item>
                  </a-descriptions>
                </div>
              </a-timeline-item>
            </a-timeline>
            <a-empty v-else description="暂无阶段信息" />
          </a-tab-pane>

          <a-tab-pane key="config" tab="配置预览">
            <a-alert
              v-if="getTemplateGuidance(currentTemplate).length > 0"
              show-icon
              type="info"
              style="margin-bottom: 12px"
              :message="getTemplateGuidance(currentTemplate).join('；')"
            />
            <a-descriptions :column="3" bordered size="small" style="margin-bottom: 12px">
              <a-descriptions-item label="阶段">{{ summarizeStages(currentTemplate) }}</a-descriptions-item>
              <a-descriptions-item label="变量">{{ summarizeVariables(currentTemplate) }}</a-descriptions-item>
              <a-descriptions-item label="CI">{{ summarizeCI(currentTemplate) }}</a-descriptions-item>
            </a-descriptions>
            <pre style="background: #f5f5f5; padding: 12px; border-radius: 4px; max-height: 400px; overflow: auto">{{ JSON.stringify(currentTemplate.config_json, null, 2) }}</pre>
          </a-tab-pane>
        </a-tabs>

        <div style="margin-top: 16px; text-align: right">
          <a-space>
            <a-button @click="toggleFavorite(currentTemplate)">
              <StarFilled v-if="isFavorite(currentTemplate.id)" style="color: #faad14" />
              <StarOutlined v-else />
              {{ isFavorite(currentTemplate.id) ? '取消收藏' : '收藏' }}
            </a-button>
            <a-button @click="rateTemplate">
              <StarOutlined /> 评分
            </a-button>
            <a-button type="primary" @click="useTemplate(currentTemplate)">
              <ThunderboltOutlined /> 使用此模板
            </a-button>
          </a-space>
        </div>
      </template>
    </a-modal>

    <!-- 创建模板弹窗 -->
    <a-modal v-model:open="createVisible" title="创建模板" width="800px" @ok="saveTemplate" :confirm-loading="saving">
      <a-form :model="form" :label-col="{ span: 4 }" :wrapper-col="{ span: 19 }">
        <a-form-item label="模板名称" required>
          <a-input v-model:value="form.name" placeholder="如：Java Maven 构建" />
        </a-form-item>
        <a-form-item label="分类">
          <a-select v-model:value="form.category" placeholder="选择分类" allowClear>
            <a-select-option value="build">构建</a-select-option>
            <a-select-option value="deploy">部署</a-select-option>
            <a-select-option value="test">测试</a-select-option>
            <a-select-option value="security">安全</a-select-option>
            <a-select-option value="other">其他</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="2" placeholder="模板描述" />
        </a-form-item>
        <a-form-item label="公开">
          <a-switch v-model:checked="form.is_public" />
          <span style="margin-left: 8px; color: #999">公开后其他用户可以使用</span>
        </a-form-item>
        <a-form-item label="配置" required>
          <a-textarea v-model:value="configJsonStr" :rows="10" placeholder="输入 JSON 格式的流水线配置" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 从现有流水线创建模板弹窗 -->
    <a-modal
      v-model:open="createFromPipelineVisible"
      title="从现有流水线创建模板"
      width="800px"
      @ok="saveTemplateFromPipeline"
      :confirm-loading="saving"
    >
      <a-form :model="form" :label-col="{ span: 4 }" :wrapper-col="{ span: 19 }">
        <a-form-item label="选择流水线" required>
          <a-select
            v-model:value="selectedPipelineId"
            placeholder="选择一个流水线"
            show-search
            :filter-option="filterPipeline"
            @change="onPipelineSelect"
          >
            <a-select-option v-for="pipeline in pipelines" :key="pipeline.id" :value="pipeline.id">
              {{ pipeline.name }}
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="模板名称" required>
          <a-input v-model:value="form.name" placeholder="如：Java Maven 构建" />
        </a-form-item>
        <a-form-item label="分类">
          <a-select v-model:value="form.category" placeholder="选择分类" allowClear>
            <a-select-option value="build">构建</a-select-option>
            <a-select-option value="deploy">部署</a-select-option>
            <a-select-option value="test">测试</a-select-option>
            <a-select-option value="security">安全</a-select-option>
            <a-select-option value="other">其他</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="描述">
          <a-textarea v-model:value="form.description" :rows="2" placeholder="模板描述" />
        </a-form-item>
        <a-form-item label="公开">
          <a-switch v-model:checked="form.is_public" />
          <span style="margin-left: 8px; color: #999">公开后其他用户可以使用</span>
        </a-form-item>
        <a-form-item label="配置预览">
          <pre style="background: #f5f5f5; padding: 12px; border-radius: 4px; max-height: 300px; overflow: auto">{{ configJsonStr }}</pre>
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 评分弹窗 -->
    <a-modal v-model:open="rateVisible" title="评分" @ok="submitRating" :confirm-loading="rating">
      <div style="text-align: center; padding: 20px">
        <p>请为模板 <strong>{{ currentTemplate?.name }}</strong> 评分</p>
        <a-rate v-model:value="rateValue" allow-half />
      </div>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, onMounted, computed, watch } from 'vue'
import { message } from 'ant-design-vue'
import { useRouter } from 'vue-router'
import {
  PlusOutlined,
  CodeOutlined,
  RocketOutlined,
  BugOutlined,
  SafetyOutlined,
  AppstoreOutlined,
  StarFilled,
  StarOutlined,
  ThunderboltOutlined,
  DownOutlined,
  FileAddOutlined,
  CopyOutlined,
  SearchOutlined,
  ClearOutlined,
  FolderOutlined,
  FileOutlined,
} from '@ant-design/icons-vue'
import request from '@/services/api'
import type { PipelineTemplate } from '@/types/pipeline'
import { useUserStore } from '@/stores/user'

type Template = Omit<PipelineTemplate, 'slug' | 'tags'> & {
  slug?: string
  config_json?: any
  is_official?: boolean
  language?: string
  framework?: string
  version?: string
  tags?: string[]
}

interface Pipeline {
  id: number
  name: string
  config_json: any
}

interface TemplateCategory {
  value: string
  label: string
}

const router = useRouter()
const loading = ref(false)
const saving = ref(false)
const rating = ref(false)
const detailVisible = ref(false)
const createVisible = ref(false)
const createFromPipelineVisible = ref(false)
const rateVisible = ref(false)
const showMyTemplates = ref(false)
const showFavoritesOnly = ref(false)
const templates = ref<Template[]>([])
const pipelines = ref<Pipeline[]>([])
const categories = ref<TemplateCategory[]>([])
const currentTemplate = ref<Template | null>(null)
const total = ref(0)
const rateValue = ref(5)
const configJsonStr = ref('{\n  "stages": []\n}')
const selectedPipelineId = ref<number>()
const selectedCategories = ref<string[]>([])
const selectedTags = ref<string[]>([])
const availableTags = ref<string[]>([])
const favorites = ref<Set<number>>(new Set())

const filter = reactive({
  category: '',
  keyword: '',
  order_by: '',
  page: 1,
  page_size: 12,
  tags: [] as string[],
})

const form = reactive({
  name: '',
  description: '',
  category: '',
  is_public: false,
  config_json: {} as any,
})

// 分类树数据
const categoryTree = computed(() => [
  {
    title: '全部分类',
    key: 'all',
    children: categories.value.map(item => ({
      title: item.label,
      key: item.value,
    })),
  },
])

const favoriteCount = computed(() => favorites.value.size)
const userStore = useUserStore()
const currentUsername = computed(() => userStore.userInfo?.username || '')

const categoryLabelMap: Record<string, string> = {
  build: '构建',
  deploy: '部署',
  test: '测试',
  security: '安全',
  other: '其他',
}

const getCategoryLabel = (category?: string) => categoryLabelMap[category || ''] || category || '未分类'
const getTemplateVariableNames = (tpl?: Template | null): string[] => {
  const variables = tpl?.config_json?.variables
  if (Array.isArray(variables)) {
    return variables
      .map((item: any) => item?.name || item?.key)
      .filter((name: string) => !!name)
  }
  if (variables && typeof variables === 'object') {
    return Object.entries(variables)
      .map(([name, value]) => {
        if (value && typeof value === 'object' && !Array.isArray(value)) {
          const variable = value as Record<string, unknown>
          return String(variable.name || variable.key || name)
        }
        return name
      })
      .filter((name: string) => !!name)
  }
  return []
}

const getTemplateGuidance = (tpl?: Template | null): string[] => {
  const variableNames = getTemplateVariableNames(tpl)
  const tips: string[] = []
  if (variableNames.includes('REGISTRY_ID')) {
    tips.push('配置 REGISTRY_ID 后，docker_push 会自动执行镜像扫描并阻断严重/高危漏洞')
  }
  if (variableNames.includes('AUTO_GITOPS_HANDOFF')) {
    tips.push('开启 AUTO_GITOPS_HANDOFF 后，流水线成功可自动发起 GitOps 变更')
  }
  if (variableNames.includes('GITOPS_REPO_ID') || variableNames.includes('GITOPS_FILE_PATH')) {
    tips.push('建议同时填写 GITOPS_REPO_ID、APP_NAME、DEPLOY_ENV、GITOPS_FILE_PATH，减少自动交接匹配失败')
  }
  if (tpl?.language === 'rust' || variableNames.includes('RUSTFLAGS')) {
    tips.push('Rust 模板默认使用 cargo build --release，可通过 BUILD_COMMAND、RUSTFLAGS、APP_PORT 调整构建和运行方式')
  }
  return tips
}

const getTemplateStages = (tpl?: Template | null) => Array.isArray(tpl?.config_json?.stages) ? tpl?.config_json?.stages || [] : []
const getTemplateVariables = (tpl?: Template | null) => {
  const variables = tpl?.config_json?.variables
  if (Array.isArray(variables)) return variables
  if (variables && typeof variables === 'object') {
    return Object.entries(variables).map(([name, value]) => {
      if (value && typeof value === 'object' && !Array.isArray(value)) {
        const variable = value as Record<string, unknown>
        return {
          name: variable.name ?? variable.key ?? name,
          value: variable.value
        }
      }
      return { name, value }
    })
  }
  return []
}
const summarizeStages = (tpl?: Template | null) => {
  const stages = getTemplateStages(tpl)
  const stepCount = stages.reduce((total: number, stage: any) => total + (Array.isArray(stage?.steps) ? stage.steps.length : 0), 0)
  return `${stages.length} 个阶段 / ${stepCount} 个步骤`
}
const summarizeVariables = (tpl?: Template | null) => `${getTemplateVariables(tpl).length} 个变量`
const summarizeCI = (tpl?: Template | null) => {
  const ci = tpl?.config_json?.ci || {}
  const dockerfileMode = ci?.dockerfile_content ? '内联 Dockerfile' : '自动 Dockerfile'
  const yamlMode = ci?.gitlab_ci_yaml_custom ? '手写 CI' : '平台生成 CI'
  return `${dockerfileMode} / ${yamlMode}`
}
const displayedTemplates = computed(() => {
  let result = templates.value
  if (showFavoritesOnly.value && !currentUsername.value) {
    result = result.filter(item => favorites.value.has(item.id))
  }
  if (showMyTemplates.value && currentUsername.value && !templates.value.some(item => item.created_by === currentUsername.value)) {
    result = result.filter(item => item.created_by === currentUsername.value)
  }
  return result
})
const visibleTotal = computed(() => displayedTemplates.value.length)

const fetchTemplates = async () => {
  loading.value = true
  try {
    const params = {
      ...filter,
      tags: selectedTags.value.join(','),
      favorites_only: showFavoritesOnly.value,
      mine: showMyTemplates.value,
    }
    const res = await request.get('/pipeline/templates', { params })
    const data = res.data || res
    templates.value = (data?.items || []).map((item: any) => ({
      ...item,
      slug: item.slug || item.name,
      is_official: item.is_official ?? item.is_builtin,
      version: item.version || '1.0.0',
      tags: item.tags || [item.language, item.framework].filter(Boolean),
    }))
    total.value = data?.total || 0
  } catch (e) {
    console.error('获取模板失败', e)
  } finally {
    loading.value = false
  }
}

const fetchCategories = async () => {
  try {
    const res = await request.get('/pipeline/templates/categories')
    const data = res.data || res
    categories.value = (data?.items || []).map((item: any) => ({
      value: item.value || item.label || item,
      label: item.label || getCategoryLabel(item.value || item),
    }))
  } catch (e) {
    console.error('获取分类失败', e)
    categories.value = Object.entries(categoryLabelMap).map(([value, label]) => ({ value, label }))
  }
}

const fetchTags = async () => {
  try {
    const res = await request.get('/pipeline/templates/tags')
    const data = res.data || res
    const items = data?.items || []
    // Extract tag values from the array of tag objects
    availableTags.value = items.map((tag: any) => tag.value || tag.label || tag)
  } catch (e) {
    console.error('获取标签失败', e)
    availableTags.value = []
  }
}

const fetchPipelines = async () => {
  try {
    const res = await request.get('/pipelines', { params: { page: 1, page_size: 100 } })
    const data = res.data || res
    pipelines.value = data?.items || []
  } catch (e) {
    console.error('获取流水线失败', e)
    pipelines.value = []
  }
}

const fetchFavorites = async () => {
  try {
    const res = await request.get('/pipeline/templates/favorites')
    const data = res.data || res
    const items = data?.items || []
    favorites.value = new Set(items.map((item: any) => item.template_id || item.id))
  } catch (e) {
    console.error('获取收藏失败', e)
    favorites.value = new Set()
  }
}

const onCategorySelect = (selectedKeys: string[]) => {
  if (selectedKeys.length > 0 && selectedKeys[0] !== 'all') {
    filter.category = selectedKeys[0]
  } else {
    filter.category = ''
  }
  fetchTemplates()
}

const onTagChange = () => {
  filter.tags = selectedTags.value
  fetchTemplates()
}

const removeTag = (tag: string) => {
  selectedTags.value = selectedTags.value.filter(t => t !== tag)
  onTagChange()
}

const resetFilters = () => {
  filter.category = ''
  filter.keyword = ''
  filter.order_by = ''
  selectedCategories.value = []
  selectedTags.value = []
  showFavoritesOnly.value = false
  fetchTemplates()
}

const showFavorites = () => {
  showFavoritesOnly.value = !showFavoritesOnly.value
  if (showFavoritesOnly.value) {
    showMyTemplates.value = false
  }
  fetchTemplates()
}

watch(showMyTemplates, () => {
  if (showMyTemplates.value) {
    showFavoritesOnly.value = false
  }
  fetchTemplates()
})

const isFavorite = (templateId: number) => {
  return favorites.value.has(templateId)
}

const toggleFavorite = async (tpl: Template) => {
  try {
    if (isFavorite(tpl.id)) {
      await request.delete(`/pipeline/templates/${tpl.id}/favorite`)
      favorites.value.delete(tpl.id)
      message.success('已取消收藏')
    } else {
      await request.post(`/pipeline/templates/${tpl.id}/favorite`)
      favorites.value.add(tpl.id)
      message.success('收藏成功')
    }
  } catch (e) {
    message.error('操作失败')
  }
}

const showDetail = (tpl: Template) => {
  currentTemplate.value = tpl
  detailVisible.value = true
}

const handleCreateMenu = ({ key }: { key: string }) => {
  if (key === 'blank') {
    showCreateModal()
  } else if (key === 'from-pipeline') {
    showCreateFromPipelineModal()
  }
}

const showCreateModal = () => {
  Object.assign(form, {
    name: '',
    description: '',
    category: 'build',
    is_public: false,
  })
  configJsonStr.value = '{\n  "stages": []\n}'
  createVisible.value = true
}

const showCreateFromPipelineModal = () => {
  Object.assign(form, {
    name: '',
    description: '',
    category: 'build',
    is_public: false,
  })
  selectedPipelineId.value = undefined
  configJsonStr.value = ''
  fetchPipelines()
  createFromPipelineVisible.value = true
}

const onPipelineSelect = async (pipelineId: number) => {
  try {
    const res = await request.get(`/pipelines/${pipelineId}`)
    const pipeline = res.data || res
    configJsonStr.value = JSON.stringify(pipeline.config_json || {}, null, 2)
    form.config_json = pipeline.config_json || {}
    
    // 自动填充模板名称
    if (!form.name) {
      form.name = `${pipeline.name} 模板`
    }
  } catch (e) {
    message.error('获取流水线配置失败')
  }
}

const filterPipeline = (input: string, option: any) => {
  return option.children[0].toLowerCase().indexOf(input.toLowerCase()) >= 0
}

const saveTemplate = async () => {
  if (!form.name) {
    message.warning('请填写必填项')
    return
  }
  try {
    form.config_json = JSON.parse(configJsonStr.value)
  } catch (e) {
    message.error('配置 JSON 格式错误')
    return
  }

  saving.value = true
  try {
    await request.post('/pipeline/templates', {
      ...form,
      config_json: form.config_json,
    })
    message.success('创建成功')
    createVisible.value = false
    fetchTemplates()
  } catch (e) {
    message.error('创建失败')
  } finally {
    saving.value = false
  }
}

const saveTemplateFromPipeline = async () => {
  if (!form.name || !selectedPipelineId.value) {
    message.warning('请填写必填项并选择流水线')
    return
  }

  saving.value = true
  try {
    await request.post('/pipeline/templates', {
      name: form.name,
      description: form.description,
      category: form.category,
      is_public: form.is_public,
      source_pipeline_id: selectedPipelineId.value,
    })
    message.success('创建成功')
    createFromPipelineVisible.value = false
    fetchTemplates()
  } catch (e) {
    message.error('创建失败')
  } finally {
    saving.value = false
  }
}

const useTemplate = async (tpl: Template) => {
  try {
    await request.post(`/pipeline/templates/${tpl.id}/use`)
    message.success('已进入流水线创建页，请补充应用、环境和仓库后保存')
    detailVisible.value = false
    router.push({ name: 'PipelineCreate', query: { template_id: tpl.id } })
  } catch (e) {
    message.error('使用模板失败')
  }
}

const rateTemplate = () => {
  rateValue.value = 5
  rateVisible.value = true
}

const submitRating = async () => {
  if (!currentTemplate.value) return
  rating.value = true
  try {
    await request.post(`/pipeline/templates/${currentTemplate.value.id}/rate`, { rating: rateValue.value })
    message.success('评分成功')
    rateVisible.value = false
    fetchTemplates()
  } catch (e) {
    message.error('评分失败')
  } finally {
    rating.value = false
  }
}

onMounted(() => {
  fetchTemplates()
  fetchCategories()
  fetchTags()
  fetchFavorites()
})
</script>

<style scoped>
.template-market {
  height: calc(100vh - 120px);
}

.market-layout {
  display: flex;
  gap: 16px;
  height: 100%;
}

.category-sidebar {
  width: 250px;
  flex-shrink: 0;
  overflow-y: auto;
}

.template-content {
  flex: 1;
  min-width: 0;
  overflow-y: auto;
}

.template-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
  gap: 16px;
}

.template-grid-item {
  min-width: 0;
}

.template-card {
  height: 100%;
}

.template-card :deep(.ant-card-body) {
  min-width: 0;
}

.template-card :deep(.ant-card-meta) {
  align-items: flex-start;
}

.template-card :deep(.ant-card-meta-title) {
  white-space: normal;
  overflow-wrap: anywhere;
  line-height: 20px;
}

.template-card :deep(.ant-card-meta-description) {
  display: -webkit-box;
  min-height: 44px;
  overflow: hidden;
  line-height: 22px;
  overflow-wrap: anywhere;
  -webkit-box-orient: vertical;
  -webkit-line-clamp: 2;
}

.template-cover {
  height: 80px;
  background: linear-gradient(135deg, #667eea 0%, #764ba2 100%);
  position: relative;
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
}

.template-icon {
  font-size: 32px;
  color: #fff;
}

.template-badges {
  position: absolute;
  top: 8px;
  right: 8px;
}

.favorite-btn {
  position: absolute;
  top: 8px;
  left: 8px;
  font-size: 18px;
  cursor: pointer;
  transition: transform 0.2s;
}

.favorite-btn:hover {
  transform: scale(1.2);
}

.template-footer {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  margin-top: 12px;
  font-size: 12px;
}

.template-summary {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 10px;
  color: #666;
  font-size: 12px;
}
</style>
