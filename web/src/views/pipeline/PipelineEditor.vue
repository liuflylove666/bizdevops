<template>
  <div class="pipeline-editor">
    <a-page-header
      :title="isEdit ? '编辑流水线' : '创建流水线向导'"
      :sub-title="isEdit ? '按步骤调整源码、GitLab Runner、触发方式和编排配置' : '按步骤完成源码、GitLab Runner、模板、触发与编排配置'"
      @back="goBack"
    >
      <template #extra>
        <a-space>
          <a-button v-if="isEdit && canManagePipelineTemplates" @click="showSaveTemplateModal"><FileAddOutlined /> 保存为模板</a-button>
          <a-button @click="goBack">取消</a-button>
          <a-button v-if="currentStep > 0" @click="prevStep">上一步</a-button>
          <a-button v-if="currentStep < wizardSteps.length - 1" type="primary" @click="nextStep">
            下一步
          </a-button>
          <a-button v-else type="primary" :loading="saving" @click="savePipeline">
            {{ isEdit ? '保存更新' : '创建流水线' }}
          </a-button>
        </a-space>
      </template>
    </a-page-header>

    <!-- 模板选择弹窗 -->
    <a-modal v-model:open="templateModalVisible" title="选择模板" width="900px" :footer="null">
      <template-selector @select="applyTemplate" @cancel="templateModalVisible = false" />
    </a-modal>

    <a-modal
      v-model:open="saveTemplateVisible"
      title="保存为模板"
      @ok="saveAsTemplate"
      :confirm-loading="saveTemplateLoading"
    >
      <a-form :model="templateForm" layout="vertical">
        <a-form-item label="模板名称" required>
          <a-input v-model:value="templateForm.name" placeholder="例如：Go 服务标准构建发布模板" />
        </a-form-item>
        <a-form-item label="模板描述">
          <a-textarea v-model:value="templateForm.description" :rows="3" placeholder="描述适用场景和使用说明" />
        </a-form-item>
        <a-form-item label="分类">
          <a-select v-model:value="templateForm.category">
            <a-select-option value="build">构建</a-select-option>
            <a-select-option value="deploy">部署</a-select-option>
            <a-select-option value="test">测试</a-select-option>
            <a-select-option value="security">安全</a-select-option>
            <a-select-option value="other">其他</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="公开模板">
          <a-switch v-model:checked="templateForm.is_public" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-card :bordered="false" class="wizard-shell">
      <a-steps :current="currentStep" class="wizard-steps">
        <a-step v-for="(step, index) in wizardSteps" :key="step.key" :title="step.title" :description="step.description">
          <template #icon>
            <span class="step-index">{{ index + 1 }}</span>
          </template>
        </a-step>
      </a-steps>

      <div class="wizard-content">
        <a-row :gutter="16">
          <a-col :span="17">
            <a-card v-if="currentStep === 0" title="步骤 1：基本信息" size="small" :bordered="false">
              <a-form :model="form" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
                <a-form-item label="流水线名称" required>
                  <a-input v-model:value="form.name" placeholder="例如：order-service-dev-delivery" />
                </a-form-item>
                <a-form-item label="描述">
                  <a-textarea v-model:value="form.description" placeholder="描述流水线用途、适用环境和发布范围" :rows="3" />
                </a-form-item>
                <a-form-item label="关联应用" required>
                  <a-select
                    v-model:value="form.application_id"
                    placeholder="选择要交付的应用"
                    show-search
                    allowClear
                    :filter-option="filterApplicationOption"
                    style="width: 100%"
                  >
                    <a-select-option v-for="app in applications" :key="app.id" :value="app.id">
                      {{ app.display_name || app.name }}（{{ app.name }}）
                    </a-select-option>
                  </a-select>
                </a-form-item>
                <a-form-item label="交付环境" required>
                  <a-select v-model:value="form.env" placeholder="选择环境" style="width: 220px">
                    <a-select-option v-for="env in envOptions" :key="env.value" :value="env.value">
                      {{ env.label }}
                    </a-select-option>
                  </a-select>
                </a-form-item>
              </a-form>
              <a-alert
                show-icon
                type="info"
                message="流水线必须挂到应用和环境，成功后才能进入应用交付记录和 GitOps 变更链路。"
              />
            </a-card>

            <a-card v-else-if="currentStep === 1" title="步骤 2：源码与 GitLab Runner" size="small" :bordered="false">
              <a-form :model="form" :label-col="{ span: 5 }" :wrapper-col="{ span: 16 }">
                <a-form-item label="Git 仓库" required>
                  <a-select v-model:value="form.git_repo_id" placeholder="选择 Git 仓库" allowClear style="width: 100%">
                    <a-select-option v-for="repo in gitRepos" :key="repo.id" :value="repo.id">
                      {{ repo.name }} ({{ repo.provider || 'gitlab' }} · {{ repo.url }})
                    </a-select-option>
                  </a-select>
                </a-form-item>
                <a-form-item label="Git 分支" v-if="form.git_repo_id">
                  <a-space direction="vertical" style="width: 100%">
                    <a-select
                      v-if="repoBranches.length > 0"
                      v-model:value="form.git_branch"
                      :loading="branchLoading"
                      placeholder="选择分支"
                      style="width: 260px"
                      show-search
                    >
                      <a-select-option v-for="branch in repoBranches" :key="branch.name" :value="branch.name">
                        {{ branch.name }}<span v-if="branch.is_default">（默认）</span>
                      </a-select-option>
                    </a-select>
                    <a-input v-else v-model:value="form.git_branch" placeholder="main" style="width: 220px" />
                    <a-alert
                      v-if="gitRepoCheck.message"
                      :type="gitRepoCheck.success ? 'success' : 'warning'"
                      show-icon
                      :message="gitRepoCheck.message"
                    />
                  </a-space>
                </a-form-item>
              </a-form>

              <a-alert
                class="section-hint"
                show-icon
                type="info"
                message="源码拉取由 GitLab Runner 自动完成；本步骤只绑定仓库与分支，CI 阶段和 YAML 在下一步配置。"
              />

              <div class="wizard-actions">
                <a-space wrap>
                  <a-button @click="goTo('/pipeline/git-repos')">管理 Git 仓库</a-button>
                </a-space>
              </div>
            </a-card>

            <a-card v-else-if="currentStep === 2" title="步骤 3：模板与编排" size="small" :bordered="false">
              <template #extra>
                <a-space>
                  <a-button @click="showTemplateSelector">
                    <FileAddOutlined /> {{ sourceTemplateId ? '重新选择模板' : '使用模板' }}
                  </a-button>
                  <a-radio-group v-if="canFreelyDesignPipeline" v-model:value="editMode" button-style="solid">
                    <a-radio-button value="visual">可视化编排</a-radio-button>
                    <a-radio-button value="yaml">YAML 配置</a-radio-button>
                  </a-radio-group>
                </a-space>
              </template>

              <a-alert
                class="section-hint"
                show-icon
                :type="canFreelyDesignPipeline ? 'info' : 'warning'"
                :message="templateSummary"
              />

              <a-alert
                class="section-hint"
                show-icon
                type="info"
                message="GitLab Runner 会自动检出源码，模板中的代码检出步骤仅作为可视化说明，不会写入最终 .gitlab-ci.yml。镜像构建阶段会在 Dockerfile 模板内完成编译与打包。"
              />

              <a-alert
                class="section-hint"
                show-icon
                type="info"
                message="若填写 `AUTO_GITOPS_HANDOFF=true`、`GITOPS_REPO_ID`、`APP_NAME`、`DEPLOY_ENV`、`GITOPS_FILE_PATH`，流水线成功后会自动发起 GitOps 变更。"
              />

              <a-alert
                v-if="!canFreelyDesignPipeline"
                class="section-hint"
                show-icon
                type="info"
                message="当前角色只能基于标准模板创建或维护流水线，可调整仓库、触发器和变量，但不能自由改写阶段步骤。"
              />

              <div v-if="editMode === 'visual'" class="visual-editor">
                <div class="stages-container">
                  <div v-if="!sourceTemplateId && !canFreelyDesignPipeline" class="locked-template-empty">
                    <a-empty description="请先选择标准模板">
                      <template #extra>
                        <a-space>
                          <a-button type="primary" @click="goTo('/pipeline/templates')">前往模板市场</a-button>
                          <a-button @click="showTemplateSelector">选择模板</a-button>
                        </a-space>
                      </template>
                    </a-empty>
                  </div>
                  <div v-for="(stage, stageIndex) in form.stages" :key="stageIndex" class="stage-card">
                    <div class="stage-header">
                      <a-input v-model:value="stage.name" placeholder="阶段名称" style="width: 150px" :disabled="!canFreelyDesignPipeline" />
                      <a-space>
                        <a-button v-if="canFreelyDesignPipeline" type="text" size="small" @click="addStep(stageIndex)">
                          <PlusOutlined /> 添加步骤
                        </a-button>
                        <a-button v-if="canFreelyDesignPipeline" type="text" size="small" danger @click="removeStage(stageIndex)">
                          <DeleteOutlined />
                        </a-button>
                      </a-space>
                    </div>

                    <div class="steps-container">
                      <div v-for="(step, stepIndex) in stage.steps" :key="stepIndex" class="step-card">
                        <div class="step-header">
                          <a-input v-model:value="step.name" placeholder="步骤名称" size="small" style="width: 120px" :disabled="!canFreelyDesignPipeline" />
                          <a-button v-if="canFreelyDesignPipeline" type="text" size="small" danger @click="removeStep(stageIndex, stepIndex)">
                            <DeleteOutlined />
                          </a-button>
                        </div>
                        <div class="step-content">
                          <a-form-item label="镜像" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
                            <a-auto-complete
                              v-model:value="step.image"
                              :options="imageOptions"
                              placeholder="node:18-alpine"
                              size="small"
                              :disabled="!canFreelyDesignPipeline"
                              @change="!form.gitlab_ci_yaml_custom && refreshGeneratedGitLabCI()"
                            />
                          </a-form-item>
                          <a-form-item label="命令" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
                            <a-textarea
                              v-model:value="step.commandsText"
                              placeholder="npm install&#10;npm run build"
                              :rows="3"
                              size="small"
                              :disabled="!canFreelyDesignPipeline"
                              @change="parseCommands(step)"
                            />
                          </a-form-item>
                          <a-collapse ghost size="small">
                            <a-collapse-panel key="advanced" header="高级配置">
                              <a-form-item label="工作目录" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
                                <a-input v-model:value="step.work_dir" placeholder="/workspace" size="small" :disabled="!canFreelyDesignPipeline" />
                              </a-form-item>
                              <a-form-item label="超时(秒)" :label-col="{ span: 6 }" :wrapper-col="{ span: 18 }">
                                <a-input-number v-model:value="step.timeout" :min="0" size="small" :disabled="!canFreelyDesignPipeline" />
                              </a-form-item>
                            </a-collapse-panel>
                          </a-collapse>
                        </div>
                      </div>

                      <div v-if="stage.steps.length === 0 && canFreelyDesignPipeline" class="empty-steps">
                        <a-button type="dashed" block @click="addStep(stageIndex)">
                          <PlusOutlined /> 添加步骤
                        </a-button>
                      </div>
                    </div>
                  </div>

                  <div v-if="canFreelyDesignPipeline" class="add-stage">
                    <a-button type="dashed" block @click="addStage">
                      <PlusOutlined /> 添加阶段
                    </a-button>
                  </div>
                </div>
              </div>

              <div v-else-if="canFreelyDesignPipeline" class="yaml-editor">
                <a-textarea
                  v-model:value="yamlContent"
                  :rows="25"
                  class="yaml-textarea"
                  @change="parseYaml"
                />
                <div v-if="yamlError" class="yaml-error">
                  <a-alert :message="yamlError" type="error" show-icon />
                </div>
              </div>

              <a-card title="Dockerfile 模板" size="small" :bordered="false" class="inner-card ci-editor-card">
                <template #extra>
                  <a-button size="small" @click="resetDockerfileTemplate">按当前配置生成</a-button>
                </template>
                <a-textarea
                  v-model:value="form.dockerfile_content"
                  :rows="13"
                  class="code-textarea"
                  placeholder="平台会在 CI 运行时把这里的内容写入 .jeridevops.Dockerfile，并用于 docker build。"
                />
              </a-card>

              <a-card title=".gitlab-ci.yml" size="small" :bordered="false" class="inner-card ci-editor-card">
                <template #extra>
                  <a-space>
                    <a-switch v-model:checked="form.gitlab_ci_yaml_custom" />
                    <span class="switch-label">手动编辑</span>
                    <a-button size="small" @click="refreshGeneratedGitLabCI">按当前配置预览</a-button>
                  </a-space>
                </template>
                <a-alert
                  class="section-hint"
                  show-icon
                  :type="form.gitlab_ci_yaml_custom ? 'warning' : 'info'"
                  :message="form.gitlab_ci_yaml_custom ? '当前会按下方内容原样提交 .gitlab-ci.yml。若 Dockerfile 模板变化，需要同步调整 YAML。' : '当前由平台根据阶段、变量和 Dockerfile 模板生成 .gitlab-ci.yml，阶段名和 Job 名使用语义化名称。'"
                />
                <a-textarea
                  v-model:value="form.gitlab_ci_yaml"
                  :rows="16"
                  class="code-textarea"
                  :disabled="!form.gitlab_ci_yaml_custom"
                  placeholder="可直接编辑 .gitlab-ci.yml 内容"
                />
              </a-card>
            </a-card>

            <a-card v-else-if="currentStep === 3" title="步骤 4：触发与变量" size="small" :bordered="false">
              <a-row :gutter="16">
                <a-col :span="24">
                  <a-card title="触发器配置" size="small" :bordered="false" class="inner-card">
                    <a-form :model="form.trigger_config" :label-col="{ span: 5 }" :wrapper-col="{ span: 18 }">
                      <a-form-item label="手动触发">
                        <a-switch v-model:checked="form.trigger_config.manual" />
                        <span class="trigger-hint">允许手动触发流水线执行</span>
                      </a-form-item>
                      <a-form-item label="定时触发">
                        <a-space direction="vertical" style="width: 100%">
                          <a-switch v-model:checked="scheduledEnabled" />
                          <template v-if="scheduledEnabled">
                            <a-input
                              v-model:value="form.trigger_config.scheduled.cron"
                              placeholder="Cron 表达式，如: 0 0 2 * * *"
                              style="width: 400px"
                            />
                            <div class="cron-presets">
                              <span>快捷设置：</span>
                              <a-button size="small" type="link" @click="setCron('0 0 2 * * *')">每天凌晨2点</a-button>
                              <a-button size="small" type="link" @click="setCron('0 0 */6 * * *')">每6小时</a-button>
                              <a-button size="small" type="link" @click="setCron('0 30 8 * * 1-5')">工作日8:30</a-button>
                            </div>
                          </template>
                        </a-space>
                      </a-form-item>
                      <a-form-item label="Webhook 触发">
                        <a-space direction="vertical" style="width: 100%">
                          <a-switch v-model:checked="webhookEnabled" />
                          <template v-if="webhookEnabled">
                            <a-input
                              v-model:value="form.trigger_config.webhook.url"
                              placeholder="Webhook URL (保存后自动生成)"
                              disabled
                              style="width: 400px"
                            >
                              <template #addonAfter>
                                <a-button type="link" size="small" @click="copyWebhookUrl" :disabled="!form.trigger_config.webhook.url">
                                  复制
                                </a-button>
                              </template>
                            </a-input>
                            <a-input
                              v-model:value="form.trigger_config.webhook.secret"
                              placeholder="Webhook Secret (可选，用于签名验证)"
                              style="width: 400px"
                            />
                            <a-select
                              v-model:value="form.trigger_config.webhook.branch_filter"
                              mode="tags"
                              placeholder="分支过滤 (留空匹配所有分支)"
                              style="width: 400px"
                            />
                          </template>
                        </a-space>
                      </a-form-item>
                    </a-form>
                  </a-card>
                </a-col>

                <a-col :span="24" style="margin-top: 16px">
                  <a-card title="环境变量" size="small" :bordered="false" class="inner-card">
                    <template #extra>
                      <a-button type="link" size="small" @click="addVariable">
                        <PlusOutlined /> 添加变量
                      </a-button>
                    </template>
                    <a-table :columns="varColumns" :data-source="form.variables" :pagination="false" size="small">
                      <template #bodyCell="{ column, record, index }">
                        <template v-if="column.key === 'name'">
                          <a-input v-model:value="record.name" placeholder="变量名" size="small" />
                        </template>
                        <template v-else-if="column.key === 'value'">
                          <a-input-password v-if="record.is_secret" v-model:value="record.value" placeholder="变量值" size="small" />
                          <a-input v-else v-model:value="record.value" placeholder="变量值" size="small" />
                        </template>
                        <template v-else-if="column.key === 'is_secret'">
                          <a-switch v-model:checked="record.is_secret" size="small" />
                        </template>
                        <template v-else-if="column.key === 'action'">
                          <a-button type="link" size="small" danger @click="removeVariable(index)">删除</a-button>
                        </template>
                      </template>
                    </a-table>
                  </a-card>
                </a-col>
              </a-row>
            </a-card>

            <a-card v-else title="步骤 5：预览与创建" size="small" :bordered="false">
              <a-descriptions :column="2" bordered size="small">
                <a-descriptions-item label="流水线名称">{{ form.name || '-' }}</a-descriptions-item>
                <a-descriptions-item label="关联应用">{{ selectedApplicationLabel }}</a-descriptions-item>
                <a-descriptions-item label="交付环境">{{ selectedEnvLabel }}</a-descriptions-item>
                <a-descriptions-item label="Git 分支">{{ form.git_branch || '-' }}</a-descriptions-item>
                <a-descriptions-item label="Git 仓库">{{ selectedRepoName }}</a-descriptions-item>
                <a-descriptions-item label="构建方式">GitLab Runner</a-descriptions-item>
                <a-descriptions-item label="CI 配置">.gitlab-ci.yml</a-descriptions-item>
                <a-descriptions-item label="Dockerfile">CI 运行时渲染临时文件</a-descriptions-item>
                <a-descriptions-item label="CI YAML 模式">{{ form.gitlab_ci_yaml_custom ? '手动编辑' : '平台生成' }}</a-descriptions-item>
                <a-descriptions-item label="阶段数">{{ form.stages.length }}</a-descriptions-item>
                <a-descriptions-item label="步骤总数">{{ totalStepCount }}</a-descriptions-item>
                <a-descriptions-item label="变量数">{{ form.variables.filter(v => v.name).length }}</a-descriptions-item>
                <a-descriptions-item label="触发方式" :span="2">{{ triggerSummary }}</a-descriptions-item>
                <a-descriptions-item label="描述" :span="2">{{ form.description || '未填写描述' }}</a-descriptions-item>
              </a-descriptions>

              <a-card title="阶段概览" size="small" class="review-card" :bordered="false">
                <a-empty v-if="form.stages.length === 0" description="尚未配置阶段" />
                <a-timeline v-else>
                  <a-timeline-item v-for="stage in form.stages" :key="stage.id">
                    <div class="review-stage-title">{{ stage.name }}</div>
                    <div class="review-stage-meta">
                      {{ stage.steps.length }} 个步骤
                      <span v-if="stage.needs?.length">，依赖：{{ stage.needs.join(' / ') }}</span>
                    </div>
                  </a-timeline-item>
                </a-timeline>
              </a-card>
            </a-card>
          </a-col>

          <a-col :span="7">
            <a-card title="向导提示" size="small" :bordered="false" class="wizard-sidebar">
              <a-typography-paragraph type="secondary">
                当前步骤：{{ wizardSteps[currentStep].title }}
              </a-typography-paragraph>
              <a-list size="small" :data-source="wizardChecklist">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <span :class="item.done ? 'check-done' : 'check-pending'">{{ item.label }}</span>
                  </a-list-item>
                </template>
              </a-list>
            </a-card>

            <a-card title="快捷入口" size="small" :bordered="false" class="wizard-sidebar" style="margin-top: 16px">
              <a-space direction="vertical" style="width: 100%">
                <a-button block type="primary" ghost @click="goTo('/pipeline/templates')"><FileAddOutlined /> 模板市场</a-button>
                <a-button block @click="showTemplateSelector"><FileAddOutlined /> 选择模板</a-button>
                <a-button block @click="goTo('/pipeline/git-repos')">Git 仓库管理</a-button>
              </a-space>
            </a-card>
          </a-col>
        </a-row>
      </div>
    </a-card>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, watch, onMounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message, Modal } from 'ant-design-vue'
