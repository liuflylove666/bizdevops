-- patch_201_incidents.sql
-- v2.1 / Sprint 4: 生产事故（Incident）持久化表，用于 DORA MTTR 真实数据源
--
-- 与 alert_event 区别：
--   alert_event 是事件流（瞬时），incidents 是 OnCall 团队认定的事故（持久化、可关联发布与复盘）

CREATE TABLE IF NOT EXISTS incidents (
    id                BIGINT UNSIGNED  NOT NULL AUTO_INCREMENT,
    title             VARCHAR(200)     NOT NULL,
    description       TEXT             NULL,
    application_id    BIGINT UNSIGNED  NULL,
    application_name  VARCHAR(100)     NULL,
    env               VARCHAR(30)      NOT NULL DEFAULT 'prod',

    severity          VARCHAR(10)      NOT NULL DEFAULT 'P2',
    status            VARCHAR(20)      NOT NULL DEFAULT 'open',

    detected_at       DATETIME         NOT NULL,
    mitigated_at      DATETIME         NULL,
    resolved_at       DATETIME         NULL,

    source            VARCHAR(30)      NOT NULL DEFAULT 'manual',
    release_id        BIGINT UNSIGNED  NULL,
    alert_fingerprint VARCHAR(100)     NULL,

    postmortem_url    VARCHAR(500)     NULL,
    root_cause        TEXT             NULL,

    created_by        BIGINT UNSIGNED  NULL,
    created_by_name   VARCHAR(100)     NULL,
    resolved_by       BIGINT UNSIGNED  NULL,
    resolved_by_name  VARCHAR(100)     NULL,

    created_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME         NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,

    PRIMARY KEY (id),
    INDEX idx_incident_app (application_id),
    INDEX idx_incident_app_name (application_name),
    INDEX idx_incident_env (env),
    INDEX idx_incident_severity (severity),
    INDEX idx_incident_status (status),
    INDEX idx_incident_detected (detected_at),
    INDEX idx_incident_resolved (resolved_at),
    INDEX idx_incident_release (release_id),
    INDEX idx_incident_fingerprint (alert_fingerprint)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='生产事故 (DORA MTTR 数据源)';
