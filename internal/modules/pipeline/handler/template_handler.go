// Package handler 流水线模块处理器
package handler

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/pipeline"
	pipelinesvc "devops/internal/service/pipeline"
	"devops/pkg/dto"
)

// TemplateHandler 流水线模板处理器
type TemplateHandler struct {
	db *gorm.DB
}

type templateMutationRequest struct {
	Name             string `json:"name"`
	Slug             string `json:"slug"`
	Description      string `json:"description"`
	Category         string `json:"category"`
	Language         string `json:"language"`
	Framework        string `json:"framework"`
	ConfigJSON       any    `json:"config_json"`
	IsPublic         bool   `json:"is_public"`
	SourcePipelineID uint   `json:"source_pipeline_id"`
}

// NewTemplateHandler 创建模板处理器
func NewTemplateHandler(db *gorm.DB) *TemplateHandler {
	return &TemplateHandler{db: db}
}

// RegisterRoutes 注册路由
func (h *TemplateHandler) RegisterRoutes(r *gin.RouterGroup) {
	g := r.Group("/pipeline/templates")
	{
		// 固定路径必须在 /:id 之前，否则 categories、tags、favorites 等会被当成 :id
		g.GET("", h.ListTemplates)
		g.GET("/categories", h.GetCategories)
		g.GET("/tags", h.GetTags)
		g.GET("/favorites", h.GetFavorites)
		g.GET("/stages", h.ListStageTemplates)
		g.GET("/steps", h.ListStepTemplates)

		g.POST("", h.CreateTemplate)

		g.POST("/:id/use", h.UseTemplate)
		g.POST("/:id/apply", h.ApplyTemplate)
		g.POST("/:id/rate", h.RateTemplate)
		g.POST("/:id/favorite", h.AddFavorite)
		g.DELETE("/:id/favorite", h.RemoveFavorite)

		g.GET("/:id", h.GetTemplate)
		g.PUT("/:id", h.UpdateTemplate)
		g.DELETE("/:id", h.DeleteTemplate)
	}
}

// ListTemplates 获取流水线模板列表
// @Summary 获取流水线模板列表
// @Tags 流水线模板
// @Param category query string false "分类"
// @Param language query string false "编程语言"
// @Param keyword query string false "关键词"
// @Success 200 {object} gin.H
// @Router /pipeline/templates [get]
func (h *TemplateHandler) ListTemplates(c *gin.Context) {
	category := c.Query("category")
	language := c.Query("language")
	keyword := c.Query("keyword")
	page := parsePositiveInt(c.DefaultQuery("page", "1"), 1)
	pageSize := parsePositiveInt(c.DefaultQuery("page_size", "12"), 12)
	orderBy := c.DefaultQuery("order_by", "default")
	tagQuery := strings.TrimSpace(c.Query("tags"))
	favoritesOnly := c.Query("favorites_only") == "true"
	mineOnly := c.Query("mine") == "true"
	userID := c.GetUint("user_id")
	username := c.GetString("username")

	// 幂等补齐新增的内置模板，避免旧环境永远看不到新模板。
	h.initBuiltinTemplates()

	query := h.db.Model(&pipeline.PipelineTemplate{}).Where("is_public = ?", true)

	if category != "" {
		query = query.Where("category = ?", normalizeTemplateCategory(category))
	}
	if language != "" {
		query = query.Where("language = ?", language)
	}
	if keyword != "" {
		query = query.Where("name LIKE ? OR description LIKE ?", "%"+keyword+"%", "%"+keyword+"%")
	}
	if mineOnly && username != "" {
		query = query.Where("created_by = ?", username)
	}
	if tagQuery != "" {
		var tagConditions []string
		var tagArgs []any
		for _, tag := range strings.Split(tagQuery, ",") {
			tag = strings.TrimSpace(tag)
			if tag == "" {
				continue
			}
			tagConditions = append(tagConditions, "(language = ? OR framework = ?)")
			tagArgs = append(tagArgs, tag, tag)
		}
		if len(tagConditions) > 0 {
			query = query.Where(strings.Join(tagConditions, " OR "), tagArgs...)
		}
	}
	if favoritesOnly && userID > 0 {
		query = query.Joins(
			"JOIN pipeline_template_favorites ON pipeline_template_favorites.template_id = pipeline_templates.id AND pipeline_template_favorites.user_id = ?",
			userID,
		)
	}

	var total int64
	query.Count(&total)

	switch orderBy {
	case "usage_count":
		query = query.Order("usage_count DESC")
	case "rating":
		query = query.Order("rating DESC, rating_count DESC")
	case "created_at":
		query = query.Order("created_at DESC")
	case "updated_at":
		query = query.Order("updated_at DESC")
	default:
		query = query.Order("is_builtin DESC, usage_count DESC, rating DESC, created_at DESC")
	}

	var templates []pipeline.PipelineTemplate
	query.Offset((page - 1) * pageSize).Limit(pageSize).Find(&templates)
	favoriteMap := h.getFavoriteTemplateMap(userID)

	items := make([]gin.H, 0, len(templates))
	for _, tpl := range templates {
		items = append(items, buildTemplateResponse(tpl, favoriteMap[tpl.ID]))
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"items":     items,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

// GetTemplate 获取模板详情
// @Summary 获取模板详情
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id} [get]
func (h *TemplateHandler) GetTemplate(c *gin.Context) {
	h.initBuiltinTemplates()

	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var template pipeline.PipelineTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return
	}

	// 解析配置
	favoriteMap := h.getFavoriteTemplateMap(c.GetUint("user_id"))
	resp := buildTemplateResponse(template, favoriteMap[template.ID])

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"template": resp,
			"config":   resp["config_json"],
		},
	})
}

