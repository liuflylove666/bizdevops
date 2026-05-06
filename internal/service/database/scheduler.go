package database

import (
	"context"
	"time"

	"gorm.io/gorm"

	"devops/internal/domain/database/model"
	"devops/pkg/logger"
)

// TicketScheduler 定时扫描到期工单并执行
type TicketScheduler struct {
	db      *gorm.DB
	ticketS *TicketService
	stop    chan struct{}
}

func NewTicketScheduler(db *gorm.DB, ticketS *TicketService) *TicketScheduler {
	return &TicketScheduler{db: db, ticketS: ticketS, stop: make(chan struct{})}
}

// Start 启动后台协程（每分钟扫描一次）。调用方通过 Stop() 关闭。
func (s *TicketScheduler) Start() {
	go s.loop()
}

func (s *TicketScheduler) Stop() { close(s.stop) }

func (s *TicketScheduler) loop() {
	log := logger.L().WithField("service", "sql-scheduler")
	log.Info("SQL 工单调度器启动")
	t := time.NewTicker(1 * time.Minute)
	defer t.Stop()
	// 启动即扫一次
	s.tick(context.Background())
	for {
		select {
		case <-s.stop:
			return
		case <-t.C:
			s.tick(context.Background())
		}
	}
}

func (s *TicketScheduler) tick(ctx context.Context) {
	log := logger.L().WithField("service", "sql-scheduler")
	var due []model.SQLChangeTicket
	err := s.db.WithContext(ctx).
		Where("status = ? AND delay_mode = ? AND execute_time IS NOT NULL AND execute_time <= ?",
			model.TicketStatusReady, "schedule", time.Now()).
		Find(&due).Error
	if err != nil {
		log.WithError(err).Warn("扫描待执行工单失败")
		return
	}
	for _, t := range due {
		log.WithField("ticket_id", t.ID).Info("触发定时执行")
		if err := s.ticketS.Execute(ctx, t.ID, "scheduler"); err != nil {
			log.WithError(err).WithField("ticket_id", t.ID).Warn("定时执行失败")
		}
	}
}