import { PlusOutlined, DeleteOutlined, FileAddOutlined } from '@ant-design/icons-vue'
import { pipelineApi, gitRepoApi } from '@/services/pipeline'
import { applicationApi, type Application } from '@/services/application'
import request from '@/services/api'
import yaml from 'js-yaml'
import TemplateSelector from '@/components/pipeline/TemplateSelector.vue'
import { useUserStore } from '@/stores/user'

interface Step {
  id: string
  name: string
  type: string
  image: string
  commands: string[]
  commandsText: string
  work_dir: string
  timeout: number
  env: Record<string, string>
}

interface Stage {
  id: string
  name: string
  steps: Step[]
  needs: string[]
}

interface Variable {
  name: string
  value: string
  is_secret: boolean
}

const stringifyVariableValue = (value: unknown) => {
  if (value === undefined || value === null) return ''
  if (typeof value === 'string') return value
  if (typeof value === 'number' || typeof value === 'boolean') return String(value)
  return JSON.stringify(value)
}

const normalizeVariables = (variables: unknown): Variable[] => {
  if (!variables) return []

  if (Array.isArray(variables)) {
    return variables
      .filter((v): v is Record<string, unknown> => !!v && typeof v === 'object' && !Array.isArray(v))
      .map((v) => ({
        name: String(v.name ?? v.key ?? '').trim(),
        value: stringifyVariableValue(v.value),
        is_secret: !!(v.is_secret ?? v.secret ?? v.masked)
      }))
      .filter(v => v.name)
  }

  if (typeof variables === 'object') {
    return Object.entries(variables as Record<string, unknown>)
      .map(([name, value]) => {
        if (value && typeof value === 'object' && !Array.isArray(value)) {
          const variable = value as Record<string, unknown>
          if ('name' in variable || 'key' in variable || 'value' in variable) {
            return {
              name: String(variable.name ?? variable.key ?? name).trim(),
              value: stringifyVariableValue(variable.value),
              is_secret: !!(variable.is_secret ?? variable.secret ?? variable.masked)
            }
          }
        }
        return {
          name: String(name).trim(),
          value: stringifyVariableValue(value),
          is_secret: false
        }
      })
      .filter(v => v.name)
  }

  return []
}

