# Sprint 2 API 契约 v1（CI/CD 转型 W3-W4）

- **状态**: Locked at Sprint 2 D1（2026-05-04）
- **BasePath**: `/app/api/v1`
- **Auth**: `Authorization: Bearer <jwt>`
- **关联**: ADR-0008（Pipeline IR）、Sprint 2 工程清单
- **去 AI 化原则**：所有字段都来自既有数据查表 / 计算，无 LLM 推理；缺字段填 `null` 或空数组，禁填猜测值

> **契约一旦 lock，本 Sprint 不再变更**。增字段需另开 PR + 评审；改字段需 RFC。

## 端点总览

| Method | Path | 用途 | 任务 ID |
|---|---|---|---|
| GET | `/pipeline/runs/:id/fix-references` | 失败诊断的"修复参考"链接 | BE-09 |
| GET | `/pipeline/runs/:id/log/tail` | 末 N 行日志（列表页 hover 用） | BE-13 |
| POST | `/pipeline/runs/:id/rerun-dryrun` | 重跑前的副作用预检 | BE-10 |
| GET | `/pipeline/:id/yaml` | IR → YAML 导出 | BE-12 |

---

## 1 · GET /pipeline/runs/:id/fix-references （BE-09）

获取一次失败 run 的"修复参考"链接（Jira / 复盘 / 改进项）。

**Path 参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int64 | `pipeline_runs.id` |

**Response（成功）**

```json
{
  "code": 0,
  "data": {
    "run_id": 33119,
    "failure_signature": "sig_a1b2c3d4e5f6",
    "matched_by": "signature",
    "items": [
      {
        "kind": "jira_issue",
        "key": "ORD-882",
        "title": "Code Review SLA 优化",
        "url": "https://jira.example.com/browse/ORD-882",
        "status": "in_progress",
        "matched_at": "2026-05-04T10:21:00Z"
      },
      {
        "kind": "postmortem",
        "id": "pm_4421",
        "title": "2026-04-15 Order 服务发布失败复盘",
        "url": "/observability/postmortems/pm_4421",
        "matched_at": "2026-05-04T10:21:00Z"
      },
      {
        "kind": "improvement_item",
        "id": "imp_4421",
        "title": "Code Review SLA 优化",
        "status": "observing",
        "url": "/observability/improvements/imp_4421",
        "matched_at": "2026-05-04T10:21:00Z"
      }
    ]
  }
}
```

**Response（无失败签名 / 未命中任何参考）**

```json
{
  "code": 0,
  "data": {
    "run_id": 33119,
    "failure_signature": null,
    "matched_by": null,
    "items": []
  }
}
```

### 字段语义

| 字段 | 来源 | 备注 |
|---|---|---|
| `failure_signature` | `pipeline_run_failures.signature_id` → `failure_signatures.signature` 的短形 | 无失败签名时为 `null` |
| `matched_by` | 枚举：`signature` / `null` | V1 仅按签名匹配；其他匹配键留 V2 |
| `items[].kind` | 枚举：`jira_issue` / `postmortem` / `improvement_item` | 与 diagnosis_v1.md 一致 |
| `items[].matched_at` | 该参考被关联到该签名的时间 | 用于灰度时排查为什么这条链接被推荐 |

### 不变量
- **无失败签名 → `items=[]`**，禁止 fallback 关键词搜索（避免误关联）
- 单 run 返回参考数 ≤ 10，按 `matched_at DESC` 排序
- `kind` 枚举新增需同步 diagnosis_v1.md + 前端 fixRefColor/fixRefLabel 映射

### 错误码

| HTTP | code | 含义 |
|---|---|---|
| 400 | `INVALID_PARAM` | run_id 非数字 |
| 404 | `RUN_NOT_FOUND` | run 不存在 |

---

## 2 · GET /pipeline/runs/:id/log/tail （BE-13）

获取一次 run 的末尾 N 行日志，用于列表页 hover 预览。

**Path / Query**

| 字段 | 类型 | 必填 | 默认 | 备注 |
|---|---|---|---|---|
| `id` | int64 | ✓ | — | `pipeline_runs.id` |
| `n` | int | | `50` | 范围 `[1, 500]`，超出则截断到 500 |

**Response**

