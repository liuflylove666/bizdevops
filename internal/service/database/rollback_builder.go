package database

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

// RollbackBuilder 为支持的 DML 预生成反向 SQL 并落库
type RollbackBuilder struct {
	repo *dbrepo.SQLRollbackRepository
}

func NewRollbackBuilder(repo *dbrepo.SQLRollbackRepository) *RollbackBuilder {
	return &RollbackBuilder{repo: repo}
}

var (
	reUpdateHead = regexp.MustCompile(`(?is)^\s*update\s+([^\s,()]+)\s+set\s+.+?\s+where\s+(.+)$`)
	reDeleteHead = regexp.MustCompile(`(?is)^\s*delete\s+from\s+([^\s,()]+)\s+(?:where\s+(.+))?$`)
	reLimitTail  = regexp.MustCompile(`(?i)\s+limit\s+\d+\s*$`)
)

// BuildForTicket 为工单的 UPDATE/DELETE 语句生成反向脚本。
// 失败单条不致命，只记录到对应 statement 的 rollback_sql 空。
func (b *RollbackBuilder) BuildForTicket(
	ctx context.Context,
	db *gorm.DB,
	schema string,
	ticket *model.SQLChangeTicket,
	stmts []model.SQLChangeStatement,
) {
	for _, s := range stmts {
		rollback, err := b.build(ctx, db, schema, s.SQLText)
		if err != nil || rollback == "" {
			continue
		}
		_ = b.repo.Create(ctx, &model.SQLRollbackScript{
			TicketID:    ticket.ID,
			WorkID:      ticket.WorkID,
			StatementID: s.ID,
			RollbackSQL: rollback,
		})
	}
}

func (b *RollbackBuilder) build(ctx context.Context, db *gorm.DB, schema, sqlText string) (string, error) {
	txt := strings.TrimSuffix(strings.TrimSpace(sqlText), ";")
	kind := classify(txt)
	switch kind {
	case KindUpdate, KindDelete:
		return b.buildDML(ctx, db, schema, txt, kind)
	}
	return "", nil
}

func (b *RollbackBuilder) buildDML(ctx context.Context, db *gorm.DB, schema, sqlText string, kind StatementKind) (string, error) {
	var table, where string
	if kind == KindUpdate {
		m := reUpdateHead.FindStringSubmatch(sqlText)
		if len(m) < 3 {
			return "", fmt.Errorf("UPDATE 无法解析")
		}
		table = trimIdent(m[1])
		where = m[2]
	} else {
		m := reDeleteHead.FindStringSubmatch(sqlText)
		if len(m) < 2 {
			return "", fmt.Errorf("DELETE 无法解析")
		}
		table = trimIdent(m[1])
		if len(m) >= 3 {
			where = m[2]
		}
	}
	if where == "" {
		return "", fmt.Errorf("缺少 WHERE 条件")
	}
	where = reLimitTail.ReplaceAllString(where, "")
	fqTable := qualify(schema, table)

	rows, err := snapshot(ctx, db, fqTable, where)
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", nil
	}

	if kind == KindUpdate {
		return buildReplaceInto(fqTable, rows), nil
	}
	return buildInsertInto(fqTable, rows), nil
}

func snapshot(ctx context.Context, db *gorm.DB, fqTable, where string) ([]map[string]any, error) {
	raw := fmt.Sprintf("SELECT * FROM %s WHERE %s LIMIT 10000", fqTable, where)
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	r, err := sqlDB.QueryContext(ctx, raw)
	if err != nil {
		return nil, err
	}
	defer r.Close()

	cols, err := r.Columns()
	if err != nil {
		return nil, err
	}
	var out []map[string]any
	for r.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := r.Scan(ptrs...); err != nil {
			return nil, err
		}
		m := make(map[string]any, len(cols))
		for i, c := range cols {
			m[c] = vals[i]
		}
		out = append(out, m)
	}
	return out, nil
}

func buildReplaceInto(fqTable string, rows []map[string]any) string {
	if len(rows) == 0 {
		return ""
	}
	cols := sortedKeys(rows[0])
	var b strings.Builder
	b.WriteString(fmt.Sprintf("REPLACE INTO %s (%s) VALUES\n", fqTable, joinIdents(cols)))
	for i, row := range rows {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  (")
		for j, c := range cols {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderSQLValue(row[c]))
		}
		b.WriteString(")")
	}
	b.WriteString(";\n")
	return b.String()
}

func buildInsertInto(fqTable string, rows []map[string]any) string {
	if len(rows) == 0 {
		return ""
	}
	cols := sortedKeys(rows[0])
	var b strings.Builder
	b.WriteString(fmt.Sprintf("INSERT INTO %s (%s) VALUES\n", fqTable, joinIdents(cols)))
	for i, row := range rows {
		if i > 0 {
			b.WriteString(",\n")
		}
		b.WriteString("  (")
		for j, c := range cols {
			if j > 0 {
				b.WriteString(", ")
			}
			b.WriteString(renderSQLValue(row[c]))
		}
		b.WriteString(")")
	}
	b.WriteString(";\n")
	return b.String()
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// 简单稳定排序
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}

func joinIdents(cols []string) string {
	out := make([]string, len(cols))
	for i, c := range cols {
		out[i] = "`" + strings.ReplaceAll(c, "`", "") + "`"
	}
	return strings.Join(out, ", ")
}

func renderSQLValue(v any) string {
	switch x := v.(type) {
	case nil:
		return "NULL"
	case []byte:
		return quoteString(string(x))
	case string:
		return quoteString(x)
	case bool:
		if x {
			return "1"
		}
		return "0"
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprintf("%v", x)
	case sql.RawBytes:
		return quoteString(string(x))
	}
	return quoteString(fmt.Sprintf("%v", v))
}

func quoteString(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `'`, `\'`)
	return "'" + s + "'"
}

func trimIdent(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "`")
	return s
}

func qualify(schema, table string) string {
	t := "`" + strings.ReplaceAll(table, "`", "") + "`"
	if schema == "" || strings.Contains(table, ".") {
		return t
	}
	return "`" + strings.ReplaceAll(schema, "`", "") + "`." + t
}
