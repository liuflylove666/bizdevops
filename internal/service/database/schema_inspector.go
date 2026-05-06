package database

import (
	"context"
	"strings"
)

// SchemaInspector 通过 information_schema 查询数据库元数据
type SchemaInspector struct {
	conn *Connector
}

func NewSchemaInspector(conn *Connector) *SchemaInspector {
	return &SchemaInspector{conn: conn}
}

type ColumnInfo struct {
	Name         string `json:"name"`
	DataType     string `json:"data_type"`
	ColumnType   string `json:"column_type"`
	IsNullable   string `json:"is_nullable"`
	ColumnKey    string `json:"column_key"`
	ColumnDefault string `json:"column_default"`
	Extra        string `json:"extra"`
	Comment      string `json:"comment"`
}

type IndexInfo struct {
	Name       string `json:"name"`
	ColumnName string `json:"column_name"`
	NonUnique  int    `json:"non_unique"`
	SeqInIndex int    `json:"seq_in_index"`
}

// Databases 列出所有非系统库，并过滤实例的 exclude_dbs
func (s *SchemaInspector) Databases(ctx context.Context, instanceID uint) ([]string, error) {
	db, inst, err := s.conn.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	var names []string
	if err := db.WithContext(ctx).Raw(
		`SELECT SCHEMA_NAME FROM information_schema.SCHEMATA
		 WHERE SCHEMA_NAME NOT IN ('mysql','information_schema','performance_schema','sys')
		 ORDER BY SCHEMA_NAME`).Scan(&names).Error; err != nil {
		return nil, err
	}
	if inst.ExcludeDBs != "" {
		blocked := map[string]struct{}{}
		for _, n := range strings.Split(inst.ExcludeDBs, ",") {
			blocked[strings.TrimSpace(n)] = struct{}{}
		}
		out := names[:0]
		for _, n := range names {
			if _, skip := blocked[n]; !skip {
				out = append(out, n)
			}
		}
		names = out
	}
	return names, nil
}

// Tables 列出指定库的所有表
func (s *SchemaInspector) Tables(ctx context.Context, instanceID uint, schema string) ([]string, error) {
	db, _, err := s.conn.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	var names []string
	err = db.WithContext(ctx).Raw(
		`SELECT TABLE_NAME FROM information_schema.TABLES
		 WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE' ORDER BY TABLE_NAME`,
		schema,
	).Scan(&names).Error
	return names, err
}

// Columns 列出表的所有列
func (s *SchemaInspector) Columns(ctx context.Context, instanceID uint, schema, table string) ([]ColumnInfo, error) {
	db, _, err := s.conn.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	var cols []ColumnInfo
	err = db.WithContext(ctx).Raw(
		`SELECT COLUMN_NAME AS name, DATA_TYPE AS data_type, COLUMN_TYPE AS column_type,
		        IS_NULLABLE AS is_nullable, COLUMN_KEY AS column_key,
		        IFNULL(COLUMN_DEFAULT,'') AS column_default, EXTRA AS extra,
		        COLUMN_COMMENT AS comment
		 FROM information_schema.COLUMNS
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION`,
		schema, table,
	).Scan(&cols).Error
	return cols, err
}

// Indexes 列出表的所有索引
func (s *SchemaInspector) Indexes(ctx context.Context, instanceID uint, schema, table string) ([]IndexInfo, error) {
	db, _, err := s.conn.Get(ctx, instanceID)
	if err != nil {
		return nil, err
	}
	var idx []IndexInfo
	err = db.WithContext(ctx).Raw(
		`SELECT INDEX_NAME AS name, COLUMN_NAME AS column_name,
		        NON_UNIQUE AS non_unique, SEQ_IN_INDEX AS seq_in_index
		 FROM information_schema.STATISTICS
		 WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX`,
		schema, table,
	).Scan(&idx).Error
	return idx, err
}
