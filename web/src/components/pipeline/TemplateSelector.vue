<template>
  <div class="template-selector">
    <div class="template-header">
      <h3>选择模板</h3>
      <el-input
        v-model="searchKeyword"
        placeholder="搜索模板..."
        prefix-icon="Search"
        clearable
        style="width: 240px"
      />
    </div>

    <div class="template-categories">
      <el-radio-group v-model="selectedCategory" size="small">
        <el-radio-button label="">全部</el-radio-button>
        <el-radio-button 
          v-for="cat in categories" 
          :key="cat.value" 
          :label="cat.value"
        >
          {{ cat.label }}
        </el-radio-button>
      </el-radio-group>
    </div>

    <div class="template-grid" v-loading="loading">
      <div
        v-for="template in filteredTemplates"
        :key="template.id"
        class="template-card"
        :class="{ selected: selectedTemplate?.id === template.id }"
        @click="selectTemplate(template)"
      >
        <div class="template-icon">
          <el-icon :size="32">
            <component :is="getTemplateIcon(template.category)" />
          </el-icon>
        </div>
        <div class="template-info">
          <h4>{{ template.name }}</h4>
          <p>{{ template.description }}</p>
          <div class="template-tags">
            <el-tag v-for="tag in template.tags" :key="tag" size="small" type="info">
              {{ tag }}
            </el-tag>
          </div>
          <div class="template-facts">
            <span>{{ summarizeStages(template) }}</span>
            <span>{{ summarizeVariables(template) }}</span>
            <span>{{ summarizeCI(template) }}</span>
          </div>
        </div>
        <div class="template-actions">
          <el-button type="primary" link @click.stop="previewTemplate(template)">
            预览
          </el-button>
        </div>
      </div>

      <div v-if="filteredTemplates.length === 0" class="empty-state">
        <el-empty description="暂无匹配的模板" />
      </div>
    </div>

    <div class="template-footer">
      <el-button @click="$emit('cancel')">取消</el-button>
      <el-button type="primary" :disabled="!selectedTemplate" @click="confirmSelect">
        使用此模板
      </el-button>
    </div>

    <!-- 模板预览对话框 -->
    <el-dialog v-model="previewDialog.visible" title="模板预览" width="70%">
      <div v-if="previewDialog.template" class="template-preview">
        <el-descriptions :column="2" border>
          <el-descriptions-item label="模板名称">
            {{ previewDialog.template.name }}
          </el-descriptions-item>
          <el-descriptions-item label="分类">
            {{ getCategoryLabel(previewDialog.template.category) }}
          </el-descriptions-item>
          <el-descriptions-item label="描述" :span="2">
            {{ previewDialog.template.description }}
          </el-descriptions-item>
        </el-descriptions>

        <h4 style="margin: 16px 0 8px">流水线配置</h4>
        <el-descriptions :column="3" border style="margin-bottom: 16px">
          <el-descriptions-item label="阶段">{{ summarizeStages(previewDialog.template) }}</el-descriptions-item>
          <el-descriptions-item label="变量">{{ summarizeVariables(previewDialog.template) }}</el-descriptions-item>
          <el-descriptions-item label="CI">{{ summarizeCI(previewDialog.template) }}</el-descriptions-item>
        </el-descriptions>
        <el-collapse>
          <el-collapse-item 
            v-for="(stage, index) in (previewDialog.template.config_json?.stages || [])" 
            :key="index"
            :title="`阶段 ${index + 1}: ${stage.name}`"
          >
            <div class="stage-preview">
              <p v-if="stage.description">{{ stage.description }}</p>
              <div class="steps-preview">
                <div v-for="(step, stepIndex) in stage.steps" :key="stepIndex" class="step-preview">
                  <el-tag size="small">{{ step.type }}</el-tag>
                  <span>{{ step.name }}</span>
                </div>
              </div>
            </div>
          </el-collapse-item>
        </el-collapse>
      </div>
    </el-dialog>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue'
import type { Component } from 'vue'
import { ElMessage } from 'element-plus'
import { 
  Document, 
  Box, 
  Monitor, 
  Setting, 
  Connection,
  Search
} from '@element-plus/icons-vue'
import { pipelineApi } from '@/services/pipeline'
import request from '@/services/api'
import type { PipelineTemplate } from '@/types/pipeline'
import type { ApiResponse } from '@/types'

type Template = Omit<PipelineTemplate, 'tags'> & {
  config_json?: any
  tags: string[]
  is_official?: boolean
}

interface TemplateCategory {
  value: string
  label: string
}

interface TemplateCategoryResponse {
  items?: TemplateCategory[]
  list?: TemplateCategory[]
}

const emit = defineEmits(['select', 'cancel'])

const loading = ref(false)
const templates = ref<Template[]>([])
const searchKeyword = ref('')
const selectedCategory = ref('')
const selectedTemplate = ref<Template | null>(null)

const previewDialog = ref({
  visible: false,
  template: null as Template | null
})

const categories = ref<TemplateCategory[]>([
  { value: 'build', label: '构建' },
  { value: 'deploy', label: '部署' },
  { value: 'test', label: '测试' },
  { value: 'security', label: '安全' },
  { value: 'other', label: '其他' }
])