interface GitRepo {
  id: number
  name: string
  url: string
  provider?: string
  credential_id?: number
  default_branch?: string
}

interface GitBranch {
  name: string
  is_default?: boolean
  commit_sha?: string
}

interface TriggerConfig {
  manual: boolean
  scheduled: {
    enabled: boolean
    cron: string
    timezone: string
  }
  webhook: {
    enabled: boolean
    secret: string
    branch_filter: string[]
    url: string
  }
}

const route = useRoute()
const router = useRouter()

const isEdit = computed(() => !!route.params.id)
const pipelineId = computed(() => Number(route.params.id) || 0)

const privilegedRoles = ['admin', 'administrator', 'super_admin']
const getStoredRoles = (): string[] => {
  const user = useUserStore().userInfo
  if (!user) return []
  if (Array.isArray(user.roles) && user.roles.length > 0) return user.roles
  if (typeof user.role === 'string' && user.role) return [user.role]
  return []
}

const saving = ref(false)
const editMode = ref<'visual' | 'yaml'>('visual')
const yamlContent = ref('')
const yamlError = ref('')
const gitRepos = ref<GitRepo[]>([])
const applications = ref<Application[]>([])
const repoBranches = ref<GitBranch[]>([])
const branchLoading = ref(false)
const gitRepoCheck = reactive({
  success: false,
  message: '',
})
const templateModalVisible = ref(false)
const saveTemplateVisible = ref(false)
const saveTemplateLoading = ref(false)
const currentStep = ref(0)
const sourceTemplateId = ref<number>()
const userRoles = ref<string[]>(getStoredRoles())
const templateForm = reactive({
  name: '',
  description: '',
  category: 'build',
  is_public: false,
})

const wizardSteps = [
  { key: 'basic', title: '基本信息', description: '明确流水线名称和用途' },
  { key: 'resource', title: '源码与 Runner', description: '选择 GitLab 仓库和分支' },
  { key: 'orchestration', title: '模板与编排', description: '使用模板或手动设计阶段步骤' },
  { key: 'trigger', title: '触发与变量', description: '配置触发方式与环境变量' },
  { key: 'review', title: '预览创建', description: '检查配置后保存' }
] as const

const canManagePipelineTemplates = computed(() =>
  userRoles.value.some(role => privilegedRoles.includes(role))
)
const canFreelyDesignPipeline = computed(() => canManagePipelineTemplates.value)

// 触发器配置的计算属性
const scheduledEnabled = computed({
  get: () => form.trigger_config.scheduled?.enabled || false,
  set: (val) => {
    if (!form.trigger_config.scheduled) {
      form.trigger_config.scheduled = { enabled: false, cron: '', timezone: 'Asia/Shanghai' }
    }
    form.trigger_config.scheduled.enabled = val
  }
})

const webhookEnabled = computed({
  get: () => form.trigger_config.webhook?.enabled || false,
  set: (val) => {
    if (!form.trigger_config.webhook) {
      form.trigger_config.webhook = { enabled: false, secret: '', branch_filter: [], url: '' }
    }
    form.trigger_config.webhook.enabled = val
  }
})

const form = reactive({
  name: '',
  description: '',
  application_id: undefined as number | undefined,
  application_name: '',
  env: 'dev',
  git_repo_id: undefined as number | undefined,
  git_branch: 'main',
  gitlab_ci_yaml: '',
  gitlab_ci_yaml_custom: false,
  dockerfile_content: '',
  stages: [] as Stage[],
  variables: [] as Variable[],
  trigger_config: {
    manual: true,
    scheduled: {
      enabled: false,
      cron: '',
      timezone: 'Asia/Shanghai'
    },
    webhook: {
      enabled: false,
      secret: '',
      branch_filter: [],
      url: ''
    }
  } as TriggerConfig
})

const imageOptions = [
  { value: 'node:20-alpine' },
  { value: 'node:18-alpine' },
  { value: 'golang:1.25-alpine' },
  { value: 'golang:1.24-alpine' },
  { value: 'rust:1.87-alpine' },
  { value: 'maven:3.9-eclipse-temurin-17' },
  { value: 'python:3.11-alpine' },
  { value: 'alpine/git:latest' },
  { value: 'gcr.io/kaniko-project/executor:latest' },
  { value: 'bitnami/kubectl:latest' },
  { value: 'docker:latest' }
]

