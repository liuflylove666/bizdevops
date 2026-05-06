package database

import (
	"context"
	"fmt"
	"net/rpc"
	"strings"
	"sync"
	"time"
)

// EngineCheckArgs 与 Yearning Engine.Check 约定的入参。
// 字段命名保持与 Yearning engine 包一致。
type EngineCheckArgs struct {
	SQL      string
	Schema   string
	IP       string
	Username string
	Port     int
	Password string
	CA       string
	Cert     string
	Key      string
	Kind     int
	Lang     string
	Rule     map[string]any // Yearning AuditRole 原始字段；规则集 config 透传
}

// EngineRecord 与 Yearning Engine.Record 对应的单条返回。
type EngineRecord struct {
	ID          int    `json:"id"`
	SQL         string `json:"sql"`
	SQLSource   string `json:"sql_source"`
	Level       int    `json:"level"` // 0 info / 1 warn / 2 error
	AffectedRow int    `json:"affected_row"`
	Result      string `json:"result"`
	Type        int    `json:"type"`
	Backup      int    `json:"backup"`
}

// EngineAuditorConfig 启用配置
type EngineAuditorConfig struct {
	Addr     string        // host:port，空则不启用
	Timeout  time.Duration // 单次 RPC 超时；<=0 默认 30s
	Fallback Auditor       // RPC 不可用时回退
}

// EngineRPCAuditor 适配 Yearning Engine.Check 的 Auditor 实现
type EngineRPCAuditor struct {
	cfg EngineAuditorConfig
	mu  sync.Mutex
	cli *rpc.Client
}

func NewEngineRPCAuditor(cfg EngineAuditorConfig) *EngineRPCAuditor {
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	return &EngineRPCAuditor{cfg: cfg}
}

func (a *EngineRPCAuditor) Audit(ctx context.Context, statements []string, opts AuditOptions) AuditReport {
	if a.cfg.Addr == "" {
		return a.fallback(ctx, statements, opts)
	}
	cli, err := a.client()
	if err != nil {
		return a.fallback(ctx, statements, opts)
	}

	args := EngineCheckArgs{
		SQL:    strings.Join(statements, ";\n") + ";",
		Schema: opts.Schema,
		Kind:   0,
	}
	var rs []EngineRecord
	done := make(chan error, 1)
	go func() { done <- cli.Call("Engine.Check", args, &rs) }()

	select {
	case err := <-done:
		if err != nil {
			a.reset()
			return a.fallback(ctx, statements, opts)
		}
	case <-time.After(a.cfg.Timeout):
		a.reset()
		return a.fallback(ctx, statements, opts)
	case <-ctx.Done():
		return a.fallback(ctx, statements, opts)
	}

	return buildReportFromEngine(statements, rs)
}

func (a *EngineRPCAuditor) fallback(ctx context.Context, statements []string, opts AuditOptions) AuditReport {
	if a.cfg.Fallback != nil {
		return a.cfg.Fallback.Audit(ctx, statements, opts)
	}
	return AuditReport{Summary: "Engine RPC 不可用且未配置回退"}
}

func (a *EngineRPCAuditor) client() (*rpc.Client, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cli != nil {
		return a.cli, nil
	}
	c, err := rpc.DialHTTP("tcp", a.cfg.Addr)
	if err != nil {
		return nil, fmt.Errorf("连接 engine 失败: %w", err)
	}
	a.cli = c
	return c, nil
}

func (a *EngineRPCAuditor) reset() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.cli != nil {
		_ = a.cli.Close()
		a.cli = nil
	}
}

func buildReportFromEngine(statements []string, rs []EngineRecord) AuditReport {
	report := AuditReport{Statements: make([]AuditStatementResult, 0, len(statements))}
	for i, s := range statements {
		report.Statements = append(report.Statements, AuditStatementResult{
			Seq: i + 1, SQL: s, Kind: classify(s),
		})
	}
	for _, r := range rs {
		if r.ID < 1 || r.ID > len(report.Statements) {
			continue
		}
		idx := r.ID - 1
		level := LevelInfo
		switch r.Level {
		case 1:
			level = LevelWarning
			report.HasWarning = true
		case 2:
			level = LevelError
			report.HasError = true
		}
		report.Statements[idx].Findings = append(report.Statements[idx].Findings, AuditFinding{
			Seq:     r.ID,
			Rule:    "engine",
			Level:   level,
			Message: r.Result,
		})
	}
	switch {
	case report.HasError:
		report.Summary = "Engine 审核：存在错误"
	case report.HasWarning:
		report.Summary = "Engine 审核：存在告警"
	default:
		report.Summary = "Engine 审核通过"
	}
	return report
}