```json
{
  "code": 0,
  "data": {
    "run_id": 33119,
    "status": "failed",
    "lines_total": 50,
    "lines_truncated": false,
    "lines": [
      { "ts": "2026-05-04T02:11:42.812Z", "stream": "stderr", "line": "FAIL\tdevops/pkg/foo\t0.421s" },
      { "ts": "2026-05-04T02:11:42.901Z", "stream": "stderr", "line": "exit status 1" }
    ]
  }
}
```

### 字段语义

| 字段 | 来源 | 备注 |
|---|---|---|
| `lines_total` | 实际返回的行数 | 可能 < n（run 日志不够长）|
| `lines_truncated` | run 日志总行数 > n 时为 `true` | 让前端显示"还有更多"提示 |
| `lines[].ts` | 优先取自日志元数据；无则空字符串 | 与 diagnosis 退化形态一致 |
| `lines[].stream` | 枚举：`stdout` / `stderr` / `""` | 来自 step_runs / log_service |

### 不变量
- run 没有日志（如 pending）→ `lines=[]`、`lines_total=0`、`lines_truncated=false`
- **不返回完整日志**：本端点仅供 hover 预览，完整日志走既有 GetStepLogs

### 错误码

| HTTP | code | 含义 |
|---|---|---|
| 400 | `INVALID_PARAM` | run_id 非数字 / n 超范围（含 0、负、>500）|
| 404 | `RUN_NOT_FOUND` | run 不存在 |

---

## 3 · POST /pipeline/runs/:id/rerun-dryrun （BE-10）

重跑前的副作用预检：列出"会复用什么、会动什么远端、会跑什么缓存"。