const selectedRepoName = computed(() => {
  const repo = gitRepos.value.find(item => item.id === form.git_repo_id)
  return repo ? `${repo.name} (${repo.url})` : '未选择'
})
const selectedApplication = computed(() => applications.value.find(item => item.id === form.application_id))
const selectedApplicationLabel = computed(() => {
  const app = selectedApplication.value
  return app ? `${app.display_name || app.name} (${app.name})` : '未选择'
})
const envOptions = [
  { label: '开发环境', value: 'dev' },
  { label: '测试环境', value: 'test' },
  { label: '预发环境', value: 'staging' },
  { label: '生产环境', value: 'prod' }
]
const selectedEnvLabel = computed(() => envOptions.find(item => item.value === form.env)?.label || form.env || '-')

const totalStepCount = computed(() => form.stages.reduce((total, stage) => total + stage.steps.length, 0))
const currentVariableCount = computed(() => form.variables.filter(v => String(v.name || '').trim()).length)
const hasTemplateContent = computed(() =>
  !!sourceTemplateId.value ||
  form.stages.length > 0 ||
  currentVariableCount.value > 0 ||
  !!form.dockerfile_content.trim() ||
  !!form.gitlab_ci_yaml.trim()
)

const inferPipelineLanguage = () => {
  const explicit = form.variables.find(v => ['APP_LANGUAGE', 'LANGUAGE', 'RUNTIME', 'FRAMEWORK'].includes(String(v.name || '').toUpperCase()))?.value
  const normalizedExplicit = String(explicit || '').trim().toLowerCase()
  if (normalizedExplicit) return normalizedExplicit

  for (const stage of form.stages) {
    for (const step of stage.steps) {
      const image = String(step.image || '').toLowerCase()
      if (image.includes('golang')) return 'go'
      if (image.includes('rust')) return 'rust'
      if (image.includes('maven') || image.includes('jdk') || image.includes('java')) return 'java'
      if (image.includes('node') || image.includes('npm')) return 'node'
      if (image.includes('python')) return 'python'
      for (const command of step.commands) {
        const lower = String(command || '').toLowerCase()
        if (lower.includes('go build') || lower.includes('go test')) return 'go'
        if (lower.includes('cargo build') || lower.includes('cargo test') || lower.includes('cargo clippy')) return 'rust'
        if (lower.includes('mvn ') || lower.includes('gradle')) return 'java'
        if (lower.includes('npm ') || lower.includes('pnpm ') || lower.includes('yarn ')) return 'node'
        if (lower.includes('pip ') || lower.includes('pytest') || lower.includes('python ')) return 'python'
      }
    }
  }
  return 'universal'
}

const defaultDockerfileTemplate = () => {
  const language = inferPipelineLanguage()
  if (['go', 'golang'].includes(language)) {
    return `FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; elif [ -d ./cmd/server ]; then CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/server; else CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .; fi

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]
`
  }
  if (['rust', 'cargo'].includes(language)) {
    return `FROM rust:1.87-alpine AS builder
WORKDIR /src
RUN apk add --no-cache musl-dev pkgconfig openssl-dev build-base
COPY Cargo.toml Cargo.lock* ./
COPY src ./src
COPY . .
ARG BUILD_COMMAND=""
ARG APP_PORT="8080"
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else cargo build --release; fi
RUN bin_path="$(find target/release -maxdepth 1 -type f -perm -111 | head -n 1)" && cp "$bin_path" /tmp/app

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata libgcc libstdc++
COPY --from=builder /tmp/app /app/app
EXPOSE \${APP_PORT}
ENTRYPOINT ["/app/app"]
`
  }
  if (['java', 'maven', 'spring'].includes(language)) {
    return `FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /src
COPY pom.xml ./
RUN mvn -B -DskipTests dependency:go-offline
COPY . .
ARG BUILD_COMMAND="mvn -B clean package -DskipTests"
RUN sh -c "$BUILD_COMMAND"

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=builder /src/target/*.jar /app/app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`
  }
  if (['node', 'nodejs', 'npm', 'vue', 'react'].includes(language)) {
    return `FROM node:20-alpine AS builder
WORKDIR /src
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
ARG BUILD_COMMAND="npm run build --if-present"
RUN sh -c "$BUILD_COMMAND"
RUN npm prune --omit=dev

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /src /app
EXPOSE 3000
CMD ["npm", "start"]
`
  }
  if (['python', 'python3', 'django', 'flask'].includes(language)) {
    return `FROM python:3.12-alpine
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements*.txt ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
COPY . .
EXPOSE 8000
CMD ["python", "app.py"]
`
  }
  return `FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache bash ca-certificates
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else echo "No BUILD_COMMAND configured; packaging repository content."; fi
CMD ["sh", "-c", "echo 'Image built by JeriDevOps GitLab Runner'; sleep infinity"]
`
}

const sanitizeGitLabIdentifier = (value?: string) => {
  const normalized = String(value || '').trim().toLowerCase().replace(/[^a-z0-9]+/g, '_').replace(/^_+|_+$/g, '')
  return normalized || 'item'
}

const looksGeneratedClientId = (value?: string) => {
  const normalized = String(value || '').trim()
  return /^[a-z0-9]{8}$/.test(normalized) && /\d/.test(normalized)
}

const looksGenericClientName = (value?: string) => {
  const raw = String(value || '').trim()
  const lower = raw.toLowerCase()
  return !raw || ['job', 'step', 'stage'].includes(lower) || raw.startsWith('步骤 ') || raw.startsWith('阶段 ')
}

const gitLabNameAlias = (value?: string) => {
  const raw = String(value || '').trim()
  const lower = raw.toLowerCase()
  if (!raw) return ''
  if (raw.includes('代码检出') || raw.includes('源码检出') || raw.includes('拉取代码') || lower.includes('git clone') || lower.includes('checkout')) return 'checkout'
  if (raw.includes('单元测试') || raw.includes('测试') || lower.includes('go test') || lower.includes('maven test') || lower.includes('npm test') || lower.includes('pytest')) return 'test'
  if (raw.includes('镜像构建') || raw.includes('生成镜像') || raw.includes('构建镜像') || raw.includes('推送镜像') || lower.includes('docker build') || lower.includes('docker push') || lower.includes('image')) return 'image'
  if (raw.includes('编译构建') || raw.includes('编译') || lower.includes('compile')) return 'build'
  if (raw.includes('GitOps') || lower.includes('gitops') || raw.includes('交接') || lower.includes('handoff')) return 'gitops'
  if (raw.includes('发布') || raw.includes('部署') || lower.includes('deploy') || lower.includes('release')) return 'deploy'
  return ''
}

const semanticGitLabIdentifier = (id?: string, name?: string, fallback = 'item') => {
  const rawId = String(id || '').trim()
  const rawName = String(name || '').trim()
  if (!looksGeneratedClientId(rawId)) {
    const stableId = sanitizeGitLabIdentifier(rawId)
    if (stableId && stableId !== 'item') return stableId
  }
  const nameAlias = gitLabNameAlias(rawName)
  if (nameAlias) return nameAlias
  const stableName = sanitizeGitLabIdentifier(rawName)
  if (stableName && stableName !== 'item') return stableName
  const idAlias = gitLabNameAlias(rawId)
  if (idAlias) return idAlias
  const fallbackId = sanitizeGitLabIdentifier(rawId)
  if (fallbackId && fallbackId !== 'item') return fallbackId
  return sanitizeGitLabIdentifier(fallback)
}

const gitLabStageName = (stage: Stage) => semanticGitLabIdentifier(stage.id, stage.name, 'stage')

const stepCommands = (step: Step) => (step.commands || []).map(cmd => String(cmd || '').trim()).filter(Boolean)

const gitLabStepProducesJob = (step: Step) => {
  if (step.type === 'git') return false
  if (step.type === 'docker_build' || step.type === 'docker_push') return true
  return stepCommands(step).length > 0
}

const pipelineHasDockerBuild = () =>
  form.stages.some(stage => stage.steps.some(step => step.type === 'docker_build' || step.type === 'docker_push'))

const isRedundantDockerfileCompileStep = (step: Step, hasDockerBuild: boolean) => {
  if (!hasDockerBuild || step.type !== 'container') return false
  const commands = stepCommands(step)
  if (commands.some(command => command.toLowerCase().includes('go test'))) return false
  return commands.some(command => command.toLowerCase().includes('go build'))
}

const stageHasGitLabJob = (stage: Stage, hasDockerBuild: boolean) =>
  stage.steps.some(step => gitLabStepProducesJob(step) && !isRedundantDockerfileCompileStep(step, hasDockerBuild))

const uniqueGitLabName = (base: string, seen: Map<string, number>) => {
  const name = sanitizeGitLabIdentifier(base) || 'job'
  const count = seen.get(name) || 0
  seen.set(name, count + 1)
  return count === 0 ? name : `${name}_${count + 1}`
}