// CreateTemplate 创建模板
// @Summary 创建流水线模板
// @Tags 流水线模板
// @Param body body pipeline.PipelineTemplate true "模板信息"
// @Success 200 {object} gin.H
// @Router /pipeline/templates [post]
func (h *TemplateHandler) CreateTemplate(c *gin.Context) {
	var req templateMutationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	configJSON, err := h.resolveTemplateConfigJSON(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	if strings.TrimSpace(req.Name) == "" || configJSON == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "名称和配置不能为空"})
		return
	}

	template := pipeline.PipelineTemplate{
		Name:        strings.TrimSpace(req.Name),
		Description: strings.TrimSpace(req.Description),
		Category:    normalizeTemplateCategory(req.Category),
		Language:    strings.TrimSpace(req.Language),
		Framework:   strings.TrimSpace(req.Framework),
		ConfigJSON:  configJSON,
		IsBuiltin:   false,
		IsPublic:    req.IsPublic,
		CreatedBy:   c.GetString("username"),
	}

	if err := h.db.Create(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "创建失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": buildTemplateResponse(template, false), "message": "创建成功"})
}

// UpdateTemplate 更新模板
// @Summary 更新流水线模板
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Param body body pipeline.PipelineTemplate true "模板信息"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id} [put]
func (h *TemplateHandler) UpdateTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var template pipeline.PipelineTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return
	}

	if template.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "内置模板不允许修改"})
		return
	}

	var updates templateMutationRequest
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	configJSON, err := h.resolveTemplateConfigJSON(updates)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}

	updateMap := map[string]any{
		"name":        strings.TrimSpace(updates.Name),
		"description": strings.TrimSpace(updates.Description),
		"category":    normalizeTemplateCategory(updates.Category),
		"language":    strings.TrimSpace(updates.Language),
		"framework":   strings.TrimSpace(updates.Framework),
		"is_public":   updates.IsPublic,
	}
	if configJSON != "" {
		updateMap["config_json"] = configJSON
	}

	if err := h.db.Model(&template).Updates(updateMap).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "更新成功"})
}