const filteredTemplates = computed(() => {
  let result = templates.value

  if (selectedCategory.value) {
    result = result.filter(t => t.category === selectedCategory.value)
  }

  if (searchKeyword.value) {
    const keyword = searchKeyword.value.toLowerCase()
    result = result.filter(t => 
      t.name.toLowerCase().includes(keyword) ||
      t.description.toLowerCase().includes(keyword) ||
      t.tags?.some(tag => tag.toLowerCase().includes(keyword))
    )
  }

  return result
})

const fetchTemplates = async () => {
  loading.value = true
  try {
    const res: any = await pipelineApi.listTemplates({ page: 1, page_size: 100 })
    const data = res.data || res
    templates.value = (data?.items || []).map((item: any) => ({
      ...item,
      tags: item.tags || [item.language, item.framework].filter(Boolean),
      is_official: item.is_official ?? item.is_builtin,
    }))
  } catch (error) {
    ElMessage.error('获取模板列表失败')
  } finally {
    loading.value = false
  }
}

const fetchCategories = async () => {
  try {
    const res = await request.get<any, ApiResponse<TemplateCategoryResponse>>('/pipeline/templates/categories')
    const data = res.data
    const categoryItems = data?.items || data?.list || []
    categories.value = categoryItems.map((item) => ({
      value: item.value,
      label: item.label,
    }))
  } catch {
    // keep fallback categories
  }
}

const selectTemplate = (template: Template) => {
  selectedTemplate.value = template
}

const previewTemplate = (template: Template) => {
  previewDialog.value.template = template
  previewDialog.value.visible = true
}

const templateStages = (template: Template) => Array.isArray(template.config_json?.stages) ? template.config_json.stages : []

const templateVariables = (template: Template) => {
  const variables = template.config_json?.variables
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

const summarizeStages = (template: Template) => {
  const stages = templateStages(template)
  const stepCount = stages.reduce((total: number, stage: any) => total + (Array.isArray(stage.steps) ? stage.steps.length : 0), 0)
  return `${stages.length} 个阶段 / ${stepCount} 个步骤`
}

const summarizeVariables = (template: Template) => `${templateVariables(template).length} 个变量`

const summarizeCI = (template: Template) => {
  const ci = template.config_json?.ci || {}
  const dockerfileMode = ci?.dockerfile_content ? '内联 Dockerfile' : '自动 Dockerfile'
  const yamlMode = ci?.gitlab_ci_yaml_custom ? '手写 CI' : '平台生成 CI'
  return `${dockerfileMode} / ${yamlMode}`
}

const confirmSelect = () => {
  if (selectedTemplate.value) {
    emit('select', {
      ...selectedTemplate.value,
      ...(selectedTemplate.value.config_json || {}),
    })
  }
}

const getTemplateIcon = (category?: string) => {
  const iconMap: Record<string, Component> = {
    build: Box,
    deploy: Monitor,
    test: Document,
    security: Connection,
    other: Setting
  }
  if (!category) {
    return Document
  }
  return iconMap[category] || Document
}

const getCategoryLabel = (category?: string) => {
  const cat = categories.value.find(c => c.value === category)
  return cat?.label || category
}

onMounted(() => {
  fetchTemplates()
  fetchCategories()
})
</script>

<style scoped>
.template-selector {
  display: flex;
  flex-direction: column;
  height: 100%;
}

.template-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 16px;
}

.template-header h3 {
  margin: 0;
}

.template-categories {
  margin-bottom: 16px;
}

.template-grid {
  flex: 1;
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(280px, 1fr));
  gap: 16px;
  overflow-y: auto;
  padding: 4px;
}

.template-card {
  display: flex;
  flex-direction: column;
  padding: 16px;
  border: 1px solid #e4e7ed;
  border-radius: 8px;
  cursor: pointer;
  transition: all 0.2s;
}

.template-card:hover {
  border-color: #409eff;
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.1);
}

.template-card.selected {
  border-color: #409eff;
  background: #ecf5ff;
}

.template-icon {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 56px;
  height: 56px;
  background: #f5f7fa;
  border-radius: 8px;
  margin-bottom: 12px;
  color: #409eff;
}

.template-info h4 {
  margin: 0 0 8px;
  font-size: 16px;
}

.template-info p {
  margin: 0 0 8px;
  color: #909399;
  font-size: 13px;
  line-height: 1.4;
}

.template-facts {
  display: flex;
  flex-direction: column;
  gap: 4px;
  margin-top: 8px;
  color: #606266;
  font-size: 12px;
}

.template-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
}

.template-actions {
  margin-top: auto;
  padding-top: 12px;
  text-align: right;
}

.template-footer {
  display: flex;
  justify-content: flex-end;
  gap: 12px;
  padding-top: 16px;
  border-top: 1px solid #e4e7ed;
  margin-top: 16px;
}

.empty-state {
  grid-column: 1 / -1;
}

.template-preview {
  padding: 0 20px;
}

.stage-preview {
  padding: 8px 0;
}

.steps-preview {
  display: flex;
  flex-direction: column;
  gap: 8px;
  margin-top: 8px;
}

.step-preview {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 4px;
}
</style>