const gitLabJobName = (stage: Stage, step: Step) => {
  if (step.type === 'docker_build' || step.type === 'docker_push') return 'build_and_push_image'
  const stableId = sanitizeGitLabIdentifier(step.id)
  if (stableId && stableId !== 'item' && !looksGeneratedClientId(step.id)) return stableId
  const stableName = sanitizeGitLabIdentifier(step.name)
  if (stableName && stableName !== 'item' && !looksGenericClientName(step.name)) return stableName
  const stepName = semanticGitLabIdentifier(step.id, step.name, 'job')
  return stepName !== 'job' ? stepName : `${gitLabStageName(stage)}_job`
}

const inferStepImage = (commands: string[]) => {
  for (const command of commands) {
    const lower = command.toLowerCase()
    if (lower.includes('go build') || lower.includes('go test')) return 'golang:1.25-alpine'
    if (lower.includes('cargo build') || lower.includes('cargo test') || lower.includes('cargo clippy')) return 'rust:1.87-alpine'
    if (lower.includes('mvn ') || lower.includes('gradle')) return 'maven:3.9-eclipse-temurin-17'
    if (lower.includes('npm ') || lower.includes('pnpm ') || lower.includes('yarn ')) return 'node:20-alpine'
    if (lower.includes('pip ') || lower.includes('pytest') || lower.includes('python ')) return 'python:3.12-alpine'
  }
  return ''
}

const rewriteNpmCIIfNoLockfile = (command: string) =>
  command.trim() === 'npm ci'
    ? 'if [ -f package-lock.json ] || [ -f npm-shrinkwrap.json ]; then npm ci; else npm install; fi'
    : command

const escapeYAMLString = (value: string) => JSON.stringify(value)

const sanitizeDockerImageSegment = (value?: string) => {
  const normalized = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_.-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[-_.]+|[-_.]+$/g, '')
  return normalized || 'app'
}

const variableValue = (name: string) => {
  const matched = form.variables.find(v => String(v.name || '').trim().toUpperCase() === name)
  return String(matched?.value || '').trim()
}

const defaultImageRepository = () => {
  const registry = (variableValue('DOCKER_REGISTRY') || 'localhost:5001').replace(/\/+$/, '')
  const namespace = (variableValue('IMAGE_NAMESPACE') || 'jeridevops').replace(/^\/+|\/+$/g, '')
  const imageName = sanitizeDockerImageSegment(
    form.application_name || variableValue('APP_NAME') || variableValue('APPLICATION_NAME') || form.name
  )
  return namespace ? `${registry}/${namespace}/${imageName}` : `${registry}/${imageName}`
}

const renderDockerfileHeredoc = () => {
  const content = (form.dockerfile_content || defaultDockerfileTemplate()).replace(/\r\n/g, '\n').replace(/\n+$/, '')
  return `cat > .jeridevops.Dockerfile <<'JERIDEVOPS_DOCKERFILE'\n${content}\nJERIDEVOPS_DOCKERFILE`
}

const buildGeneratedGitLabCI = () => {
  const hasDockerBuild = pipelineHasDockerBuild()
  const stages: string[] = []
  const seenStages = new Set<string>()
  form.stages.filter(stage => stageHasGitLabJob(stage, hasDockerBuild)).forEach(stage => {
    const stageName = gitLabStageName(stage)
    if (!seenStages.has(stageName)) {
      seenStages.add(stageName)
      stages.push(stageName)
    }
  })
  if (stages.length === 0) {
    stages.push('package')
  }

  const lines: string[] = [
    '# JeriDevOps managed GitLab Runner pipeline',
    'stages:',
    ...stages.map(stage => `  - ${stage}`),
    'variables:',
    '  DOCKER_TLS_CERTDIR: ""',
    '  DOCKER_DRIVER: overlay2',
    '  IMAGE_TAG: "$CI_COMMIT_SHORT_SHA"',
    `  IMAGE_NAME: ${escapeYAMLString(variableValue('IMAGE_NAME') || variableValue('GITOPS_IMAGE_REPOSITORY') || defaultImageRepository())}`,
    `  GITOPS_IMAGE_REPOSITORY: ${escapeYAMLString(variableValue('GITOPS_IMAGE_REPOSITORY') || variableValue('IMAGE_NAME') || defaultImageRepository())}`,
    '  DOCKER_IMAGE: "$IMAGE_NAME:$IMAGE_TAG"',
  ]

  form.variables
    .filter(v => v.name && !v.is_secret)
    .forEach(v => {
      const name = String(v.name || '').trim().toUpperCase().replace(/[^A-Z0-9_]/g, '')
      if (!name || ['DOCKER_TLS_CERTDIR', 'DOCKER_DRIVER', 'IMAGE_TAG', 'IMAGE_NAME', 'GITOPS_IMAGE_REPOSITORY', 'DOCKER_IMAGE'].includes(name) || /^[0-9]/.test(name)) return
      lines.push(`  ${name}: ${escapeYAMLString(String(v.value ?? ''))}`)
    })

  const jobNames = new Map<string, number>()
  let hasJob = false
  form.stages.forEach(stage => {
    let hasDockerJob = false
    stage.steps.forEach(step => {
      if (!gitLabStepProducesJob(step)) return
      if (isRedundantDockerfileCompileStep(step, hasDockerBuild)) return
      if (step.type === 'docker_push' && hasDockerJob) return
      const stageName = gitLabStageName(stage)
      const jobName = uniqueGitLabName(gitLabJobName(stage, step), jobNames)
      hasJob = true

      if (step.type === 'docker_build' || step.type === 'docker_push') {
        hasDockerJob = true
        lines.push(
          '',
          `${jobName}:`,
          `  stage: ${stageName}`,
          '  image: docker:26',
          '  services:',
          '    - docker:26-dind',
          '  before_script:',
          '    - docker info',
          '    - if [ -n "$CI_REGISTRY" ]; then docker login -u "$CI_REGISTRY_USER" -p "$CI_REGISTRY_PASSWORD" "$CI_REGISTRY"; fi',
          '    - if [ -n "$DOCKER_REGISTRY" ] && [ -n "$DOCKER_REGISTRY_USERNAME" ] && [ -n "$DOCKER_REGISTRY_PASSWORD" ]; then docker login -u "$DOCKER_REGISTRY_USERNAME" -p "$DOCKER_REGISTRY_PASSWORD" "$DOCKER_REGISTRY"; fi',
          '  script:',
          `    - ${escapeYAMLString(renderDockerfileHeredoc())}`,
          '    - docker build --pull -f .jeridevops.Dockerfile -t "$DOCKER_IMAGE" .',
          '    - docker push "$DOCKER_IMAGE"',
          '  rules:',
          `    - if: '$CI_COMMIT_BRANCH == "${form.git_branch || 'main'}"'`,
          '      when: on_success'
        )
        return
      }

      const commands = stepCommands(step).map(rewriteNpmCIIfNoLockfile)
      const image = step.image || inferStepImage(commands) || 'alpine:3.20'
      lines.push(
        '',
        `${jobName}:`,
        `  stage: ${stageName}`,
        `  image: ${escapeYAMLString(image)}`,
        '  script:',
        ...commands.map(command => `    - ${escapeYAMLString(command)}`),
        '  rules:',
        `    - if: '$CI_COMMIT_BRANCH == "${form.git_branch || 'main'}"'`,
        '      when: on_success'
      )
    })
  })

  if (!hasJob) {
    lines.push(
      '',
      'build_image:',
      '  stage: package',
      '  image: docker:26',
      '  services:',
      '    - docker:26-dind',
      '  before_script:',
      '    - docker info',
      '  script:',
      `    - ${escapeYAMLString(renderDockerfileHeredoc())}`,
      '    - docker build --pull -f .jeridevops.Dockerfile -t "$DOCKER_IMAGE" .',
      '    - docker push "$DOCKER_IMAGE"',
      '  rules:',
      `    - if: '$CI_COMMIT_BRANCH == "${form.git_branch || 'main'}"'`,
      '      when: on_success'
    )
  }

  return `${lines.join('\n')}\n`
}

const refreshGeneratedGitLabCI = () => {
  form.gitlab_ci_yaml = buildGeneratedGitLabCI()
}

const ensureCITemplates = () => {
  if (!form.dockerfile_content.trim()) {
    form.dockerfile_content = defaultDockerfileTemplate()
  }
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  } else if (!form.gitlab_ci_yaml.trim()) {
    form.gitlab_ci_yaml = buildGeneratedGitLabCI()
  }
}

const resetDockerfileTemplate = () => {
  form.dockerfile_content = defaultDockerfileTemplate()
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
  message.success('已按当前阶段配置生成 Dockerfile 模板')
}

const templateSummary = computed(() => {
  if (!canFreelyDesignPipeline.value) {
    if (!sourceTemplateId.value) {
      return '当前角色必须先从模板市场选择标准模板，系统会按模板固化阶段步骤。'
    }
    return `当前已绑定标准模板，创建时会锁定阶段步骤，只保留仓库、触发器和变量可调整。`
  }
  if (form.stages.length === 0) {
    return '尚未配置阶段，建议先从模板开始，再根据实际链路微调。'
  }
  return `当前已配置 ${form.stages.length} 个阶段、${totalStepCount.value} 个步骤、${currentVariableCount.value} 个变量，可继续调整镜像、命令、Dockerfile 与 CI。`
})

