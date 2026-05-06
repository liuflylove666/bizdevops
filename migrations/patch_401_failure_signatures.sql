-- patch_401_failure_signatures.sql
-- CI/CD 转型 Sprint 1 / BE-01: 失败签名归一化结果表 + Run ↔ 签名关联表
--
-- 用于「失败诊断卡」（去 AI 化）：log normalize → SHA1 → 跨 run 聚合为签名。
-- 上线后由 service/pipeline/diagnosis_service.go 在 run 完结时写入。
--
-- 兼容：使用 IF NOT EXISTS，重复执行幂等。
-- 不进 AutoMigrate 白名单；仅通过本补丁建表。

CREATE TABLE IF NOT EXISTS failure_signatures (
    id                  BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    created_at          DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at          DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    signature           CHAR(40)         NOT NULL COMMENT 'SHA1(归一化日志末 N 行) 全长 hex；前端用 sig_<前12位> 展示',
    normalized_sample   TEXT             COMMENT '签名所对应的归一化样本（保留首次出现时的内容，便于排查归一化规则）',

    first_seen_run_id   BIGINT UNSIGNED  NOT NULL COMMENT '首次命中该签名的 run_id',
    first_seen_at       DATETIME         NOT NULL COMMENT '首次出现时间',
    last_seen_run_id    BIGINT UNSIGNED  NOT NULL COMMENT '最近一次命中的 run_id',
    last_seen_at        DATETIME         NOT NULL COMMENT '最近一次出现时间',

    occurrences         INT UNSIGNED     NOT NULL DEFAULT 1 COMMENT '累计命中次数',
    distinct_commits    INT UNSIGNED     NOT NULL DEFAULT 1 COMMENT '不同 commit 数（Flaky 跨 commit 判定阈值）',

    PRIMARY KEY (id),
    UNIQUE KEY uk_signature (signature),
    KEY idx_last_seen_at (last_seen_at),
    KEY idx_first_seen_at (first_seen_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='CI/CD 失败签名归一化结果（Sprint 1, 去 AI 化失败诊断）';


CREATE TABLE IF NOT EXISTS pipeline_run_failures (
    run_id              BIGINT UNSIGNED  NOT NULL COMMENT 'pipeline_runs.id；一个 run 至多一条失败签名',
    signature_id        BIGINT UNSIGNED  NOT NULL COMMENT 'failure_signatures.id',
    pipeline_id         BIGINT UNSIGNED  NOT NULL DEFAULT 0 COMMENT 'pipeline_runs.pipeline_id 冗余，便于按 pipeline 聚合',

    commit_sha          VARCHAR(40)      NOT NULL DEFAULT '' COMMENT 'run 关联 commit；用于 Flaky 同 commit 重试判定',
    is_flaky_retry      TINYINT(1)       NOT NULL DEFAULT 0 COMMENT '同 commit 后续重试转绿后回填为 1',
    fixed_by_commit     VARCHAR(40)      NOT NULL DEFAULT '' COMMENT '该签名被哪个 commit 修复（修复 commit 上首次出现该签名的下一个成功 run 关联 commit）',

    log_tail            TEXT             COMMENT '失败时末 N 行原始日志（脱敏后）',

    created_at          DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,

    PRIMARY KEY (run_id),
    KEY idx_signature_id (signature_id),
    KEY idx_pipeline_id (pipeline_id),
    KEY idx_commit_sha (commit_sha),
    KEY idx_signature_pipeline (signature_id, pipeline_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Run ↔ 失败签名 关联（一 run 至多一签名）';