// DeleteTemplate 删除模板
// @Summary 删除流水线模板
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id} [delete]
func (h *TemplateHandler) DeleteTemplate(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)

	var template pipeline.PipelineTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return
	}

	if template.IsBuiltin {
		c.JSON(http.StatusForbidden, gin.H{"code": 403, "message": "内置模板不允许删除"})
		return
	}

	if err := h.db.Delete(&template).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "删除失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "删除成功"})
}

// UseTemplate 记录「使用模板」（前端随后跳转创建页并带 template_id 拉取详情）
// @Summary 使用流水线模板
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id}/use [post]
func (h *TemplateHandler) UseTemplate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的模板 ID"})
		return
	}
	var template pipeline.PipelineTemplate
	if err := h.db.First(&template, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return
	}
	if err := h.db.Model(&template).UpdateColumn("usage_count", gorm.Expr("usage_count + ?", 1)).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "更新失败: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "ok", "data": gin.H{"template_id": id}})
}

// AddFavorite 收藏模板（当前为占位实现，持久化待用户体系与收藏表）
func (h *TemplateHandler) AddFavorite(c *gin.Context) {
	templateID, err := h.ensureTemplateID(c)
	if err != nil {
		return
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "请先登录"})
		return
	}
	favorite := pipeline.PipelineTemplateFavorite{TemplateID: templateID, UserID: userID}
	if err := h.db.Where("template_id = ? AND user_id = ?", templateID, userID).FirstOrCreate(&favorite).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "收藏失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "成功"})
}

// RemoveFavorite 取消收藏
func (h *TemplateHandler) RemoveFavorite(c *gin.Context) {
	templateID, err := h.ensureTemplateID(c)
	if err != nil {
		return
	}
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "请先登录"})
		return
	}
	if err := h.db.Where("template_id = ? AND user_id = ?", templateID, userID).Delete(&pipeline.PipelineTemplateFavorite{}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "取消收藏失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "成功"})
}

func (h *TemplateHandler) ensureTemplateID(c *gin.Context) (uint64, error) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的模板 ID"})
		if err != nil {
			return 0, err
		}
		return 0, errors.New("invalid template id")
	}
	var n int64
	if err := h.db.Model(&pipeline.PipelineTemplate{}).Where("id = ?", id).Count(&n).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询失败"})
		return 0, err
	}
	if n == 0 {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return 0, gorm.ErrRecordNotFound
	}
	return id, nil
}

// ApplyTemplate 应用模板创建流水线
// @Summary 应用模板创建流水线
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Param body body ApplyTemplateRequest true "应用请求"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id}/apply [post]
func (h *TemplateHandler) ApplyTemplate(c *gin.Context) {
	h.initBuiltinTemplates()

	templateID, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || templateID == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "无效的模板 ID"})
		return
	}

	var req ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	// 获取模板
	var template pipeline.PipelineTemplate
	if err := h.db.First(&template, templateID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "模板不存在"})
		return
	}

	userID := c.GetUint("user_id")
	sourceTemplateID := uint(templateID)

	var templateConfig struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
		CI        struct {
			GitLabCIYAML       string `json:"gitlab_ci_yaml"`
			GitLabCIYAMLCustom bool   `json:"gitlab_ci_yaml_custom"`
			DockerfileContent  string `json:"dockerfile_content"`
		} `json:"ci"`
	}
	if err := json.Unmarshal([]byte(template.ConfigJSON), &templateConfig); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "模板配置无效: " + err.Error()})
		return
	}

	pipelineReq := &dto.PipelineRequest{
		Name:             req.Name,
		Description:      req.Description,
		ProjectID:        req.ProjectID,
		SourceTemplateID: &sourceTemplateID,
		GitRepoID:        req.GitRepoID,
		GitBranch:        req.GitBranch,
		GitLabCIYAML:     templateConfig.CI.GitLabCIYAML,
		GitLabCIYAMLCustom: templateConfig.CI.GitLabCIYAMLCustom,
		DockerfileContent:  templateConfig.CI.DockerfileContent,
		Stages:           templateConfig.Stages,
		Variables:        templateConfig.Variables,
		TriggerConfig:    dto.TriggerConfig{Manual: true},
	}

	pipelineSvc := pipelinesvc.NewPipelineService(h.db)
	role := c.GetString("role")
	if err := pipelineSvc.PrepareForCreate(c.Request.Context(), pipelineReq, role); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := pipelineSvc.Validate(c.Request.Context(), pipelineReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if err := pipelineSvc.Create(c.Request.Context(), pipelineReq, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "创建流水线失败: " + err.Error()})
		return
	}

	// 更新模板使用次数
	h.db.Model(&template).UpdateColumn("usage_count", template.UsageCount+1)

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "流水线创建成功"})
}

