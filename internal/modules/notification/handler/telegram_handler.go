package handler

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/domain/notification/service/telegram"
	"devops/internal/models"
	"devops/internal/repository"
	"devops/pkg/ioc"
	"devops/pkg/logger"
)

func init() {
	ioc.Api.RegisterContainer("TelegramHandler", &TelegramApiHandler{})
}

type TelegramApiHandler struct {
	handler *TelegramHandler
}

func (h *TelegramApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	h.handler = NewTelegramHandler(cfg.GetDB())

	root := cfg.Application.GinRootRouter().Group("telegram")
	h.Register(root)
	return nil
}

func (h *TelegramApiHandler) Register(r gin.IRouter) {
	r.POST("/send-message", h.handler.SendMessage)
	r.GET("/logs", h.handler.ListMessageLogs)

	bot := r.Group("/bot")
	{
		bot.GET("", h.handler.ListBots)
		bot.GET("/:id", h.handler.GetBot)
		bot.POST("", h.handler.CreateBot)
		bot.PUT("/:id", h.handler.UpdateBot)
		bot.DELETE("/:id", h.handler.DeleteBot)
		bot.POST("/:id/default", h.handler.SetDefaultBot)
		bot.POST("/:id/test", h.handler.TestBot)
	}
}

type TelegramHandler struct {
	logger  *logger.Logger
	botRepo *repository.TelegramBotRepository
	logRepo *repository.TelegramMessageLogRepository
	db      *gorm.DB
}

func NewTelegramHandler(db *gorm.DB) *TelegramHandler {
	return &TelegramHandler{
		logger:  logger.NewLogger("INFO"),
		botRepo: repository.NewTelegramBotRepository(db),
		logRepo: repository.NewTelegramMessageLogRepository(db),
		db:      db,
	}
}

// SendMessage 发送 Telegram 消息
func (h *TelegramHandler) SendMessage(c *gin.Context) {
	var req struct {
		BotID                 uint   `json:"bot_id"`
		ChatID                string `json:"chat_id"`
		Content               string `json:"content"`
		ParseMode             string `json:"parse_mode"`
		DisableWebPagePreview bool   `json:"disable_web_page_preview"`
		DisableNotification   bool   `json:"disable_notification"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	var bot *models.TelegramBot
	var err error
	if req.BotID > 0 {
		bot, err = h.botRepo.GetByID(c.Request.Context(), req.BotID)
	} else {
		bot, err = h.botRepo.GetDefault(c.Request.Context())
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "机器人不存在"})
		return
	}

	chatID := req.ChatID
	if chatID == "" {
		chatID = bot.DefaultChatID
	}
	if chatID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "chat_id 不能为空"})
		return
	}

	client := telegram.NewClient(bot.Token, bot.APIBaseURL)
	sendErr := client.SendMessage(c.Request.Context(), &telegram.SendMessageRequest{
		ChatID:                chatID,
		Text:                  req.Content,
		ParseMode:             req.ParseMode,
		DisableWebPagePreview: req.DisableWebPagePreview,
		DisableNotification:   req.DisableNotification,
	})

	logEntry := &models.TelegramMessageLog{
		BotID:     bot.ID,
		ChatID:    chatID,
		ParseMode: req.ParseMode,
		Content:   req.Content,
		Source:    "manual",
		Status:    "success",
	}
	if sendErr != nil {
		logEntry.Status = "failed"
		logEntry.ErrorMsg = sendErr.Error()
	}
	h.logRepo.Create(c.Request.Context(), logEntry)

	if sendErr != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": sendErr.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "发送成功"})
}

// ListMessageLogs 消息日志
func (h *TelegramHandler) ListMessageLogs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	source := c.Query("source")

	list, total, err := h.logRepo.List(c.Request.Context(), page, pageSize, source)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}

func (h *TelegramHandler) ListBots(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "100"))

	list, total, err := h.botRepo.List(c.Request.Context(), page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}

func (h *TelegramHandler) GetBot(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	bot, err := h.botRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": bot})
}

func (h *TelegramHandler) CreateBot(c *gin.Context) {
	var bot models.TelegramBot
	if err := c.ShouldBindJSON(&bot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	bot.ID = 0
	if bot.Status == "" {
		bot.Status = "active"
	}
	if err := h.botRepo.Create(c.Request.Context(), &bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": bot})
}

func (h *TelegramHandler) UpdateBot(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var bot models.TelegramBot
	if err := c.ShouldBindJSON(&bot); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	bot.ID = uint(id)
	if err := h.botRepo.Update(c.Request.Context(), &bot); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": bot})
}

func (h *TelegramHandler) DeleteBot(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.botRepo.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *TelegramHandler) SetDefaultBot(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.botRepo.SetDefault(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

// TestBot 调用 getMe 校验机器人凭据
func (h *TelegramHandler) TestBot(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	bot, err := h.botRepo.GetByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
		return
	}
	info, err := telegram.NewClient(bot.Token, bot.APIBaseURL).GetMe(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": info})
}
