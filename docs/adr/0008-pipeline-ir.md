# ADR-0008: Pipeline IR — 设计器 / YAML / DB 三向中间表示

- **状态**: Superseded (2026-05-04：流水线设计器功能已下线，CI/CD 收口到列表、模板、代码仓库、制品和构建治理)
- **日期**: 2026-04-29
- **决策人**: 产品总监 + Tech Lead
- **涉及 Epic**: CI/CD 转型「可代码化」支柱

## 背景

CI/CD 转型方案要把流水线从「设计器→DB」单向写入升级为「设计器 ↔ YAML ↔ Git」双向可代码化（Pipeline as Code）。当前痛点：

- 已下线的画布式设计器曾直接产出 DB 行格式，无 YAML 导出
- `pkg/dto/PipelineYAMLConfig` 只用于 YAML 解析输入，不承担画布布局
- 设计器节点位置（`x/y/w/h`）信息一旦丢失，YAML 导入后画布无法还原

如果不引入显式中间层，三种表示之间需要 `3×2 = 6` 路转换函数（DB↔Designer、DB↔YAML、Designer↔YAML），耦合面大。

## 决策

引入一个**单一的、内存中的、不依赖 GORM 的** `PipelineIR` 类型，作为所有形态之间的**唯一中间表示**。

```
┌──────────┐                ┌────┐                ┌──────────┐
│ Designer │ ◀──to/from──▶ │ IR │ ◀──to/from──▶ │   YAML   │
└──────────┘                │    │                └──────────┘
                            │    │ ◀──to/from──▶ ┌──────────┐
                            └────┘                │   DB     │
                                                  └──────────┘
```

- IR 是 Go 内存类型，**无任何 ORM/HTTP 依赖**，便于单测、快照、签名
- IR 携带可选的 `Layout` 字段：YAML 导出时按需省略（`emit_layout=false`），保 round-trip 时保留
- 节点 ID 采用稳定字符串（`stage:<name>` / `step:<stage>:<name>`），不引入数据库自增 ID 到 IR 内
- 转换函数集中在 `internal/service/pipeline/ir/` 包，每方向一组（`FromYAML/ToYAML`、`FromDB/ToDB`、`FromDesigner/ToDesigner`），共 **6** 个但**复杂度都退化为 IR ↔ X 一步**，不再交叉耦合
- 校验（`Validate`）只针对 IR，不再分别在 DTO/DB 各做一遍

## 范围（V1，Sprint 1 启动）

**纳入**：
- `Pipeline / Stage / Step / Trigger / Variable / Cache / Matrix / Layout` 8 类节点
- `Validate(ir *Pipeline) error` 基础校验
- 单测覆盖：合法/各类非法 pipeline、Layout 可选、stage `needs` 引用完整性

**Sprint 2-3 推进**：
- `ToYAML / FromYAML`（Sprint 2）
- `FromDesigner / ToDesigner`（Sprint 3，X6 ↔ IR）
- `FromDB / ToDB`（Sprint 4 配合 Templates as Code）

**非目标**：
- 不引入新的 YAML 方言；导出/导入与现有 `dto.PipelineYAMLConfig` 字段同名同义，IR 仅在其上加 `Layout`
- 不替代 `service/pipeline/config_parser.go`：后者在 V2 起改为「YAML → IR」的薄包装

## 关键不变量（必须由 `Validate` 保证）

1. `Pipeline.Name` 非空
2. `Pipeline.Stages` 至少 1 个
3. Stage 名称在 pipeline 内唯一
4. Step 名称在 stage 内唯一
5. `Stage.Needs[i]` 必须指向**先于本 stage 出现**的某个 stage（DAG 拓扑保证）
6. `Step.Image` 非空
7. `Cache` 给出时 `Key` 和 `Paths` 不可同时为空
8. `Layout` 给出时，每个 `NodeID` 在 IR 中能找到对应节点（容忍多余 layout 节点，但若引用不存在节点则报错）

## 后果

**好处**：
- 任一表示新增字段，只需扩 IR + 改对应转换函数，其他两路不变
- 失败诊断、PR Diff、Lint 等下游能力都基于 IR，与表示形态解耦
- 单测可不起 DB 不起 HTTP，毫秒级跑

**代价**：
- 新增一层概念，需文档与培训
- 转换函数初期重复写 3 套；2-4 周内一次性投入
- IR 的字段命名一旦稳定**不可随意改**，否则 YAML 兼容性破坏

## 与既有约定的对齐

| 既有约定 | 落地方式 |
|---|---|
| ADR-0001 ArgoCD 唯一 CD | IR 不涉及 CD，仅 CI |
| ADR-0003 Argo Rollouts 替自建 | IR 不再保留自建 canary/bluegreen 节点类型 |
| ADR-0005 通知仅 Telegram | Trigger 中的 notify 字段仅允许 `telegram` 渠道（在 Validate 中限制） |
| AES-GCM 包级密钥 | IR 内的 `Variable.FromCred` 只存凭证名引用，不存密文 |
| AutoMigrate 白名单 | 相关 DB 表（`failure_signatures` 等）走 `migrations/patch_NNN_*.sql`，不进 AutoMigrate |
| 不本地 `go build` | IR 单测在 docker compose 容器内跑（`docker compose exec devops go test ./internal/service/pipeline/ir/...`）|

## 当前不依赖 AI

本 RFC 与"失败诊断 / 修复参考 / 复盘起草"等下游能力均**不依赖 AI**：失败签名靠日志 normalize + SHA1，相似查询靠表 join，复盘靠模板填充。AI 解禁后再行扩展。