// RateTemplate 评价模板
// @Summary 评价流水线模板
// @Tags 流水线模板
// @Param id path int true "模板ID"
// @Param body body RateTemplateRequest true "评价请求"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/{id}/rate [post]
func (h *TemplateHandler) RateTemplate(c *gin.Context) {
	templateID, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	userID := c.GetUint("user_id")

	var req RateTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "参数错误: " + err.Error()})
		return
	}

	if req.Rating < 1 || req.Rating > 5 {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "评分必须在 1-5 之间"})
		return
	}

	// 检查是否已评价
	var existing pipeline.PipelineTemplateRating
	if h.db.Where("template_id = ? AND user_id = ?", templateID, userID).First(&existing).Error == nil {
		// 更新评价
		h.db.Model(&existing).Updates(map[string]any{
			"rating":  req.Rating,
			"comment": req.Comment,
		})
	} else {
		// 创建评价
		rating := &pipeline.PipelineTemplateRating{
			TemplateID: templateID,
			UserID:     userID,
			Rating:     req.Rating,
			Comment:    req.Comment,
		}
		h.db.Create(rating)
	}

	// 更新模板平均评分
	var avgRating float64
	var count int64
	h.db.Model(&pipeline.PipelineTemplateRating{}).
		Where("template_id = ?", templateID).
		Select("AVG(rating)").Scan(&avgRating)
	h.db.Model(&pipeline.PipelineTemplateRating{}).
		Where("template_id = ?", templateID).Count(&count)

	h.db.Model(&pipeline.PipelineTemplate{}).Where("id = ?", templateID).
		Updates(map[string]any{
			"rating":       avgRating,
			"rating_count": count,
		})

	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "评价成功"})
}

// ListStageTemplates 获取阶段模板列表
// @Summary 获取阶段模板列表
// @Tags 流水线模板
// @Success 200 {object} gin.H
// @Router /pipeline/templates/stages [get]
func (h *TemplateHandler) ListStageTemplates(c *gin.Context) {
	h.initBuiltinStageTemplates()

	var templates []pipeline.PipelineStageTemplate
	h.db.Order("sort_order ASC").Find(&templates)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"items": templates}})
}

// ListStepTemplates 获取步骤模板列表
// @Summary 获取步骤模板列表
// @Tags 流水线模板
// @Param category query string false "分类"
// @Success 200 {object} gin.H
// @Router /pipeline/templates/steps [get]
func (h *TemplateHandler) ListStepTemplates(c *gin.Context) {
	category := c.Query("category")

	h.initBuiltinStepTemplates()

	query := h.db.Model(&pipeline.PipelineStepTemplate{})
	if category != "" {
		query = query.Where("category = ?", category)
	}

	var templates []pipeline.PipelineStepTemplate
	query.Order("sort_order ASC").Find(&templates)

	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"items": templates}})
}

