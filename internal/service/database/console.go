package database

import (
	"context"
	"errors"
	"regexp"
	"strings"
	"time"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

const DefaultQueryLimit = 1000

var (
	// 允许的只读语句起始词
	allowedStmts = map[string]struct{}{
		"SELECT":   {},
		"SHOW":     {},
		"DESC":     {},
		"DESCRIBE": {},
		"EXPLAIN":  {},
	}
	stmtHead        = regexp.MustCompile(`^\s*([A-Za-z]+)`)
	limitDetector   = regexp.MustCompile(`(?i)\blimit\s+\d+`)
	multiStmtSep    = regexp.MustCompile(`;\s*\S`)
)

// Console SQL 查询控制台（只读）
type Console struct {
	conn    *Connector
	logRepo *dbrepo.DBQueryLogRepository
}

func NewConsole(conn *Connector, logRepo *dbrepo.DBQueryLogRepository) *Console {
	return &Console{conn: conn, logRepo: logRepo}
}

type QueryRequest struct {
	InstanceID uint   `json:"instance_id"`
	Schema     string `json:"schema"`
	SQL        string `json:"sql"`
	Limit      int    `json:"limit"`
}

type QueryResult struct {
	Columns    []string         `json:"columns"`
	Rows       []map[string]any `json:"rows"`
	AffectRows int              `json:"affect_rows"`
	ExecMs     int              `json:"exec_ms"`
}

func (c *Console) Execute(ctx context.Context, user string, req *QueryRequest) (*QueryResult, error) {
	sqlStripped := strings.TrimSpace(req.SQL)
	sqlStripped = strings.TrimRight(sqlStripped, ";")
	if sqlStripped == "" {
		return nil, errors.New("SQL 不能为空")
	}
	if multiStmtSep.MatchString(sqlStripped) {
		c.writeLog(ctx, user, req, 0, 0, "blocked", "仅支持单条语句")
		return nil, errors.New("仅支持单条语句")
	}
	head := strings.ToUpper(stmtHead.FindStringSubmatch(sqlStripped + " ")[1])
	if _, ok := allowedStmts[head]; !ok {
		c.writeLog(ctx, user, req, 0, 0, "blocked", "仅允许 SELECT/SHOW/DESC/EXPLAIN 等只读语句")
		return nil, errors.New("仅允许 SELECT/SHOW/DESC/EXPLAIN 等只读语句")
	}

	limit := req.Limit
	if limit <= 0 || limit > DefaultQueryLimit {
		limit = DefaultQueryLimit
	}
	execSQL := sqlStripped
	if head == "SELECT" && !limitDetector.MatchString(sqlStripped) {
		execSQL = sqlStripped + " LIMIT " + itoa(limit)
	}

	db, _, err := c.conn.Get(ctx, req.InstanceID)
	if err != nil {
		c.writeLog(ctx, user, req, 0, 0, "failed", err.Error())
		return nil, err
	}
	if req.Schema != "" {
		if err := db.WithContext(ctx).Exec("USE `" + sanitizeIdent(req.Schema) + "`").Error; err != nil {
			c.writeLog(ctx, user, req, 0, 0, "failed", err.Error())
			return nil, err
		}
	}

	start := time.Now()
	rows, err := db.WithContext(ctx).Raw(execSQL).Rows()
	if err != nil {
		c.writeLog(ctx, user, req, 0, int(time.Since(start).Milliseconds()), "failed", err.Error())
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	result := &QueryResult{Columns: cols, Rows: []map[string]any{}}
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, col := range cols {
			v := vals[i]
			if b, ok := v.([]byte); ok {
				v = string(b)
			}
			m[col] = v
		}
		result.Rows = append(result.Rows, m)
	}
	result.AffectRows = len(result.Rows)
	result.ExecMs = int(time.Since(start).Milliseconds())

	c.writeLog(ctx, user, req, result.AffectRows, result.ExecMs, "success", "")
	return result, nil
}

func (c *Console) writeLog(ctx context.Context, user string, req *QueryRequest, affect, ms int, status, errMsg string) {
	if c.logRepo == nil {
		return
	}
	_ = c.logRepo.Create(ctx, &model.DBQueryLog{
		InstanceID: req.InstanceID,
		Username:   user,
		SchemaName: req.Schema,
		SQLText:    req.SQL,
		AffectRows: affect,
		ExecMs:     ms,
		Status:     status,
		ErrorMsg:   errMsg,
	})
}

var identRe = regexp.MustCompile(`[^A-Za-z0-9_]+`)

func sanitizeIdent(s string) string { return identRe.ReplaceAllString(s, "") }

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	buf := [20]byte{}
	i := len(buf)
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}
