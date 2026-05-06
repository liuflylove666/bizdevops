<template>
  <a-modal
    v-model:open="visible"
    :footer="null"
    :mask-closable="false"
    :closable="true"
    width="720px"
    class="v2-welcome-tour"
    @cancel="close"
  >
    <div class="tour-header">
      <span class="badge">NEW</span>
      <h2>欢迎使用 V2 信息架构</h2>
      <p class="subtitle">7 大领域一屏可达；以下是本次升级你最该知道的变化。</p>
    </div>

    <a-carousel
      ref="carouselRef"
      :dots="true"
      :autoplay="false"
      :after-change="(i: number) => (step = i)"
    >
      <div v-for="(s, idx) in steps" :key="idx" class="tour-slide">
        <div class="tour-icon">
          <component :is="s.icon" />
        </div>
        <h3>{{ s.title }}</h3>
        <p class="desc">{{ s.desc }}</p>
        <ul class="bullets">
          <li v-for="b in s.bullets" :key="b">{{ b }}</li>
        </ul>
      </div>
    </a-carousel>

    <a-divider style="margin: 16px 0" />

    <div class="tour-footer">
      <a-button v-if="step > 0" @click="prev">上一步</a-button>
      <a-space>
        <a-checkbox v-model:checked="neverShow">不再显示</a-checkbox>
        <a-button v-if="step < steps.length - 1" type="primary" @click="next">下一步</a-button>
        <a-button v-else type="primary" @click="finish">开始使用</a-button>
      </a-space>
    </div>
  </a-modal>
</template>

<script setup lang="ts">
/**
 * V2WelcomeTour —— V2 菜单首次访问引导浮层（v2.1）。
 *
 * 逻辑：
 *   - V2 菜单固定启用时，若 localStorage 内无 `v2.tour.dismissed` 则显示
 *   - 用户点击"开始使用"或勾选"不再显示"后写入 dismissed 标志
 *   - Reset 方法可由设置页或 bug 排查时调用
 */
import { ref, computed, onMounted } from 'vue'
import {
  RocketOutlined,
  NodeIndexOutlined,
  BarChartOutlined,
  SafetyOutlined,
  DashboardOutlined,
} from '@ant-design/icons-vue'

const STORAGE_KEY = 'v2.tour.dismissed'

const visible = ref(false)
const step = ref(0)
const neverShow = ref(false)
const carouselRef = ref<any>(null)

const steps = computed(() => [
  {
    icon: RocketOutlined,
    title: '发布主单（Release v2）',
    desc: '一次业务迭代 = 一张 Release；子项（流水线 / Nacos / DB 工单）自动归集，串行审批。',
    bullets: [
      '新菜单：发布与变更 → 发布主单',
      '命中规则雷达图：一眼看清风险构成',
      'GitOps PR 一键生成（审批通过后可见）',
    ],
  },
  {
    icon: SafetyOutlined,
    title: '风险评分与审批',
    desc: '每张 Release 在提交审批时自动计算风险分 0-100，高分自动走更严格审批路径。',
    bullets: [
      '7 条默认规则：生产直发 / 数据库变更 / 非工作时间 …',
      '高分触发升级审批',
      '详情页支持命中规则追溯',
    ],
  },
  {
    icon: NodeIndexOutlined,
    title: '环境与流量一屏可达',
    desc: '应用详情新增「运行态」Tab，实例、Service、Ingress、HPA 不再跳页。',
    bullets: [
      '新菜单：应用与环境 → 实例总览',
      '告警与事故直接下钻到应用',
      'Ingress / Service 权重调整原地完成',
    ],
  },
  {
    icon: BarChartOutlined,
    title: 'DORA 指标上工作台',
    desc: '首页默认展示 4 个 DORA 指标 + 环比，MTTR 接入真实事故数据。',
    bullets: [
      '部署频次 / 变更前置 / 变更失败率 / MTTR',
      '环比箭头：一眼识别恶化趋势',
      'DORA 2024 行业基准自动比对',
    ],
  },
  {
    icon: DashboardOutlined,
    title: '旧菜单仍可用',
    desc: '过渡期仍可通过旧路径访问常用页面，系统会自动重定向到统一入口。',
    bullets: [
      '双入口共存过渡期：旧链接可继续打开',
      '不会遗失已有流水线/任务',
      '权限与角色完全沿用',
    ],
  },
])

function next() {
  carouselRef.value?.next()
}
function prev() {
  carouselRef.value?.prev()
}
function close() {
  if (neverShow.value) localStorage.setItem(STORAGE_KEY, '1')
  visible.value = false
}
function finish() {
  localStorage.setItem(STORAGE_KEY, '1')
  visible.value = false
}

/**
 * 首次进入时按"是否曾经 dismiss"判断是否显示。
 */
function check() {
  if (localStorage.getItem(STORAGE_KEY) === '1') return
  visible.value = true
}

onMounted(() => {
  check()
})

defineExpose({
  /** 供"设置 → 重新查看引导"调用 */
  reset() {
    localStorage.removeItem(STORAGE_KEY)
    neverShow.value = false
    step.value = 0
    visible.value = true
  },
})
</script>

<style scoped>
.v2-welcome-tour :deep(.ant-modal-body) {
  padding: 24px 28px;
}
.tour-header {
  text-align: center;
  margin-bottom: 12px;
}
.tour-header .badge {
  display: inline-block;
  background: linear-gradient(90deg, #722ed1, #1677ff);
  color: #fff;
  font-size: 12px;
  font-weight: 600;
  padding: 2px 8px;
  border-radius: 10px;
  letter-spacing: 0.5px;
}
.tour-header h2 {
  margin: 8px 0 4px;
  font-size: 22px;
}
.tour-header .subtitle {
  color: #666;
  margin: 0;
}
.tour-slide {
  text-align: center;
  padding: 16px 24px 40px;
}
.tour-icon {
  font-size: 44px;
  color: #1677ff;
  margin-bottom: 12px;
}
.tour-slide h3 {
  font-size: 18px;
  margin: 8px 0 4px;
}
.tour-slide .desc {
  color: #666;
  margin: 0 auto 12px;
  max-width: 520px;
}
.tour-slide .bullets {
  list-style: none;
  padding: 0;
  margin: 0 auto;
  max-width: 520px;
  text-align: left;
}
.tour-slide .bullets li {
  padding: 4px 0 4px 20px;
  position: relative;
  color: #333;
}
.tour-slide .bullets li::before {
  content: '✓';
  color: #52c41a;
  position: absolute;
  left: 0;
  font-weight: 700;
}
.tour-footer {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
:deep(.slick-dots li button) {
  background: #1677ff !important;
}
</style>