// initBuiltinTemplates 初始化内置流水线模板
func (h *TemplateHandler) initBuiltinTemplates() {
	for _, t := range pipeline.BuiltinPipelineTemplates {
		t = normalizeBuiltinPipelineTemplate(t)
		var existing pipeline.PipelineTemplate
		if h.db.Where("name = ?", t.Name).First(&existing).Error == nil {
			if existing.IsBuiltin &&
				(existing.Description != t.Description ||
					existing.Category != t.Category ||
					existing.Language != t.Language ||
					existing.Framework != t.Framework ||
					existing.ConfigJSON != t.ConfigJSON ||
					existing.IconURL != t.IconURL ||
					existing.IsPublic != t.IsPublic) {
				h.db.Model(&existing).Updates(map[string]any{
					"description": t.Description,
					"category":    t.Category,
					"language":    t.Language,
					"framework":   t.Framework,
					"config_json": t.ConfigJSON,
					"icon_url":    t.IconURL,
					"is_public":   t.IsPublic,
				})
			}
			continue
		}
		h.db.Create(&t)
	}
}

func normalizeBuiltinPipelineTemplate(t pipeline.PipelineTemplate) pipeline.PipelineTemplate {
	type templateConfigEnvelope struct {
		Stages    json.RawMessage `json:"stages,omitempty"`
		Variables json.RawMessage `json:"variables,omitempty"`
		CI        map[string]any  `json:"ci,omitempty"`
	}

	var envelope templateConfigEnvelope
	if err := json.Unmarshal([]byte(t.ConfigJSON), &envelope); err != nil {
		return t
	}
	if envelope.CI == nil {
		envelope.CI = map[string]any{}
	}
	if _, exists := envelope.CI["engine"]; !exists {
		envelope.CI["engine"] = "gitlab_runner"
	}
	if _, exists := envelope.CI["config_path"]; !exists {
		envelope.CI["config_path"] = ".gitlab-ci.yml"
	}
	if _, exists := envelope.CI["dockerfile_path"]; !exists {
		envelope.CI["dockerfile_path"] = ".jeridevops.Dockerfile"
	}
	if _, exists := envelope.CI["dockerfile_mode"]; !exists {
		envelope.CI["dockerfile_mode"] = "inline"
	}
	if _, exists := envelope.CI["gitlab_ci_yaml"]; !exists {
		envelope.CI["gitlab_ci_yaml"] = ""
	}
	if _, exists := envelope.CI["gitlab_ci_yaml_custom"]; !exists {
		envelope.CI["gitlab_ci_yaml_custom"] = false
	}
	if current, ok := envelope.CI["dockerfile_content"].(string); !ok || strings.TrimSpace(current) == "" {
		envelope.CI["dockerfile_content"] = builtinTemplateDockerfile(t.Language, t.Framework, t.Name)
	}

	raw, err := json.MarshalIndent(envelope, "", "\t")
	if err != nil {
		return t
	}
	t.ConfigJSON = string(raw)
	return t
}

