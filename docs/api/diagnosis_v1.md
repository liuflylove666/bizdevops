# Diagnosis API v1（失败诊断，去 AI 化）

- **状态**: Locked at Sprint 1 D1（2026-04-29）
- **BasePath**: `/app/api/v1`
- **Auth**: `Authorization: Bearer <jwt>`
- **关联**: ADR-0008（Pipeline IR）、Sprint 1 工程清单

> **契约一旦 lock，本 Sprint 不再变更**。增字段需另开 PR + 评审；改字段需 RFC。

## 1 · 设计原则

1. **数据驱动，零 AI**：所有字段都来自既有数据查表 / 计算，无 LLM 推理
2. **缺字段填 `null` 或空数组，禁填猜测值**：保信任
3. **失败签名计算失败时**响应退化为仅 `status` + `log_tail`，前端展示"无法识别失败签名"占位
4. **P50 延迟目标 < 2s**（仅查表 + 数 join）

## 2 · 端点

### 2.1 GET /pipeline/runs/:id/diagnosis

获取一次失败 run 的结构化诊断。

**Path 参数**

| 字段 | 类型 | 说明 |
|---|---|---|
| `id` | int64 | `pipeline_runs.id` |

**Response（成功）**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "run_id": 33119,
    "pipeline_id": 217,
    "status": "failed",

    "failure_signature": "sig_a1b2c3d4e5f6",
    "signature_first_seen_at": "2026-04-15T03:21:08Z",
    "signature_occurrences": 7,
    "signature_distinct_commits": 4,

    "is_flaky": false,
    "flaky_reason": null,

    "last_success": {
      "run_id": 33102,
      "commit": "a1b2c3d",
      "happened_at": "2026-04-27T08:45:11Z",
      "diff_url": "https://git.example.com/org/repo/compare/a1b2c3d...e5f6g7h"
    },

    "changed_files": [
      { "path": "internal/service/foo.go", "additions": 12, "deletions": 3 },
      { "path": "go.sum", "additions": 1, "deletions": 1 }
    ],

    "similar_runs": [
      {
        "run_id": 32801,
        "happened_at": "2026-04-22T10:11:00Z",
        "fixed_by_commit": "x9y8z7w",
        "fix_diff_url": "https://git.example.com/org/repo/commit/x9y8z7w"
      }
    ],

    "fix_references": [
      {
        "kind": "jira_issue",
        "key": "ORD-882",
        "title": "Code Review SLA 优化",
        "url": "https://jira.example.com/browse/ORD-882"
      },
      {
        "kind": "postmortem",
        "id": "pm_4421",
        "title": "2026-04-15 Order 服务发布失败复盘",
        "url": "/observability/postmortems/pm_4421"
      },
      {
        "kind": "improvement_item",
        "id": "imp_4421",
        "title": "Code Review SLA 优化",
        "status": "observing"
      }
    ],

    "log_tail": [
      { "ts": "2026-04-29T02:11:42.812Z", "stream": "stderr", "line": "FAIL\tdevops/pkg/foo\t0.421s" },
      { "ts": "2026-04-29T02:11:42.901Z", "stream": "stderr", "line": "exit status 1" }
    ]
  }
}
```

**Response（签名计算失败的退化形态）**

```json
{
  "code": 0,
  "data": {
    "run_id": 33119,
    "status": "failed",
    "failure_signature": null,
    "log_tail": [ /* ... */ ]
  }
}
```

> 前端在 `failure_signature == null` 时**仅渲染** `log_tail`，并在卡片头标注"无法识别失败签名"，其余字段隐藏。

## 3 · 字段语义（关键三字段）

| 字段 | 来源 | 计算 |
|---|---|---|
| `failure_signature` | 日志归一化 → SHA1 截前 12 位前缀 `sig_` | 仅取末 N 行（默认 50），去时间戳 / 路径 / PID / 端口 / UUID / 行号 |
| `is_flaky` | `failure_signatures` 表 + run 重试关系 | 同一 commit 后续重试转绿 → `same_commit_retry_succeeded`；同签名近 7d 跨 ≥3 个不同 commit → `cross_commit_recurrence` |
| `similar_runs` | 按 `signature_id` join `pipeline_run_failures` | 取近 30d，最多 3 条；按时间倒序；优先返回**已被某 commit 修复**的 run |

### `flaky_reason` 枚举

| 值 | 含义 |
|---|---|
| `null` | 不是 Flaky |
| `same_commit_retry_succeeded` | 同 commit 重试转绿 |
| `cross_commit_recurrence` | 同签名近 7d 跨 ≥3 个不同 commit |

### `fix_references.kind` 枚举

| 值 | 来源表 |
|---|---|
| `jira_issue` | `service/jira/` 按签名匹配的工单 |
| `postmortem` | 复盘库（Confluence / 内部表） |
| `improvement_item` | DORA 闭环 PRD 的 `improvement_items` 表（如已落地） |

## 4 · 错误码（沿用 `pkg/errors`）

| HTTP | code | 含义 |
|---|---|---|
| 400 | `INVALID_PARAM` | run_id 非数字 |
| 401 | `UNAUTHORIZED` | JWT 失效 |
| 403 | `FORBIDDEN` | 无权访问该 pipeline |
| 404 | `RUN_NOT_FOUND` | run 不存在 |
| 409 | `RUN_NOT_FAILED` | run 状态非 failed/cancelled，不允许诊断 |

## 5 · 性能目标（Sprint 1 D8 必验）

| 指标 | 目标 | 备注 |
|---|---|---|
| P50 延迟 | < 2s | 仅查表，无外网调用 |
| P95 延迟 | < 5s | 含 `similar_runs` 与 `fix_references` 的 join |
| 失败签名一致性 | ≥ 95% | 100 个真实失败 case 上人工校验 |

## 6 · 鉴权

| 端点 | 普通用户 | admin |
|---|---|---|
| GET /pipeline/runs/:id/diagnosis | ✓（仅自己有权限的 pipeline） | ✓ |

## 7 · 与既有约定的兼容

| 项 | 落地 |
|---|---|
| 路由注册 | IOC `init() → ioc.Api.RegisterContainer → Init()` |
| DB 句柄 | `repository.GetDB(ctx)` |
| Audit | 此端点为查询，不写 audit |
| 通知 | 不涉及（推送在 S4 BE-25） |
| AI | **本契约不含任何 AI 字段**，不预留 |

## 8 · 不在 V1 内（明确）

| 项 | 进入 |
|---|---|
| `ai_explanation` / `ai_suggestions` | AI 解禁后开 v2 |
| 修复路径自动跳转 | S2 BE-09 |
| 重跑预检 | S2 BE-10（独立端点） |
| Blast Radius | S3 BE-16（独立端点） |
