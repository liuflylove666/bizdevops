package security

import (
	"context"
	"encoding/json"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/pkg/dto"
	"devops/pkg/logger"
)

// ImageScannerService 镜像扫描服务
type ImageScannerService struct {
	db      *gorm.DB
	scanner *TrivyScanner
}

// NewImageScannerService 创建镜像扫描服务
func NewImageScannerService(db *gorm.DB) *ImageScannerService {
	return &ImageScannerService{
		db:      db,
		scanner: NewTrivyScanner(),
	}
}

// ScanImage 扫描镜像
func (s *ImageScannerService) ScanImage(ctx context.Context, req *dto.ScanImageRequest) (*dto.ScanResultResponse, error) {
	log := logger.L().WithField("image", req.Image)
	log.Info("开始扫描镜像")

	applicationID, applicationName := s.resolveApplicationIdentity(req.ApplicationID, req.ApplicationName)

	// 创建扫描记录
	imageName, imageTag := splitImageNameAndTag(req.Image)
	scan := &models.ImageScan{
		ImageName:       imageName,
		ImageTag:        imageTag,
		ScanStatus:      "scanning",
		ApplicationName: applicationName,
		Image:           req.Image,
		RegistryID:      &req.RegistryID,
		Status:          "scanning",
		CreatedAt:       time.Now(),
	}
	if req.RegistryID == 0 {
		scan.RegistryID = nil
	}
	if applicationID > 0 {
		scan.ApplicationID = &applicationID
	}
	if req.PipelineRunID > 0 {
		scan.PipelineRunID = &req.PipelineRunID
	}

	if err := s.db.Create(scan).Error; err != nil {
		log.WithField("error", err).Error("创建扫描记录失败")
		return nil, err
	}

	// 获取仓库凭证
	var registry *models.ImageRegistry
	if req.RegistryID > 0 {
		registry = &models.ImageRegistry{}
		if err := s.db.First(registry, req.RegistryID).Error; err != nil {
			log.WithField("error", err).Warn("获取仓库配置失败")
		}
	}

	// 执行扫描
	result, err := s.scanner.Scan(ctx, req.Image, registry)
	now := time.Now()

	if err != nil {
		log.WithField("error", err).Error("扫描镜像失败")
		scan.ScanStatus = "failed"
		scan.Status = "failed"
		scan.ErrorMessage = err.Error()
		scan.ScannedAt = &now
		s.db.Save(scan)

		return &dto.ScanResultResponse{
			ID:           scan.ID,
			Image:        scan.Image,
			Status:       scan.Status,
			ErrorMessage: scan.ErrorMessage,
			ScannedAt:    scan.ScannedAt,
		}, nil
	}

	// 更新扫描结果
	scan.ScanStatus = "completed"
	scan.Status = "completed"
	scan.RiskLevel = result.RiskLevel
	scan.CriticalCount = result.VulnSummary.Critical
	scan.HighCount = result.VulnSummary.High
	scan.MediumCount = result.VulnSummary.Medium
	scan.LowCount = result.VulnSummary.Low
	scan.ScannedAt = &now

	// 保存详细结果
	resultJSON, _ := json.Marshal(result.Vulnerabilities)
	scan.ScanResult = string(resultJSON)
	scan.ResultJSON = string(resultJSON)

	if err := s.db.Save(scan).Error; err != nil {
		log.WithField("error", err).Error("保存扫描结果失败")
		return nil, err
	}

	log.WithField("risk_level", result.RiskLevel).Info("镜像扫描完成")

	return &dto.ScanResultResponse{
		ID:              scan.ID,
		Image:           scan.Image,
		Status:          scan.Status,
		RiskLevel:       scan.RiskLevel,
		VulnSummary:     result.VulnSummary,
		Vulnerabilities: result.Vulnerabilities,
		ScannedAt:       scan.ScannedAt,
	}, nil
}

// GetScanHistory 获取扫描历史
func (s *ImageScannerService) GetScanHistory(ctx context.Context, req *dto.ScanHistoryRequest) (*dto.ScanHistoryResponse, error) {
	var scans []models.ImageScan
	var total int64

	query := s.db.Model(&models.ImageScan{})
	resolvedAppID, resolvedAppName := s.resolveApplicationIdentity(req.ApplicationID, req.ApplicationName)
	legacyKeywords := s.buildApplicationScanKeywords(resolvedAppID, resolvedAppName)

	if req.Image != "" {
		query = query.Where("image LIKE ?", "%"+req.Image+"%")
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}
	if req.PipelineRunID > 0 {
		query = query.Where("pipeline_run_id = ?", req.PipelineRunID)
	}
	query = s.applyApplicationHistoryFilter(query, req)

	query.Count(&total)

	page := req.Page
	if page <= 0 {
		page = 1
	}
	pageSize := req.PageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	query.Order("created_at DESC").
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		Find(&scans)

	items := make([]dto.ImageScanItem, 0, len(scans))
	for _, scan := range scans {
		items = append(items, dto.ImageScanItem{
			ID:                scan.ID,
			Image:             scan.Image,
			ApplicationID:     scan.ApplicationID,
			ApplicationName:   scan.ApplicationName,
			PipelineRunID:     scan.PipelineRunID,
			AssociationSource: deriveAssociationSource(&scan, req, resolvedAppID, resolvedAppName, legacyKeywords),
			Status:            scan.Status,
			RiskLevel:         scan.RiskLevel,
			CriticalCount:     scan.CriticalCount,
			HighCount:         scan.HighCount,
			MediumCount:       scan.MediumCount,
			LowCount:          scan.LowCount,
			ScannedAt:         scan.ScannedAt,
			CreatedAt:         scan.CreatedAt,
		})
	}

	return &dto.ScanHistoryResponse{
		Total: int(total),
		Items: items,
	}, nil
}