func builtinTemplateDockerfile(language, framework, name string) string {
	normalized := strings.ToLower(strings.TrimSpace(language))
	switch normalized {
	case "go", "golang":
		return `FROM golang:1.25-alpine AS builder
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; elif [ -d ./cmd/server ]; then CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./cmd/server; else CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app .; fi

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /out/app /app/app
EXPOSE 8080
ENTRYPOINT ["/app/app"]
`
	case "rust", "cargo":
		return `FROM rust:1.87-alpine AS builder
WORKDIR /src
RUN apk add --no-cache musl-dev pkgconfig openssl-dev build-base
COPY Cargo.toml Cargo.lock* ./
COPY src ./src
COPY . .
ARG BUILD_COMMAND=""
ARG APP_PORT="8080"
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else cargo build --release; fi
RUN bin_path="$(find target/release -maxdepth 1 -type f -perm -111 | head -n 1)" && cp "$bin_path" /tmp/app

FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache ca-certificates tzdata libgcc libstdc++
COPY --from=builder /tmp/app /app/app
EXPOSE ${APP_PORT}
ENTRYPOINT ["/app/app"]
`
	case "java":
		return `FROM maven:3.9-eclipse-temurin-17 AS builder
WORKDIR /src
COPY pom.xml ./
RUN mvn -B -DskipTests dependency:go-offline
COPY . .
ARG BUILD_COMMAND="mvn -B clean package -DskipTests"
RUN sh -c "$BUILD_COMMAND"

FROM eclipse-temurin:17-jre-alpine
WORKDIR /app
COPY --from=builder /src/target/*.jar /app/app.jar
EXPOSE 8080
ENTRYPOINT ["java", "-jar", "/app/app.jar"]
`
	case "nodejs", "node":
		if strings.EqualFold(strings.TrimSpace(framework), "vue") || strings.Contains(strings.ToLower(name), "frontend") {
			return `FROM node:20-alpine AS builder
WORKDIR /src
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
ARG BUILD_COMMAND="npm run build"
RUN sh -c "$BUILD_COMMAND"

FROM nginx:1.27-alpine
COPY --from=builder /src/dist /usr/share/nginx/html
EXPOSE 80
CMD ["nginx", "-g", "daemon off;"]
`
		}
		return `FROM node:20-alpine AS builder
WORKDIR /src
COPY package*.json ./
RUN if [ -f package-lock.json ]; then npm ci; else npm install; fi
COPY . .
ARG BUILD_COMMAND="npm run build --if-present"
RUN sh -c "$BUILD_COMMAND"
RUN npm prune --omit=dev

FROM node:20-alpine
WORKDIR /app
ENV NODE_ENV=production
COPY --from=builder /src /app
EXPOSE 3000
CMD ["npm", "start"]
`
	case "python":
		return `FROM python:3.12-alpine
WORKDIR /app
ENV PYTHONDONTWRITEBYTECODE=1
ENV PYTHONUNBUFFERED=1
COPY requirements*.txt ./
RUN if [ -f requirements.txt ]; then pip install --no-cache-dir -r requirements.txt; fi
COPY . .
EXPOSE 8000
CMD ["python", "app.py"]
`
	default:
		return `FROM alpine:3.20
WORKDIR /app
RUN apk add --no-cache bash ca-certificates
COPY . .
ARG BUILD_COMMAND=""
RUN if [ -n "$BUILD_COMMAND" ]; then sh -c "$BUILD_COMMAND"; else echo "No BUILD_COMMAND configured; packaging repository content."; fi
CMD ["sh", "-c", "echo 'Image built by JeriDevOps GitLab Runner'; sleep infinity"]
`
	}
}

// initBuiltinStageTemplates 初始化内置阶段模板
func (h *TemplateHandler) initBuiltinStageTemplates() {
	for _, t := range pipeline.BuiltinStageTemplates {
		var existing pipeline.PipelineStageTemplate
		if h.db.Where("name = ?", t.Name).First(&existing).Error == nil {
			if existing.IsBuiltin &&
				(existing.Description != t.Description ||
					existing.Category != t.Category ||
					existing.IconName != t.IconName ||
					existing.Color != t.Color ||
					existing.ConfigJSON != t.ConfigJSON ||
					existing.SortOrder != t.SortOrder) {
				h.db.Model(&existing).Updates(map[string]any{
					"description": t.Description,
					"category":    t.Category,
					"icon_name":   t.IconName,
					"color":       t.Color,
					"config_json": t.ConfigJSON,
					"sort_order":  t.SortOrder,
				})
			}
			continue
		}
		h.db.Create(&t)
	}
}