const triggerSummary = computed(() => {
  const result: string[] = []
  if (form.trigger_config.manual) result.push('手动触发')
  if (scheduledEnabled.value) result.push(`定时触发(${form.trigger_config.scheduled.cron || '未填写 Cron'})`)
  if (webhookEnabled.value) {
    const branches = form.trigger_config.webhook.branch_filter?.length
      ? form.trigger_config.webhook.branch_filter.join(', ')
      : '全部分支'
    result.push(`Webhook(${branches})`)
  }
  return result.length > 0 ? result.join(' / ') : '未配置触发方式'
})

const wizardChecklist = computed(() => [
  { label: form.name ? `已命名：${form.name}` : '填写流水线名称', done: !!form.name },
  { label: form.application_id ? `已关联应用：${selectedApplicationLabel.value}` : '选择关联应用', done: !!form.application_id },
  { label: form.env ? `交付环境：${selectedEnvLabel.value}` : '选择交付环境', done: !!form.env },
  { label: form.git_repo_id ? `已选仓库：${selectedRepoName.value}` : '选择 Git 仓库', done: !!form.git_repo_id },
  { label: '使用 GitLab Runner 构建', done: !!form.git_repo_id },
  { label: form.gitlab_ci_yaml_custom ? '手动维护 .gitlab-ci.yml' : '平台生成 .gitlab-ci.yml', done: !!form.git_repo_id },
  { label: 'Dockerfile 模板在 CI 运行时渲染', done: !!form.dockerfile_content.trim() },
  { label: form.stages.length > 0 ? `已配置 ${form.stages.length} 个阶段` : '配置至少一个阶段', done: form.stages.length > 0 },
  { label: triggerSummary.value, done: triggerSummary.value !== '未配置触发方式' }
])

const varColumns = [
  { title: '变量名', key: 'name', width: 200 },
  { title: '变量值', key: 'value' },
  { title: '敏感', key: 'is_secret', width: 80 },
  { title: '操作', key: 'action', width: 80 }
]

const generateId = () => Math.random().toString(36).substring(2, 10)

const pipelineNamePattern = /^[a-zA-Z][a-zA-Z0-9_-]{1,63}$/

const toPipelineSlug = (value?: string) => {
  const slug = String(value || '')
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/-+/g, '-')
    .replace(/^[^a-z]+/, '')
    .replace(/[-_]+$/g, '')
  return slug || 'app'
}

const buildDeliveryPipelineName = (appName?: string, envName?: string) =>
  `${toPipelineSlug(appName)}-${toPipelineSlug(envName || 'dev')}-delivery`

