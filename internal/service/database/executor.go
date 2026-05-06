package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
)

// Executor 负责把工单语句按顺序打到目标实例
type Executor struct {
	connector *Connector
	ticketR   *dbrepo.SQLChangeTicketRepository
	stmtR     *dbrepo.SQLChangeStatementRepository
	rollback  *RollbackBuilder
	ghost     *GhostExecutor
}

func NewExecutor(
	connector *Connector,
	ticketR *dbrepo.SQLChangeTicketRepository,
	stmtR *dbrepo.SQLChangeStatementRepository,
	rollback *RollbackBuilder,
) *Executor {
	return &Executor{connector: connector, ticketR: ticketR, stmtR: stmtR, rollback: rollback}
}

// SetGhost 注入可选的 gh-ost 执行器
func (e *Executor) SetGhost(g *GhostExecutor) { e.ghost = g }

// Execute 同步执行工单全部语句。失败即中断，返回 error。
func (e *Executor) Execute(ctx context.Context, ticketID uint) error {
	ticket, err := e.ticketR.GetByID(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("工单不存在: %w", err)
	}
	if ticket.Status != model.TicketStatusReady {
		return fmt.Errorf("工单状态 %d 不允许执行", ticket.Status)
	}
	stmts, err := e.stmtR.ListByTicket(ctx, ticketID)
	if err != nil {
		return fmt.Errorf("加载语句失败: %w", err)
	}
	if len(stmts) == 0 {
		return fmt.Errorf("工单无可执行语句")
	}

	db, _, err := e.connector.Get(ctx, ticket.InstanceID)
	if err != nil {
		return err
	}

	// 标记运行中
	if err := e.ticketR.UpdateFields(ctx, ticket.ID, map[string]any{
		"status": model.TicketStatusRunning,
	}); err != nil {
		return err
	}

	// 预生成回滚脚本（失败不影响执行）
	if e.rollback != nil && ticket.NeedBackup {
		e.rollback.BuildForTicket(ctx, db, ticket.SchemaName, ticket, stmts)
	}

	var execErr error
	for i := range stmts {
		s := stmts[i]
		if s.State == "success" {
			continue // 断点续跑
		}
		if e.ghost != nil && e.ghost.CanHandle(s.SQLText) {
			inst, pwd, perr := e.connector.ResolvePlainPassword(ctx, ticket.InstanceID)
			if perr != nil {
				execErr = e.markFailed(ctx, &s, time.Now(), perr)
				break
			}
			_ = e.stmtR.UpdateFields(ctx, s.ID, map[string]any{"state": "running"})
			if err := e.ghost.Run(ctx, inst, pwd, ticket.SchemaName, &s); err != nil {
				execErr = e.markFailed(ctx, &s, time.Now(), err)
				break
			}
			continue
		}
		if err := e.runOne(ctx, db, ticket.SchemaName, &s); err != nil {
			execErr = err
			break
		}
	}

	finalStatus := model.TicketStatusSucceeded
	if execErr != nil {
		finalStatus = model.TicketStatusFailed
	}
	_ = e.ticketR.UpdateFields(ctx, ticket.ID, map[string]any{"status": finalStatus})
	return execErr
}

func (e *Executor) runOne(ctx context.Context, db *gorm.DB, schema string, s *model.SQLChangeStatement) error {
	now := time.Now()
	_ = e.stmtR.UpdateFields(ctx, s.ID, map[string]any{"state": "running"})

	conn, err := db.DB()
	if err != nil {
		return e.markFailed(ctx, s, now, err)
	}
	sqlConn, err := conn.Conn(ctx)
	if err != nil {
		return e.markFailed(ctx, s, now, err)
	}
	defer sqlConn.Close()

	if schema != "" {
		if _, err := sqlConn.ExecContext(ctx, "USE `"+strings.ReplaceAll(schema, "`", "")+"`"); err != nil {
			return e.markFailed(ctx, s, now, err)
		}
	}

	res, err := sqlConn.ExecContext(ctx, s.SQLText)
	elapsed := int(time.Since(now) / time.Millisecond)
	if err != nil {
		executedAt := time.Now()
		_ = e.stmtR.UpdateFields(ctx, s.ID, map[string]any{
			"state":       "failed",
			"error_msg":   err.Error(),
			"exec_ms":     elapsed,
			"executed_at": executedAt,
		})
		return fmt.Errorf("第 %d 条失败: %w", s.Seq, err)
	}
	affected := int64(0)
	if res != nil {
		if a, e2 := res.RowsAffected(); e2 == nil {
			affected = a
		}
	}
	executedAt := time.Now()
	return e.stmtR.UpdateFields(ctx, s.ID, map[string]any{
		"state":       "success",
		"affect_rows": affected,
		"exec_ms":     elapsed,
		"executed_at": executedAt,
	})
}

func (e *Executor) markFailed(ctx context.Context, s *model.SQLChangeStatement, start time.Time, err error) error {
	elapsed := int(time.Since(start) / time.Millisecond)
	executedAt := time.Now()
	_ = e.stmtR.UpdateFields(ctx, s.ID, map[string]any{
		"state":       "failed",
		"error_msg":   err.Error(),
		"exec_ms":     elapsed,
		"executed_at": executedAt,
	})
	return fmt.Errorf("第 %d 条失败: %w", s.Seq, err)
}