func (s *ImageScannerService) resolveApplicationIdentity(applicationID uint, applicationName string) (uint, string) {
	var app models.Application
	name := strings.TrimSpace(applicationName)
	switch {
	case applicationID > 0:
		if err := s.db.Select("id", "name").First(&app, applicationID).Error; err == nil {
			return app.ID, strings.TrimSpace(app.Name)
		}
	case name != "":
		if err := s.db.Select("id", "name").Where("name = ?", name).First(&app).Error; err == nil {
			return app.ID, strings.TrimSpace(app.Name)
		}
		if err := s.db.Select("id", "name").Where("display_name = ?", name).First(&app).Error; err == nil {
			return app.ID, strings.TrimSpace(app.Name)
		}
	}
	return 0, name
}

func (s *ImageScannerService) applyApplicationHistoryFilter(query *gorm.DB, req *dto.ScanHistoryRequest) *gorm.DB {
	appID, appName := s.resolveApplicationIdentity(req.ApplicationID, req.ApplicationName)
	if appID == 0 && appName == "" {
		return query
	}

	legacyKeywords := s.buildApplicationScanKeywords(appID, appName)
	var scope *gorm.DB
	appendScope := func(condition string, args ...interface{}) {
		if scope == nil {
			scope = s.db.Where(condition, args...)
			return
		}
		scope = scope.Or(condition, args...)
	}
	if appID > 0 {
		appendScope("application_id = ?", appID)
	}
	if appName != "" {
		appendScope("application_name = ?", appName)
	}
	for _, keyword := range legacyKeywords {
		appendScope("image LIKE ?", "%"+keyword+"%")
	}
	if scope == nil {
		return query
	}
	return query.Where(scope)
}

func (s *ImageScannerService) buildApplicationScanKeywords(applicationID uint, applicationName string) []string {
	keywords := make([]string, 0, 8)
	add := func(raw string) {
		value := strings.TrimSpace(raw)
		if value == "" {
			return
		}
		for _, existing := range keywords {
			if existing == value {
				return
			}
		}
		keywords = append(keywords, value)
		normalized := strings.ToLower(strings.ReplaceAll(value, " ", "-"))
		if normalized != "" && normalized != value {
			for _, existing := range keywords {
				if existing == normalized {
					return
				}
			}
			keywords = append(keywords, normalized)
		}
	}

	if applicationName != "" {
		add(applicationName)
	}
	if applicationID == 0 {
		return keywords
	}

	var app models.Application
	if err := s.db.Select("id", "name", "display_name").First(&app, applicationID).Error; err == nil {
		add(app.Name)
		add(app.DisplayName)
	}

	var envs []models.ApplicationEnv
	if err := s.db.Select("k8s_deployment").Where("app_id = ?", applicationID).Find(&envs).Error; err == nil {
		for _, env := range envs {
			add(env.K8sDeployment)
		}
	}
	return keywords
}

func deriveAssociationSource(scan *models.ImageScan, req *dto.ScanHistoryRequest, resolvedAppID uint, resolvedAppName string, legacyKeywords []string) string {
	if scan == nil {
		return ""
	}
	if resolvedAppID > 0 && scan.ApplicationID != nil && *scan.ApplicationID == resolvedAppID {
		return "application_id"
	}
	if resolvedAppName != "" && strings.EqualFold(strings.TrimSpace(scan.ApplicationName), strings.TrimSpace(resolvedAppName)) {
		return "application_name"
	}
	if req.PipelineRunID > 0 && scan.PipelineRunID != nil && *scan.PipelineRunID == req.PipelineRunID {
		return "pipeline_run"
	}
	image := strings.ToLower(strings.TrimSpace(scan.Image))
	for _, keyword := range legacyKeywords {
		if keyword == "" {
			continue
		}
		if strings.Contains(image, strings.ToLower(keyword)) {
			return "legacy_image_keyword"
		}
	}
	return ""
}

// GetScanResult 获取扫描结果
func (s *ImageScannerService) GetScanResult(ctx context.Context, scanID uint) (*dto.ScanResultResponse, error) {
	var scan models.ImageScan
	if err := s.db.First(&scan, scanID).Error; err != nil {
		return nil, err
	}

	result := &dto.ScanResultResponse{
		ID:              scan.ID,
		Image:           scan.Image,
		ApplicationID:   scan.ApplicationID,
		ApplicationName: scan.ApplicationName,
		PipelineRunID:   scan.PipelineRunID,
		Status:          scan.Status,
		RiskLevel:       scan.RiskLevel,
		VulnSummary: dto.VulnSummary{
			Critical: scan.CriticalCount,
			High:     scan.HighCount,
			Medium:   scan.MediumCount,
			Low:      scan.LowCount,
			Total:    scan.CriticalCount + scan.HighCount + scan.MediumCount + scan.LowCount,
		},
		ScannedAt:    scan.ScannedAt,
		ErrorMessage: scan.ErrorMessage,
	}

	// 解析漏洞详情
	if scan.ResultJSON != "" {
		var vulns []dto.Vulnerability
		if err := json.Unmarshal([]byte(scan.ResultJSON), &vulns); err == nil {
			result.Vulnerabilities = vulns
		}
	}

	return result, nil
}

func splitImageNameAndTag(image string) (string, string) {
	image = strings.TrimSpace(image)
	if image == "" {
		return "", ""
	}
	lastSlash := strings.LastIndex(image, "/")
	lastColon := strings.LastIndex(image, ":")
	if lastColon > lastSlash {
		return image[:lastColon], image[lastColon+1:]
	}
	return image, ""
}