const addStage = () => {
  if (!canFreelyDesignPipeline.value) return
  const name = `阶段 ${form.stages.length + 1}`
  form.stages.push({
    id: toPipelineSlug(name),
    name,
    steps: [],
    needs: []
  })
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

const removeStage = (index: number) => {
  if (!canFreelyDesignPipeline.value) return
  form.stages.splice(index, 1)
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

const addStep = (stageIndex: number) => {
  if (!canFreelyDesignPipeline.value) return
  const name = `步骤 ${form.stages[stageIndex].steps.length + 1}`
  form.stages[stageIndex].steps.push({
    id: `${toPipelineSlug(form.stages[stageIndex].name)}-${toPipelineSlug(name)}`,
    name,
    type: 'container',
    image: imageOptions[0].value,
    commands: [],
    commandsText: '',
    work_dir: '/workspace',
    timeout: 600,
    env: {}
  })
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

const removeStep = (stageIndex: number, stepIndex: number) => {
  if (!canFreelyDesignPipeline.value) return
  form.stages[stageIndex].steps.splice(stepIndex, 1)
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

const parseCommands = (step: Step) => {
  if (!canFreelyDesignPipeline.value) return
  step.commands = step.commandsText.split('\n').filter(cmd => cmd.trim())
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

const addVariable = () => {
  form.variables.push({ name: '', value: '', is_secret: false })
}

const removeVariable = (index: number) => {
  form.variables.splice(index, 1)
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
}

// 触发器相关方法
const setCron = (cron: string) => {
  if (!form.trigger_config.scheduled) {
    form.trigger_config.scheduled = { enabled: true, cron: '', timezone: 'Asia/Shanghai' }
  }
  form.trigger_config.scheduled.cron = cron
}

const validateStep = (step: number) => {
  if (step === 0 && !form.name.trim()) {
    message.error('请先填写流水线名称')
    return false
  }
  if (step === 0 && !pipelineNamePattern.test(form.name.trim())) {
    message.error('流水线名称只能包含英文字母、数字、下划线和横线，且必须以字母开头，长度 2-64 个字符')
    return false
  }
  if (step === 0 && !form.application_id) {
    message.error('请先选择关联应用')
    return false
  }
  if (step === 0 && !form.env) {
    message.error('请先选择交付环境')
    return false
  }
  if (step === 1) {
    if (!form.git_repo_id) {
      message.error('请先选择 Git 仓库')
      return false
    }
    const selectedRepo = gitRepos.value.find(item => item.id === form.git_repo_id)
    if (selectedRepo && selectedRepo.provider && selectedRepo.provider !== 'gitlab') {
      message.error('请选择 GitLab 仓库')
      return false
    }
    if (!form.dockerfile_content.trim()) {
      form.dockerfile_content = defaultDockerfileTemplate()
    }
    if (form.gitlab_ci_yaml_custom && !form.gitlab_ci_yaml.trim()) {
      message.error('已启用手动编辑 .gitlab-ci.yml，请填写 CI 内容')
      return false
    }
  }
  if (step === 2 && form.stages.length === 0) {
    message.error(canFreelyDesignPipeline.value ? '请至少添加一个阶段或从模板载入配置' : '请先从模板市场选择标准模板')
    return false
  }
  if (step === 2 && !canFreelyDesignPipeline.value && !sourceTemplateId.value) {
    message.error('当前角色必须先选择标准模板')
    return false
  }
  if (step === 3 && scheduledEnabled.value && !form.trigger_config.scheduled.cron.trim()) {
    message.error('已启用定时触发，请填写 Cron 表达式')
    return false
  }
  return true
}

const nextStep = () => {
  if (!validateStep(currentStep.value)) return
  if (currentStep.value < wizardSteps.length - 1) {
    currentStep.value += 1
  }
}

const prevStep = () => {
  if (currentStep.value > 0) {
    currentStep.value -= 1
  }
}

const goTo = (path: string) => {
  router.push(path)
}

const copyWebhookUrl = () => {
  if (form.trigger_config.webhook?.url) {
    navigator.clipboard.writeText(window.location.origin + form.trigger_config.webhook.url)
    message.success('Webhook URL 已复制')
  }
}

// 同步可视化配置到 YAML
const syncToYaml = () => {
  const config: any = {
    name: form.name,
    variables: {} as Record<string, string>,
    stages: form.stages.map(stage => ({
      name: stage.name,
      needs: stage.needs.length > 0 ? stage.needs : undefined,
      steps: stage.steps.map(step => ({
        name: step.name,
        type: step.type || 'container',
        image: step.image,
        commands: step.commands,
        work_dir: step.work_dir !== '/workspace' ? step.work_dir : undefined,
        timeout: step.timeout !== 600 ? step.timeout : undefined
      }))
    }))
  }

  form.variables.forEach(v => {
    if (v.name) {
      config.variables[v.name] = v.value
    }
  })

  if (Object.keys(config.variables).length === 0) {
    delete config.variables
  }

  yamlContent.value = yaml.dump(config, { indent: 2, lineWidth: -1 })
}

// 解析 YAML 到可视化配置
const parseYaml = () => {
  if (!canFreelyDesignPipeline.value) return
  try {
    const config = yaml.load(yamlContent.value) as any
    if (!config) {
      yamlError.value = ''
      return
    }

    form.name = config.name || form.name
    
    form.variables = normalizeVariables(config.variables)

    if (config.stages) {
      form.stages = config.stages.map((stage: any) => ({
        id: stage.id || toPipelineSlug(stage.name || 'stage'),
        name: stage.name,
        needs: stage.needs || [],
        steps: (stage.steps || []).map((step: any) => ({
          id: step.id || toPipelineSlug(step.name || 'job'),
          name: step.name,
          type: step.type || 'container',
          image: step.image || '',
          commands: step.commands || [],
          commandsText: (step.commands || []).join('\n'),
          work_dir: step.work_dir || '/workspace',
          timeout: step.timeout || 600,
          env: step.env || {}
        }))
      }))
    }

    yamlError.value = ''
  } catch (e: any) {
    yamlError.value = `YAML 解析错误: ${e.message}`
  }
}

// 监听编辑模式切换
watch(editMode, (mode) => {
  if (!canFreelyDesignPipeline.value && mode !== 'visual') {
    editMode.value = 'visual'
    return
  }
  if (mode === 'yaml') {
    syncToYaml()
  }
})

watch(() => form.git_repo_id, (repoId) => {
  loadRepoBranches(repoId)
})

watch(() => form.git_branch, () => {
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
})

watch(() => form.dockerfile_content, () => {
  if (!form.gitlab_ci_yaml_custom) {
    refreshGeneratedGitLabCI()
  }
})

watch(() => form.gitlab_ci_yaml_custom, (custom) => {
  if (!custom) {
    refreshGeneratedGitLabCI()
  } else if (!form.gitlab_ci_yaml.trim()) {
    form.gitlab_ci_yaml = buildGeneratedGitLabCI()
  }
})

const loadGitRepos = async () => {
  try {
    const res = await gitRepoApi.list({ provider: 'gitlab', page_size: 100 })
    if (res?.data?.items) {
      gitRepos.value = res.data.items
    }
  } catch (error) {
    console.error('加载 Git 仓库失败:', error)
  }
}

const loadApplications = async () => {
  try {
    const res = await applicationApi.list({ page: 1, page_size: 1000, status: 'active' })
    applications.value = res?.data?.list || []
  } catch (error) {
    console.error('加载应用列表失败:', error)
  }
}

const applyPrefillFromRoute = () => {
  if (isEdit.value) return
  const appIdRaw = Array.isArray(route.query.application_id) ? route.query.application_id[0] : route.query.application_id
  const envRaw = Array.isArray(route.query.env) ? route.query.env[0] : route.query.env
  const repoIdRaw = Array.isArray(route.query.git_repo_id) ? route.query.git_repo_id[0] : route.query.git_repo_id
  const branchRaw = Array.isArray(route.query.git_branch) ? route.query.git_branch[0] : route.query.git_branch

  const applicationId = Number(appIdRaw)
  if (applicationId) {
    form.application_id = applicationId
    const matchedApp = applications.value.find(item => item.id === applicationId)
    if (matchedApp) {
      form.application_name = matchedApp.name
      if (!form.name) {
        form.name = buildDeliveryPipelineName(matchedApp.name, envRaw || 'dev')
      }
      if (!form.description) {
        form.description = `${matchedApp.display_name || matchedApp.name} ${envRaw || 'dev'} delivery pipeline`
      }
    }
  }
  if (typeof envRaw === 'string' && envRaw) {
    form.env = envRaw
  }
  const gitRepoID = Number(repoIdRaw)
  if (gitRepoID) {
    form.git_repo_id = gitRepoID
  }
  if (typeof branchRaw === 'string' && branchRaw) {
    form.git_branch = branchRaw
  }
}

const filterApplicationOption = (input: string, option: any) => {
  const app = applications.value.find(item => item.id === option.value)
  if (!app) return false
  const text = `${app.name} ${app.display_name || ''} ${app.team || ''}`.toLowerCase()
  return text.includes(input.toLowerCase())
}

const loadRepoBranches = async (repoId?: number) => {
  repoBranches.value = []
  gitRepoCheck.success = false
  gitRepoCheck.message = ''
  if (!repoId) return

  const repo = gitRepos.value.find(item => item.id === repoId)
  if (!repo) return

  branchLoading.value = true
  try {
    if (String(repo.provider || '').toLowerCase() === 'gitlab' && !repo.credential_id) {
      throw new Error('该 GitLab 仓库未绑定 Token 凭证，无法读取私有项目分支，也无法由平台写入 .gitlab-ci.yml。请先到 Git 仓库管理绑定凭证。')
    }
    const [branchRes, testRes] = await Promise.all([
      gitRepoApi.getBranches(repoId),
      gitRepoApi.testConnection({ url: repo.url, credential_id: repo.credential_id }),
    ])

    repoBranches.value = branchRes?.data || []
    if (!form.git_branch && repo.default_branch) {
      form.git_branch = repo.default_branch
    }
    const defaultBranch = repoBranches.value.find(item => item.is_default)
    if ((!form.git_branch || form.git_branch === 'main') && defaultBranch?.name) {
      form.git_branch = defaultBranch.name
    }

    gitRepoCheck.success = !!testRes?.data?.success
    gitRepoCheck.message = testRes?.data?.message || (repoBranches.value.length > 0 ? `已加载 ${repoBranches.value.length} 个分支` : '')
  } catch (error: any) {
    gitRepoCheck.success = false
    gitRepoCheck.message = error?.message || '仓库校验失败，请检查凭证和仓库地址'
    repoBranches.value = []
  } finally {
    branchLoading.value = false
  }
}

const loadPipeline = async () => {
  if (!pipelineId.value) return

  try {
    const res = await pipelineApi.get(pipelineId.value)
    const data = res?.data
    if (data) {
      form.name = data.name
      form.description = data.description
      sourceTemplateId.value = data.source_template_id
      form.git_repo_id = data.git_repo_id
      form.git_branch = data.git_branch || 'main'
      form.gitlab_ci_yaml = data.gitlab_ci_yaml || ''
      form.gitlab_ci_yaml_custom = !!data.gitlab_ci_yaml_custom
      form.dockerfile_content = data.dockerfile_content || ''

      // 加载触发器配置
      if (data.trigger_config) {
        form.trigger_config = {
          manual: data.trigger_config.manual ?? true,
          scheduled: data.trigger_config.scheduled || { enabled: false, cron: '', timezone: 'Asia/Shanghai' },
          webhook: data.trigger_config.webhook || { enabled: false, secret: '', branch_filter: [], url: '' }
        }
      }

      // 转换阶段和步骤
      if (data.stages) {
        form.stages = data.stages.map((stage: any) => ({
          id: stage.id || generateId(),
          name: stage.name,
          needs: stage.depends_on || [],
          steps: (stage.steps || []).map((step: any) => {
            const commands = step.config?.commands || []
            return {
              id: step.id || generateId(),
              name: step.name,
              type: step.type || 'container',
              image: step.config?.image || '',
              commands: commands,
              commandsText: commands.join('\n'),
              work_dir: step.config?.work_dir || '/workspace',
              timeout: step.timeout || 600,
              env: step.config?.env || {}
            }
          })
        }))
      }

      form.variables = normalizeVariables(data.variables)

      if (data.git_repo_id) {
        await loadRepoBranches(data.git_repo_id)
      }
      ensureCITemplates()
    }
  } catch (error) {
    console.error('加载流水线失败:', error)
    message.error('加载流水线失败')
  }
}

const savePipeline = async () => {
  for (const step of [0, 1, 2, 3]) {
    if (!validateStep(step)) {
      currentStep.value = step
      return
    }
  }
  ensureCITemplates()

  saving.value = true
  try {
    // 转换为 API 格式
    const stages = form.stages.map(stage => ({
      id: stage.id,
      name: stage.name,
      depends_on: stage.needs,
      steps: stage.steps.map(step => ({
        id: step.id,
        name: step.name,
        type: step.type || 'container',
        timeout: step.timeout,
        config: {
          image: step.image,
          commands: step.commands,
          work_dir: step.work_dir,
          env: step.env
        }
      }))
    }))

    // 构建触发器配置
    const triggerConfig = {
      manual: form.trigger_config.manual,
      scheduled: scheduledEnabled.value ? form.trigger_config.scheduled : null,
      webhook: webhookEnabled.value ? form.trigger_config.webhook : null
    }

    const data = {
      name: form.name,
      description: form.description,
      application_id: form.application_id,
      application_name: selectedApplication.value?.name || form.application_name,
      env: form.env,
      source_template_id: sourceTemplateId.value,
      git_repo_id: form.git_repo_id,
      git_branch: form.git_branch,
      gitlab_ci_yaml: form.gitlab_ci_yaml_custom ? form.gitlab_ci_yaml : '',
      gitlab_ci_yaml_custom: form.gitlab_ci_yaml_custom,
      dockerfile_content: form.dockerfile_content,
      stages,
      variables: form.variables.filter(v => v.name),
      trigger_config: triggerConfig
    }

    if (isEdit.value) {
      await pipelineApi.update(pipelineId.value, data)
      message.success('更新成功')
    } else {
      await pipelineApi.create(data)
      message.success('创建成功')
    }

    router.push('/pipeline/list')
  } catch (error: any) {
    message.error(error?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const goBack = () => {
  router.push('/pipeline/list')
}

const showTemplateSelector = () => {
  if (!hasTemplateContent.value || (!canFreelyDesignPipeline.value && !sourceTemplateId.value)) {
    templateModalVisible.value = true
    return
  }
  Modal.confirm({
    title: '替换当前模板与编排',
    content: '重新选择模板会覆盖当前阶段、变量、Dockerfile 模板和 .gitlab-ci.yml 配置，仓库、应用、环境与触发器保留。',
    okText: '继续选择模板',
    cancelText: '取消',
    onOk: () => {
      templateModalVisible.value = true
    }
  })
}

const showSaveTemplateModal = () => {
  templateForm.name = form.name ? `${form.name} 模板` : '未命名流水线模板'
  templateForm.description = form.description || ''
  templateForm.category = 'build'
  templateForm.is_public = false
  saveTemplateVisible.value = true
}

const applyTemplateCIConfig = (ci: any) => {
  form.gitlab_ci_yaml_custom = !!ci?.gitlab_ci_yaml_custom
  form.gitlab_ci_yaml = typeof ci?.gitlab_ci_yaml === 'string' ? ci.gitlab_ci_yaml : ''
  form.dockerfile_content = typeof ci?.dockerfile_content === 'string' ? ci.dockerfile_content : ''
  ensureCITemplates()
}

const templateVariablesFromAny = (template: any) => normalizeVariables(template?.variables)

const templateApplySummary = (template: any) => {
  const stages = Array.isArray(template?.stages) ? template.stages : []
  const stepCount = stages.reduce((total: number, stage: any) => total + (Array.isArray(stage?.steps) ? stage.steps.length : 0), 0)
  const variableCount = templateVariablesFromAny(template).length
  const ciMode = template?.ci?.gitlab_ci_yaml_custom ? '手动维护 .gitlab-ci.yml' : '平台生成 .gitlab-ci.yml'
  const dockerfileMode = template?.ci?.dockerfile_content ? '内联 Dockerfile 模板' : '自动生成 Dockerfile 模板'
  return `将覆盖当前编排为 ${stages.length} 个阶段、${stepCount} 个步骤、${variableCount} 个变量，并切换为${dockerfileMode} / ${ciMode}。`
}

const buildTemplateConfigPayload = () => ({
  stages: form.stages.map(stage => ({
    id: stage.id,
    name: stage.name,
    depends_on: stage.needs,
    steps: stage.steps.map(step => ({
      id: step.id,
      name: step.name,
      type: step.type || 'container',
      timeout: step.timeout,
      config: {
        image: step.image,
        commands: step.commands,
        work_dir: step.work_dir,
        env: step.env
      }
    }))
  })),
  variables: form.variables.filter(v => v.name),
  ci: {
    gitlab_ci_yaml: form.gitlab_ci_yaml_custom ? form.gitlab_ci_yaml : '',
    gitlab_ci_yaml_custom: form.gitlab_ci_yaml_custom,
    dockerfile_content: form.dockerfile_content
  }
})

const applyTemplate = (template: any) => {
  const doApply = () => {
    templateModalVisible.value = false
    sourceTemplateId.value = Number(template?.id || template?.template_id || 0) || undefined
    form.name = template.name || form.name
    form.description = template.description || ''
    
    if (template.stages) {
      form.stages = template.stages.map((stage: any) => ({
        id: stage.id || toPipelineSlug(stage.name || 'stage'),
        name: stage.name,
        needs: stage.depends_on || [],
        steps: (stage.steps || []).map((step: any) => {
          const sc = step.config || {}
          const commands = Array.isArray(step.commands)
            ? step.commands
            : Array.isArray(sc.commands)
              ? sc.commands
              : []
          return {
            id: step.id || toPipelineSlug(step.name || 'job'),
            name: step.name,
            type: step.type || sc.type || 'container',
            image: step.image || sc.image || '',
            commands,
            commandsText: commands.join('\n'),
            work_dir: step.work_dir || sc.work_dir || '/workspace',
            timeout: step.timeout || sc.timeout || 600,
            env: step.env || sc.env || {}
          }
        })
      }))
    }
    
    form.variables = normalizeVariables(template.variables)
    applyTemplateCIConfig(template.ci)
    
    currentStep.value = Math.max(currentStep.value, 2)
    message.success('模板已应用')
  }

  if (sourceTemplateId.value || form.stages.length > 0 || currentVariableCount.value > 0) {
    Modal.confirm({
      title: '确认应用模板',
      content: templateApplySummary(template),
      okText: '应用并覆盖',
      cancelText: '取消',
      onOk: doApply
    })
    return
  }

  doApply()
}

const saveAsTemplate = async () => {
  if (!pipelineId.value || !templateForm.name.trim()) {
    message.warning('请填写模板名称')
    return
  }

  saveTemplateLoading.value = true
  try {
    await request.post('/pipeline/templates', {
      name: templateForm.name,
      description: templateForm.description,
      category: templateForm.category,
      is_public: templateForm.is_public,
      config_json: buildTemplateConfigPayload(),
    })
    saveTemplateVisible.value = false
    message.success('已保存为模板')
  } catch (error) {
    console.error(error)
    message.error('保存模板失败')
  } finally {
    saveTemplateLoading.value = false
  }
}

/** 从模版市场详情 API 的 config 映射到编辑器表单（与 loadPipeline 步骤结构对齐） */
const applyMarketConfig = (tmpl: any, cfg: any) => {
  sourceTemplateId.value = Number(tmpl?.id || sourceTemplateId.value || 0) || undefined
  form.name = tmpl.name || form.name || '未命名流水线'
  form.description = tmpl.description || ''
  const stages = cfg?.stages || []
  if (stages.length > 0) {
    form.stages = stages.map((stage: any) => ({
      id: stage.id || toPipelineSlug(stage.name || 'stage'),
      name: stage.name || stage.id || '阶段',
      needs: stage.depends_on || stage.needs || [],
      steps: (stage.steps || []).map((step: any) => {
        const sc = step.config || {}
        const commands = Array.isArray(step.commands)
          ? step.commands
          : Array.isArray(sc.commands)
            ? sc.commands
            : []
        return {
          id: step.id || toPipelineSlug(step.name || 'job'),
          name: step.name || '步骤',
          type: step.type || sc.type || 'container',
          image: step.image || sc.image || '',
          commands,
          commandsText: commands.join('\n'),
          work_dir: step.work_dir || sc.work_dir || '/workspace',
          timeout: step.timeout || sc.timeout || 600,
          env: step.env || sc.env || {}
        }
      })
    }))
  } else {
    addStage()
  }

  form.variables = normalizeVariables(cfg?.variables)
  applyTemplateCIConfig(cfg?.ci)

  message.success('已从模版市场载入配置')
  currentStep.value = Math.max(currentStep.value, 2)
}

const loadMarketTemplate = async (id: number) => {
  if (!id || Number.isNaN(id)) {
    addStage()
    ensureCITemplates()
    return
  }
  try {
    const res = await request.get(`/pipeline/templates/${id}`)
    const payload = res.data || res
    applyMarketConfig(payload.template || {}, payload.config || {})
  } catch (e) {
    console.error('加载市场模板失败', e)
    message.error('加载模板失败')
    addStage()
    ensureCITemplates()
  }
}

onMounted(async () => {
  await Promise.all([loadGitRepos(), loadApplications()])
  if (isEdit.value) {
    await loadPipeline()
    if (!canFreelyDesignPipeline.value) {
      editMode.value = 'visual'
    }
    currentStep.value = 0
    return
  }
  const qid = route.query.template_id
  applyPrefillFromRoute()
  const tidRaw = Array.isArray(qid) ? qid[0] : qid
  if (tidRaw) {
    await loadMarketTemplate(Number(tidRaw))
  } else {
    if (canFreelyDesignPipeline.value) {
      addStage()
    }
  }
  ensureCITemplates()
})
</script>

<style scoped>
.pipeline-editor {
  padding: 0;
}

.wizard-shell {
  margin-top: 16px;
}

.wizard-steps {
  margin-bottom: 24px;
}

.wizard-content {
  min-height: 480px;
}

.wizard-sidebar {
  background: #fafafa;
}

.wizard-actions {
  margin-top: 16px;
}

.runner-summary {
  margin-top: 16px;
}

.ci-editor-card {
  margin-top: 16px;
}

.switch-label {
  color: #595959;
  font-size: 12px;
}

.section-hint {
  margin-bottom: 16px;
}

.inner-card {
  background: #fafafa;
}

.review-card {
  margin-top: 16px;
}

.review-stage-title {
  font-weight: 600;
  color: #1f1f1f;
}

.review-stage-meta {
  color: #8c8c8c;
  font-size: 12px;
}

.step-index {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  background: #1677ff;
  color: #fff;
  font-size: 12px;
  font-weight: 600;
}

.check-done {
  color: #1677ff;
}

.check-pending {
  color: #8c8c8c;
}

.visual-editor {
  min-height: 400px;
}

.stages-container {
  display: flex;
  gap: 16px;
  overflow-x: auto;
  padding: 16px 0;
}

.locked-template-empty {
  min-width: 100%;
  padding: 40px 0;
  background: #fafafa;
  border: 1px dashed #d9d9d9;
  border-radius: 8px;
}

.stage-card {
  min-width: 320px;
  max-width: 320px;
  background: #fafafa;
  border-radius: 8px;
  padding: 12px;
}

.stage-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 12px;
  padding-bottom: 8px;
  border-bottom: 1px solid #e8e8e8;
}

.steps-container {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.step-card {
  background: #fff;
  border: 1px solid #e8e8e8;
  border-radius: 6px;
  padding: 12px;
}

.step-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
}

.step-content :deep(.ant-form-item) {
  margin-bottom: 8px;
}

.empty-steps {
  padding: 20px;
}

.add-stage {
  min-width: 200px;
  display: flex;
  align-items: center;
}

.yaml-editor {
  position: relative;
}

.yaml-textarea,
.code-textarea {
  font-family: 'Monaco', 'Menlo', 'Ubuntu Mono', monospace;
  font-size: 13px;
}

.yaml-error {
  margin-top: 8px;
}

.trigger-hint {
  margin-left: 12px;
  color: #999;
  font-size: 12px;
}

.cron-presets {
  margin-top: 8px;
  color: #666;
  font-size: 12px;
}

.cron-presets span {
  margin-right: 8px;
}
</style>
