<template>
  <div class="argocd-page">
    <a-card :bordered="false" class="hero-card">
      <a-row :gutter="[16, 16]" align="middle">
        <a-col :xs="24" :lg="16">
          <div class="hero-title">GitOps 交付中心</div>
          <div class="hero-subtitle">围绕 Argo CD 同步状态、GitOps 仓库和变更请求，统一管理金融场景下的声明式交付链路。</div>
          <a-tag v-if="projectId" color="blue" style="margin-top: 12px">当前项目视图 #{{ projectId }}</a-tag>
          <div class="hero-priority">
            <div class="hero-priority-label">当前重点</div>
            <div class="hero-priority-title">{{ heroPriorityTitle }}</div>
            <div class="hero-priority-desc">{{ heroPriorityDescription }}</div>
          </div>
        </a-col>
        <a-col :xs="24" :lg="8">
          <a-space wrap>
            <a-button type="primary" @click="activeTab = 'changes'; openChangeRequestModal()">发起变更</a-button>
            <a-button @click="activeTab = 'apps'">查看应用</a-button>
            <a-button @click="activeTab = 'repos'">管理仓库</a-button>
            <a-button @click="activeTab = 'instances'">Argo CD 实例</a-button>
          </a-space>
        </a-col>
      </a-row>
    </a-card>

    <a-row :gutter="[16, 16]" class="summary-row">
      <a-col :xs="24" :sm="12" :xl="6">
        <a-card :bordered="false">
          <a-statistic title="Argo CD 实例" :value="summary.instance_total" />
          <div class="summary-extra">活跃 {{ summary.instance_active }} 个</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :xl="6">
        <a-card :bordered="false">
          <a-statistic title="GitOps 应用" :value="summary.app_total" />
          <div class="summary-extra">已同步 {{ summary.app_synced }} 个，漂移 {{ summary.app_drifted }} 个</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :xl="6">
        <a-card :bordered="false">
          <a-statistic title="GitOps 仓库" :value="summary.repo_total" />
          <div class="summary-extra">自动同步 {{ summary.repo_sync_enabled }} 个</div>
        </a-card>
      </a-col>
      <a-col :xs="24" :sm="12" :xl="6">
        <a-card :bordered="false">
          <a-statistic title="变更请求" :value="summary.change_request_open" />
          <div class="summary-extra">草稿 {{ summary.change_request_draft }} 个，失败 {{ summary.change_request_failed }} 个</div>
        </a-card>
      </a-col>
    </a-row>

    <a-row :gutter="[16, 16]" class="summary-row">
      <a-col :xs="24" :lg="8">
        <a-card :bordered="false" title="处理建议" class="advice-card">
          <a-list :data-source="heroSuggestions" size="small">
            <template #renderItem="{ item }">
              <a-list-item>
                <div class="advice-main">
                  <div class="advice-title">{{ item.title }}</div>
                  <div class="summary-extra">{{ item.description }}</div>
                </div>
                <a-button type="link" @click="item.action()">{{ item.label }}</a-button>
              </a-list-item>
            </template>
          </a-list>
        </a-card>
      </a-col>
      <a-col :xs="24" :lg="16">
        <a-card :bordered="false" title="交付关注点" class="advice-card">
          <a-row :gutter="[12, 12]">
            <a-col :xs="24" :md="8">
              <div class="focus-panel">
                <div class="focus-label">同步异常</div>
                <div class="focus-value warning">{{ summary.app_out_of_sync }}</div>
                <div class="summary-extra">优先看 OutOfSync 和 Drift 应用</div>
              </div>
            </a-col>
            <a-col :xs="24" :md="8">
              <div class="focus-panel">
                <div class="focus-label">待处理变更</div>
                <div class="focus-value">{{ summary.change_request_open }}</div>
                <div class="summary-extra">先清理待审批、失败和未合并请求</div>
              </div>
            </a-col>
            <a-col :xs="24" :md="8">
              <div class="focus-panel">
                <div class="focus-label">自动同步覆盖</div>
                <div class="focus-value success">{{ summary.app_auto_sync }}</div>
                <div class="summary-extra">继续补齐仓库和应用的自动同步配置</div>
              </div>
            </a-col>
          </a-row>
        </a-card>
      </a-col>
    </a-row>

    <a-tabs v-model:activeKey="activeTab" @change="handleTabChange">
      <a-tab-pane key="overview" tab="交付大盘">
        <a-row :gutter="[16, 16]">
          <a-col :xs="24" :lg="12">
            <a-card title="同步健康概览" :bordered="false">
              <a-row :gutter="[12, 12]">
                <a-col :span="12"><a-statistic title="OutOfSync" :value="summary.app_out_of_sync" :value-style="{ color: '#fa8c16' }" /></a-col>
                <a-col :span="12"><a-statistic title="Degraded" :value="summary.app_degraded" :value-style="{ color: '#ff4d4f' }" /></a-col>
                <a-col :span="12"><a-statistic title="Healthy" :value="summary.app_healthy" :value-style="{ color: '#52c41a' }" /></a-col>
                <a-col :span="12"><a-statistic title="Auto Sync" :value="summary.app_auto_sync" :value-style="{ color: '#1677ff' }" /></a-col>
              </a-row>
            </a-card>
          </a-col>
          <a-col :xs="24" :lg="12">
            <a-card title="最近变更请求" :bordered="false" extra="自动创建 MR 目前优先支持 GitLab Token 仓库">
              <a-empty v-if="recentChangeRequests.length === 0" description="暂无变更请求" />
              <a-list v-else :data-source="recentChangeRequests" size="small">
                <template #renderItem="{ item }">
                  <a-list-item>
                    <a-list-item-meta :title="item.title" :description="`${item.application_name || '-'} / ${item.env || '-'} / ${item.file_path}`" />
                    <template #actions>
                      <a-tag :color="approvalStatusColor(item.approval_status || '')">{{ item.approval_status || '-' }}</a-tag>
                      <a-tag :color="changeRequestStatusColor(item.status || '')">{{ item.status }}</a-tag>
                      <a @click="openChangeDetail(item.id!)">详情</a>
                      <a v-if="item.approval_instance_id" @click="goToApprovalInstance(item.approval_instance_id)">审批详情</a>
                      <a v-if="item.merge_request_url" :href="item.merge_request_url" target="_blank">查看 MR</a>
                    </template>
                  </a-list-item>
                </template>
              </a-list>
            </a-card>
          </a-col>
          <a-col :xs="24">
            <a-card title="待处理漂移应用" :bordered="false">
              <a-empty v-if="driftApps.length === 0" description="暂无漂移应用" />
              <a-table v-else :columns="overviewAppColumns" :data-source="driftApps" row-key="id" :pagination="false" size="small">
                <template #bodyCell="{ column, record }">
                  <template v-if="column.key === 'sync_status'">
                    <a-tag :color="syncStatusColor(record.sync_status)">{{ record.sync_status }}</a-tag>
                  </template>
                  <template v-if="column.key === 'health_status'">
                    <a-tag :color="healthStatusColor(record.health_status)">{{ record.health_status }}</a-tag>
                  </template>
                  <template v-if="column.key === 'action'">
                    <a-space>
                      <a @click="handleTriggerSync(record)">触发同步</a>
                      <a @click="openChangeRequestModal(record)">发起变更</a>
                    </a-space>
                  </template>
                </template>
              </a-table>
            </a-card>
          </a-col>
        </a-row>
      </a-tab-pane>

      <a-tab-pane key="apps" tab="GitOps 应用">
        <div class="toolbar">
          <a-select v-model:value="appFilter.instance_id" placeholder="选择实例" allow-clear style="width: 180px" @change="loadApps">
            <a-select-option v-for="inst in instances" :key="inst.id" :value="inst.id">{{ inst.name }}</a-select-option>
          </a-select>
          <a-select v-model:value="appFilter.sync_status" placeholder="同步状态" allow-clear style="width: 140px" @change="loadApps">
            <a-select-option value="Synced">Synced</a-select-option>
            <a-select-option value="OutOfSync">OutOfSync</a-select-option>
            <a-select-option value="Unknown">Unknown</a-select-option>
          </a-select>
          <a-select v-model:value="appFilter.health_status" placeholder="健康状态" allow-clear style="width: 140px" @change="loadApps">
            <a-select-option value="Healthy">Healthy</a-select-option>
            <a-select-option value="Degraded">Degraded</a-select-option>
            <a-select-option value="Progressing">Progressing</a-select-option>
            <a-select-option value="Missing">Missing</a-select-option>
          </a-select>
          <a-checkbox v-model:checked="driftOnly" @change="loadApps">仅看漂移</a-checkbox>
          <a-tooltip title="从 Argo CD 实时拉取所有应用的最新 Sync / Health 状态">
            <a-button :loading="refreshingApps" @click="manualRefreshApps">
              <template #icon><ReloadOutlined /></template>
              刷新状态
            </a-button>
          </a-tooltip>
        </div>
        <a-table
          :columns="appColumns"
          :data-source="apps"
          :loading="loadingApps"
          row-key="id"
          :pagination="{ current: appPage, pageSize: appPageSize, total: appTotal, showSizeChanger: true }"
          @change="handleAppTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'sync_status'">
              <a-tag :color="syncStatusColor(record.sync_status)">{{ record.sync_status }}</a-tag>
            </template>
            <template v-if="column.key === 'health_status'">
              <a-tag :color="healthStatusColor(record.health_status)">{{ record.health_status }}</a-tag>
            </template>
            <template v-if="column.key === 'drift_detected'">
              <a-tag v-if="record.drift_detected" color="red">Drift</a-tag>
              <span v-else>-</span>
            </template>
            <template v-if="column.key === 'sync_policy'">
              <a-tag :color="record.sync_policy === 'auto' ? 'blue' : 'default'">{{ record.sync_policy }}</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a @click="handleTriggerSync(record)">触发同步</a>
                <a @click="openResourceModal(record)">资源树</a>
                <a @click="openChangeRequestModal(record)">发起变更</a>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="changes" tab="变更请求">
        <div class="toolbar toolbar-right">
          <a-button type="primary" @click="openChangeRequestModal()">发起变更请求</a-button>
        </div>
        <a-table
          :columns="changeColumns"
          :data-source="changeRequests"
          :loading="loadingChanges"
          row-key="id"
          :pagination="{ current: changePage, pageSize: changePageSize, total: changeTotal, showSizeChanger: true }"
          @change="handleChangeTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="changeRequestStatusColor(record.status || '')">{{ record.status }}</a-tag>
            </template>
            <template v-if="column.key === 'approval_status'">
              <a-tag :color="approvalStatusColor(record.approval_status || '')">{{ record.approval_status || '-' }}</a-tag>
            </template>
            <template v-if="column.key === 'auto_merge_status'">
              <a-tag :color="autoMergeStatusColor(record.auto_merge_status || '')">{{ autoMergeStatusText(record.auto_merge_status || '') }}</a-tag>
            </template>
            <template v-if="column.key === 'mr'">
              <a v-if="record.merge_request_url" :href="record.merge_request_url" target="_blank">查看 MR</a>
              <span v-else>-</span>
            </template>
            <template v-if="column.key === 'approval'">
              <a v-if="record.approval_instance_id" @click="goToApprovalInstance(record.approval_instance_id)">审批详情</a>
              <span v-else>-</span>
            </template>
            <template v-if="column.key === 'detail'">
              <a @click="openChangeDetail(record.id!)">详情</a>
            </template>
            <template v-if="column.key === 'error_message'">
              <span class="error-text">{{ record.error_message || '-' }}</span>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="repos" tab="GitOps 仓库">
        <div class="toolbar toolbar-right">
          <a-button type="primary" @click="openRepoModal()">新建仓库</a-button>
        </div>
        <a-table
          :columns="repoColumns"
          :data-source="repos"
          :loading="loadingRepos"
          row-key="id"
          :pagination="{ current: repoPage, pageSize: repoPageSize, total: repoTotal, showSizeChanger: true }"
          @change="handleRepoTableChange"
        >
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'sync_enabled'">
              <a-tag :color="record.sync_enabled ? 'green' : 'default'">{{ record.sync_enabled ? 'ON' : 'OFF' }}</a-tag>
            </template>
            <template v-if="column.key === 'auth_type'">
              <a-tag>{{ record.auth_type }}</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a @click="openRepoModal(record)">编辑</a>
                <a @click="openChangeRequestModal(undefined, record)">发起变更</a>
                <a-popconfirm title="确定删除该仓库配置吗？" @confirm="handleDeleteRepo(record.id!)">
                  <a style="color: #ff4d4f">删除</a>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>

      <a-tab-pane key="instances" tab="Argo CD 实例">
        <div class="toolbar toolbar-right">
          <a-button type="primary" @click="openInstanceModal()">新建实例</a-button>
        </div>
        <a-table :columns="instanceColumns" :data-source="instances" :loading="loadingInst" row-key="id" :pagination="false">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'status'">
              <a-tag :color="record.status === 'active' ? 'green' : 'default'">{{ record.status }}</a-tag>
            </template>
            <template v-if="column.key === 'is_default'">
              <a-tag v-if="record.is_default" color="blue">默认</a-tag>
            </template>
            <template v-if="column.key === 'action'">
              <a-space>
                <a @click="handleTestConnection(record)">测试连接</a>
                <a @click="handleSyncApps(record)">拉取状态</a>
                <a @click="openInstanceModal(record)">编辑</a>
                <a-popconfirm title="确定删除该实例吗？" @confirm="handleDeleteInstance(record.id!)">
                  <a style="color: #ff4d4f">删除</a>
                </a-popconfirm>
              </a-space>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>

    <a-modal v-model:open="instModalVisible" :title="instForm.id ? '编辑实例' : '新建实例'" @ok="handleSaveInstance" :confirm-loading="saving">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="名称">
          <a-input v-model:value="instForm.name" />
        </a-form-item>
        <a-form-item label="地址">
          <a-input v-model:value="instForm.server_url" placeholder="https://argocd.example.com" />
        </a-form-item>
        <a-form-item label="Token">
          <a-input-password v-model:value="instForm.auth_token" :placeholder="instForm.id ? '留空表示不修改' : ''" />
        </a-form-item>
        <a-form-item label="跳过 TLS 校验">
          <a-switch v-model:checked="instForm.insecure" />
        </a-form-item>
        <a-form-item label="默认">
          <a-switch v-model:checked="instForm.is_default" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="repoModalVisible" :title="repoForm.id ? '编辑 GitOps 仓库' : '新建 GitOps 仓库'" @ok="handleSaveRepo" :confirm-loading="saving">
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="名称">
          <a-input v-model:value="repoForm.name" />
        </a-form-item>
        <a-form-item label="仓库地址">
          <a-input v-model:value="repoForm.repo_url" placeholder="https://gitlab.example.com/group/repo.git" />
        </a-form-item>
        <a-form-item label="目标分支">
          <a-input v-model:value="repoForm.branch" placeholder="main" />
        </a-form-item>
        <a-form-item label="清单目录">
          <a-input v-model:value="repoForm.path" placeholder="/" />
        </a-form-item>
        <a-form-item label="认证方式">
          <a-select v-model:value="repoForm.auth_type">
            <a-select-option value="token">Token</a-select-option>
            <a-select-option value="ssh">SSH</a-select-option>
            <a-select-option value="none">None</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item v-if="repoForm.auth_type !== 'none'" label="凭证">
          <a-input-password v-model:value="repoForm.auth_credential" :placeholder="repoForm.id ? '留空表示不修改' : ''" />
        </a-form-item>
        <a-form-item label="应用名">
          <a-input v-model:value="repoForm.application_name" />
        </a-form-item>
        <a-form-item label="环境">
          <a-select v-model:value="repoForm.env" allow-clear>
            <a-select-option value="dev">dev</a-select-option>
            <a-select-option value="test">test</a-select-option>
            <a-select-option value="uat">uat</a-select-option>
            <a-select-option value="prod">prod</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="自动同步">
          <a-switch v-model:checked="repoForm.sync_enabled" />
        </a-form-item>
      </a-form>
    </a-modal>

    <a-modal v-model:open="changeModalVisible" :title="'发起 GitOps 变更请求'" @ok="handleCreateChangeRequest" :confirm-loading="savingChange" width="720px">
      <a-alert
        type="info"
        show-icon
        style="margin-bottom: 16px"
        message="当前自动 MR 优先支持 GitLab + Token 仓库"
        description="平台会尝试在目标仓库新建分支、更新清单文件中的镜像标签并创建 Merge Request。"
      />
      <a-form :label-col="{ span: 6 }" :wrapper-col="{ span: 16 }">
        <a-form-item label="GitOps 仓库">
          <a-select v-model:value="changeForm.gitops_repo_id" placeholder="选择仓库" show-search>
            <a-select-option v-for="repo in repos" :key="repo.id" :value="repo.id">{{ repo.name }} / {{ repo.application_name || '-' }} / {{ repo.env || '-' }}</a-select-option>
          </a-select>
        </a-form-item>
        <a-form-item label="应用名称">
          <a-select
            v-model:value="changeForm.application_name"
            placeholder="从应用列表选择"
            show-search
            allow-clear
            :loading="loadingPlatformApps"
            :options="platformAppSelectOptions"
            option-filter-prop="label"
          />
        </a-form-item>
        <a-form-item label="环境">
          <a-input v-model:value="changeForm.env" placeholder="如 prod" />
        </a-form-item>
        <a-form-item label="标题">
          <a-input v-model:value="changeForm.title" placeholder="为空时自动生成" />
        </a-form-item>
        <a-form-item label="说明">
          <a-textarea v-model:value="changeForm.description" :rows="3" placeholder="变更说明、审批背景、关联事项" />
        </a-form-item>
        <a-form-item label="清单文件">
          <a-input v-model:value="changeForm.file_path" placeholder="如 apps/pay/prod/deployment.yaml" />
        </a-form-item>
        <a-form-item label="镜像仓库">
          <a-input v-model:value="changeForm.image_repository" placeholder="如 localhost:5001/jeridevops/pay-service" />
        </a-form-item>
        <a-form-item label="镜像标签">
          <a-input v-model:value="changeForm.image_tag" placeholder="如 v1.2.3" />
        </a-form-item>
        <a-form-item label="目标分支">
          <a-input v-model:value="changeForm.target_branch" placeholder="为空时使用仓库默认分支" />
        </a-form-item>
      </a-form>
      <a-divider orientation="left">变更预检</a-divider>
      <div class="precheck-toolbar">
        <a-button size="small" @click="runPrecheck">执行预检</a-button>
        <span v-if="changePrecheck" :class="changePrecheck.can_create ? 'ok-text' : 'error-text'">
          {{ changePrecheck.can_create ? '预检通过，可发起变更' : '预检未通过，请先处理失败项' }}
        </span>
      </div>
      <a-empty v-if="!changePrecheck" description="尚未执行预检" />
      <div v-else>
        <a-alert
          v-if="changePrecheck.policy"
          type="info"
          show-icon
          style="margin-bottom: 12px"
          :message="`环境策略：${changePrecheck.policy.env_name}`"
          :description="buildPolicyDescription(changePrecheck.policy)"
        />
        <a-list :data-source="changePrecheck.checks" size="small" bordered>
          <template #renderItem="{ item }">
            <a-list-item>
              <a-space direction="vertical" style="width: 100%">
                <div class="precheck-item">
                  <div>
                    <strong>{{ item.name }}</strong>
                    <span class="precheck-required" v-if="item.required">必需</span>
                  </div>
                  <a-tag :color="item.passed ? 'green' : 'red'">{{ item.passed ? '通过' : '失败' }}</a-tag>
                </div>
                <div>{{ item.message }}</div>
                <div v-if="item.detail" class="precheck-detail">{{ item.detail }}</div>
              </a-space>
            </a-list-item>
          </template>
        </a-list>
      </div>
    </a-modal>

    <a-drawer v-model:open="changeDetailVisible" title="GitOps 变更详情" width="760" :footer-style="{ textAlign: 'right' }">
      <a-spin :spinning="loadingChangeDetail">
        <a-empty v-if="!changeDetail" description="未找到变更详情" />
        <template v-else>
          <a-descriptions :column="2" bordered size="small">
            <a-descriptions-item label="标题">{{ changeDetail.title }}</a-descriptions-item>
            <a-descriptions-item label="状态">
              <a-tag :color="changeRequestStatusColor(changeDetail.status || '')">{{ changeDetail.status || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="应用">{{ changeDetail.application_name || '-' }}</a-descriptions-item>
            <a-descriptions-item label="环境">{{ changeDetail.env || '-' }}</a-descriptions-item>
            <a-descriptions-item label="审批状态">
              <a-tag :color="approvalStatusColor(changeDetail.approval_status || '')">{{ changeDetail.approval_status || '-' }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="自动合并">
              <a-tag :color="autoMergeStatusColor(changeDetail.auto_merge_status || '')">{{ autoMergeStatusText(changeDetail.auto_merge_status || '') }}</a-tag>
            </a-descriptions-item>
            <a-descriptions-item label="清单文件" :span="2">{{ changeDetail.file_path || '-' }}</a-descriptions-item>
            <a-descriptions-item label="镜像仓库" :span="2">{{ changeDetail.image_repository || '-' }}</a-descriptions-item>
            <a-descriptions-item label="镜像标签">{{ changeDetail.image_tag || '-' }}</a-descriptions-item>
            <a-descriptions-item label="目标分支">{{ changeDetail.target_branch || '-' }}</a-descriptions-item>
            <a-descriptions-item label="源分支">{{ changeDetail.source_branch || '-' }}</a-descriptions-item>
            <a-descriptions-item label="提交 SHA">{{ changeDetail.last_commit_sha || '-' }}</a-descriptions-item>
            <a-descriptions-item label="审批链">{{ changeDetail.approval_chain_name || '-' }}</a-descriptions-item>
            <a-descriptions-item label="审批完成">{{ changeDetail.approval_finished_at || '-' }}</a-descriptions-item>
            <a-descriptions-item label="自动合并时间">{{ changeDetail.auto_merged_at || '-' }}</a-descriptions-item>
            <a-descriptions-item label="描述" :span="2">{{ changeDetail.description || '-' }}</a-descriptions-item>
            <a-descriptions-item v-if="changeDetail.error_message" label="错误信息" :span="2">
              <span class="error-text">{{ changeDetail.error_message }}</span>
            </a-descriptions-item>
          </a-descriptions>
          <a-divider orientation="left">变更预检</a-divider>
          <a-space style="margin-bottom: 12px">
            <a-button size="small" @click="refreshDetailPrecheck(changeDetail)">重新预检</a-button>
            <a-button v-if="changeDetail.approval_instance_id" size="small" @click="goToApprovalInstance(changeDetail.approval_instance_id)">审批详情</a-button>
            <a-button v-if="changeDetail.merge_request_url" size="small" @click="windowOpen(changeDetail.merge_request_url)">查看 MR</a-button>
          </a-space>
          <a-empty v-if="!changeDetailPrecheck" description="暂无预检结果" />
          <div v-else>
            <a-alert
              v-if="changeDetailPrecheck.policy"
              type="info"
              show-icon
              style="margin-bottom: 12px"
              :message="`环境策略：${changeDetailPrecheck.policy.env_name}`"
              :description="buildPolicyDescription(changeDetailPrecheck.policy)"
            />
            <a-list :data-source="changeDetailPrecheck.checks" size="small" bordered>
              <template #renderItem="{ item }">
                <a-list-item>
                  <a-space direction="vertical" style="width: 100%">
                    <div class="precheck-item">
                      <div>
                        <strong>{{ item.name }}</strong>
                        <span class="precheck-required" v-if="item.required">必需</span>
                      </div>
                      <a-tag :color="item.passed ? 'green' : 'red'">{{ item.passed ? '通过' : '失败' }}</a-tag>
                    </div>
                    <div>{{ item.message }}</div>
                    <div v-if="item.detail" class="precheck-detail">{{ item.detail }}</div>
                  </a-space>
                </a-list-item>
              </template>
            </a-list>
          </div>
        </template>
      </a-spin>
    </a-drawer>

    <a-modal v-model:open="resourceModalVisible" title="资源树" :footer="null" width="820px">
      <a-spin :spinning="loadingResources">
        <a-table :columns="resourceColumns" :data-source="resources" row-key="name" :pagination="false" size="small">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'health'">
              <a-tag v-if="record.health" :color="healthStatusColor(record.health.status)">{{ record.health.status }}</a-tag>
              <span v-else>-</span>
            </template>
          </template>
        </a-table>
      </a-spin>
    </a-modal>
  </div>
</template>

<script setup lang="ts">
import { computed, onMounted, reactive, ref, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { message } from 'ant-design-vue'
import { ReloadOutlined } from '@ant-design/icons-vue'
import { argocdApi } from '@/services/argocd'
import { applicationApi } from '@/services/application'
import type { Application } from '@/services/application'
import type {
  ArgoCDApplication,
  ArgoCDInstance,
  ChangeRequestPolicySummary,
  ChangeRequestPrecheck,
  GitOpsChangeRequest,
  GitOpsDashboardSummary,
  GitOpsRepo,
  ResourceNode,
} from '@/services/argocd'

const activeTab = ref('overview')
const route = useRoute()
const router = useRouter()
const saving = ref(false)
const projectId = computed(() => {
  const raw = Array.isArray(route.query.project_id) ? route.query.project_id[0] : route.query.project_id
  const parsed = Number(raw || 0)
  return parsed > 0 ? parsed : undefined
})

const summary = reactive<GitOpsDashboardSummary>({
  instance_total: 0,
  instance_active: 0,
  app_total: 0,
  app_synced: 0,
  app_out_of_sync: 0,
  app_healthy: 0,
  app_degraded: 0,
  app_drifted: 0,
  app_auto_sync: 0,
  repo_total: 0,
  repo_sync_enabled: 0,
  change_request_open: 0,
  change_request_draft: 0,
  change_request_failed: 0,
})

const instances = ref<ArgoCDInstance[]>([])
const repos = ref<GitOpsRepo[]>([])
const apps = ref<ArgoCDApplication[]>([])
const driftApps = ref<ArgoCDApplication[]>([])
const changeRequests = ref<GitOpsChangeRequest[]>([])
const recentChangeRequests = ref<GitOpsChangeRequest[]>([])
const resources = ref<ResourceNode[]>([])

const loadingInst = ref(false)
const loadingRepos = ref(false)
const loadingApps = ref(false)
const loadingChanges = ref(false)
const loadingResources = ref(false)
const savingChange = ref(false)
const loadingChangeDetail = ref(false)

const instModalVisible = ref(false)
const repoModalVisible = ref(false)
const changeModalVisible = ref(false)
const changeDetailVisible = ref(false)
const resourceModalVisible = ref(false)

const appPage = ref(1)
const appPageSize = ref(20)
const appTotal = ref(0)
const repoPage = ref(1)
const repoPageSize = ref(20)
const repoTotal = ref(0)
const changePage = ref(1)
const changePageSize = ref(20)
const changeTotal = ref(0)
const driftOnly = ref(false)

const appFilter = reactive<{ instance_id?: number; sync_status?: string; health_status?: string }>({})

const instForm = ref<Partial<ArgoCDInstance>>({})
const repoForm = ref<Partial<GitOpsRepo>>({})
const changeForm = ref<Partial<GitOpsChangeRequest>>({})
const changePrecheck = ref<ChangeRequestPrecheck | null>(null)
const changeDetail = ref<GitOpsChangeRequest | null>(null)
const changeDetailPrecheck = ref<ChangeRequestPrecheck | null>(null)

const overviewAppColumns = [
  { title: '应用', dataIndex: 'name', key: 'name' },
  { title: '项目', dataIndex: 'project', key: 'project', width: 120 },
  { title: '同步状态', key: 'sync_status', width: 120 },
  { title: '健康状态', key: 'health_status', width: 120 },
  { title: '命名空间', dataIndex: 'dest_namespace', key: 'dest_namespace', width: 140 },
  { title: '操作', key: 'action', width: 180 },
]

const appColumns = [
  { title: '应用', dataIndex: 'name', key: 'name' },
  { title: '项目', dataIndex: 'project', key: 'project', width: 110 },
  { title: '同步状态', key: 'sync_status', width: 110 },
  { title: '健康状态', key: 'health_status', width: 110 },
  { title: '同步策略', key: 'sync_policy', width: 100 },
  { title: '漂移', key: 'drift_detected', width: 80 },
  { title: '命名空间', dataIndex: 'dest_namespace', key: 'dest_namespace', width: 120 },
  { title: '目标版本', dataIndex: 'target_revision', key: 'target_revision', width: 120 },
  { title: '最后同步', dataIndex: 'last_sync_at', key: 'last_sync_at', width: 180 },
  { title: '操作', key: 'action', width: 220 },
]

const repoColumns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '仓库地址', dataIndex: 'repo_url', key: 'repo_url', ellipsis: true },
  { title: '分支', dataIndex: 'branch', key: 'branch', width: 100 },
  { title: '认证', key: 'auth_type', width: 90 },
  { title: '自动同步', key: 'sync_enabled', width: 90 },
  { title: '应用', dataIndex: 'application_name', key: 'application_name', width: 140 },
  { title: '环境', dataIndex: 'env', key: 'env', width: 90 },
  { title: '操作', key: 'action', width: 180 },
]

const instanceColumns = [
  { title: '名称', dataIndex: 'name', key: 'name' },
  { title: '地址', dataIndex: 'server_url', key: 'server_url' },
  { title: '状态', dataIndex: 'status', key: 'status', width: 100 },
  { title: '默认', key: 'is_default', width: 80 },
  { title: '操作', key: 'action', width: 240 },
]

const changeColumns = [
  { title: '标题', dataIndex: 'title', key: 'title' },
  { title: '应用', dataIndex: 'application_name', key: 'application_name', width: 140 },
  { title: '环境', dataIndex: 'env', key: 'env', width: 90 },
  { title: '镜像标签', dataIndex: 'image_tag', key: 'image_tag', width: 120 },
  { title: '状态', key: 'status', width: 100 },
  { title: '审批状态', key: 'approval_status', width: 120 },
  { title: '自动合并', key: 'auto_merge_status', width: 120 },
  { title: '分支', dataIndex: 'source_branch', key: 'source_branch', width: 180 },
  { title: '详情', key: 'detail', width: 80 },
  { title: '审批', key: 'approval', width: 100 },
  { title: 'MR', key: 'mr', width: 90 },
  { title: '错误信息', key: 'error_message', width: 220 },
]

const resourceColumns = [
  { title: 'Kind', dataIndex: 'kind', key: 'kind', width: 140 },
  { title: 'Name', dataIndex: 'name', key: 'name' },
  { title: 'Namespace', dataIndex: 'namespace', key: 'namespace', width: 180 },
  { title: 'Group', dataIndex: 'group', key: 'group', width: 140 },
  { title: 'Health', key: 'health', width: 120 },
]

const matchedRepo = computed(() => {
  return repos.value.find((repo) => repo.id === changeForm.value.gitops_repo_id)
})

const platformApps = ref<Application[]>([])
const loadingPlatformApps = ref(false)

const platformAppSelectOptions = computed(() =>
  (platformApps.value || []).map((a) => ({
    label:
      a.display_name && String(a.display_name).trim() && a.display_name !== a.name
        ? `${a.display_name}（${a.name}）`
        : a.name,
    value: a.name,
  }))
)

const loadPlatformApplications = async () => {
  loadingPlatformApps.value = true
  try {
    const res = await applicationApi.list({ page: 1, page_size: 500, status: 'active', project_id: projectId.value })
    platformApps.value = res?.data?.list || []
  } catch (e) {
    console.error(e)
    message.warning('加载应用列表失败，请稍后重试或检查应用管理权限')
    platformApps.value = []
  } finally {
    loadingPlatformApps.value = false
  }
}

const loadSummary = async () => {
  const res = await argocdApi.getDashboardSummary({ project_id: projectId.value })
  Object.assign(summary, res.data?.data || {})
}

const loadInstances = async () => {
  loadingInst.value = true
  try {
    const res = await argocdApi.listInstances()
    instances.value = res.data?.data || []
  } finally {
    loadingInst.value = false
  }
}

const loadRepos = async () => {
  loadingRepos.value = true
  try {
    const res = await argocdApi.listRepos({ page: repoPage.value, page_size: repoPageSize.value, project_id: projectId.value })
    repos.value = res.data?.data?.list || []
    repoTotal.value = res.data?.data?.total || 0
  } finally {
    loadingRepos.value = false
  }
}

const loadApps = async () => {
  loadingApps.value = true
  try {
    const res = await argocdApi.listApps({
      page: appPage.value,
      page_size: appPageSize.value,
      instance_id: appFilter.instance_id,
      project_id: projectId.value,
      sync_status: appFilter.sync_status,
      health_status: appFilter.health_status,
      drift_only: driftOnly.value ? 'true' : undefined,
    })
    apps.value = res.data?.data?.list || []
    appTotal.value = res.data?.data?.total || 0
  } finally {
    loadingApps.value = false
  }
}

const loadDriftApps = async () => {
  const res = await argocdApi.listApps({ page: 1, page_size: 5, project_id: projectId.value, drift_only: 'true' })
  driftApps.value = res.data?.data?.list || []
}

const loadChangeRequests = async () => {
  loadingChanges.value = true
  try {
    const res = await argocdApi.listChangeRequests({ page: changePage.value, page_size: changePageSize.value, project_id: projectId.value })
    changeRequests.value = res.data?.data?.list || []
    changeTotal.value = res.data?.data?.total || 0
  } finally {
    loadingChanges.value = false
  }
}

const loadRecentChangeRequests = async () => {
  const res = await argocdApi.listChangeRequests({ page: 1, page_size: 5, project_id: projectId.value })
  recentChangeRequests.value = res.data?.data?.list || []
}

const reloadOverview = async () => {
  await Promise.all([loadSummary(), loadDriftApps(), loadRecentChangeRequests()])
}

const openInstanceModal = (record?: ArgoCDInstance) => {
  instForm.value = record
    ? { ...record, auth_token: '', insecure: Boolean(record.insecure) }
    : { name: '', server_url: '', auth_token: '', insecure: false, is_default: false }
  instModalVisible.value = true
}

const openRepoModal = (record?: GitOpsRepo) => {
  repoForm.value = record
    ? { ...record, auth_credential: '' }
    : { name: '', repo_url: '', branch: 'main', path: '/', auth_type: 'token', auth_credential: '', env: '', sync_enabled: true, application_name: '' }
  repoModalVisible.value = true
}

const openChangeRequestModal = async (app?: ArgoCDApplication, repo?: GitOpsRepo) => {
  await loadPlatformApplications()
  const autoRepo = repo || repos.value.find((item) =>
    item.application_name && app?.name && item.application_name === app.name && (!item.env || !app.env || item.env === app.env)
  )
  changeForm.value = {
    gitops_repo_id: autoRepo?.id,
    argocd_application_id: app?.id,
    application_name: app?.name || repo?.application_name || '',
    env: app?.env || repo?.env || '',
    title: '',
    description: '',
    file_path: autoRepo?.path && autoRepo.path !== '/' ? `${autoRepo.path.replace(/^\/+/, '').replace(/\/+$/, '')}/deployment.yaml` : 'deployment.yaml',
    image_repository: '',
    image_tag: '',
    target_branch: autoRepo?.branch || '',
  }
  changePrecheck.value = null
  changeModalVisible.value = true
}

const handleSaveInstance = async () => {
  saving.value = true
  try {
    if (instForm.value.id) {
      await argocdApi.updateInstance(instForm.value.id, instForm.value)
    } else {
      await argocdApi.createInstance(instForm.value)
    }
    message.success('保存成功')
    instModalVisible.value = false
    await Promise.all([loadInstances(), loadSummary()])
  } finally {
    saving.value = false
  }
}

const handleSaveRepo = async () => {
  saving.value = true
  try {
    if (repoForm.value.id) {
      await argocdApi.updateRepo(repoForm.value.id, repoForm.value)
    } else {
      await argocdApi.createRepo(repoForm.value)
    }
    message.success('保存成功')
    repoModalVisible.value = false
    await Promise.all([loadRepos(), loadSummary()])
  } finally {
    saving.value = false
  }
}

const handleDeleteInstance = async (id: number) => {
  await argocdApi.deleteInstance(id)
  message.success('删除成功')
  await Promise.all([loadInstances(), loadSummary()])
}

const handleDeleteRepo = async (id: number) => {
  await argocdApi.deleteRepo(id)
  message.success('删除成功')
  await Promise.all([loadRepos(), loadSummary()])
}

const handleTestConnection = async (record: ArgoCDInstance) => {
  await argocdApi.testConnection(record.id!)
  message.success('连接成功')
}

const handleSyncApps = async (record: ArgoCDInstance) => {
  await argocdApi.syncApps(record.id!)
  message.success('已拉取最新状态')
  await Promise.all([loadApps(), reloadOverview()])
}

const refreshingApps = ref(false)
const lastAppsRefreshAt = ref(0)
const APPS_REFRESH_TTL_MS = 30_000

const refreshAppStatus = async () => {
  const targets = instances.value.filter(i => i.status === 'active' && i.id)
  if (targets.length === 0) return
  refreshingApps.value = true
  try {
    await Promise.all(targets.map(i => argocdApi.syncApps(i.id!).catch(() => null)))
    lastAppsRefreshAt.value = Date.now()
  } finally {
    refreshingApps.value = false
  }
}

const manualRefreshApps = async () => {
  await refreshAppStatus()
  await Promise.all([loadApps(), reloadOverview()])
  message.success('应用状态已刷新')
}

const handleTriggerSync = async (record: ArgoCDApplication) => {
  await argocdApi.triggerSync(record.id!)
  message.success('已触发同步')
  await Promise.all([loadApps(), reloadOverview()])
}

const handleCreateChangeRequest = async () => {
  if (!changeForm.value.gitops_repo_id) {
    message.error('请选择 GitOps 仓库')
    return
  }
  savingChange.value = true
  try {
    if (!changeForm.value.target_branch && matchedRepo.value?.branch) {
      changeForm.value.target_branch = matchedRepo.value.branch
    }
    const precheck = await runPrecheck()
    if (!precheck?.can_create) {
      message.error('预检未通过，请先处理失败项')
      return
    }
    const res = await argocdApi.createChangeRequest(changeForm.value)
    const item = res.data?.data
    if (item?.approval_status === 'pending') {
      message.success('变更请求已创建，已进入审批流程，审批通过后将自动合并 MR')
    } else if (item?.status === 'open') {
      message.success('变更请求已创建，并已自动生成 Merge Request')
    } else if (item?.status === 'skipped') {
      message.info(item.error_message || '目标清单无变化，已跳过创建变更请求')
    } else if (item?.status === 'failed') {
      message.warning(`变更请求已记录，但自动创建 MR 失败：${item.error_message || '未知错误'}`)
    } else {
      message.success('变更请求已创建')
    }
    changeModalVisible.value = false
    activeTab.value = 'changes'
    await Promise.all([loadChangeRequests(), reloadOverview()])
    if (item?.id) {
      await openChangeDetail(item.id)
    }
  } finally {
    savingChange.value = false
  }
}

const openResourceModal = async (record: ArgoCDApplication) => {
  resourceModalVisible.value = true
  loadingResources.value = true
  try {
    const res = await argocdApi.getResources(record.id!)
    resources.value = res.data?.data || []
  } finally {
    loadingResources.value = false
  }
}

const handleAppTableChange = (pagination: any) => {
  appPage.value = pagination.current
  appPageSize.value = pagination.pageSize
  loadApps()
}

const handleRepoTableChange = (pagination: any) => {
  repoPage.value = pagination.current
  repoPageSize.value = pagination.pageSize
  loadRepos()
}

const handleChangeTableChange = (pagination: any) => {
  changePage.value = pagination.current
  changePageSize.value = pagination.pageSize
  loadChangeRequests()
}

const goToApprovalInstance = (id: number) => {
  router.push(`/approval/instances/${id}`)
}

const runPrecheck = async () => {
  const res = await argocdApi.precheckChangeRequest(changeForm.value)
  changePrecheck.value = res.data?.data || null
  return res.data?.data
}

const buildPolicyDescription = (policy: ChangeRequestPolicySummary) => {
  const parts = []
  if (policy.require_approval) parts.push('需要审批')
  if (policy.require_chain) parts.push('需要审批链')
  if (policy.require_code_review) parts.push('要求代码评审')
  if (policy.require_test_pass) parts.push('要求测试通过')
  if (policy.require_deploy_window) parts.push('要求发布窗口')
  return parts.length > 0 ? parts.join('，') : '当前环境未启用额外门禁'
}

const openChangeDetail = async (id: number) => {
  loadingChangeDetail.value = true
  changeDetailVisible.value = true
  try {
    const res = await argocdApi.getChangeRequest(id)
    changeDetail.value = res.data?.data || null
    if (res.data?.data) {
      await refreshDetailPrecheck(res.data.data)
    }
    if (route.query.changeId !== String(id) || route.query.tab !== 'changes') {
      router.replace({ query: { ...route.query, tab: 'changes', changeId: String(id) } })
    }
  } finally {
    loadingChangeDetail.value = false
  }
}

const refreshDetailPrecheck = async (detail: GitOpsChangeRequest) => {
  const res = await argocdApi.precheckChangeRequest({
    gitops_repo_id: detail.gitops_repo_id,
    argocd_application_id: detail.argocd_application_id,
    application_name: detail.application_name,
    env: detail.env,
    title: detail.title,
    description: detail.description,
    file_path: detail.file_path,
    image_repository: detail.image_repository,
    image_tag: detail.image_tag,
    target_branch: detail.target_branch,
  })
  changeDetailPrecheck.value = res.data?.data || null
}

const windowOpen = (url: string) => {
  window.open(url, '_blank')
}

const handleTabChange = async (key: string) => {
  if (key === 'overview') {
    await reloadOverview()
  } else if (key === 'apps') {
    await loadInstances()
    if (Date.now() - lastAppsRefreshAt.value > APPS_REFRESH_TTL_MS) {
      await refreshAppStatus()
    }
    await loadApps()
  } else if (key === 'repos') {
    await loadRepos()
  } else if (key === 'instances') {
    await loadInstances()
  } else if (key === 'changes') {
    await Promise.all([loadRepos(), loadChangeRequests()])
  }
}

const syncTabFromRoute = async () => {
  const routeTab = typeof route.query.tab === 'string' ? route.query.tab : ''
  if (routeTab && ['overview', 'apps', 'changes', 'repos', 'instances'].includes(routeTab) && activeTab.value !== routeTab) {
    activeTab.value = routeTab
    await handleTabChange(routeTab)
  }
  const changeId = typeof route.query.changeId === 'string' ? Number(route.query.changeId) : 0
  if (routeTab === 'changes' && changeId) {
    await openChangeDetail(changeId)
  }
}

const syncStatusColor = (status: string) => {
  const map: Record<string, string> = { Synced: 'green', OutOfSync: 'orange', Unknown: 'default' }
  return map[status] || 'default'
}

const healthStatusColor = (status: string) => {
  const map: Record<string, string> = { Healthy: 'green', Degraded: 'red', Progressing: 'blue', Missing: 'orange', Unknown: 'default' }
  return map[status] || 'default'
}

const changeRequestStatusColor = (status: string) => {
  const map: Record<string, string> = { open: 'green', draft: 'blue', failed: 'red', skipped: 'default', submitted: 'orange', merged: 'cyan', rejected: 'red', cancelled: 'default' }
  return map[status] || 'default'
}

const approvalStatusColor = (status: string) => {
  const map: Record<string, string> = { pending: 'orange', approved: 'green', rejected: 'red', cancelled: 'default', failed: 'red', none: 'default' }
  return map[status] || 'default'
}

const autoMergeStatusColor = (status: string) => {
  const map: Record<string, string> = { manual: 'orange', pending: 'blue', success: 'green', failed: 'red', skipped: 'default' }
  return map[status] || 'default'
}

const autoMergeStatusText = (status: string) => {
  const map: Record<string, string> = {
    manual: '待人工合并',
    pending: '待审批后合并',
    success: '已自动合并',
    failed: '自动合并失败',
    skipped: '已跳过',
  }
  return status ? (map[status] || status) : '-'
}

const heroPriorityTitle = computed(() => {
  if (summary.app_out_of_sync > 0 || summary.app_drifted > 0) {
    return `先处理 ${summary.app_out_of_sync + summary.app_drifted} 个同步异常或漂移项`
  }
  if (summary.change_request_open > 0 || summary.change_request_failed > 0) {
    return `先推进 ${summary.change_request_open + summary.change_request_failed} 个待处理变更请求`
  }
  return '当前 GitOps 交付整体稳定'
})

const heroPriorityDescription = computed(() => {
  if (summary.app_out_of_sync > 0 || summary.app_drifted > 0) {
    return '建议先看漂移应用和 OutOfSync 应用，再决定是手动同步还是发起新的 GitOps 变更。'
  }
  if (summary.change_request_open > 0 || summary.change_request_failed > 0) {
    return '当前没有明显同步风险，建议优先清理待审批、失败和未合并的变更请求。'
  }
  return '当同步状态平稳时，优先补齐仓库治理和自动同步覆盖，提升后续交付效率。'
})

const heroSuggestions = computed(() => [
  {
    title: '先看漂移应用',
    description: summary.app_drifted > 0 ? `当前有 ${summary.app_drifted} 个应用检测到 Drift。` : '当前没有 Drift 应用，可以继续关注变更请求。',
    label: '查看应用',
    action: () => {
      activeTab.value = 'apps'
      void handleTabChange('apps')
    },
  },
  {
    title: '推进待处理变更',
    description: summary.change_request_open > 0 ? `当前有 ${summary.change_request_open} 个开放中的变更请求。` : '当前开放中的变更请求不多，可以继续补齐仓库接入。',
    label: '查看变更',
    action: () => {
      activeTab.value = 'changes'
      void handleTabChange('changes')
    },
  },
  {
    title: '补齐仓库治理',
    description: summary.repo_total > 0 ? `当前已接入 ${summary.repo_total} 个 GitOps 仓库，其中自动同步 ${summary.repo_sync_enabled} 个。` : '当前还没有接入 GitOps 仓库，应先完成仓库接入。',
    label: '查看仓库',
    action: () => {
      activeTab.value = 'repos'
      void handleTabChange('repos')
    },
  },
])

onMounted(async () => {
  await Promise.all([loadInstances(), loadRepos(), loadApps(), loadChangeRequests(), reloadOverview()])
  await syncTabFromRoute()
})

watch(() => [route.query.tab, route.query.changeId], async () => {
  await syncTabFromRoute()
})
</script>

<style scoped>
.argocd-page {
  padding: 0;
}

.hero-card {
  margin-bottom: 16px;
}

.hero-title {
  font-size: 22px;
  font-weight: 600;
}

.hero-subtitle {
  margin-top: 8px;
  color: #666;
}

.hero-priority {
  margin-top: 16px;
  padding: 14px 16px;
  border-radius: 12px;
  background: rgba(255, 255, 255, 0.92);
  border: 1px solid #e6efff;
}

.hero-priority-label,
.focus-label {
  color: #8c8c8c;
  font-size: 12px;
}

.hero-priority-title {
  margin-top: 6px;
  font-size: 18px;
  font-weight: 600;
  color: #1f1f1f;
}

.hero-priority-desc {
  margin-top: 6px;
  color: #6b7280;
  line-height: 1.6;
}

.summary-row {
  margin-bottom: 16px;
}

.advice-card {
  height: 100%;
}

.advice-main {
  display: flex;
  flex-direction: column;
  gap: 4px;
}

.advice-title {
  font-weight: 500;
}

.focus-panel {
  height: 100%;
  padding: 16px;
  border-radius: 12px;
  background: #fafafa;
}

.focus-value {
  margin-top: 8px;
  font-size: 26px;
  font-weight: 700;
  color: #1f1f1f;
}

.focus-value.warning {
  color: #d46b08;
}

.focus-value.success {
  color: #52c41a;
}

.summary-extra {
  margin-top: 8px;
  color: #666;
  font-size: 12px;
}

.toolbar {
  margin-bottom: 16px;
  display: flex;
  gap: 12px;
  flex-wrap: wrap;
}

.toolbar-right {
  justify-content: flex-end;
}

.error-text {
  color: #ff4d4f;
}

.ok-text {
  color: #52c41a;
}

.precheck-toolbar {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
}

.precheck-item {
  display: flex;
  justify-content: space-between;
  gap: 12px;
}

.precheck-required {
  margin-left: 8px;
  color: #999;
  font-size: 12px;
}

.precheck-detail {
  color: #666;
  font-size: 12px;
}
</style>
