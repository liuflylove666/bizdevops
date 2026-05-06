package repository

import (
	"context"

	"gorm.io/gorm"

	"devops/internal/domain/notification/model"
)

// TelegramBotRepository Telegram 机器人仓储
type TelegramBotRepository struct {
	db *gorm.DB
}

func NewTelegramBotRepository(db *gorm.DB) *TelegramBotRepository {
	return &TelegramBotRepository{db: db}
}

func (r *TelegramBotRepository) Create(ctx context.Context, bot *model.TelegramBot) error {
	return r.db.WithContext(ctx).Create(bot).Error
}

func (r *TelegramBotRepository) GetByID(ctx context.Context, id uint) (*model.TelegramBot, error) {
	var bot model.TelegramBot
	if err := r.db.WithContext(ctx).First(&bot, id).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) List(ctx context.Context, page, pageSize int) ([]model.TelegramBot, int64, error) {
	var list []model.TelegramBot
	var total int64

	query := r.db.WithContext(ctx).Model(&model.TelegramBot{})
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}

func (r *TelegramBotRepository) Update(ctx context.Context, bot *model.TelegramBot) error {
	return r.db.WithContext(ctx).Model(bot).Where("id = ?", bot.ID).Updates(map[string]interface{}{
		"name":            bot.Name,
		"token":           bot.Token,
		"default_chat_id": bot.DefaultChatID,
		"api_base_url":    bot.APIBaseURL,
		"description":     bot.Description,
		"status":          bot.Status,
		"is_default":      bot.IsDefault,
	}).Error
}

func (r *TelegramBotRepository) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(&model.TelegramBot{}, id).Error
}

func (r *TelegramBotRepository) GetDefault(ctx context.Context) (*model.TelegramBot, error) {
	var bot model.TelegramBot
	if err := r.db.WithContext(ctx).Where("is_default = ? AND status = ?", true, "active").First(&bot).Error; err != nil {
		return nil, err
	}
	return &bot, nil
}

func (r *TelegramBotRepository) SetDefault(ctx context.Context, id uint) error {
	if err := r.db.WithContext(ctx).Model(&model.TelegramBot{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
		return err
	}
	return r.db.WithContext(ctx).Model(&model.TelegramBot{}).Where("id = ?", id).Update("is_default", true).Error
}

// TelegramMessageLogRepository Telegram 消息日志仓储
type TelegramMessageLogRepository struct {
	db *gorm.DB
}

func NewTelegramMessageLogRepository(db *gorm.DB) *TelegramMessageLogRepository {
	return &TelegramMessageLogRepository{db: db}
}

func (r *TelegramMessageLogRepository) Create(ctx context.Context, log *model.TelegramMessageLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *TelegramMessageLogRepository) List(ctx context.Context, page, pageSize int, source string) ([]model.TelegramMessageLog, int64, error) {
	var list []model.TelegramMessageLog
	var total int64

	query := r.db.WithContext(ctx).Model(&model.TelegramMessageLog{})
	if source != "" {
		query = query.Where("source = ?", source)
	}
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	offset := (page - 1) * pageSize
	if err := query.Offset(offset).Limit(pageSize).Order("created_at DESC").Find(&list).Error; err != nil {
		return nil, 0, err
	}
	return list, total, nil
}
