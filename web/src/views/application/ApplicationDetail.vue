<template>
  <div class="app-detail">
    <div class="page-header">
      <div class="header-left">
        <a-button type="text" @click="goBack"><ArrowLeftOutlined /> 返回</a-button>
        <h1>{{ app?.display_name || app?.name || '应用详情' }}</h1>
        <a-tag v-if="app?.language" :color="langColors[app.language] || 'default'">{{ app.language }}</a-tag>
      </div>
      <div class="header-right">
        <!-- 发布窗口状态 -->
        <a-tooltip v-if="deployWindowStatus" :title="deployWindowStatus.message">
          <a-tag :color="deployWindowStatus.in_window ? 'green' : 'orange'">
            <ClockCircleOutlined /> {{ deployWindowStatus.in_window ? '发布窗口内' : '窗口外' }}
          </a-tag>
        </a-tooltip>
        <!-- 发布锁状态 -->
        <a-tooltip v-if="deployLockStatus?.locked" :title="`锁定者: ${deployLockStatus.locked_by}, 时间: ${fmtTime(deployLockStatus.locked_at)}`">
          <a-tag color="red"><LockOutlined /> 已锁定</a-tag>
        </a-tooltip>
        <a-button type="primary" @click="showDeployModal()">
          <RocketOutlined /> 执行流水线
        </a-button>
        <a-button @click="showEditModal"><EditOutlined /> 编辑</a-button>
      </div>
    </div>

    <a-spin :spinning="loading">
      <a-card :bordered="false" class="readiness-card" :loading="readinessLoading">
        <a-row :gutter="[16, 12]" align="middle">
          <a-col :xs="24" :lg="6">
            <a-statistic title="交付链路完整度" :value="readiness?.score || 0" suffix="%" :value-style="{ color: readinessColor }" />
            <div class="muted-text">{{ readiness ? `${readiness.completed}/${readiness.total} 项已完成` : '检查中' }}</div>
          </a-col>
          <a-col :xs="24" :lg="12">
            <a-space wrap>
              <a-tag v-for="check in readiness?.checks || []" :key="check.key" :color="readinessCheckColor(check)">
                {{ check.title }}
              </a-tag>
            </a-space>
          </a-col>
          <a-col :xs="24" :lg="6" class="readiness-actions">
            <a-space wrap>
              <a-button @click="fetchReadiness(true)">重新检查</a-button>
              <a-button @click="showOnboardingWizard">接入向导</a-button>
              <a-button v-if="readiness?.next_actions?.length" type="primary" @click="goReadinessNextAction">补齐下一项</a-button>
            </a-space>
          </a-col>
        </a-row>
      </a-card>

      <a-tabs v-model:activeKey="activeTab">
        <!-- 基本信息 -->
        <a-tab-pane key="info" tab="基本信息">
          <a-card :bordered="false">
            <a-descriptions :column="2" bordered>
              <a-descriptions-item label="应用名称">{{ app?.name }}</a-descriptions-item>
              <a-descriptions-item label="显示名称">{{ app?.display_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="语言"><a-tag v-if="app?.language" :color="langColors[app.language]">{{ app.language }}</a-tag><span v-else>-</span></a-descriptions-item>
              <a-descriptions-item label="框架">{{ app?.framework || '-' }}</a-descriptions-item>
              <a-descriptions-item label="所属组织">{{ app?.org_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="所属项目">{{ app?.project_name || '-' }}</a-descriptions-item>
              <a-descriptions-item label="团队">{{ app?.team || '-' }}</a-descriptions-item>
              <a-descriptions-item label="负责人">{{ app?.owner || '-' }}</a-descriptions-item>
              <a-descriptions-item label="Git 仓库" :span="2"><a v-if="app?.git_repo" :href="app.git_repo" target="_blank">{{ app.git_repo }}</a><span v-else>-</span></a-descriptions-item>
              <a-descriptions-item label="描述" :span="2">{{ app?.description || '-' }}</a-descriptions-item>
            </a-descriptions>
          </a-card>
        </a-tab-pane>

        <!-- 仓库绑定 -->
        <a-tab-pane key="repos" tab="仓库绑定">
          <a-card :bordered="false" :loading="repoBindingLoading">
            <template #extra>
              <a-space>
                <a-button type="primary" @click="showRepoBindingModal"><PlusOutlined /> 绑定仓库</a-button>
                <a-button @click="loadRepoBindings">刷新</a-button>
              </a-space>
            </template>

            <a-alert
              v-if="!repoBindings.length && !repoBindingLoading"
              type="info"
              show-icon
              style="margin-bottom: 16px"
              message="当前应用尚未绑定标准 Git 仓库"
              description="请先绑定主仓库，再从应用详情创建交付流水线。"
            />

            <a-table
              v-else
              :columns="repoBindingColumns"
              :data-source="repoBindings"
              row-key="id"
              :pagination="false"
              size="small"
            >
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'repo_name'">
                  <a-space>
                    <span>{{ record.repo_name || `仓库 #${record.git_repo_id}` }}</span>
                    <a-tag v-if="record.is_default" color="green">主仓库</a-tag>
                  </a-space>
                </template>
                <template v-if="column.key === 'repo_provider'">
                  <a-tag>{{ record.repo_provider || '-' }}</a-tag>
                </template>
                <template v-if="column.key === 'repo_url'">
                  <a :href="record.repo_url" target="_blank">{{ record.repo_url }}</a>
                </template>
                <template v-if="column.key === 'default_branch'">
                  {{ record.default_branch || '-' }}
                </template>
                <template v-if="column.key === 'action'">
                  <a-space>
                    <a-button type="link" size="small" :disabled="record.is_default" @click="setDefaultRepoBinding(record)">设为主仓库</a-button>
                    <a-button type="link" size="small" @click="goCreatePipeline(record)">创建流水线</a-button>
                    <a-popconfirm title="确定解除绑定？" @confirm="deleteRepoBinding(record.id)">
                      <a-button type="link" size="small" danger>解除绑定</a-button>
                    </a-popconfirm>
                  </a-space>
                </template>
              </template>
            </a-table>
          </a-card>
        </a-tab-pane>

        <!-- 环境配置 -->
        <a-tab-pane key="envs" tab="环境配置">
          <a-card :bordered="false">
            <template #extra><a-button type="primary" size="small" @click="showEnvModal()"><PlusOutlined /> 添加</a-button></template>
            <a-table :columns="envColumns" :data-source="envDeliveryRows" row-key="id" :pagination="false" :loading="deliveryLoading">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'env_name'"><a-tag :color="envColors[record.env_name]">{{ record.env_name }}</a-tag></template>
                <template v-if="column.key === 'delivery_target'">
                  <a-space v-if="record.delivery_target || record.gitops_target" direction="vertical" size="small" class="env-target-cell">
                    <span v-if="record.gitops_target">{{ record.gitops_target }}</span>
                    <span v-if="record.delivery_target" class="muted-text">{{ record.delivery_target }}</span>
                  </a-space>
                  <span v-else>-</span>
                </template>
                <template v-if="column.key === 'pipeline_binding'">
                  <a-space v-if="record.pipelines.length" size="small" wrap>
                    <a-tag v-for="pipeline in record.pipelines" :key="pipeline.id" color="blue">{{ pipeline.name }}</a-tag>
                  </a-space>
                  <a-tag v-else color="default">未绑定</a-tag>
                </template>
                <template v-if="column.key === 'latest_delivery'">
                  <a-space v-if="record.latest_delivery" size="small" wrap>
                    <a-badge :status="statusType[record.latest_delivery.status] || 'default'" :text="statusText[record.latest_delivery.status] || record.latest_delivery.status" />
                    <span>{{ record.latest_delivery.image_tag || record.latest_delivery.version || '-' }}</span>
                    <span class="muted-text">{{ fmtTime(record.latest_delivery.created_at) }}</span>
                  </a-space>
                  <span v-else>-</span>
                </template>
                <template v-if="column.key === 'action'">
                  <a-space size="small">
                    <a-button type="link" size="small" :disabled="!record.pipelines.length" @click="showDeployModalForEnv(record)">执行</a-button>
                    <a-button type="link" size="small" @click="showEnvDeliveryRecords(record.env_name)">交付记录</a-button>
                    <a-button type="link" size="small" @click="showEnvModal(record)">编辑</a-button>
                    <a-popconfirm title="确定删除？" @confirm="deleteEnv(record.id)"><a-button type="link" size="small" danger>删除</a-button></a-popconfirm>
                  </a-space>
                </template>
              </template>
            </a-table>
          </a-card>
        </a-tab-pane>

        <!-- 交付流水线 -->
        <a-tab-pane key="delivery" tab="交付流水线">
          <a-card :bordered="false" :loading="deliveryLoading">
            <a-row :gutter="16" style="margin-bottom: 16px">
              <a-col :span="6">
                <a-statistic title="关联流水线" :value="deliverySummary.pipelineCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="CI 成功运行" :value="deliverySummary.successRunCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="发布单" :value="deliverySummary.releaseCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="GitOps 变更" :value="deliverySummary.gitopsChangeCount" />
              </a-col>
            </a-row>

            <a-space style="margin-bottom: 16px" wrap>
              <a-tag color="blue">应用：{{ app?.name || '-' }}</a-tag>
              <a-tag v-for="envName in deliveryEnvNames" :key="envName" :color="getEnvColor(envName)">{{ getEnvName(envName) }}</a-tag>
              <a-button type="primary" @click="goCreatePipeline()">创建交付流水线</a-button>
              <a-button @click="fetchDeliveryContext(true)">刷新交付链路</a-button>
            </a-space>

            <a-empty v-if="!hasDeliveryContext && !deliveryLoading" description="暂无交付链路。请先绑定主仓库，再从应用详情创建交付流水线。" />
            <template v-else>
              <a-table
                :columns="relatedPipelineColumns"
                :data-source="relatedPipelines"
                row-key="id"
                :pagination="false"
                size="small"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'env'">
                    <a-tag v-if="record.env" :color="getEnvColor(record.env)">{{ getEnvName(record.env) }}</a-tag>
                    <span v-else>-</span>
                  </template>
                  <template v-if="column.key === 'last_run_status'">
                    <a-badge :status="deliveryStatusType[record.last_run_status] || 'default'" :text="deliveryStatusText[record.last_run_status] || record.last_run_status || '-'" />
                  </template>
                  <template v-if="column.key === 'last_run_at'">{{ fmtTime(record.last_run_at) }}</template>
                  <template v-if="column.key === 'action'">
                    <a-space>
                      <a @click="showDeployModal(record)">执行</a>
                      <a @click="router.push(`/pipeline/${record.id}`)">查看</a>
                    </a-space>
                  </template>
                </template>
              </a-table>

              <a-divider orientation="left">最近运行</a-divider>
              <a-table
                :columns="deliveryRunColumns"
                :data-source="deliveryRuns"
                row-key="id"
                :pagination="false"
                size="small"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'created_at'">{{ fmtTime(record.created_at) }}</template>
                  <template v-if="column.key === 'status'">
                    <a-badge :status="deliveryStatusType[record.status] || 'default'" :text="deliveryStatusText[record.status] || record.status" />
                  </template>
                  <template v-if="column.key === 'gitops'">
                    <a-space v-if="record.gitops_change_request_id" size="small" wrap>
                      <a-tag :color="getGitOpsHandoffColor(record.gitops_handoff_status)">
                        {{ getGitOpsHandoffText(record.gitops_handoff_status) }}
                      </a-tag>
                      <a @click="router.push(`/argocd?tab=changes&changeId=${record.gitops_change_request_id}`)">CR #{{ record.gitops_change_request_id }}</a>
                    </a-space>
                    <a-tooltip v-else-if="record.gitops_handoff_message" :title="record.gitops_handoff_message">
                      <a-tag :color="getGitOpsHandoffColor(record.gitops_handoff_status)">
                        {{ getGitOpsHandoffText(record.gitops_handoff_status) }}
                      </a-tag>
                    </a-tooltip>
                    <a-tag v-else color="default">未交接</a-tag>
                  </template>
                  <template v-if="column.key === 'action'">
                    <a-space>
                      <a @click="router.push(`/pipeline/${record.pipeline_id}?run=${record.id}`)">运行详情</a>
                      <a-button
                        v-if="record.status === 'success'"
                        type="link"
                        size="small"
                        :loading="creatingReleaseRunId === record.id"
                        @click="createReleaseFromRun(record)"
                      >
                        生成发布单
                      </a-button>
                    </a-space>
                  </template>
                </template>
              </a-table>

              <a-divider orientation="left">最近发布单</a-divider>
              <a-table
                :columns="releaseColumns"
                :data-source="recentReleases"
                row-key="id"
                :pagination="false"
                size="small"
              >
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'title'">
                    <a @click="router.push(`/releases/${record.id}`)">{{ record.title }}</a>
                    <div class="muted-text">{{ record.version || '-' }}</div>
                  </template>
                  <template v-if="column.key === 'env'">
                    <a-tag :color="getEnvColor(record.env)">{{ getEnvName(record.env) }}</a-tag>
                  </template>
                  <template v-if="column.key === 'status'">
                    <a-tag :color="releaseStatusColor(record.status)">{{ releaseStatusText(record.status) }}</a-tag>
                  </template>
                  <template v-if="column.key === 'approval'">
                    <a v-if="record.approval_instance_id" @click="router.push(`/approval/instances/${record.approval_instance_id}`)">审批 #{{ record.approval_instance_id }}</a>
                    <span v-else>{{ releaseApprovalText(record) }}</span>
                  </template>
                  <template v-if="column.key === 'gitops'">
                    <a v-if="record.gitops_change_request_id" @click="router.push(`/argocd?tab=changes&changeId=${record.gitops_change_request_id}`)">CR #{{ record.gitops_change_request_id }}</a>
                    <span v-else>-</span>
                  </template>
                  <template v-if="column.key === 'argocd'">
                    <a-space size="small">
                      <a-tag v-if="record.argo_sync_status" :color="argoSyncColor(record.argo_sync_status)">{{ argoSyncText(record.argo_sync_status) }}</a-tag>
                      <span v-else>-</span>
                      <span v-if="record.argo_app_name" class="muted-text">{{ record.argo_app_name }}</span>
                    </a-space>
                  </template>
                  <template v-if="column.key === 'created_at'">{{ fmtTime(record.created_at) }}</template>
                  <template v-if="column.key === 'action'">
                    <a-space>
                      <a @click="router.push(`/releases/${record.id}`)">发布详情</a>
                      <a v-if="record.gitops_change_request_id" @click="router.push(`/argocd?tab=changes&changeId=${record.gitops_change_request_id}`)">GitOps</a>
                    </a-space>
                  </template>
                </template>
              </a-table>
            </template>
          </a-card>
        </a-tab-pane>

        <!-- 交付记录 -->
        <a-tab-pane key="deploys" tab="交付记录">
          <a-card :bordered="false">
            <div style="margin-bottom: 16px">
              <a-space>
                <a-select v-model:value="deployFilter.env_name" placeholder="环境" allow-clear style="width: 100px">
                  <a-select-option v-for="e in ['dev','test','staging','prod']" :key="e" :value="e">{{ e }}</a-select-option>
                </a-select>
                <a-select v-model:value="deployFilter.status" placeholder="状态" allow-clear style="width: 100px">
                  <a-select-option v-for="s in ['success','failed','running','pending']" :key="s" :value="s">{{ statusText[s] }}</a-select-option>
                </a-select>
                <a-button type="primary" @click="fetchDeploys">查询</a-button>
              </a-space>
            </div>
            <a-table :columns="deployColumns" :data-source="deploys" row-key="id" :loading="deploysLoading" :pagination="deployPagination" @change="onDeployPageChange">
              <template #bodyCell="{ column, record }">
                <template v-if="column.key === 'created_at'">{{ fmtTime(record.created_at) }}</template>
                <template v-if="column.key === 'env_name'"><a-tag :color="envColors[record.env_name]">{{ record.env_name }}</a-tag></template>
                <template v-if="column.key === 'deploy_method'">
                  <a-tag :color="record.deploy_method === 'gitops' ? 'blue' : 'default'">{{ getDeployMethodText(record.deploy_method) }}</a-tag>
                </template>
                <template v-if="column.key === 'status'"><a-badge :status="statusType[record.status]" :text="statusText[record.status]" /></template>
                <template v-if="column.key === 'duration'">{{ record.duration ? `${record.duration}s` : '-' }}</template>
                <template v-if="column.key === 'action'"><a-button type="link" size="small" @click="viewDeploy(record)">详情</a-button></template>
              </template>
            </a-table>
          </a-card>
        </a-tab-pane>

        <!-- 可观测 -->
        <a-tab-pane key="observability" tab="可观测">
          <a-card :bordered="false" :loading="appTimelineLoading">
            <a-row :gutter="16" style="margin-bottom: 16px">
              <a-col :span="6">
                <a-statistic title="事故" :value="observabilitySummary.incidentCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="告警" :value="observabilitySummary.alertCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="待处理告警" :value="observabilitySummary.pendingAlertCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="最近事件" :value="observabilitySummary.latestAtText" />
              </a-col>
            </a-row>

            <a-space style="margin-bottom: 16px" wrap>
              <a-button type="primary" @click="goEventTimeline">打开统一事件时间线</a-button>
              <a-button @click="router.push('/logs/unified')">打开日志统一台</a-button>
              <a-button @click="router.push('/alert/center')">打开告警中心</a-button>
            </a-space>

            <a-empty v-if="!observabilityItems.length && !appTimelineLoading" description="暂无可观测事件" />
            <a-list v-else item-layout="vertical" :data-source="observabilityItems.slice(0, 8)">
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-space direction="vertical" size="small" style="width: 100%">
                    <a-space wrap>
                      <a-tag :color="timelineKindColor(item.kind)">{{ timelineKindText(item.kind) }}</a-tag>
                      <a-tag v-if="item.status" :color="timelineStatusColor(item)">{{ timelineStatusText(item) }}</a-tag>
                      <a-tag v-if="item.severity" :color="timelineSeverityColor(item.severity)">{{ item.severity }}</a-tag>
                      <span style="color: #8c8c8c">{{ fmtTime(item.at) }}</span>
                    </a-space>
                    <a @click="openTimelineItem(item)">{{ item.title }}</a>
                    <div v-if="item.summary" style="color: #595959">{{ item.summary }}</div>
                  </a-space>
                </a-list-item>
              </template>
            </a-list>
          </a-card>
        </a-tab-pane>

        <!-- 成本 -->
        <a-tab-pane key="cost" tab="成本">
          <a-card :bordered="false" :loading="costLoading">
            <a-row :gutter="16" style="margin-bottom: 16px">
              <a-col :span="6">
                <a-statistic title="近 30 天应用成本" :value="costSummary.totalCost" :precision="2" prefix="¥" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="本月分摊成本" :value="costSummary.monthlyAllocatedCost" :precision="2" prefix="¥" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="资源效率" :value="costSummary.efficiency" :precision="1" suffix="%" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="待处理建议可节省" :value="costSummary.pendingSavings" :precision="2" prefix="¥" />
              </a-col>
            </a-row>

            <a-space style="margin-bottom: 16px" wrap>
              <a-tag color="blue">应用：{{ app?.display_name || app?.name || '-' }}</a-tag>
              <a-tag v-for="clusterName in appCostClusterNames" :key="clusterName" color="geekblue">集群：{{ clusterName }}</a-tag>
              <a-tag v-for="ns in costAppAggregate?.namespaces || []" :key="ns" color="cyan">命名空间：{{ ns }}</a-tag>
              <a-tag v-for="envName in appCostEnvNames" :key="envName" :color="getEnvColor(envName)">{{ getEnvName(envName) }}</a-tag>
              <a-button type="primary" @click="router.push('/cost/analysis')">打开成本分析</a-button>
              <a-button @click="router.push('/cost/suggestions')">打开优化建议</a-button>
            </a-space>

            <a-alert
              v-if="!costAppAggregate && !costAllocationItem && !costLoading"
              type="info"
              show-icon
              style="margin-bottom: 16px"
              message="暂无当前应用的成本归属数据"
              description="应用成本按 app_name 聚合；未命中时会继续回退展示本月分摊成本、环境基线与优化建议。"
            />

            <template v-else>
              <a-row :gutter="16" style="margin-bottom: 16px">
                <a-col :span="12">
                  <a-card size="small" title="应用成本画像" :bordered="true">
                    <a-descriptions :column="2" size="small">
                      <a-descriptions-item label="资源数">{{ costAppAggregate?.resourceCount ?? costAllocationItem?.resource_count ?? 0 }}</a-descriptions-item>
                      <a-descriptions-item label="应用名">{{ costAppAggregate?.appName || costAllocationItem?.name || app?.name || '-' }}</a-descriptions-item>
                      <a-descriptions-item label="CPU 成本">¥{{ formatCurrency(costAppAggregate?.cpuCost) }}</a-descriptions-item>
                      <a-descriptions-item label="内存成本">¥{{ formatCurrency(costAppAggregate?.memoryCost) }}</a-descriptions-item>
                      <a-descriptions-item label="存储成本">¥{{ formatCurrency(costAppAggregate?.storageCost) }}</a-descriptions-item>
                      <a-descriptions-item label="建议数">{{ costSuggestions.length }}</a-descriptions-item>
                      <a-descriptions-item label="直接成本">¥{{ formatCurrency(costAllocationItem?.direct_cost || costAppAggregate?.totalCost) }}</a-descriptions-item>
                      <a-descriptions-item label="共享分摊">¥{{ formatCurrency(costAllocationItem?.shared_cost) }}</a-descriptions-item>
                    </a-descriptions>
                  </a-card>
                </a-col>
                <a-col :span="12">
                  <a-card size="small" title="资源利用率" :bordered="true">
                    <div class="cost-usage-item">
                      <div class="cost-usage-header">
                        <span>CPU</span>
                        <span>{{ formatNumber(costAppAggregate?.cpuUsage) }} / {{ formatNumber(costAppAggregate?.cpuRequest) }} 核</span>
                      </div>
                      <a-progress :percent="clampPercent(costAppAggregate?.cpuUsageRate)" size="small" :status="getUsageStatus(costAppAggregate?.cpuUsageRate || 0)" />
                    </div>
                    <div class="cost-usage-item">
                      <div class="cost-usage-header">
                        <span>内存</span>
                        <span>{{ formatNumber(costAppAggregate?.memoryUsage) }} / {{ formatNumber(costAppAggregate?.memoryRequest) }} GB</span>
                      </div>
                      <a-progress :percent="clampPercent(costAppAggregate?.memoryUsageRate)" size="small" :status="getUsageStatus(costAppAggregate?.memoryUsageRate || 0)" />
                    </div>
                    <div class="cost-usage-item">
                      <div class="cost-usage-header">
                        <span>效率评级</span>
                        <a-tag :color="getEfficiencyColor(costSummary.efficiency)">{{ formatNumber(costSummary.efficiency, 1) }}%</a-tag>
                      </div>
                      <div class="cost-usage-hint">基于 CPU / 内存请求与实际使用汇总，用于识别超配和降本空间。</div>
                    </div>
                  </a-card>
                </a-col>
              </a-row>

              <a-row :gutter="16" style="margin-bottom: 16px">
                <a-col :span="12">
                  <a-card size="small" title="所在环境成本基线" :bordered="true">
                    <template v-if="costEnvBaseline.length">
                      <a-list size="small" :data-source="costEnvBaseline">
                        <template #renderItem="{ item }">
                          <a-list-item>
                            <a-space direction="vertical" size="small" style="width: 100%">
                              <div class="cost-env-row">
                                <a-space>
                                  <a-tag :color="getEnvColor(item.environment)">{{ getEnvName(item.environment) }}</a-tag>
                                  <span>环境总成本 ¥{{ formatCurrency(item.total_cost) }}</span>
                                </a-space>
                                <span style="color: #8c8c8c">效率 {{ formatNumber(item.avg_efficiency, 1) }}%</span>
                              </div>
                              <div style="color: #8c8c8c">
                                命名空间 {{ item.namespace_count }} 个，应用 {{ item.app_count }} 个，占集群 {{ formatNumber(item.percentage, 1) }}%
                              </div>
                            </a-space>
                          </a-list-item>
                        </template>
                      </a-list>
                    </template>
                    <a-empty v-else description="暂无环境基线数据" />
                    <div class="cost-panel-hint">环境基线来自环境维度成本接口，用于对比当前应用所处环境的整体治理压力。</div>
                  </a-card>
                </a-col>
                <a-col :span="12">
                  <a-card size="small" title="成本优化建议" :bordered="true">
                    <template v-if="costSuggestions.length">
                      <a-list size="small" :data-source="costSuggestions">
                        <template #renderItem="{ item }">
                          <a-list-item>
                            <a-space direction="vertical" size="small" style="width: 100%">
                              <a-space wrap>
                                <a-tag :color="costSuggestionSeverityColor(item.severity)">{{ costSuggestionSeverityText(item.severity) }}</a-tag>
                                <a-tag>{{ item.resource_type }}</a-tag>
                                <a-tag color="green">节省 ¥{{ formatCurrency(item.savings) }}</a-tag>
                              </a-space>
                              <div>{{ item.title }}</div>
                              <div style="color: #595959">{{ item.description }}</div>
                              <div style="color: #8c8c8c">
                                {{ item.namespace || '-' }} / {{ item.resource_name }} / 当前状态：{{ costSuggestionStatusText(item.status) }}
                              </div>
                            </a-space>
                          </a-list-item>
                        </template>
                      </a-list>
                    </template>
                    <a-empty v-else description="暂无匹配当前应用的待处理建议" />
                  </a-card>
                </a-col>
              </a-row>
            </template>
          </a-card>
        </a-tab-pane>

        <!-- 安全 -->
        <a-tab-pane key="security" tab="安全">
          <a-card :bordered="false" :loading="securityLoading">
            <a-row :gutter="16" style="margin-bottom: 16px">
              <a-col :span="6">
                <a-statistic title="扫描记录" :value="securitySummary.scanCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="严重漏洞" :value="securitySummary.criticalCount" :valueStyle="{ color: '#cf1322' }" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="高危漏洞" :value="securitySummary.highCount" :valueStyle="{ color: '#d46b08' }" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="最新风险" :value="securitySummary.latestRiskLabel" />
              </a-col>
            </a-row>

            <a-space style="margin-bottom: 16px" wrap>
              <a-tag v-if="securityScans.length" color="blue">
                关联应用：{{ securityScans[0]?.application_name || app?.name || '-' }}
              </a-tag>
              <a-tag v-if="securityAssociationSummary.explicitCount" color="green">
                显式关联 {{ securityAssociationSummary.explicitCount }}
              </a-tag>
              <a-tag v-if="securityAssociationSummary.legacyCount" color="gold">
                关键字兜底 {{ securityAssociationSummary.legacyCount }}
              </a-tag>
              <a-tag v-if="!securityScans.length" color="default">
                关联方式：优先 application_id，其次 application_name
              </a-tag>
              <a-button type="primary" @click="router.push('/security/image-scan')">打开镜像扫描页</a-button>
              <a-button v-if="securityLatestScan" @click="viewSecurityScanResult(securityLatestScan.id)">查看最新扫描详情</a-button>
              <a-button @click="refreshSecurityScans">刷新当前应用扫描</a-button>
            </a-space>

            <a-space style="margin-bottom: 16px" wrap>
              <a-tag
                :color="securityAssociationFilter === 'all' ? 'blue' : 'default'"
                class="filter-tag"
                @click="setSecurityAssociationFilter('all')"
              >
                全部归属 {{ securityScans.length }}
              </a-tag>
              <a-tag
                :color="securityAssociationFilter === 'explicit' ? 'green' : 'default'"
                class="filter-tag"
                @click="setSecurityAssociationFilter('explicit')"
              >
                显式关联 {{ securityAssociationSummary.explicitCount }}
              </a-tag>
              <a-tag
                :color="securityAssociationFilter === 'legacy' ? 'gold' : 'default'"
                class="filter-tag"
                @click="setSecurityAssociationFilter('legacy')"
              >
                关键字兜底 {{ securityAssociationSummary.legacyCount }}
              </a-tag>
              <a-tag
                :color="securityRiskFilter === 'critical' ? 'red' : 'default'"
                class="filter-tag"
                @click="setSecurityRiskFilter('critical')"
              >
                严重 {{ securityRiskSummary.critical }}
              </a-tag>
              <a-tag
                :color="securityRiskFilter === 'high' ? 'orange' : 'default'"
                class="filter-tag"
                @click="setSecurityRiskFilter('high')"
              >
                高危 {{ securityRiskSummary.high }}
              </a-tag>
              <a-tag
                :color="securityRiskFilter === 'medium' ? 'blue' : 'default'"
                class="filter-tag"
                @click="setSecurityRiskFilter('medium')"
              >
                中危 {{ securityRiskSummary.medium }}
              </a-tag>
              <a-tag
                :color="securityRiskFilter === 'low' ? 'green' : 'default'"
                class="filter-tag"
                @click="setSecurityRiskFilter('low')"
              >
                低危 {{ securityRiskSummary.low }}
              </a-tag>
              <a-tag v-if="securityAssociationFilter !== 'all' || securityRiskFilter" class="filter-tag" @click="resetSecurityFilters">
                清除筛选
              </a-tag>
            </a-space>

            <a-empty
              v-if="!filteredSecurityScans.length && !securityLoading"
              :description="securityScans.length ? '当前筛选条件下暂无扫描记录。' : '暂无匹配当前应用的镜像扫描记录，可补充应用或环境中的 K8s Deployment 名称以提升匹配精度。'"
            />
            <a-list v-else item-layout="vertical" :data-source="filteredSecurityScans.slice(0, 8)">
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-space direction="vertical" size="small" style="width: 100%">
                    <a-space wrap>
                      <a-tag color="purple">镜像扫描</a-tag>
                      <a-tag :color="securityStatusColor(item.status)">{{ securityStatusText(item.status) }}</a-tag>
                      <a-tag v-if="item.risk_level" :color="securityRiskColor(item.risk_level)">{{ securityRiskText(item.risk_level) }}</a-tag>
                      <a-tag v-if="item.association_source" :color="securityAssociationColor(item.association_source)">
                        {{ securityAssociationText(item.association_source) }}
                      </a-tag>
                      <a-tag v-if="item.pipeline_run_id" color="geekblue">运行 #{{ item.pipeline_run_id }}</a-tag>
                      <span style="color: #8c8c8c">{{ fmtTime(item.scanned_at || item.created_at) }}</span>
                    </a-space>
                    <a @click="viewSecurityScanResult(item.id)">{{ item.image }}</a>
                    <div style="color: #8c8c8c">
                      关联应用：{{ item.application_name || app?.name || '-' }}
                    </div>
                    <div class="security-scan-summary">
                      <span class="critical">严重 {{ item.critical_count }}</span>
                      <span class="high">高危 {{ item.high_count }}</span>
                      <span class="medium">中危 {{ item.medium_count }}</span>
                      <span class="low">低危 {{ item.low_count }}</span>
                    </div>
                  </a-space>
                </a-list-item>
              </template>
            </a-list>
          </a-card>
        </a-tab-pane>

        <!-- 变更事件 -->
        <a-tab-pane key="change-events" tab="变更事件">
          <a-card :bordered="false" :loading="appTimelineLoading">
            <a-row :gutter="16" style="margin-bottom: 16px">
              <a-col :span="6">
                <a-statistic title="变更事件" :value="changeSummary.changeEventCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="发布单" :value="changeSummary.releaseCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="审批流" :value="changeSummary.approvalCount" />
              </a-col>
              <a-col :span="6">
                <a-statistic title="已拒绝审批" :value="changeSummary.rejectedApprovalCount" />
              </a-col>
            </a-row>

            <a-empty v-if="!changeItems.length && !appTimelineLoading" description="暂无变更事件" />
            <a-timeline v-else mode="left">
              <a-timeline-item
                v-for="item in changeItems.slice(0, 12)"
                :key="`${item.kind}-${item.id}`"
                :color="timelineKindColor(item.kind)"
              >
                <p style="color: #8c8c8c; margin-bottom: 4px">{{ fmtTime(item.at) }}</p>
                <p style="margin-bottom: 4px">
                  <a-tag :color="timelineKindColor(item.kind)">{{ timelineKindText(item.kind) }}</a-tag>
                  <a-tag v-if="item.status" :color="timelineStatusColor(item)">{{ timelineStatusText(item) }}</a-tag>
                  <a @click="openTimelineItem(item)">{{ item.title }}</a>
                </p>
                <p v-if="item.summary" style="margin: 0; color: #595959">{{ item.summary }}</p>
              </a-timeline-item>
            </a-timeline>
          </a-card>
        </a-tab-pane>
    </a-tabs>
    </a-spin>

    <a-modal
      v-model:open="repoBindingModalVisible"
      title="绑定 Git 仓库"
      @ok="saveRepoBinding"
      :confirmLoading="savingRepoBinding"
      width="520px"
    >
      <a-form :model="repoBindingForm" layout="vertical">
        <a-form-item label="Git 仓库" required>
          <a-select v-model:value="repoBindingForm.git_repo_id" placeholder="选择标准 Git 仓库" show-search option-filter-prop="label">
            <a-select-option
              v-for="repo in availableGitRepos"
              :key="repo.id"
              :value="repo.id"
              :label="`${repo.name} ${repo.url}`"
            >
              {{ repo.name }}（{{ repo.provider || 'gitlab' }} · {{ repo.url }}）
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="绑定角色">
          <a-select v-model:value="repoBindingForm.role">
            <a-select-option value="primary">主仓库</a-select-option>
            <a-select-option value="secondary">辅助仓库</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="设为主仓库">
          <a-switch v-model:checked="repoBindingForm.is_default" />
        </a-form-item>
      </a-form>
    </a-modal>

    <!-- 编辑应用 -->
    <a-modal v-model:open="editModalVisible" title="编辑应用" @ok="saveApp" :confirmLoading="saving" width="600px">
      <a-form :model="editForm" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="应用名称"><a-input v-model:value="editForm.name" disabled /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="显示名称"><a-input v-model:value="editForm.display_name" /></a-form-item></a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="8"><a-form-item label="语言"><a-select v-model:value="editForm.language" allow-clear>
            <a-select-option v-for="l in ['go','java','python','nodejs']" :key="l" :value="l">{{ l }}</a-select-option>
          </a-select></a-form-item></a-col>
          <a-col :span="8"><a-form-item label="框架"><a-input v-model:value="editForm.framework" /></a-form-item></a-col>
          <a-col :span="8"><a-form-item label="团队"><a-input v-model:value="editForm.team" /></a-form-item></a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="所属组织">
              <a-select v-model:value="editForm.organization_id" allow-clear placeholder="选择组织" @change="onEditOrganizationChange">
                <a-select-option v-for="org in organizations" :key="org.id" :value="org.id">{{ org.display_name || org.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="所属项目">
              <a-select v-model:value="editForm.project_id" allow-clear placeholder="选择项目">
                <a-select-option v-for="project in editableProjects" :key="project.id" :value="project.id">{{ project.display_name || project.name }}</a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="负责人"><a-input v-model:value="editForm.owner" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="Git 仓库"><a-input v-model:value="editForm.git_repo" /></a-form-item></a-col>
        </a-row>
        <a-form-item label="描述"><a-textarea v-model:value="editForm.description" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 环境配置 -->
    <a-modal v-model:open="envModalVisible" :title="envForm.id ? '编辑环境' : '添加环境'" @ok="saveEnv" :confirmLoading="savingEnv" width="880px">
      <a-form :model="envForm" layout="vertical">
        <a-row :gutter="16">
          <a-col :span="8"><a-form-item label="环境" required><a-select v-model:value="envForm.env_name" :disabled="!!envForm.id">
            <a-select-option v-for="e in ['dev','test','staging','prod']" :key="e" :value="e">{{ e }}</a-select-option>
          </a-select></a-form-item></a-col>
          <a-col :span="8"><a-form-item label="代码分支"><a-input v-model:value="envForm.branch" placeholder="main" /></a-form-item></a-col>
          <a-col :span="8"><a-form-item label="副本数"><a-input-number v-model:value="envForm.replicas" :min="1" style="width:100%" /></a-form-item></a-col>
        </a-row>
        <a-divider orientation="left">GitOps Helm</a-divider>
        <a-row :gutter="16">
          <a-col :span="12">
            <a-form-item label="部署仓库">
              <a-select v-model:value="envForm.gitops_repo_id" placeholder="选择 GitOps 仓库" allow-clear show-search option-filter-prop="label">
                <a-select-option
                  v-for="repo in gitOpsRepos"
                  :key="repo.id"
                  :value="repo.id"
                  :label="`${repo.name} ${repo.repo_url}`"
                >
                  {{ repo.name }}<span class="muted-text">（{{ repo.branch || 'main' }}）</span>
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="12">
            <a-form-item label="GitOps 分支"><a-input v-model:value="envForm.gitops_branch" placeholder="main" /></a-form-item>
          </a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="部署目录"><a-input v-model:value="envForm.gitops_path" placeholder="apps/ui-runner-smoke-0505" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="Chart 路径"><a-input v-model:value="envForm.helm_chart_path" placeholder="apps/ui-runner-smoke-0505" /></a-form-item></a-col>
        </a-row>
        <a-row :gutter="16">
          <a-col :span="12"><a-form-item label="Values 文件"><a-input v-model:value="envForm.helm_values_path" placeholder="apps/ui-runner-smoke-0505/values/dev.yaml" /></a-form-item></a-col>
          <a-col :span="12"><a-form-item label="Release 名称"><a-input v-model:value="envForm.helm_release_name" placeholder="ui-runner-smoke-0505" /></a-form-item></a-col>
        </a-row>
        <a-divider orientation="left">运行配置</a-divider>
        <a-row :gutter="16">
          <a-col :span="6"><a-form-item label="CPU Request"><a-input v-model:value="envForm.cpu_request" placeholder="100m" /></a-form-item></a-col>
          <a-col :span="6"><a-form-item label="CPU Limit"><a-input v-model:value="envForm.cpu_limit" placeholder="500m" /></a-form-item></a-col>
          <a-col :span="6"><a-form-item label="Memory Request"><a-input v-model:value="envForm.memory_request" placeholder="128Mi" /></a-form-item></a-col>
          <a-col :span="6"><a-form-item label="Memory Limit"><a-input v-model:value="envForm.memory_limit" placeholder="512Mi" /></a-form-item></a-col>
        </a-row>
        <a-divider orientation="left">运行时定位</a-divider>
        <a-row :gutter="16">
          <a-col :span="8">
            <a-form-item label="K8s 集群">
              <a-select v-model:value="envForm.k8s_cluster_id" placeholder="选择集群" allow-clear show-search option-filter-prop="label">
                <a-select-option
                  v-for="cluster in k8sClusters"
                  :key="cluster.id"
                  :value="cluster.id"
                  :label="cluster.name"
                >
                  {{ cluster.name }}
                </a-select-option>
              </a-select>
            </a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="Namespace"><a-input v-model:value="envForm.k8s_namespace" placeholder="default" /></a-form-item>
          </a-col>
          <a-col :span="8">
            <a-form-item label="Deployment"><a-input v-model:value="envForm.k8s_deployment" placeholder="deployment 名称" /></a-form-item>
          </a-col>
        </a-row>
      </a-form>
    </a-modal>

    <!-- 执行关联流水线 -->
    <a-modal v-model:open="deployModalVisible" title="执行流水线" @ok="submitDeploy" :confirmLoading="deploying" width="640px">
      <a-alert v-if="!relatedPipelines.length" type="warning" show-icon style="margin-bottom: 16px" message="当前应用暂无关联流水线" />
      <a-form :model="deployForm" :labelCol="{ span: 6 }">
        <a-form-item label="流水线" required>
          <a-select v-model:value="deployForm.pipeline_id" placeholder="选择流水线" show-search option-filter-prop="label">
            <a-select-option
              v-for="pipeline in selectablePipelines"
              :key="pipeline.id"
              :value="pipeline.id"
              :label="`${pipeline.name} ${pipeline.env || ''}`"
            >
              {{ pipeline.name }}<span v-if="pipeline.env">（{{ getEnvName(pipeline.env) }}）</span>
            </a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="Git 仓库" v-if="selectedDeployPipeline?.git_repo_url">
          <a :href="selectedDeployPipeline.git_repo_url" target="_blank">{{ getRepoName(selectedDeployPipeline.git_repo_url) }}</a>
        </a-form-item>
        <a-form-item label="Git Ref" v-if="selectedDeployPipeline?.git_repo_id">
          <a-radio-group v-model:value="deployForm.ref_type" style="margin-bottom: 8px">
            <a-radio-button value="branch">分支 ({{ deployBranches.length }})</a-radio-button>
            <a-radio-button value="tag">Tag ({{ deployTags.length }})</a-radio-button>
          </a-radio-group>
          <a-spin :spinning="deployBranchesLoading">
            <a-auto-complete
              v-model:value="deployForm.branch"
              :options="deployRefOptions"
              :placeholder="deployForm.ref_type === 'branch' ? '选择或输入分支' : '选择或输入 Tag'"
              style="width: 100%"
              allow-clear
            />
          </a-spin>
          <div style="color: #999; font-size: 12px; margin-top: 4px">
            <template v-if="deployForm.ref_type === 'branch'">默认分支: {{ selectedDeployDefaultBranch }}</template>
            <template v-else>{{ deployTags.length > 0 ? '选择一个 Tag 版本' : '暂无 Tag' }}</template>
          </div>
        </a-form-item>
        <a-divider orientation="left">交付参数</a-divider>
        <a-form-item label="环境" required>
          <a-select v-model:value="deployForm.env_name">
            <a-select-option v-for="envName in deployEnvOptions" :key="envName" :value="envName">{{ getEnvName(envName) }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-descriptions v-if="selectedDeployEnv" :column="1" size="small" bordered style="margin-bottom: 12px">
          <a-descriptions-item label="交付目标">{{ formatEnvTarget(selectedDeployEnv) || '-' }}</a-descriptions-item>
          <a-descriptions-item label="环境默认分支">{{ selectedDeployEnv.branch || '-' }}</a-descriptions-item>
          <a-descriptions-item label="副本数">{{ selectedDeployEnv.replicas || 1 }}</a-descriptions-item>
        </a-descriptions>
        <!-- 审批状态提示 -->
        <a-alert v-if="needApproval" type="info" show-icon style="margin-bottom: 12px">
          <template #message>此环境配置了审批策略，流水线成功后生成的 GitOps 变更会进入审批链路</template>
        </a-alert>
        <!-- 发布窗口提示 -->
        <a-alert v-if="!deployWindowStatus?.in_window" type="warning" show-icon style="margin-bottom: 12px">
          <template #message>当前不在发布窗口内，下次可发布时间: {{ deployWindowStatus?.next_window || '-' }}</template>
        </a-alert>
        <a-form-item label="镜像标签"><a-input v-model:value="deployForm.image_tag" placeholder="默认使用 CI_COMMIT_SHORT_SHA，可填 v1.0.0" /></a-form-item>
        <a-form-item label="发布说明"><a-textarea v-model:value="deployForm.description" :rows="2" /></a-form-item>
      </a-form>
    </a-modal>

    <!-- 交付详情 -->
    <a-drawer v-model:open="deployDetailVisible" title="交付详情" :width="450">
      <a-descriptions v-if="currentDeploy" :column="1" bordered size="small">
        <a-descriptions-item label="环境"><a-tag :color="envColors[currentDeploy.env_name]">{{ currentDeploy.env_name }}</a-tag></a-descriptions-item>
        <a-descriptions-item label="状态"><a-badge :status="statusType[currentDeploy.status]" :text="statusText[currentDeploy.status]" /></a-descriptions-item>
        <a-descriptions-item label="版本">{{ currentDeploy.version || currentDeploy.image_tag || '-' }}</a-descriptions-item>
        <a-descriptions-item label="分支">{{ currentDeploy.branch || '-' }}</a-descriptions-item>
        <a-descriptions-item label="操作人">{{ currentDeploy.operator || '-' }}</a-descriptions-item>
        <a-descriptions-item label="开始时间">{{ fmtTime(currentDeploy.started_at) }}</a-descriptions-item>
        <a-descriptions-item label="结束时间">{{ fmtTime(currentDeploy.finished_at) }}</a-descriptions-item>
        <a-descriptions-item label="耗时">{{ currentDeploy.duration ? `${currentDeploy.duration}s` : '-' }}</a-descriptions-item>
        <a-descriptions-item v-if="currentDeploy.error_msg" label="错误"><span style="color:#ff4d4f">{{ currentDeploy.error_msg }}</span></a-descriptions-item>
      </a-descriptions>
    </a-drawer>

    <AppOnboardingWizard
      v-model:open="onboardingVisible"
      :application="app"
      @success="onOnboardingSuccess"
    />

    <a-drawer v-model:open="securityResultVisible" title="扫描结果详情" width="60%">
      <template v-if="currentSecurityResult">
        <a-descriptions :column="2" bordered>
          <a-descriptions-item label="镜像">{{ currentSecurityResult.image }}</a-descriptions-item>
          <a-descriptions-item label="风险等级">
            <a-tag :color="securityRiskColor(currentSecurityResult.risk_level)">{{ securityRiskText(currentSecurityResult.risk_level) }}</a-tag>
          </a-descriptions-item>
          <a-descriptions-item label="关联应用">{{ currentSecurityResult.application_name || app?.name || '-' }}</a-descriptions-item>
          <a-descriptions-item label="关联运行">
            <span v-if="currentSecurityResult.pipeline_run_id">#{{ currentSecurityResult.pipeline_run_id }}</span>
            <span v-else>-</span>
          </a-descriptions-item>
          <a-descriptions-item label="扫描时间">{{ fmtTime(currentSecurityResult.scanned_at) }}</a-descriptions-item>
          <a-descriptions-item label="漏洞总数">{{ currentSecurityResult.vuln_summary?.total || 0 }}</a-descriptions-item>
        </a-descriptions>

        <div class="vuln-summary">
          <a-tag color="red" :class="{ active: securitySeverityFilter === 'critical' }" @click="toggleSecuritySeverity('critical')">
            严重 {{ currentSecurityResult.vuln_summary?.critical || 0 }}
          </a-tag>
          <a-tag color="orange" :class="{ active: securitySeverityFilter === 'high' }" @click="toggleSecuritySeverity('high')">
            高危 {{ currentSecurityResult.vuln_summary?.high || 0 }}
          </a-tag>
          <a-tag color="blue" :class="{ active: securitySeverityFilter === 'medium' }" @click="toggleSecuritySeverity('medium')">
            中危 {{ currentSecurityResult.vuln_summary?.medium || 0 }}
          </a-tag>
          <a-tag color="green" :class="{ active: securitySeverityFilter === 'low' }" @click="toggleSecuritySeverity('low')">
            低危 {{ currentSecurityResult.vuln_summary?.low || 0 }}
          </a-tag>
          <a-tag v-if="securitySeverityFilter" @click="toggleSecuritySeverity('')">清除筛选</a-tag>
        </div>

        <a-table
          :data-source="filteredSecurityVulnerabilities"
          style="margin-top: 16px"
          :scroll="{ y: 400 }"
          row-key="vuln_id"
          :pagination="{ pageSize: 10 }"
        >
          <a-table-column title="CVE ID" dataIndex="vuln_id" :width="150" />
          <a-table-column title="包名" dataIndex="pkg_name" :width="150" :ellipsis="true" />
          <a-table-column title="严重程度" dataIndex="severity" :width="100">
            <template #default="{ record }">
              <a-tag :color="securityRiskColor(record.severity)">{{ securityRiskText(record.severity) }}</a-tag>
            </template>
          </a-table-column>
          <a-table-column title="当前版本" dataIndex="installed_ver" :width="120" />
          <a-table-column title="修复版本" dataIndex="fixed_ver" :width="120" />
          <a-table-column title="描述" dataIndex="title" :ellipsis="true" />
        </a-table>
      </template>
    </a-drawer>
  </div>
</template>

<script setup lang="ts">
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { ArrowLeftOutlined, RocketOutlined, EditOutlined, PlusOutlined, ClockCircleOutlined, LockOutlined } from '@ant-design/icons-vue'
import { applicationApi, type Application, type ApplicationEnv, type DeliveryRecord, type ApplicationRepoBinding, type ApplicationReadiness, type ApplicationReadinessCheck, type ApplicationOnboardingResponse } from '@/services/application'
import { pipelineApi, gitRepoApi } from '@/services/pipeline'
import { releaseApi, type Release } from '@/services/release'
import { observabilityTimelineApi, type TimelineItem } from '@/services/observabilityTimeline'
import { getScanHistory, getScanResult, type ImageScanHistoryItem, type ScanResultDetail } from '@/services/security'
import { costApi } from '@/services/cost'
import { k8sClusterApi } from '@/services/k8s'
import { argocdApi, type GitOpsRepo } from '@/services/argocd'
import { catalogApi, type Organization, type Project } from '@/services/catalog'
import type { K8sCluster } from '@/types'
import AppOnboardingWizard from './components/AppOnboardingWizard.vue'

const route = useRoute()
const router = useRouter()
const appId = Number(route.params.id)

const loading = ref(false)
const saving = ref(false)
const savingEnv = ref(false)
const deploying = ref(false)
const deploysLoading = ref(false)
const appTimelineLoading = ref(false)
const securityLoading = ref(false)
const costLoading = ref(false)
const deliveryLoading = ref(false)
const repoBindingLoading = ref(false)
const readinessLoading = ref(false)
const creatingReleaseRunId = ref<number | null>(null)
const activeTab = ref('info')

const app = ref<Application | null>(null)
const envs = ref<ApplicationEnv[]>([])
const deploys = ref<DeliveryRecord[]>([])
const currentDeploy = ref<DeliveryRecord | null>(null)
const appTimeline = ref<TimelineItem[]>([])
const relatedPipelines = ref<any[]>([])
const deliveryRuns = ref<any[]>([])
const recentReleases = ref<Release[]>([])
const releaseTotal = ref(0)
const repoBindings = ref<ApplicationRepoBinding[]>([])
const readiness = ref<ApplicationReadiness | null>(null)
const availableGitRepos = ref<Array<{ id: number; name: string; url: string; provider?: string; default_branch?: string }>>([])
const gitOpsRepos = ref<GitOpsRepo[]>([])
const k8sClusters = ref<K8sCluster[]>([])
const organizations = ref<Organization[]>([])
const projects = ref<Project[]>([])
const securityScans = ref<ImageScanHistoryItem[]>([])
const costAppAggregate = ref<AppCostAggregate | null>(null)
const costAllocationItem = ref<CostAllocationItem | null>(null)
const costSuggestions = ref<CostSuggestionItem[]>([])
const costEnvBaseline = ref<EnvCostItem[]>([])
const securityResultVisible = ref(false)
const currentSecurityResult = ref<ScanResultDetail | null>(null)
const securitySeverityFilter = ref('')
const securityAssociationFilter = ref<'all' | 'explicit' | 'legacy'>('all')
const securityRiskFilter = ref('')
let appTimelineLoaded = false
let securityLoaded = false
let costLoaded = false
let deliveryLoaded = false

// 发布窗口和锁状态
const deployWindowStatus = ref<{ in_window: boolean; message: string; next_window?: string } | null>(null)
const deployLockStatus = ref<{ locked: boolean; locked_by?: string; locked_at?: string } | null>(null)
const needApproval = ref(false)

const editModalVisible = ref(false)
const repoBindingModalVisible = ref(false)
const envModalVisible = ref(false)
const deployModalVisible = ref(false)
const onboardingVisible = ref(false)
const deployDetailVisible = ref(false)
const savingRepoBinding = ref(false)

const editForm = reactive<Partial<Application>>({})
const editableProjects = computed(() => editForm.organization_id
  ? projects.value.filter(project => project.organization_id === editForm.organization_id)
  : projects.value)
const repoBindingForm = reactive<{ git_repo_id?: number; role: string; is_default: boolean }>({
  git_repo_id: undefined,
  role: 'primary',
  is_default: true,
})
const envForm = reactive<Partial<ApplicationEnv>>({ replicas: 1 })
const deployForm = reactive<{ env_name: string; pipeline_id?: number; image_tag: string; branch: string; ref_type: 'branch' | 'tag'; description: string }>({
  env_name: '',
  pipeline_id: undefined,
  image_tag: '',
  branch: '',
	ref_type: 'branch',
  description: '',
})
const deployFilter = reactive({ env_name: '', status: '' })
const deployPagination = reactive({ current: 1, pageSize: 10, total: 0 })
const deployBranches = ref<string[]>([])
const deployTags = ref<string[]>([])
const deployRefOptions = ref<{ value: string }[]>([])
const deployBranchesLoading = ref(false)

const langColors: Record<string, string> = { go: 'cyan', java: 'orange', python: 'blue', nodejs: 'green' }
const envColors: Record<string, string> = { dev: 'blue', test: 'cyan', staging: 'orange', prod: 'red' }
const statusType: Record<string, string> = { pending: 'warning', approved: 'processing', running: 'processing', success: 'success', failed: 'error', rejected: 'error', cancelled: 'default' }
const statusText: Record<string, string> = { pending: '待审批', approved: '已通过', running: '运行中', success: '成功', failed: '失败', rejected: '已拒绝', cancelled: '已取消' }
const deliveryStatusType: Record<string, string> = { pending: 'default', running: 'processing', success: 'success', failed: 'error', cancelled: 'warning' }
const deliveryStatusText: Record<string, string> = { pending: '等待中', running: '运行中', success: 'CI 成功', failed: 'CI 失败', cancelled: '已取消' }
const gitOpsHandoffText: Record<string, string> = { created: '已创建变更', failed: '交接失败', skipped: '已跳过', pending: '处理中' }
const gitOpsHandoffColor: Record<string, string> = { created: 'green', failed: 'red', skipped: 'default', pending: 'blue' }
const releaseStatusMap: Record<string, { text: string; color: string }> = {
  draft: { text: '草稿', color: 'default' },
  pending_approval: { text: '待审批', color: 'gold' },
  approved: { text: '已审批', color: 'blue' },
  pr_opened: { text: 'PR 已创建', color: 'purple' },
  pr_merged: { text: 'PR 已合并', color: 'cyan' },
  publishing: { text: '发布中', color: 'processing' },
  published: { text: '已发布', color: 'green' },
  rolled_back: { text: '已回滚', color: 'orange' },
  rejected: { text: '已拒绝', color: 'red' },
}
const argoSyncColorMap: Record<string, string> = {
  Synced: 'green',
  OutOfSync: 'orange',
  Unknown: 'default',
  Progressing: 'blue',
  Missing: 'red',
}

interface CostAppItem {
  app_name: string
  namespace: string
  resource_count: number
  cpu_request: number
  cpu_usage: number
  cpu_usage_rate: number
  memory_request: number
  memory_usage: number
  memory_usage_rate: number
  cpu_cost: number
  memory_cost: number
  storage_cost: number
  total_cost: number
  percentage: number
  efficiency: number
}

interface AppCostAggregate {
  appName: string
  namespaces: string[]
  resourceCount: number
  cpuRequest: number
  cpuUsage: number
  cpuUsageRate: number
  memoryRequest: number
  memoryUsage: number
  memoryUsageRate: number
  cpuCost: number
  memoryCost: number
  storageCost: number
  totalCost: number
  efficiency: number
}

interface CostAllocationItem {
  name: string
  direct_cost: number
  shared_cost: number
  total_cost: number
  resource_count: number
  avg_efficiency: number
}

interface CostSuggestionItem {
  id: number
  namespace: string
  resource_type: string
  resource_name: string
  severity: string
  title: string
  description: string
  savings: number
  status: string
}

interface EnvCostItem {
  environment: string
  namespace_count: number
  app_count: number
  total_cost: number
  percentage: number
  avg_efficiency: number
}

const envColumns = [
  { title: '环境', dataIndex: 'env_name', key: 'env_name', width: 80 },
  { title: '分支', dataIndex: 'branch', key: 'branch', width: 100 },
  { title: 'GitOps Helm / 运行目标', key: 'delivery_target', width: 320 },
  { title: '关联流水线', key: 'pipeline_binding', width: 240 },
  { title: '最近交付', key: 'latest_delivery', width: 260 },
  { title: '副本', dataIndex: 'replicas', key: 'replicas', width: 70 },
  { title: '操作', key: 'action', width: 240 }
]
const repoBindingColumns = [
  { title: '仓库名称', key: 'repo_name' },
  { title: 'Provider', key: 'repo_provider', width: 120 },
  { title: '仓库地址', key: 'repo_url' },
  { title: '默认分支', key: 'default_branch', width: 120 },
  { title: '操作', key: 'action', width: 240 },
]
const deployColumns = [
  { title: '时间', key: 'created_at', width: 150 },
  { title: '环境', key: 'env_name', width: 80 },
  { title: '来源', key: 'deploy_method', width: 110 },
  { title: '版本', dataIndex: 'version', width: 100 },
  { title: '分支', dataIndex: 'branch', width: 100 },
  { title: '操作人', dataIndex: 'operator', width: 80 },
  { title: '状态', key: 'status', width: 80 },
  { title: '耗时', key: 'duration', width: 60 },
  { title: '操作', key: 'action', width: 60 }
]
const relatedPipelineColumns = [
  { title: '流水线', dataIndex: 'name', key: 'name' },
  { title: '环境', key: 'env', width: 100 },
  { title: 'Git 分支', dataIndex: 'git_branch', key: 'git_branch', width: 120 },
  { title: '最近状态', key: 'last_run_status', width: 120 },
  { title: '最近运行', dataIndex: 'last_run_at', key: 'last_run_at', width: 170 },
  { title: '操作', key: 'action', width: 100 },
]
const deliveryRunColumns = [
  { title: '运行 ID', dataIndex: 'id', key: 'id', width: 90 },
  { title: '流水线', dataIndex: 'pipeline_name', key: 'pipeline_name' },
  { title: '状态', key: 'status', width: 100 },
  { title: 'GitOps', key: 'gitops', width: 130 },
  { title: '触发人', dataIndex: 'trigger_by', key: 'trigger_by', width: 120 },
  { title: '时间', key: 'created_at', width: 170 },
  { title: '操作', key: 'action', width: 180 },
]
const releaseColumns = [
  { title: '发布单', key: 'title' },
  { title: '环境', key: 'env', width: 90 },
  { title: '状态', key: 'status', width: 120 },
  { title: '审批', key: 'approval', width: 130 },
  { title: 'GitOps', key: 'gitops', width: 120 },
  { title: 'ArgoCD', key: 'argocd', width: 190 },
  { title: '创建时间', key: 'created_at', width: 170 },
  { title: '操作', key: 'action', width: 150 },
]

const fmtTime = (t?: string) => t ? t.replace('T', ' ').substring(0, 19) : '-'
const formatNumber = (value?: number | null, digits = 2) => Number(value || 0).toFixed(digits)
const formatCurrency = (value?: number | null, digits = 2) => Number(value || 0).toFixed(digits)
const clampPercent = (value?: number | null) => Math.max(0, Math.min(Number(value || 0), 100))
const goBack = () => router.push('/applications')
const goEventTimeline = () => {
  router.push({ path: '/observability/event-timeline', query: { application_id: String(appId) } })
}
const openTimelineItem = (item: TimelineItem) => {
  if (item.ref) router.push(item.ref)
}

const timelineKindText = (kind: string) => {
  if (kind === 'incident') return '事故'
  if (kind === 'alert') return '告警'
  if (kind === 'approval') return '审批'
  if (kind === 'release') return '发布'
  if (kind === 'change_event') return '变更'
  return kind
}

const timelineKindColor = (kind: string) => {
  if (kind === 'incident') return 'red'
  if (kind === 'alert') return 'orange'
  if (kind === 'approval') return 'purple'
  if (kind === 'release') return 'green'
  return 'blue'
}

const timelineSeverityColor = (severity?: string) => {
  if (severity === 'P0' || severity === 'P1' || severity === 'critical') return 'red'
  if (severity === 'P2' || severity === 'warning' || severity === 'error') return 'orange'
  return 'default'
}

const timelineStatusColor = (item: TimelineItem) => {
  const status = (item.status || '').toLowerCase()
  if (item.kind === 'approval') {
    if (status === 'approved' || status === 'merged') return 'green'
    if (status === 'rejected' || status === 'failed' || status === 'cancelled') return 'red'
    if (status === 'pending') return 'gold'
  }
  if (item.kind === 'alert') {
    if (status === 'resolved' || status === 'acked') return 'green'
    if (status === 'pending' || status === 'firing') return 'gold'
    if (status === 'failed') return 'red'
  }
  if (status === 'success' || status === 'published') return 'green'
  if (status === 'failed' || status === 'rejected') return 'red'
  return 'blue'
}

const timelineStatusText = (item: TimelineItem) => {
  const status = item.status || ''
  const map: Record<string, string> = {
    pending: '待处理',
    approved: '已通过',
    rejected: '已拒绝',
    failed: '失败',
    merged: '已合并',
    published: '已发布',
    success: '成功',
    resolved: '已恢复',
    acked: '已确认',
    firing: '告警中',
  }
  return map[status] || status
}

const securityStatusColor = (status?: string) => ({ completed: 'green', scanning: 'orange', failed: 'red' }[status || ''] || 'default')
const securityStatusText = (status?: string) => ({ completed: '已完成', scanning: '扫描中', failed: '失败' }[status || ''] || (status || '-'))
const securityRiskColor = (level?: string) => ({ critical: 'red', high: 'orange', medium: 'blue', low: 'green', none: 'green' }[level || ''] || 'default')
const securityRiskText = (level?: string) => ({ critical: '严重', high: '高危', medium: '中危', low: '低危', none: '安全' }[level || ''] || (level || '-'))
const securityAssociationText = (source?: string) => ({
  application_id: '显式应用 ID',
  application_name: '显式应用名',
  pipeline_run: '流水线运行',
  legacy_image_keyword: '镜像关键字兜底',
}[source || ''] || (source || '-'))
const securityAssociationColor = (source?: string) => ({
  application_id: 'green',
  application_name: 'blue',
  pipeline_run: 'purple',
  legacy_image_keyword: 'gold',
}[source || ''] || 'default')
const getUsageStatus = (rate: number) => rate < 30 ? 'exception' : rate < 60 ? 'active' : 'success'
const getEfficiencyColor = (efficiency: number) => efficiency < 30 ? 'red' : efficiency < 60 ? 'orange' : 'green'
const getEnvColor = (env: string) => ({ dev: 'blue', test: 'cyan', staging: 'orange', prod: 'red', other: 'default' }[env] || 'default')
const getEnvName = (env: string) => ({ dev: '开发', test: '测试', staging: '预发', prod: '生产', other: '其他' }[env] || env)
const releaseStatusText = (status?: string) => releaseStatusMap[status || '']?.text || status || '-'
const releaseStatusColor = (status?: string) => releaseStatusMap[status || '']?.color || 'default'
const argoSyncText = (status?: string) => status || '-'
const argoSyncColor = (status?: string) => argoSyncColorMap[status || ''] || 'default'
const releaseApprovalText = (record: Release) => {
  if (record.status === 'pending_approval') return '待审批'
  if (record.approved_by_name) return `已通过：${record.approved_by_name}`
  if (record.status === 'rejected') return '已拒绝'
  return '-'
}
const getDeployMethodText = (method: string) => ({ gitops: 'CI/GitOps', k8s: '直接部署' }[method] || method || '-')
const getGitOpsHandoffText = (status?: string) => gitOpsHandoffText[status || ''] || status || '未交接'
const getGitOpsHandoffColor = (status?: string) => gitOpsHandoffColor[status || ''] || 'default'
const costSuggestionSeverityColor = (severity: string) => ({ high: 'red', medium: 'orange', low: 'blue' }[severity] || 'default')
const costSuggestionSeverityText = (severity: string) => ({ high: '高优先级', medium: '中优先级', low: '低优先级' }[severity] || severity)
const costSuggestionStatusText = (status: string) => ({ pending: '待处理', applied: '已应用', ignored: '已忽略' }[status] || status)

const normalizeValue = (value?: string) => (value || '').trim().toLowerCase()
const uniqueTextList = (values: Array<string | undefined>) => Array.from(new Set(values.map((item) => item?.trim()).filter(Boolean) as string[]))
const formatDate = (date: Date) => {
  const year = date.getFullYear()
  const month = String(date.getMonth() + 1).padStart(2, '0')
  const day = String(date.getDate()).padStart(2, '0')
  return `${year}-${month}-${day}`
}
const getDateRangeDays = (days: number) => {
  const end = new Date()
  const start = new Date(end)
  start.setDate(end.getDate() - days)
  return { start: formatDate(start), end: formatDate(end) }
}
const getCurrentMonthRange = () => {
  const end = new Date()
  const start = new Date(end.getFullYear(), end.getMonth(), 1)
  return { start: formatDate(start), end: formatDate(end) }
}

const appCostMatchTokens = computed(() =>
  uniqueTextList([
    app.value?.name,
    app.value?.display_name,
    ...envs.value.map((env) => env.k8s_deployment),
  ]).map((item) => normalizeValue(item)),
)

const appCostNamespaces = computed(() =>
  uniqueTextList(envs.value.map((env) => env.k8s_namespace)),
)

const appCostEnvNames = computed(() =>
  uniqueTextList(envs.value.map((env) => env.env_name)),
)

const appCostClusterIds = computed(() =>
  Array.from(new Set(envs.value.map((env) => env.k8s_cluster_id).filter((id): id is number => typeof id === 'number' && id > 0))),
)

const appCostClusterNames = computed(() => {
  const clusterNameMap = new Map(k8sClusters.value.map((cluster) => [cluster.id, cluster.name]))
  return appCostClusterIds.value.map((id) => clusterNameMap.get(id) || `#${id}`)
})

const primaryCostClusterId = computed(() => appCostClusterIds.value[0])

const isMatchedByAppToken = (value?: string) => {
  const normalized = normalizeValue(value)
  if (!normalized) return false
  return appCostMatchTokens.value.some((token) => {
    if (!token) return false
    if (normalized === token) return true
    if (token.length < 4) return false
    return normalized.includes(token) || token.includes(normalized)
  })
}

const aggregateAppCostItems = (items: CostAppItem[]): AppCostAggregate | null => {
  if (!items.length) return null
  const namespaces = uniqueTextList(items.map((item) => item.namespace))
  const cpuRequest = items.reduce((sum, item) => sum + Number(item.cpu_request || 0), 0)
  const cpuUsage = items.reduce((sum, item) => sum + Number(item.cpu_usage || 0), 0)
  const memoryRequest = items.reduce((sum, item) => sum + Number(item.memory_request || 0), 0)
  const memoryUsage = items.reduce((sum, item) => sum + Number(item.memory_usage || 0), 0)
  const cpuUsageRate = cpuRequest > 0 ? cpuUsage / cpuRequest * 100 : 0
  const memoryUsageRate = memoryRequest > 0 ? memoryUsage / memoryRequest * 100 : 0
  return {
    appName: items[0]?.app_name || app.value?.name || '-',
    namespaces,
    resourceCount: items.reduce((sum, item) => sum + Number(item.resource_count || 0), 0),
    cpuRequest,
    cpuUsage,
    cpuUsageRate,
    memoryRequest,
    memoryUsage,
    memoryUsageRate,
    cpuCost: items.reduce((sum, item) => sum + Number(item.cpu_cost || 0), 0),
    memoryCost: items.reduce((sum, item) => sum + Number(item.memory_cost || 0), 0),
    storageCost: items.reduce((sum, item) => sum + Number(item.storage_cost || 0), 0),
    totalCost: items.reduce((sum, item) => sum + Number(item.total_cost || 0), 0),
    efficiency: (cpuUsageRate + memoryUsageRate) / 2,
  }
}

const fetchCostInsights = async () => {
  if (costLoaded || !app.value) return
  costLoading.value = true
  try {
    const last30 = getDateRangeDays(30)
    const currentMonth = getCurrentMonthRange()
    const clusterId = primaryCostClusterId.value
    const [appCostRes, suggestionRes, envCostRes, allocationRes] = await Promise.all([
      (costApi as any).getAppCost({
        cluster_id: clusterId,
        start_time: last30.start,
        end_time: last30.end,
        top_n: 500,
      }),
      costApi.getSuggestions(clusterId, 'pending'),
      (costApi as any).getEnvCost({
        cluster_id: clusterId,
        start_time: last30.start,
        end_time: last30.end,
      }),
      (costApi as any).getCostAllocation({
        cluster_id: clusterId,
        start_time: currentMonth.start,
        end_time: currentMonth.end,
        group_by: 'app',
        include_shared: true,
      }),
    ])

    const appCostItems: CostAppItem[] = appCostRes?.code === 0 ? (appCostRes.data?.items || []) : []
    const matchedAppItems = appCostItems.filter((item) => isMatchedByAppToken(item.app_name))
    costAppAggregate.value = aggregateAppCostItems(matchedAppItems)

    const allocationItems: CostAllocationItem[] = allocationRes?.code === 0 ? (allocationRes.data?.items || []) : []
    costAllocationItem.value = allocationItems.find((item) => isMatchedByAppToken(item.name)) || null

    const envCostItems: EnvCostItem[] = envCostRes?.code === 0 ? (envCostRes.data?.items || []) : []
    costEnvBaseline.value = envCostItems
      .filter((item) => appCostEnvNames.value.includes(item.environment))
      .sort((a, b) => Number(b.total_cost || 0) - Number(a.total_cost || 0))

    const namespaceSet = new Set(appCostNamespaces.value.map((item) => normalizeValue(item)))
    const suggestionPayload = suggestionRes as any
    const suggestions: CostSuggestionItem[] = suggestionPayload?.code === 0 ? (suggestionPayload.data?.items || []) : []
    costSuggestions.value = suggestions
      .filter((item) => {
        const namespaceMatched = namespaceSet.has(normalizeValue(item.namespace))
        const textMatched = [item.resource_name, item.title, item.description].some((value) => isMatchedByAppToken(value))
        return namespaceMatched || textMatched
      })
      .sort((a, b) => Number(b.savings || 0) - Number(a.savings || 0))
      .slice(0, 5)

    costLoaded = true
  } catch (e) {
    console.error('[ApplicationDetail] 加载成本聚合失败', e)
  } finally {
    costLoading.value = false
  }
}

const fetchApp = async () => {
  loading.value = true
  try {
    const res = await applicationApi.get(appId)
    if (res.code === 0 && res.data) {
      app.value = res.data.app || res.data
      envs.value = res.data.envs || []
      if (Array.isArray(res.data.repo_bindings)) {
        repoBindings.value = res.data.repo_bindings
      }
    }
  } catch (e) { console.error(e) }
  finally { loading.value = false }
}

const fetchK8sClusters = async () => {
  try {
    const response = await k8sClusterApi.list()
    if (response.code === 0 && response.data) {
      k8sClusters.value = response.data.items || []
    }
  } catch (error) {
    console.error('获取 K8s 集群失败', error)
  }
}

const fetchCatalog = async () => {
  try {
    const [orgRes, projectRes] = await Promise.all([
      catalogApi.listOrgs(),
      catalogApi.listProjects(),
    ])
    organizations.value = orgRes.data || []
    projects.value = projectRes.data || []
  } catch (error) {
    console.error('获取组织项目目录失败', error)
  }
}

const fetchGitOpsRepos = async () => {
  try {
    const res = await argocdApi.listRepos({ page: 1, page_size: 200 }) as any
    gitOpsRepos.value = res?.data?.items || res?.data?.list || []
  } catch (error) {
    console.error('[ApplicationDetail] 加载 GitOps 仓库失败', error)
  }
}

const fetchDeploys = async () => {
  deploysLoading.value = true
  try {
    const res = await applicationApi.listDeliveryRecords(appId, { page: deployPagination.current, page_size: deployPagination.pageSize, ...deployFilter })
    if (res.code === 0 && res.data) { deploys.value = res.data.list || []; deployPagination.total = res.data.total }
  } catch (e) { console.error(e) }
  finally { deploysLoading.value = false }
}

const loadAvailableGitRepos = async () => {
  try {
    const res = await gitRepoApi.list({ page_size: 200 })
    availableGitRepos.value = res?.data?.items || []
  } catch (e) {
    console.error('[ApplicationDetail] 加载 Git 仓库失败', e)
  }
}

const loadRepoBindings = async () => {
  repoBindingLoading.value = true
  try {
    const res = await applicationApi.listRepoBindings(appId)
    repoBindings.value = res?.data || []
  } catch (e) {
    console.error('[ApplicationDetail] 加载仓库绑定失败', e)
  } finally {
    repoBindingLoading.value = false
  }
}

const fetchReadiness = async (refresh = false) => {
  readinessLoading.value = true
  try {
    const res = refresh ? await applicationApi.refreshReadiness(appId) : await applicationApi.getReadiness(appId)
    if (res.code === 0 && res.data) {
      readiness.value = res.data
    }
  } catch (e) {
    console.error('[ApplicationDetail] 加载接入完整度失败', e)
  } finally {
    readinessLoading.value = false
  }
}

const fetchDeliveryContext = async (force = false) => {
  if (!force && deliveryLoaded) return
  if (!app.value?.id) return
  deliveryLoading.value = true
  try {
    const [pipelineRes, runRes, releaseRes] = await Promise.all([
      pipelineApi.list({ application_id: appId, page: 1, page_size: 50 }),
      pipelineApi.listRuns({ application_id: appId, page: 1, page_size: 10 }),
      releaseApi.list({ application_id: appId, page: 1, page_size: 5 }),
    ])
    relatedPipelines.value = pipelineRes?.data?.items || pipelineRes?.data?.list || []
    deliveryRuns.value = (runRes?.data?.items || [])
      .sort((a: any, b: any) => new Date(b.created_at || 0).getTime() - new Date(a.created_at || 0).getTime())
      .slice(0, 10)
    recentReleases.value = releaseRes?.data?.list || releaseRes?.data?.items || []
    releaseTotal.value = releaseRes?.data?.total || recentReleases.value.length
    deliveryLoaded = true
  } catch (e) {
    console.error('[ApplicationDetail] 加载交付流水线失败', e)
  } finally {
    deliveryLoading.value = false
  }
}

const fetchAppTimeline = async () => {
  if (appTimelineLoaded) return
  appTimelineLoading.value = true
  try {
    const res = await observabilityTimelineApi.get({ application_id: appId, limit: 100 })
    if (res.code === 0 && res.data) {
      appTimeline.value = res.data.items || []
      appTimelineLoaded = true
    }
  } catch (e) {
    console.error('[ApplicationDetail] 加载统一时间线失败', e)
  } finally {
    appTimelineLoading.value = false
  }
}

const fetchSecurityScans = async (force = false) => {
  if (!force && securityLoaded) return
  securityLoading.value = true
  try {
    const res = await getScanHistory({
      application_id: appId,
      application_name: app.value?.name,
      page: 1,
      page_size: 10,
    })
    securityScans.value = res?.data?.items || []
    securityLoaded = true
  } catch (e) {
    console.error('[ApplicationDetail] 加载安全扫描失败', e)
  } finally {
    securityLoading.value = false
  }
}

const refreshSecurityScans = async () => {
  securityLoaded = false
  await fetchSecurityScans(true)
}

const viewSecurityScanResult = async (id: number) => {
  try {
    const res = await getScanResult(id)
    currentSecurityResult.value = (res?.data || null) as ScanResultDetail | null
    securitySeverityFilter.value = ''
    securityResultVisible.value = true
  } catch (e) {
    message.error('获取扫描结果失败')
  }
}

const toggleSecuritySeverity = (severity: string) => {
  securitySeverityFilter.value = securitySeverityFilter.value === severity ? '' : severity
}

const setSecurityAssociationFilter = (value: 'all' | 'explicit' | 'legacy') => {
  securityAssociationFilter.value = securityAssociationFilter.value === value ? 'all' : value
}

const setSecurityRiskFilter = (value: string) => {
  securityRiskFilter.value = securityRiskFilter.value === value ? '' : value
}

const resetSecurityFilters = () => {
  securityAssociationFilter.value = 'all'
  securityRiskFilter.value = ''
}

const filteredSecurityVulnerabilities = computed(() => {
  const vulnerabilities = currentSecurityResult.value?.vulnerabilities || []
  if (!securitySeverityFilter.value) return vulnerabilities
  return vulnerabilities.filter((item) => item.severity === securitySeverityFilter.value)
})

const filteredSecurityScans = computed(() => {
  return securityScans.value.filter((item) => {
    const associationMatched =
      securityAssociationFilter.value === 'all' ||
      (securityAssociationFilter.value === 'explicit' &&
        (item.association_source === 'application_id' || item.association_source === 'application_name')) ||
      (securityAssociationFilter.value === 'legacy' && item.association_source === 'legacy_image_keyword')
    const riskMatched = !securityRiskFilter.value || item.risk_level === securityRiskFilter.value
    return associationMatched && riskMatched
  })
})

const observabilityItems = computed(() =>
  appTimeline.value.filter((item) => item.kind === 'incident' || item.kind === 'alert'),
)

const changeItems = computed(() =>
  appTimeline.value.filter((item) => item.kind === 'change_event' || item.kind === 'release' || item.kind === 'approval'),
)

const observabilitySummary = computed(() => {
  const items = observabilityItems.value
  return {
    incidentCount: items.filter((item) => item.kind === 'incident').length,
    alertCount: items.filter((item) => item.kind === 'alert').length,
    pendingAlertCount: items.filter((item) => item.kind === 'alert' && ['pending', 'firing'].includes((item.status || '').toLowerCase())).length,
    latestAtText: items[0]?.at ? fmtTime(items[0].at) : '-',
  }
})

const changeSummary = computed(() => {
  const items = changeItems.value
  return {
    changeEventCount: items.filter((item) => item.kind === 'change_event').length,
    releaseCount: items.filter((item) => item.kind === 'release').length,
    approvalCount: items.filter((item) => item.kind === 'approval').length,
    rejectedApprovalCount: items.filter((item) => item.kind === 'approval' && (item.status || '').toLowerCase() === 'rejected').length,
  }
})

const deliveryEnvNames = computed(() =>
  uniqueTextList([
    ...relatedPipelines.value.map((pipeline) => pipeline.env),
    ...recentReleases.value.map((release) => release.env),
    ...envs.value.map((env) => env.env_name),
  ]),
)

const deployEnvOptions = computed(() => deliveryEnvNames.value.length > 0 ? deliveryEnvNames.value : ['dev'])

const selectedDeployEnv = computed(() =>
  envs.value.find((env) => env.env_name === deployForm.env_name) || null,
)

const selectablePipelines = computed(() => {
  if (!deployForm.env_name) return relatedPipelines.value
  const matched = relatedPipelines.value.filter((pipeline) => !pipeline.env || pipeline.env === deployForm.env_name)
  return matched.length > 0 ? matched : relatedPipelines.value
})

const selectedDeployPipeline = computed(() =>
	selectablePipelines.value.find((pipeline) => pipeline.id === deployForm.pipeline_id) ||
	relatedPipelines.value.find((pipeline) => pipeline.id === deployForm.pipeline_id) ||
	null,
)

const selectedDeployDefaultBranch = computed(() =>
	selectedDeployEnv.value?.branch || selectedDeployPipeline.value?.git_branch || 'main',
)

const formatEnvTarget = (env?: Pick<ApplicationEnv, 'k8s_cluster_id' | 'k8s_namespace' | 'k8s_deployment'> | null) => {
  if (!env) return ''
  const clusterName = env.k8s_cluster_id ? k8sClusters.value.find((cluster) => cluster.id === env.k8s_cluster_id)?.name || `#${env.k8s_cluster_id}` : ''
  return uniqueTextList([clusterName, env.k8s_namespace, env.k8s_deployment]).join(' / ')
}

const getGitOpsRepoName = (repoId?: number) => {
  if (!repoId) return ''
  return gitOpsRepos.value.find((repo) => repo.id === repoId)?.name || `GitOps #${repoId}`
}

const formatGitOpsTarget = (env?: ApplicationEnv | null) => {
  if (!env) return ''
  const repoName = getGitOpsRepoName(env.gitops_repo_id)
  const valuesPath = env.helm_values_path || env.gitops_path
  const releaseName = env.helm_release_name
  return uniqueTextList([repoName, valuesPath, releaseName]).join(' / ')
}

const getEnvPipelines = (envName: string) =>
  relatedPipelines.value.filter((pipeline) => !pipeline.env || pipeline.env === envName)

const getLatestDelivery = (envName: string) =>
  deploys.value.find((record) => record.env_name === envName)

type EnvDeliveryRow = ApplicationEnv & {
  delivery_target: string
  gitops_target: string
  pipelines: any[]
  latest_delivery?: DeliveryRecord
}

const envDeliveryRows = computed<EnvDeliveryRow[]>(() =>
  envs.value.map((env) => ({
    ...env,
    delivery_target: formatEnvTarget(env),
    gitops_target: formatGitOpsTarget(env),
    pipelines: getEnvPipelines(env.env_name),
    latest_delivery: getLatestDelivery(env.env_name),
  })),
)

const deliverySummary = computed(() => ({
  pipelineCount: relatedPipelines.value.length,
  successRunCount: deliveryRuns.value.filter((run) => run.status === 'success').length,
  releaseCount: releaseTotal.value || recentReleases.value.length,
  gitopsChangeCount: new Set([
    ...deliveryRuns.value.map((run) => run.gitops_change_request_id).filter(Boolean),
    ...recentReleases.value.map((release) => release.gitops_change_request_id).filter(Boolean),
  ]).size,
}))

const hasDeliveryContext = computed(() => relatedPipelines.value.length > 0 || deliveryRuns.value.length > 0 || recentReleases.value.length > 0)

const securityLatestScan = computed(() => securityScans.value[0] || null)

const securitySummary = computed(() => {
  const scans = securityScans.value
  return {
    scanCount: scans.length,
    criticalCount: scans.reduce((sum, item) => sum + (item.critical_count || 0), 0),
    highCount: scans.reduce((sum, item) => sum + (item.high_count || 0), 0),
    latestRiskLabel: securityLatestScan.value ? securityRiskText(securityLatestScan.value.risk_level) : '-',
  }
})

const securityAssociationSummary = computed(() => {
  const explicitCount = securityScans.value.filter((item) =>
    item.association_source === 'application_id' || item.association_source === 'application_name',
  ).length
  const legacyCount = securityScans.value.filter((item) => item.association_source === 'legacy_image_keyword').length
  return {
    explicitCount,
    legacyCount,
  }
})

const securityRiskSummary = computed(() => ({
  critical: securityScans.value.filter((item) => item.risk_level === 'critical').length,
  high: securityScans.value.filter((item) => item.risk_level === 'high').length,
  medium: securityScans.value.filter((item) => item.risk_level === 'medium').length,
  low: securityScans.value.filter((item) => item.risk_level === 'low').length,
}))

const costSummary = computed(() => {
  const aggregate = costAppAggregate.value
  const allocation = costAllocationItem.value
  return {
    totalCost: aggregate?.totalCost || allocation?.direct_cost || 0,
    monthlyAllocatedCost: allocation?.total_cost || 0,
    efficiency: aggregate?.efficiency || allocation?.avg_efficiency || 0,
    pendingSavings: costSuggestions.value.reduce((sum, item) => sum + Number(item.savings || 0), 0),
  }
})

const readinessColor = computed(() => {
  const score = readiness.value?.score || 0
  if (score >= 80) return '#52c41a'
  if (score >= 50) return '#faad14'
  return '#ff4d4f'
})

const onDeployPageChange = (p: any) => { deployPagination.current = p.current; fetchDeploys() }

const showRepoBindingModal = async () => {
  repoBindingForm.git_repo_id = undefined
  repoBindingForm.role = 'primary'
  repoBindingForm.is_default = repoBindings.value.length === 0
  if (!availableGitRepos.value.length) {
    await loadAvailableGitRepos()
  }
  repoBindingModalVisible.value = true
}

const saveRepoBinding = async () => {
  if (!repoBindingForm.git_repo_id) {
    message.error('请选择 Git 仓库')
    return
  }
  savingRepoBinding.value = true
  try {
    const res = await applicationApi.bindRepo(appId, {
      git_repo_id: repoBindingForm.git_repo_id,
      role: repoBindingForm.role,
      is_default: repoBindingForm.is_default,
    })
    if (res.code === 0) {
      message.success('仓库绑定成功')
      repoBindingModalVisible.value = false
      await loadRepoBindings()
      await fetchReadiness(true)
    }
  } catch (e: any) {
    message.error(e.message || '仓库绑定失败')
  } finally {
    savingRepoBinding.value = false
  }
}

const setDefaultRepoBinding = async (record: ApplicationRepoBinding) => {
  try {
    const res = await applicationApi.setDefaultRepoBinding(appId, record.id)
    if (res.code === 0) {
      message.success('已设为主仓库')
      await loadRepoBindings()
    }
  } catch (e: any) {
    message.error(e.message || '设置主仓库失败')
  }
}

const deleteRepoBinding = async (bindingId: number) => {
  try {
    const res = await applicationApi.deleteRepoBinding(appId, bindingId)
    if (res.code === 0) {
      message.success('已解除仓库绑定')
      await loadRepoBindings()
      await fetchReadiness(true)
    }
  } catch (e: any) {
    message.error(e.message || '解除绑定失败')
  }
}

const getRepoName = (url: string) => {
  if (!url) return ''
  const match = url.match(/[:/]([^/:]+\/[^/.]+)(\.git)?$/)
  return match ? match[1] : url
}

const normalizeGitRefs = (items: any[]) =>
  items.map((item) => typeof item === 'string' ? item : item.name).filter(Boolean)

const syncDeployRefOptions = () => {
  deployRefOptions.value = deployForm.ref_type === 'branch'
    ? deployBranches.value.map((branch) => ({ value: branch }))
    : deployTags.value.map((tag) => ({ value: tag }))
}

const loadDeployBranchesAndTags = async () => {
  const pipeline = selectedDeployPipeline.value
  deployBranches.value = []
  deployTags.value = []
  deployRefOptions.value = []
  if (!pipeline?.git_repo_id) return

  deployBranchesLoading.value = true
  try {
    const [branchRes, tagRes] = await Promise.allSettled([
      gitRepoApi.getBranches(pipeline.git_repo_id),
      gitRepoApi.getTags(pipeline.git_repo_id),
    ])
    if (branchRes.status === 'fulfilled') {
      deployBranches.value = normalizeGitRefs(branchRes.value?.data || [])
    }
    if (deployBranches.value.length === 0) {
      deployBranches.value = [selectedDeployDefaultBranch.value]
    }
    if (tagRes.status === 'fulfilled') {
      deployTags.value = normalizeGitRefs(tagRes.value?.data || [])
    }
    syncDeployRefOptions()
    if (deployForm.ref_type === 'branch' && !deployForm.branch) {
      deployForm.branch = selectedDeployDefaultBranch.value
    }
    if (deployForm.ref_type === 'tag' && !deployForm.branch) {
      deployForm.branch = deployTags.value[0] || ''
    }
  } finally {
    deployBranchesLoading.value = false
  }
}

const readinessCheckColor = (check: ApplicationReadinessCheck) => {
  if (check.status === 'pass') return 'green'
  if (check.severity === 'high') return 'red'
  if (check.severity === 'medium') return 'orange'
  return 'default'
}

const goReadinessNextAction = () => {
  const action = readiness.value?.next_actions?.[0]
  if (action?.path) {
    router.push(action.path)
  }
}

const showOnboardingWizard = () => {
  onboardingVisible.value = true
}

const onOnboardingSuccess = async (result: ApplicationOnboardingResponse) => {
  if (result.readiness) {
    readiness.value = result.readiness
  }
  deliveryLoaded = false
  await Promise.allSettled([fetchApp(), loadRepoBindings(), fetchDeliveryContext(true), fetchReadiness(true)])
}

const goCreatePipeline = (binding?: ApplicationRepoBinding) => {
  const selectedBinding = binding || repoBindings.value.find((item) => item.is_default) || repoBindings.value[0]
  if (!selectedBinding) {
    message.warning('请先绑定主仓库')
    activeTab.value = 'repos'
    return
  }
  const envName = envs.value[0]?.env_name || 'dev'
  router.push({
    path: '/pipeline/create',
    query: {
      application_id: String(appId),
      env: envName,
      git_repo_id: String(selectedBinding.git_repo_id),
      git_branch: selectedBinding.default_branch || 'main',
      from_app: '1',
    },
  })
}

const createReleaseFromRun = async (run: any) => {
  if (!run?.id) return
  creatingReleaseRunId.value = run.id
  try {
    const res = await releaseApi.createFromPipelineRun({
      pipeline_run_id: run.id,
      env: run.env || envs.value[0]?.env_name || 'dev',
      version: run.git_commit ? String(run.git_commit).slice(0, 12) : `run-${run.id}`,
      description: `由流水线 ${run.pipeline_name || ''} 运行 #${run.id} 生成`,
    })
    const release = (res as any)?.data
    if (release?.id) {
      message.success('发布单已生成')
      router.push(`/releases/${release.id}`)
      return
    }
    message.success('发布单已生成')
  } catch (e: any) {
    message.error(e.message || '生成发布单失败')
  } finally {
    creatingReleaseRunId.value = null
  }
}

const DETAIL_HASH_TO_TAB: Record<string, string> = {
  repos: 'repos',
  delivery: 'delivery',
  observability: 'observability',
  cost: 'cost',
  security: 'security',
  'change-events': 'change-events',
}

const TAB_TO_DETAIL_HASH: Record<string, string> = Object.fromEntries(
  Object.entries(DETAIL_HASH_TO_TAB).map(([hash, tab]) => [tab, `#${hash}`])
) as Record<string, string>

const applyDeepLinkFromRoute = async () => {
  const h = (route.hash || '').replace(/^#/, '')
  const detailTab = DETAIL_HASH_TO_TAB[h]
  if (!detailTab) return

  if (activeTab.value !== detailTab) {
    activeTab.value = detailTab
  }
}

// 切换 Tab 时按需加载，并同步 URL hash 便于从应用中心直达交付链路。
watch(activeTab, (key) => {
  const nextHash = TAB_TO_DETAIL_HASH[key] || ''
  const currentHashKey = (route.hash || '').replace(/^#/, '')
  if (nextHash && route.hash !== nextHash) {
    void router.replace({ path: route.path, hash: nextHash, query: route.query })
  } else if (!nextHash && DETAIL_HASH_TO_TAB[currentHashKey]) {
    void router.replace({ path: route.path, hash: '', query: route.query })
  }

  if (key === 'cost') {
    void fetchCostInsights()
  } else if (key === 'repos') {
    void loadRepoBindings()
    void loadAvailableGitRepos()
    void fetchGitOpsRepos()
  } else if (key === 'security') {
    void fetchSecurityScans()
  } else if (key === 'envs' || key === 'delivery') {
    void fetchGitOpsRepos()
    void fetchDeliveryContext()
  } else if (key === 'observability' || key === 'change-events') {
    void fetchAppTimeline()
  }
})

watch(
  () => route.hash,
  () => {
    void applyDeepLinkFromRoute()
  },
)

watch(
  () => deployForm.env_name,
  (envName) => {
    const selected = selectablePipelines.value.find((pipeline) => pipeline.id === deployForm.pipeline_id)
    const envConfig = envs.value.find((env) => env.env_name === envName)
    if (!selected || (envName && selected.env && selected.env !== envName)) {
      const next = selectablePipelines.value[0]
      deployForm.pipeline_id = next?.id
			if (deployForm.ref_type === 'branch') {
				deployForm.branch = envConfig?.branch || next?.git_branch || deployForm.branch || 'main'
			}
    } else if (envConfig?.branch) {
			if (deployForm.ref_type === 'branch') {
				deployForm.branch = envConfig.branch
			}
    }
    if (envName) void checkDeployStatus()
  },
)

watch(
	() => deployForm.pipeline_id,
	() => {
		void loadDeployBranchesAndTags()
	},
)

watch(
	() => deployForm.ref_type,
	(type) => {
		syncDeployRefOptions()
		if (type === 'branch') {
			deployForm.branch = selectedDeployDefaultBranch.value
		} else {
			deployForm.branch = deployTags.value[0] || ''
		}
	},
)

const showEditModal = () => { if (app.value) Object.assign(editForm, app.value); editModalVisible.value = true }
const onEditOrganizationChange = () => {
  const currentProjectId = editForm.project_id
  if (!currentProjectId) return
  const matched = projects.value.find(project => project.id === currentProjectId)
  if (!matched || matched.organization_id !== editForm.organization_id) {
    editForm.project_id = undefined
  }
}
const saveApp = async () => {
  saving.value = true
  try {
    const res = await applicationApi.update(appId, editForm)
    if (res.code === 0) {
      message.success('保存成功')
      editModalVisible.value = false
      await fetchApp()
      await fetchReadiness(true)
    }
  } catch (e: any) { message.error(e.message || '保存失败') }
  finally { saving.value = false }
}

const assignEnvForm = (env?: Partial<ApplicationEnv>) => {
  Object.assign(envForm, {
    id: env?.id,
    application_id: env?.application_id || appId,
    env_name: env?.env_name || '',
    branch: env?.branch || '',
    replicas: env?.replicas || 1,
    gitops_repo_id: env?.gitops_repo_id,
    argocd_application_id: env?.argocd_application_id,
    gitops_branch: env?.gitops_branch || '',
    gitops_path: env?.gitops_path || '',
    helm_chart_path: env?.helm_chart_path || '',
    helm_values_path: env?.helm_values_path || '',
    helm_release_name: env?.helm_release_name || '',
    k8s_cluster_id: env?.k8s_cluster_id,
    k8s_namespace: env?.k8s_namespace || '',
    k8s_deployment: env?.k8s_deployment || '',
    cpu_request: env?.cpu_request || '',
    cpu_limit: env?.cpu_limit || '',
    memory_request: env?.memory_request || '',
    memory_limit: env?.memory_limit || '',
    config: env?.config || '',
  })
}

const showEnvModal = async (env?: ApplicationEnv) => {
  assignEnvForm(env)
  if (!gitOpsRepos.value.length) {
    await fetchGitOpsRepos()
  }
  envModalVisible.value = true
}
const saveEnv = async () => {
  if (!envForm.env_name) { message.error('请选择环境'); return }
  savingEnv.value = true
  try {
    const payload: Partial<ApplicationEnv> = {
      application_id: appId,
      env_name: envForm.env_name,
      branch: envForm.branch,
      replicas: envForm.replicas,
      gitops_repo_id: envForm.gitops_repo_id,
      argocd_application_id: envForm.argocd_application_id,
      gitops_branch: envForm.gitops_branch,
      gitops_path: envForm.gitops_path,
      helm_chart_path: envForm.helm_chart_path,
      helm_values_path: envForm.helm_values_path,
      helm_release_name: envForm.helm_release_name,
      k8s_cluster_id: envForm.k8s_cluster_id,
      k8s_namespace: envForm.k8s_namespace,
      k8s_deployment: envForm.k8s_deployment,
      cpu_request: envForm.cpu_request,
      cpu_limit: envForm.cpu_limit,
      memory_request: envForm.memory_request,
      memory_limit: envForm.memory_limit,
      config: envForm.config,
    }
    const res = envForm.id ? await applicationApi.updateEnv(appId, envForm.id, payload) : await applicationApi.createEnv(appId, payload)
    if (res.code === 0) {
      message.success('保存成功')
      envModalVisible.value = false
      await fetchApp()
      await fetchReadiness(true)
      deliveryLoaded = false
      await fetchDeliveryContext(true)
    }
  } catch (e: any) { message.error(e.message || '保存失败') }
  finally { savingEnv.value = false }
}
const deleteEnv = async (id: number) => {
  try {
    const res = await applicationApi.deleteEnv(appId, id)
    if (res.code === 0) {
      message.success('删除成功')
      await fetchApp()
      await fetchReadiness(true)
    }
  }
  catch (e: any) { message.error(e.message || '删除失败') }
}

const showDeployModal = async (pipeline?: any, env?: ApplicationEnv) => {
  if (!deliveryLoaded && app.value?.name) {
    await fetchDeliveryContext(true)
  }
  const envName = env?.env_name || pipeline?.env || deployEnvOptions.value[0] || envs.value[0]?.env_name || ''
  const envConfig = env || envs.value.find((item) => item.env_name === envName)
  const selectedPipeline = pipeline || relatedPipelines.value.find((item) => !envName || item.env === envName) || relatedPipelines.value[0]
  Object.assign(deployForm, {
    env_name: envName,
    pipeline_id: selectedPipeline?.id,
    image_tag: '',
    branch: envConfig?.branch || selectedPipeline?.git_branch || 'main',
    ref_type: 'branch',
    description: '',
  })
  if (deployForm.env_name) await checkDeployStatus()
  deployModalVisible.value = true
  await loadDeployBranchesAndTags()
}

const showDeployModalForEnv = (env: ApplicationEnv) => {
  const pipeline = getEnvPipelines(env.env_name)[0]
  void showDeployModal(pipeline, env)
}

const showEnvDeliveryRecords = async (envName: string) => {
  deployFilter.env_name = envName
  deployPagination.current = 1
  await fetchDeploys()
  activeTab.value = 'deploys'
}

const checkDeployStatus = async () => {
  if (!deployForm.env_name) return
  try {
    // 检查发布窗口状态
    const windowRes = await applicationApi.getDeployWindowStatus(appId, deployForm.env_name)
    if (windowRes.code === 0 && windowRes.data) deployWindowStatus.value = windowRes.data
    // 检查发布锁状态
    const lockRes = await applicationApi.getDeployLockStatus(appId, deployForm.env_name)
    if (lockRes.code === 0 && lockRes.data) deployLockStatus.value = lockRes.data
    // 检查是否需要审批
    const approvalRes = await applicationApi.checkApprovalRequired(appId, deployForm.env_name)
    if (approvalRes.code === 0) needApproval.value = approvalRes.data?.required || false
  } catch (e) { console.error(e) }
}

const submitDeploy = async () => {
  if (!deployForm.env_name) { message.error('请选择环境'); return }
  if (!deployForm.pipeline_id) { message.error('请选择关联流水线'); return }
  deploying.value = true
  try {
    const envConfig = selectedDeployEnv.value
    const params: Record<string, string> = {
      APP_NAME: app.value?.name || '',
      APPLICATION_NAME: app.value?.name || '',
      APPLICATION_ID: String(appId),
      DEPLOY_ENV: deployForm.env_name,
      GITOPS_ENV: deployForm.env_name,
    }
    if (envConfig?.k8s_namespace) {
      params.K8S_NAMESPACE = envConfig.k8s_namespace
    }
    if (envConfig?.k8s_deployment) {
      params.K8S_DEPLOYMENT = envConfig.k8s_deployment
    }
    if (envConfig?.gitops_repo_id) {
      params.GITOPS_REPO_ID = String(envConfig.gitops_repo_id)
    }
    if (envConfig?.gitops_branch) {
      params.GITOPS_TARGET_BRANCH = envConfig.gitops_branch
    }
    if (envConfig?.helm_values_path) {
      params.GITOPS_FILE_PATH = envConfig.helm_values_path
      params.HELM_VALUES_PATH = envConfig.helm_values_path
    }
    if (envConfig?.helm_chart_path) {
      params.HELM_CHART_PATH = envConfig.helm_chart_path
    }
    if (envConfig?.helm_release_name) {
      params.HELM_RELEASE_NAME = envConfig.helm_release_name
    }
    if (envConfig?.replicas) {
      params.HELM_REPLICAS = String(envConfig.replicas)
    }
    if (envConfig?.cpu_request) {
      params.CPU_REQUEST = envConfig.cpu_request
    }
    if (envConfig?.cpu_limit) {
      params.CPU_LIMIT = envConfig.cpu_limit
    }
    if (envConfig?.memory_request) {
      params.MEMORY_REQUEST = envConfig.memory_request
    }
    if (envConfig?.memory_limit) {
      params.MEMORY_LIMIT = envConfig.memory_limit
    }
    if (deployForm.image_tag) {
      params.IMAGE_TAG = deployForm.image_tag
      params.GITOPS_IMAGE_TAG = deployForm.image_tag
    }
    if (deployForm.description) {
      params.GITOPS_CHANGE_DESCRIPTION = deployForm.description
    }
    await pipelineApi.run(Number(deployForm.pipeline_id), {
      parameters: params,
      branch: deployForm.branch || undefined,
    })
    message.success('流水线已开始执行')
    deployModalVisible.value = false
    deliveryLoaded = false
    setTimeout(() => {
      void fetchDeliveryContext(true)
      void fetchDeploys()
    }, 1200)
  } catch (e: any) { message.error(e.message || '执行失败') }
  finally { deploying.value = false }
}

const viewDeploy = (r: DeliveryRecord) => { currentDeploy.value = r; deployDetailVisible.value = true }

onMounted(async () => {
  await applyDeepLinkFromRoute()
  await Promise.allSettled([fetchApp(), fetchCatalog(), fetchK8sClusters(), fetchGitOpsRepos(), fetchDeploys(), loadRepoBindings(), loadAvailableGitRepos(), fetchReadiness()])
  checkDeployStatus()
  await applyDeepLinkFromRoute()
  if (activeTab.value === 'observability' || activeTab.value === 'change-events') {
    await fetchAppTimeline()
  }
  if (activeTab.value === 'cost') {
    await fetchCostInsights()
  }
  if (activeTab.value === 'security') {
    await fetchSecurityScans()
  }
  if (activeTab.value === 'envs' || activeTab.value === 'delivery') {
    await fetchDeliveryContext()
  }
})
</script>

<style scoped>
.readiness-card {
  margin-bottom: 16px;
}
.readiness-actions {
  text-align: right;
}
.app-detail { padding: 0; }
.page-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px; }
.header-left { display: flex; align-items: center; gap: 12px; }
.header-left h1 { font-size: 20px; font-weight: 500; margin: 0; }
.header-right { display: flex; gap: 8px; }
.muted-text { color: #8c8c8c; }
.env-target-cell { max-width: 300px; }
.env-target-cell span { overflow-wrap: anywhere; line-height: 1.5; }
.security-scan-summary span { margin-right: 8px; padding: 2px 8px; border-radius: 4px; font-size: 12px; }
.security-scan-summary .critical { background: #fff1f0; color: #cf1322; }
.security-scan-summary .high { background: #fff7e6; color: #d46b08; }
.security-scan-summary .medium { background: #e6f7ff; color: #096dd9; }
.security-scan-summary .low { background: #f6ffed; color: #389e0d; }
.filter-tag { cursor: pointer; user-select: none; }
.cost-usage-item + .cost-usage-item { margin-top: 16px; }
.cost-usage-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.cost-usage-hint { color: #8c8c8c; font-size: 12px; line-height: 1.6; }
.cost-env-row { display: flex; justify-content: space-between; align-items: center; gap: 12px; width: 100%; }
.cost-panel-hint { margin-top: 12px; color: #8c8c8c; font-size: 12px; line-height: 1.6; }
.vuln-summary { margin-top: 16px; display: flex; gap: 8px; flex-wrap: wrap; }
.vuln-summary :deep(.ant-tag) { cursor: pointer; transition: all 0.2s; }
.vuln-summary :deep(.ant-tag:hover) { opacity: 0.8; transform: scale(1.05); }
.vuln-summary :deep(.ant-tag.active) { border-width: 2px; font-weight: bold; box-shadow: 0 0 4px rgba(0,0,0,0.2); }
</style>
