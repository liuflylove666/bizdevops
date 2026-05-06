package database

import (
	"context"
	"regexp"
	"strings"

	"devops/internal/domain/database/model"
)

type AuditLevel string

const (
	LevelInfo    AuditLevel = "info"
	LevelWarning AuditLevel = "warning"
	LevelError   AuditLevel = "error"
)

type StatementKind string

const (
	KindSelect  StatementKind = "SELECT"
	KindInsert  StatementKind = "INSERT"
	KindUpdate  StatementKind = "UPDATE"
	KindDelete  StatementKind = "DELETE"
	KindDDL     StatementKind = "DDL"
	KindOther   StatementKind = "OTHER"
)

// AuditFinding 单条语句的一项审核结论
type AuditFinding struct {
	Seq     int        `json:"seq"`     // 语句序号，从 1 开始
	Rule    string     `json:"rule"`    // 规则名
	Level   AuditLevel `json:"level"`
	Message string     `json:"message"`
}

// AuditStatementResult 单条语句的聚合结果
type AuditStatementResult struct {
	Seq      int            `json:"seq"`
	SQL      string         `json:"sql"`
	Kind     StatementKind  `json:"kind"`
	Findings []AuditFinding `json:"findings"`
}

// AuditReport 整个工单的审核报告
type AuditReport struct {
	Statements []AuditStatementResult `json:"statements"`
	HasError   bool                   `json:"has_error"`
	HasWarning bool                   `json:"has_warning"`
	Summary    string                 `json:"summary"`
}

// AuditOptions 审核时的上下文
type AuditOptions struct {
	Schema       string
	AllowDrop    bool // 允许 DROP TABLE / DROP DATABASE
	AllowTrunc   bool // 允许 TRUNCATE
	MaxStatement int  // 单工单最大语句数，<=0 表示不限
}

// Auditor 可插拔的审核器接口。未来可替换为 YearningEngineAuditor。
type Auditor interface {
	Audit(ctx context.Context, statements []string, opts AuditOptions) AuditReport
}

// BuiltinAuditor 内置规则集，config 可由外部规则集覆盖
type BuiltinAuditor struct {
	cfg model.AuditRuleConfig
}

func NewBuiltinAuditor() *BuiltinAuditor {
	return &BuiltinAuditor{cfg: model.DefaultAuditRuleConfig()}
}

// NewBuiltinAuditorWith 使用自定义配置构造
func NewBuiltinAuditorWith(cfg model.AuditRuleConfig) *BuiltinAuditor {
	return &BuiltinAuditor{cfg: cfg}
}

func flag(p *bool, def bool) bool {
	if p == nil {
		return def
	}
	return *p
}

var (
	reWhere       = regexp.MustCompile(`(?i)\bwhere\b`)
	reLimit       = regexp.MustCompile(`(?i)\blimit\b`)
	reSelectStar  = regexp.MustCompile(`(?i)select\s+\*`)
	reDropTable   = regexp.MustCompile(`(?i)^\s*drop\s+(table|database|schema)\b`)
	reTruncate    = regexp.MustCompile(`(?i)^\s*truncate\b`)
	reRename      = regexp.MustCompile(`(?i)^\s*rename\s+table\b`)
	reAlterDrop   = regexp.MustCompile(`(?i)^\s*alter\s+table\s+\S+\s+drop\s+`)
	reCreateTable = regexp.MustCompile(`(?i)^\s*create\s+table\b`)
	reInsertNoCol = regexp.MustCompile(`(?i)^\s*insert\s+into\s+\S+\s*values\b`)
	reIdent       = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
)

func (a *BuiltinAuditor) Audit(ctx context.Context, statements []string, opts AuditOptions) AuditReport {
	report := AuditReport{Statements: make([]AuditStatementResult, 0, len(statements))}

	maxStmt := opts.MaxStatement
	if maxStmt <= 0 {
		maxStmt = a.cfg.MaxStatementPerTicket
	}
	if maxStmt > 0 && len(statements) > maxStmt {
		report.HasError = true
		report.Summary = "语句数量超出上限"
	}

	for i, s := range statements {
		res := AuditStatementResult{Seq: i + 1, SQL: s, Kind: classify(s)}
		res.Findings = append(res.Findings, a.runRules(res.Kind, s, opts)...)
		for _, f := range res.Findings {
			switch f.Level {
			case LevelError:
				report.HasError = true
			case LevelWarning:
				report.HasWarning = true
			}
		}
		report.Statements = append(report.Statements, res)
	}

	if report.Summary == "" {
		switch {
		case report.HasError:
			report.Summary = "存在不允许执行的高危语句"
		case report.HasWarning:
			report.Summary = "通过但存在风险提示"
		default:
			report.Summary = "通过"
		}
	}
	return report
}