// initBuiltinStepTemplates 初始化内置步骤模板
func (h *TemplateHandler) initBuiltinStepTemplates() {
	h.db.Where("is_builtin = ? AND step_type = ?", true, "k8s_deploy").Delete(&pipeline.PipelineStepTemplate{})

	for _, t := range pipeline.BuiltinStepTemplates {
		var existing pipeline.PipelineStepTemplate
		if h.db.Where("name = ?", t.Name).First(&existing).Error == nil {
			if existing.IsBuiltin &&
				(existing.Description != t.Description ||
					existing.StepType != t.StepType ||
					existing.Category != t.Category ||
					existing.IconName != t.IconName ||
					existing.ConfigSchema != t.ConfigSchema ||
					existing.DefaultJSON != t.DefaultJSON ||
					existing.SortOrder != t.SortOrder) {
				h.db.Model(&existing).Updates(map[string]any{
					"description":   t.Description,
					"step_type":     t.StepType,
					"category":      t.Category,
					"icon_name":     t.IconName,
					"config_schema": t.ConfigSchema,
					"default_json":  t.DefaultJSON,
					"sort_order":    t.SortOrder,
				})
			}
			continue
		}
		h.db.Create(&t)
	}
}

// ApplyTemplateRequest 应用模板请求
type ApplyTemplateRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	ProjectID   *uint  `json:"project_id"`
	GitRepoID   *uint  `json:"git_repo_id"`
	GitBranch   string `json:"git_branch"`
}

// RateTemplateRequest 评价模板请求
type RateTemplateRequest struct {
	Rating  int    `json:"rating" binding:"required"`
	Comment string `json:"comment"`
}

// GetCategories 获取模板分类列表
// @Summary 获取模板分类列表
// @Tags 流水线模板
// @Success 200 {object} gin.H
// @Router /pipeline/templates/categories [get]
func (h *TemplateHandler) GetCategories(c *gin.Context) {
	// 定义所有可用的分类
	categories := []map[string]interface{}{
		{
			"value":       "build",
			"label":       "构建",
			"description": "代码构建和编译",
			"icon":        "build",
		},
		{
			"value":       "deploy",
			"label":       "部署",
			"description": "应用部署到各种环境",
			"icon":        "deploy",
		},
		{
			"value":       "test",
			"label":       "测试",
			"description": "自动化测试和质量检查",
			"icon":        "test",
		},
		{
			"value":       "security",
			"label":       "安全",
			"description": "安全检查和扫描",
			"icon":        "safety",
		},
		{
			"value":       "other",
			"label":       "其他",
			"description": "其他通用模板",
			"icon":        "appstore",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"items": categories,
		},
	})
}

// GetTags 获取模板标签列表
// @Summary 获取模板标签列表
// @Tags 流水线模板
// @Success 200 {object} gin.H
// @Router /pipeline/templates/tags [get]
func (h *TemplateHandler) GetTags(c *gin.Context) {
	// 从数据库中获取所有使用的语言和框架作为标签
	var languages []string
	h.db.Model(&pipeline.PipelineTemplate{}).
		Where("language IS NOT NULL AND language != ''").
		Distinct("language").
		Pluck("language", &languages)

	var frameworks []string
	h.db.Model(&pipeline.PipelineTemplate{}).
		Where("framework IS NOT NULL AND framework != ''").
		Distinct("framework").
		Pluck("framework", &frameworks)

	// 组合标签
	tags := []map[string]interface{}{}

	// 添加语言标签
	for _, lang := range languages {
		tags = append(tags, map[string]interface{}{
			"value": lang,
			"label": lang,
			"type":  "language",
		})
	}

	// 添加框架标签
	for _, fw := range frameworks {
		tags = append(tags, map[string]interface{}{
			"value": fw,
			"label": fw,
			"type":  "framework",
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"items": tags,
		},
	})
}