**Path / Body**

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` (path) | int64 | `pipeline_runs.id`（要重跑的 run）|
| `from_stage` (body) | string | 可选；从此阶段开始重跑（V1 仅 metadata，实际重跑由既有 retry 走）|

**Body**
```json
{ "from_stage": "" }
```

**Response**

```json
{
  "code": 0,
  "data": {
    "run_id": 33119,
    "pipeline_id": 217,
    "would_rerun_from": "build",
    "estimated_duration_seconds": 240,

    "artifact_reuse": [
      {
        "kind": "build_cache",
        "key": "go-mod-{{ checksum go.sum }}",
        "size_bytes": 45000000,
        "last_used_at": "2026-05-03T14:11:08Z"
      }
    ],

    "remote_side_effects": [
      {
        "kind": "registry_push",
        "target": "registry.example.com/order-svc:v1.42.4",
        "step_name": "docker-push",
        "irreversible": true
      },
      {
        "kind": "argocd_sync",
        "target": "order-svc-staging",
        "step_name": "deploy",
        "irreversible": false
      }
    ],

    "cache_hits": [
      { "key": "go-mod-checksum", "would_hit": true, "size_bytes": 45000000 }
    ],

    "warnings": [
      "镜像 v1.42.4 已存在，重跑将覆盖（registry policy 允许 overwrite）"
    ]
  }
}
```

### 字段语义

| 字段 | 来源 | 备注 |
|---|---|---|
| `would_rerun_from` | 默认从 `from_stage` 或失败 stage | 与既有 retry 行为一致 |
| `estimated_duration_seconds` | 历史同 pipeline 平均时长 | 无历史时返回 `null` |
| `artifact_reuse[]` | 命中的 BuildCache 行 | 用既有 cache_service.Match() 接口 |
| `remote_side_effects[]` | 静态扫描 step 类型识别 | V1 识别 4 类：`registry_push` / `git_push` / `argocd_sync` / `db_migration` |
| `remote_side_effects[].irreversible` | 推 registry / db_migration 为 true；ArgoCD sync 为 false（可回滚） | 前端用红色标 |
| `cache_hits[]` | 与 artifact_reuse 部分重叠但聚焦 step 内显式声明的 cache | — |
| `warnings[]` | 辅助提示串数组 | 不强类型 |

### 不变量
- 完全基于**静态分析**，不实际启动 dry-run 容器（V1 性价比够用）
- 没识别出的 step 类型不入 `remote_side_effects`（"已知未列入 = 假设无副作用"是错的，前端需提示"V1 仅识别 4 类副作用"）
- `irreversible=true` 的项前端必须显式确认（双确认）

### 错误码

| HTTP | code | 含义 |
|---|---|---|
| 400 | `INVALID_PARAM` | run_id 非数字 |
| 404 | `RUN_NOT_FOUND` | run 不存在 |
| 409 | `RUN_IN_PROGRESS` | run 仍在跑，不允许预检（避免误判） |

---

## 4 · GET /pipeline/:id/yaml （BE-12）

将流水线（DB → IR → YAML）导出为 YAML 文本。

**Path / Query**

| 字段 | 类型 | 必填 | 默认 | 备注 |
|---|---|---|---|---|
| `id` (path) | int64 | ✓ | — | `pipelines.id` |
| `include_layout` | bool | | `false` | 是否在 YAML 中保留设计器画布坐标（`__layout`）|

**Response**

```yaml
# 成功（Content-Type: text/yaml; charset=utf-8）
version: "1.0"
name: order-svc-build
trigger:
  branches: [main, release/*]
  events: [push]
variables:
  - name: GO_VERSION
    value: "1.25"
cache:
  key: go-mod-{{ checksum go.sum }}
  paths:
    - ~/.cache/go-build
stages:
  - name: build
    steps:
      - name: compile
        image: golang:1.25
        commands:
          - go build ./...
  - name: test
    needs: [build]
    steps:
      - name: go-test
        image: golang:1.25
        commands:
          - go test ./...
```

**Response（include_layout=true 时）**

`version` 等字段同上，末尾追加：

```yaml
__layout:
  nodes:
    - node_id: stage:build
      x: 100
      y: 50
    - node_id: step:build:compile
      x: 200
      y: 100
```

### 关键约束

| 项 | 行为 |
|---|---|
| Content-Type | `text/yaml; charset=utf-8` |
| Status 200 | 成功，body 是 raw YAML 文本（不包 `{code, message, data}` 信封）|
| Status 错误 | 走标准 JSON 信封（沿用 `pkg/response`）|
| 文件名提示 | `Content-Disposition: inline; filename="<pipeline_name>.yaml"` |
| Layout 字段名 | 必须 `__layout`（与 IR `Pipeline.Layout` json tag 一致）|
| Round-trip 一致性 | YAML → IR → YAML 必须字段一致（含顺序）；BE-14 守门 |

### 错误码

| HTTP | code | 含义 |
|---|---|---|
| 400 | `INVALID_PARAM` | id 非数字 |
| 404 | `PIPELINE_NOT_FOUND` | pipeline 不存在 |
| 500 | `IR_BUILD_FAILED` | DB → IR 转换失败（数据非法）|
| 500 | `YAML_MARSHAL_FAILED` | IR → YAML 序列化失败（IR 字段错误）|

### V1 限制

- **导出，不含导入**（YAML 导入 + 设计器还原是 S3 BE-18）
- 不带签名 / 不带版本注释（V2 `?with_meta=true` 再加）
- 大流水线（> 100 step）目前未做分页 / 流式（DBS 时再考虑）

---

## 鉴权矩阵（沿用 RBAC）

| 端点 | 普通用户 | admin |
|---|---|---|
| GET /fix-references | ✓（自己 scope） | ✓ |
| GET /log/tail | ✓（自己 scope） | ✓ |
| POST /rerun-dryrun | ✓（执行权限范围内） | ✓ |
| GET /yaml | ✓（pipeline 读权限） | ✓ |

## 与既有约定的兼容

| 项 | 落地 |
|---|---|
| 路由注册 | IOC `init() → Init()` 模式 |
| DB 句柄 | `repository.GetDB(ctx)` |
| Audit | 全部为读 / 静态分析，不写 audit |
| 通知 | 不涉及 |
| 加密 | 凭证字段不出现在 YAML（仅 `from_cred` 引用名）|
| AI | **本契约不含任何 AI 字段**，不预留 |

## 不在 V1 内（明确）

| 项 | 进入 |
|---|---|
| `ai_explanation` / 任何生成式字段 | AI 解禁后 v2 |
| 修复参考关键词 fallback 匹配 | V2 |
| YAML 导入 + 设计器还原 | S3 BE-18 |
| 重跑预检"实际跑容器" dry-run | DBS（不准备做）|
| 全文日志 / 流式日志 | 走既有 GetStepLogs |