func classify(sql string) StatementKind {
	t := strings.ToUpper(strings.TrimSpace(sql))
	switch {
	case strings.HasPrefix(t, "SELECT"), strings.HasPrefix(t, "WITH"):
		return KindSelect
	case strings.HasPrefix(t, "INSERT"), strings.HasPrefix(t, "REPLACE"):
		return KindInsert
	case strings.HasPrefix(t, "UPDATE"):
		return KindUpdate
	case strings.HasPrefix(t, "DELETE"):
		return KindDelete
	case strings.HasPrefix(t, "CREATE"),
		strings.HasPrefix(t, "ALTER"),
		strings.HasPrefix(t, "DROP"),
		strings.HasPrefix(t, "RENAME"),
		strings.HasPrefix(t, "TRUNCATE"):
		return KindDDL
	}
	return KindOther
}

func (a *BuiltinAuditor) runRules(kind StatementKind, sql string, opts AuditOptions) []AuditFinding {
	var out []AuditFinding
	add := func(rule string, lvl AuditLevel, msg string) {
		out = append(out, AuditFinding{Rule: rule, Level: lvl, Message: msg})
	}

	if (kind == KindUpdate || kind == KindDelete) && flag(a.cfg.RequireWhere, true) && !reWhere.MatchString(sql) {
		add("require_where", LevelError, string(kind)+" 语句必须包含 WHERE 条件")
	}
	if (kind == KindUpdate || kind == KindDelete) && flag(a.cfg.SuggestDMLLimit, true) && !reLimit.MatchString(sql) {
		add("suggest_limit", LevelWarning, string(kind)+" 建议加上 LIMIT 以限制影响行数")
	}
	if flag(a.cfg.NoDrop, true) && reDropTable.MatchString(sql) && !opts.AllowDrop {
		add("no_drop", LevelError, "禁止 DROP 表 / 库操作")
	}
	if flag(a.cfg.NoTruncate, true) && reTruncate.MatchString(sql) && !opts.AllowTrunc {
		add("no_truncate", LevelError, "禁止 TRUNCATE 操作")
	}
	if flag(a.cfg.RenameTable, true) && reRename.MatchString(sql) {
		add("rename_table", LevelWarning, "RENAME TABLE 可能影响上下游，请确认")
	}
	if flag(a.cfg.AlterDrop, true) && reAlterDrop.MatchString(sql) {
		add("alter_drop", LevelWarning, "ALTER 语句包含 DROP 子句，删除列/索引会导致数据丢失")
	}
	if kind == KindSelect && flag(a.cfg.SelectStar, true) && reSelectStar.MatchString(sql) {
		add("select_star", LevelWarning, "SELECT * 可能返回不必要的列，建议指定字段")
	}
	if kind == KindSelect && flag(a.cfg.SelectLimit, true) && !reLimit.MatchString(sql) {
		add("select_limit", LevelInfo, "SELECT 未使用 LIMIT，可能返回大量数据")
	}
	if kind == KindInsert && flag(a.cfg.InsertColumns, true) && reInsertNoCol.MatchString(sql) {
		add("insert_columns", LevelWarning, "INSERT 未指定列名，表结构变更后易出错")
	}
	if reCreateTable.MatchString(sql) {
		low := strings.ToLower(sql)
		if flag(a.cfg.CreateEngine, true) && !strings.Contains(low, "engine=") {
			add("create_engine", LevelWarning, "CREATE TABLE 建议显式指定 ENGINE=InnoDB")
		}
		if flag(a.cfg.CreateCharset, true) && !strings.Contains(low, "charset=") && !strings.Contains(low, "character set") {
			add("create_charset", LevelWarning, "CREATE TABLE 建议显式指定字符集 utf8mb4")
		}
		if flag(a.cfg.CreatePrimaryKey, true) && !strings.Contains(low, "primary key") {
			add("create_pk", LevelWarning, "CREATE TABLE 建议包含主键")
		}
	}
	maxBytes := a.cfg.MaxStatementBytes
	if maxBytes <= 0 {
		maxBytes = 100 * 1024
	}
	if len(sql) > maxBytes {
		add("too_long", LevelError, "单条 SQL 过长，请拆分")
	}
	up := strings.ToUpper(strings.TrimSpace(sql))
	if flag(a.cfg.NoLockTables, true) && (strings.HasPrefix(up, "LOCK TABLES") || strings.HasPrefix(up, "UNLOCK TABLES")) {
		add("no_lock_tables", LevelError, "禁止使用 LOCK/UNLOCK TABLES")
	}
	if flag(a.cfg.NoSetGlobal, true) && strings.HasPrefix(up, "SET GLOBAL") {
		add("no_set_global", LevelError, "禁止修改全局变量")
	}
	if flag(a.cfg.NoGrant, true) && (strings.HasPrefix(up, "GRANT ") || strings.HasPrefix(up, "REVOKE ")) {
		add("no_grant", LevelError, "禁止在工单中执行授权语句")
	}
	if (kind == KindUpdate || kind == KindDelete) && flag(a.cfg.TautologyWhere, true) {
		low := strings.ToLower(sql)
		if strings.Contains(low, "1=1") || strings.Contains(low, "1 = 1") {
			add("tautology_where", LevelError, "检测到恒真条件 1=1，等同于全表操作")
		}
	}
	return out
}

// 防止 reIdent 在未来扩展前被 unused 检查误报
var _ = reIdent