// GetFavorites 获取收藏的模板列表
// @Summary 获取收藏的模板列表
// @Tags 流水线模板
// @Success 200 {object} gin.H
// @Router /pipeline/templates/favorites [get]
func (h *TemplateHandler) GetFavorites(c *gin.Context) {
	userID := c.GetUint("user_id")
	if userID == 0 {
		c.JSON(http.StatusUnauthorized, gin.H{"code": 401, "message": "请先登录"})
		return
	}

	var favorites []pipeline.PipelineTemplate
	if err := h.db.
		Joins("JOIN pipeline_template_favorites ON pipeline_template_favorites.template_id = pipeline_templates.id").
		Where("pipeline_template_favorites.user_id = ?", userID).
		Order("pipeline_template_favorites.created_at DESC").
		Find(&favorites).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": "查询收藏失败"})
		return
	}

	items := make([]gin.H, 0, len(favorites))
	for _, tpl := range favorites {
		items = append(items, buildTemplateResponse(tpl, true))
	}

	c.JSON(http.StatusOK, gin.H{
		"code": 0,
		"data": gin.H{
			"items": items,
		},
	})
}

func (h *TemplateHandler) resolveTemplateConfigJSON(req templateMutationRequest) (string, error) {
	if req.SourcePipelineID > 0 {
		var src models.Pipeline
		if err := h.db.First(&src, req.SourcePipelineID).Error; err != nil {
			return "", errors.New("源流水线不存在")
		}
		if strings.TrimSpace(src.ConfigJSON) == "" {
			return "", errors.New("源流水线配置为空")
		}
		return src.ConfigJSON, nil
	}

	switch cfg := req.ConfigJSON.(type) {
	case nil:
		return "", nil
	case string:
		if strings.TrimSpace(cfg) == "" {
			return "", nil
		}
		var payload any
		if err := json.Unmarshal([]byte(cfg), &payload); err != nil {
			return "", errors.New("配置必须是有效的 JSON")
		}
		return cfg, nil
	default:
		raw, err := json.Marshal(cfg)
		if err != nil {
			return "", errors.New("配置必须是有效的 JSON")
		}
		var payload any
		if err := json.Unmarshal(raw, &payload); err != nil {
			return "", errors.New("配置必须是有效的 JSON")
		}
		return string(raw), nil
	}
}

func buildTemplateResponse(tpl pipeline.PipelineTemplate, isFavorite bool) gin.H {
	var config any
	_ = json.Unmarshal([]byte(tpl.ConfigJSON), &config)

	tags := make([]string, 0, 2)
	if strings.TrimSpace(tpl.Language) != "" {
		tags = append(tags, tpl.Language)
	}
	if strings.TrimSpace(tpl.Framework) != "" {
		tags = append(tags, tpl.Framework)
	}

	return gin.H{
		"id":           tpl.ID,
		"name":         tpl.Name,
		"slug":         tpl.Name,
		"description":  tpl.Description,
		"category":     tpl.Category,
		"language":     tpl.Language,
		"framework":    tpl.Framework,
		"config_json":  config,
		"is_public":    tpl.IsPublic,
		"is_builtin":   tpl.IsBuiltin,
		"is_official":  tpl.IsBuiltin,
		"is_favorite":  isFavorite,
		"usage_count":  tpl.UsageCount,
		"rating":       tpl.Rating,
		"rating_count": tpl.RatingCount,
		"version":      "1.0.0",
		"tags":         tags,
		"created_by":   tpl.CreatedBy,
		"created_at":   tpl.CreatedAt,
		"updated_at":   tpl.UpdatedAt,
	}
}

func (h *TemplateHandler) getFavoriteTemplateMap(userID uint) map[uint64]bool {
	result := map[uint64]bool{}
	if userID == 0 {
		return result
	}

	var favorites []pipeline.PipelineTemplateFavorite
	if err := h.db.Where("user_id = ?", userID).Find(&favorites).Error; err != nil {
		return result
	}
	for _, item := range favorites {
		result[item.TemplateID] = true
	}
	return result
}

func normalizeTemplateCategory(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "构建", "build":
		return "build"
	case "部署", "deploy":
		return "deploy"
	case "测试", "test":
		return "test"
	case "安全", "安全扫描", "security":
		return "security"
	case "", "其他", "other":
		return "other"
	default:
		return strings.TrimSpace(raw)
	}
}

func parsePositiveInt(raw string, fallback int) int {
	value, err := strconv.Atoi(raw)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}
