package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"gorm.io/gorm"

	"devops/internal/models"
	"devops/internal/models/infrastructure"
	"devops/pkg/dto"
	"devops/pkg/logger"
)

const builderConfigKey = "builder_pod_config"

// RunService 流水线执行服务
type RunService struct {
	db         *gorm.DB
	engine     *ExecutorEngine
	ciProvider *gitLabCIProvisioner
}

// NewRunService 创建执行服务
func NewRunService(db *gorm.DB) *RunService {
	engine := NewExecutorEngine(db)
	svc := &RunService{
		db:         db,
		engine:     engine,
		ciProvider: newGitLabCIProvisioner(db),
	}
	// 从数据库加载配置
	svc.loadBuilderConfig()
	return svc
}

// loadBuilderConfig 从数据库加载构建器配置
func (s *RunService) loadBuilderConfig() {
	var sysConfig models.SystemConfig
	if err := s.db.Where("`key` = ?", builderConfigKey).First(&sysConfig).Error; err == nil {
		var cfg BuilderConfig
		if json.Unmarshal([]byte(sysConfig.Value), &cfg) == nil {
			s.engine.SetBuilderConfig(&cfg)
			if cfg.IdleTimeoutMinutes > 0 {
				s.engine.SetBuilderIdleTimeout(time.Duration(cfg.IdleTimeoutMinutes) * time.Minute)
			}
			logger.L().Info("已从数据库加载构建器配置")
		}
	}
}

// Run 运行流水线
func (s *RunService) Run(ctx context.Context, pipelineID uint, req *dto.RunPipelineRequest, triggerType, triggerBy string) (*dto.PipelineRunItem, error) {
	log := logger.L().WithField("pipeline_id", pipelineID)

	// 获取流水线
	var pipeline models.Pipeline
	if err := s.db.First(&pipeline, pipelineID).Error; err != nil {
		return nil, err
	}
	if req == nil {
		req = &dto.RunPipelineRequest{}
	}

	if pipeline.Status != "active" {
		return nil, &ValidationError{Message: "流水线已禁用"}
	}

	// 确定使用的分支
	branch := req.Branch
	if branch == "" {
		branch = pipeline.GitBranch
	}
	if branch == "" {
		branch = "main"
	}

	if isGitLabRunnerPipeline(&pipeline) {
		return s.runGitLabRunnerPipeline(ctx, &pipeline, req, branch, triggerType, triggerBy)
	}

	// 创建执行记录
	parametersJSON, _ := json.Marshal(req.Parameters)
	run := &models.PipelineRun{
		PipelineID:      pipelineID,
		PipelineName:    pipeline.Name,
		ApplicationID:   pipeline.ApplicationID,
		ApplicationName: pipeline.ApplicationName,
		Env:             pipeline.Env,
		Status:          "pending",
		TriggerType:     triggerType,
		TriggerBy:       triggerBy,
		GitBranch:       branch,
		ParametersJSON:  string(parametersJSON),
		CreatedAt:       time.Now(),
	}

	if err := s.db.Create(run).Error; err != nil {
		log.WithField("error", err).Error("创建执行记录失败")
		return nil, err
	}

	// 异步执行
	go s.engine.Execute(context.Background(), run.ID)

	log.WithField("run_id", run.ID).Info("流水线开始执行")

	return &dto.PipelineRunItem{
		ID:              run.ID,
		PipelineID:      run.PipelineID,
		PipelineName:    run.PipelineName,
		ApplicationID:   run.ApplicationID,
		ApplicationName: run.ApplicationName,
		Env:             run.Env,
		Status:          run.Status,
		TriggerType:     run.TriggerType,
		TriggerBy:       run.TriggerBy,
		CreatedAt:       run.CreatedAt,
	}, nil
}

func (s *RunService) runGitLabRunnerPipeline(ctx context.Context, pipeline *models.Pipeline, req *dto.RunPipelineRequest, branch, triggerType, triggerBy string) (*dto.PipelineRunItem, error) {
	log := logger.L().WithField("pipeline_id", pipeline.ID).WithField("runner", "gitlab")

	if pipeline.GitRepoID == nil || *pipeline.GitRepoID == 0 {
		return nil, &ValidationError{Message: "GitLab Runner 流水线缺少 GitLab 仓库"}
	}

	var config struct {
		Stages    []dto.Stage    `json:"stages"`
		Variables []dto.Variable `json:"variables"`
	}
	if pipeline.ConfigJSON != "" {
		_ = json.Unmarshal([]byte(pipeline.ConfigJSON), &config)
	}
	if len(config.Stages) == 0 {
		return nil, &ValidationError{Message: "流水线缺少阶段配置，无法生成 GitLab CI"}
	}
	managedCI := parseManagedConfig(pipeline.ConfigJSON)

	provisionReq := &dto.PipelineRequest{
		Name:               pipeline.Name,
		Description:        pipeline.Description,
		ProjectID:          pipeline.ProjectID,
		ApplicationID:      pipeline.ApplicationID,
		ApplicationName:    pipeline.ApplicationName,
		Env:                pipeline.Env,
		SourceTemplateID:   pipeline.SourceTemplateID,
		GitRepoID:          pipeline.GitRepoID,
		GitBranch:          branch,
		GitLabCIYAML:       managedCI.GitLabCIYAML,
		GitLabCIYAMLCustom: managedCI.GitLabCIYAMLCustom,
		DockerfileContent:  managedCI.DockerfileContent,
		Stages:             config.Stages,
		Variables:          config.Variables,
	}

	provisionResult, err := s.ciProvider.provision(ctx, pipeline, provisionReq, true)
	if err != nil {
		log.WithField("error", err).Error("触发 GitLab Runner 流水线失败")
		return nil, err
	}

	now := time.Now()
	status := gitLabStatusToRunStatus(provisionResult.GitLabPipeline.Status)
	if status == "pending" {
		status = "running"
	}
	parameters := mergePipelineParameters(req.Parameters, pipelineProvisioningVariables(provisionResult.GitLabPipeline))
	parametersJSON, _ := json.Marshal(parameters)
	run := &models.PipelineRun{
		PipelineID:      pipeline.ID,
		PipelineName:    pipeline.Name,
		ApplicationID:   pipeline.ApplicationID,
		ApplicationName: pipeline.ApplicationName,
		Env:             pipeline.Env,
		Status:          status,
		TriggerType:     triggerType,
		TriggerBy:       triggerBy,
		ParametersJSON:  string(parametersJSON),
		GitCommit:       provisionResult.GitLabPipeline.SHA,
		GitBranch:       branch,
		GitMessage:      firstNonEmptyString(provisionResult.GitLabPipeline.WebURL, "GitLab Runner"),
		StartedAt:       &now,
		CreatedAt:       now,
	}
	if gitLabStatusIsTerminal(provisionResult.GitLabPipeline.Status) {
		run.FinishedAt = &now
		run.Duration = 0
	}

	if err := s.db.WithContext(ctx).Create(run).Error; err != nil {
		log.WithField("error", err).Error("创建 GitLab Runner 执行记录失败")
		return nil, err
	}

	if err := s.db.WithContext(ctx).Model(pipeline).Updates(map[string]interface{}{
		"last_run_at":     &now,
		"last_run_status": status,
	}).Error; err != nil {
		log.WithField("error", err).Warn("更新流水线 GitLab Runner 执行摘要失败")
	}

	log.WithField("run_id", run.ID).WithField("gitlab_pipeline_id", provisionResult.GitLabPipeline.ID).Info("GitLab Runner 流水线已触发")
	return &dto.PipelineRunItem{
		ID:              run.ID,
		PipelineID:      run.PipelineID,
		PipelineName:    run.PipelineName,
		ApplicationID:   run.ApplicationID,
		ApplicationName: run.ApplicationName,
		Env:             run.Env,
		Status:          run.Status,
		TriggerType:     run.TriggerType,
		TriggerBy:       run.TriggerBy,
		GitBranch:       run.GitBranch,
		GitCommit:       run.GitCommit,
		GitMessage:      run.GitMessage,
		ExternalURL:     runExternalURL(run),
		StartedAt:       run.StartedAt,
		FinishedAt:      run.FinishedAt,
		Duration:        run.Duration,
		CreatedAt:       run.CreatedAt,
	}, nil
}

// Cancel 取消执行
func (s *RunService) Cancel(ctx context.Context, runID uint) error {
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err == nil {
		info, ok := gitLabPipelineInfoFromRun(&run)
		if !ok {
			info, ok = decodeGitLabExternalRef(run.GitCommit)
		}
		if ok {
			var pipeline models.Pipeline
			if err := s.db.WithContext(ctx).First(&pipeline, run.PipelineID).Error; err == nil && pipeline.GitRepoID != nil {
				var repo models.GitRepository
				if err := s.db.WithContext(ctx).First(&repo, *pipeline.GitRepoID).Error; err == nil {
					if cancelErr := s.ciProvider.cancelGitLabPipeline(ctx, &repo, run.GitBranch, info.ID); cancelErr != nil {
						return cancelErr
					}
					now := time.Now()
					duration := run.Duration
					if run.StartedAt != nil {
						duration = int(now.Sub(*run.StartedAt).Seconds())
					}
					s.db.WithContext(ctx).Model(&run).Updates(map[string]interface{}{
						"status":      "cancelled",
						"finished_at": &now,
						"duration":    duration,
					})
					s.db.WithContext(ctx).Model(&pipeline).Updates(map[string]interface{}{"last_run_status": "cancelled"})
					return nil
				}
			}
		}
	}
	return s.engine.Cancel(ctx, runID)
}

// Retry 重试执行
func (s *RunService) Retry(ctx context.Context, runID uint, fromStage string) error {
	return s.engine.Retry(ctx, runID, fromStage)
}

// ListRuns 获取执行历史
func (s *RunService) ListRuns(ctx context.Context, req *dto.PipelineRunListRequest) (*dto.PipelineRunListResponse, error) {
	var runs []models.PipelineRun
	var total int64

	query := s.db.Model(&models.PipelineRun{})

	if req.PipelineID > 0 {
		query = query.Where("pipeline_id = ?", req.PipelineID)
	}
	if req.ApplicationID > 0 {
		query = query.Where("application_id = ?", req.ApplicationID)
	}
	if strings.TrimSpace(req.ApplicationName) != "" {
		query = query.Where("application_name = ?", strings.TrimSpace(req.ApplicationName))
	}
	if req.Status != "" {
		query = query.Where("status = ?", req.Status)
	}

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
		Find(&runs)

	items := make([]dto.PipelineRunItem, 0, len(runs))
	for _, r := range runs {
		if refreshed, ok := s.refreshGitLabRunStatus(ctx, &r); ok {
			r = *refreshed
		}
		items = append(items, dto.PipelineRunItem{
			ID:                    r.ID,
			PipelineID:            r.PipelineID,
			PipelineName:          r.PipelineName,
			ApplicationID:         r.ApplicationID,
			ApplicationName:       r.ApplicationName,
			Env:                   r.Env,
			Status:                r.Status,
			TriggerType:           r.TriggerType,
			TriggerBy:             r.TriggerBy,
			GitBranch:             r.GitBranch,
			GitCommit:             r.GitCommit,
			GitMessage:            r.GitMessage,
			ExternalURL:           runExternalURL(&r),
			GitOpsChangeRequestID: r.GitOpsChangeRequestID,
			GitOpsHandoffStatus:   r.GitOpsHandoffStatus,
			GitOpsHandoffMessage:  r.GitOpsHandoffMessage,
			StartedAt:             r.StartedAt,
			FinishedAt:            r.FinishedAt,
			Duration:              r.Duration,
			CreatedAt:             r.CreatedAt,
		})
	}

	return &dto.PipelineRunListResponse{
		Total: int(total),
		Items: items,
	}, nil
}

// GetRun 获取执行详情
func (s *RunService) GetRun(ctx context.Context, runID uint) (*dto.PipelineRunDetailResponse, error) {
	var run models.PipelineRun
	if err := s.db.First(&run, runID).Error; err != nil {
		return nil, err
	}
	if refreshed, ok := s.refreshGitLabRunStatus(ctx, &run); ok {
		run = *refreshed
	}
	s.syncGitLabRunnerLogs(ctx, &run)

	result := &dto.PipelineRunDetailResponse{
		ID:              run.ID,
		PipelineID:      run.PipelineID,
		PipelineName:    run.PipelineName,
		ApplicationID:   run.ApplicationID,
		ApplicationName: run.ApplicationName,
		Env:             run.Env,
		Status:          run.Status,
		TriggerType:     run.TriggerType,
		TriggerBy:       run.TriggerBy,
		GitBranch:       run.GitBranch,
		GitCommit:       run.GitCommit,
		GitMessage:      run.GitMessage,
		StartedAt:       run.StartedAt,
		FinishedAt:      run.FinishedAt,
		Duration:        run.Duration,
		CreatedAt:       run.CreatedAt,
	}
	result.ExternalURL = runExternalURL(&run)
	if run.ScannedImage != "" || run.ImageScanStatus != "" {
		result.ImageScan = &dto.PipelineRunImageScan{
			Image:     run.ScannedImage,
			Status:    run.ImageScanStatus,
			RiskLevel: run.ImageScanRiskLevel,
			Critical:  run.ImageScanCritical,
			High:      run.ImageScanHigh,
			Medium:    run.ImageScanMedium,
			Low:       run.ImageScanLow,
		}
	}
	if run.GitOpsHandoffStatus != "" || run.GitOpsChangeRequestID != nil {
		result.GitOpsHandoff = &dto.PipelineRunGitOpsInfo{
			Status:          run.GitOpsHandoffStatus,
			Message:         run.GitOpsHandoffMessage,
			ChangeRequestID: run.GitOpsChangeRequestID,
		}
		if run.GitOpsChangeRequestID != nil && *run.GitOpsChangeRequestID > 0 {
			var change infrastructure.GitOpsChangeRequest
			if err := s.db.First(&change, *run.GitOpsChangeRequestID).Error; err == nil {
				result.GitOpsHandoff.ChangeRequestTitle = change.Title
				result.GitOpsHandoff.ChangeRequestStatus = change.Status
				result.GitOpsHandoff.ApprovalInstanceID = change.ApprovalInstanceID
				result.GitOpsHandoff.ApprovalStatus = change.ApprovalStatus
				result.GitOpsHandoff.AutoMergeStatus = change.AutoMergeStatus
				result.GitOpsHandoff.MergeRequestURL = change.MergeRequestURL
			}
		}
	}

	// 解析参数
	if run.ParametersJSON != "" {
		json.Unmarshal([]byte(run.ParametersJSON), &result.Parameters)
	}

	// 获取阶段执行记录
	var stageRuns []models.StageRun
	s.db.Where("pipeline_run_id = ?", runID).Order("id").Find(&stageRuns)

	result.StageRuns = make([]dto.StageRunItem, 0, len(stageRuns))
	for _, sr := range stageRuns {
		stageItem := dto.StageRunItem{
			ID:         sr.ID,
			StageID:    sr.StageID,
			StageName:  sr.StageName,
			Status:     sr.Status,
			StartedAt:  sr.StartedAt,
			FinishedAt: sr.FinishedAt,
		}

		// 获取步骤执行记录
		var stepRuns []models.StepRun
		s.db.Where("stage_run_id = ?", sr.ID).Order("id").Find(&stepRuns)

		stageItem.StepRuns = make([]dto.StepRunItem, 0, len(stepRuns))
		for _, step := range stepRuns {
			stageItem.StepRuns = append(stageItem.StepRuns, dto.StepRunItem{
				ID:         step.ID,
				StepID:     step.StepID,
				StepName:   step.StepName,
				StepType:   step.StepType,
				Status:     step.Status,
				Logs:       step.Logs,
				ExitCode:   step.ExitCode,
				StartedAt:  step.StartedAt,
				FinishedAt: step.FinishedAt,
			})
		}

		result.StageRuns = append(result.StageRuns, stageItem)
	}

	return result, nil
}

// RefreshGitLabRunnerState polls GitLab for pipeline status and job traces and writes them to the database.
// WebSocket log streaming and other paths that only read pipeline_runs must call this; otherwise GitLab-backed
// runs can stay "running" in MySQL until GET /pipelines/runs/:id runs.
func (s *RunService) RefreshGitLabRunnerState(ctx context.Context, runID uint) {
	var run models.PipelineRun
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		return
	}
	switch run.Status {
	case "success", "failed", "cancelled":
		return
	}
	_, _ = s.refreshGitLabRunStatus(ctx, &run)
	if err := s.db.WithContext(ctx).First(&run, runID).Error; err != nil {
		return
	}
	s.syncGitLabRunnerLogs(ctx, &run)
}

func (s *RunService) syncGitLabRunnerLogs(ctx context.Context, run *models.PipelineRun) {
	if s == nil || s.db == nil || s.ciProvider == nil || run == nil {
		return
	}
	info, ok := gitLabPipelineInfoFromRun(run)
	if !ok {
		info, ok = decodeGitLabExternalRef(run.GitCommit)
	}
	if !ok || info.ID <= 0 {
		return
	}

	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).First(&pipeline, run.PipelineID).Error; err != nil || pipeline.GitRepoID == nil || *pipeline.GitRepoID == 0 {
		return
	}
	if !isGitLabRunnerPipeline(&pipeline) {
		return
	}
	var repo models.GitRepository
	if err := s.db.WithContext(ctx).First(&repo, *pipeline.GitRepoID).Error; err != nil {
		return
	}

	jobs, err := s.ciProvider.gitLabPipelineJobs(ctx, &repo, run.GitBranch, info.ID)
	if err != nil {
		logger.L().WithField("run_id", run.ID).WithError(err).Warn("同步 GitLab Runner Job 列表失败")
		s.ensureSyntheticGitLabLog(ctx, run, info, err.Error())
		return
	}
	if len(jobs) == 0 {
		s.ensureSyntheticGitLabLog(ctx, run, info, "GitLab Pipeline 暂无 Job 日志")
		return
	}

	for _, job := range jobs {
		stageID := sanitizeGitLabIdentifier(firstNonEmptyString(job.Stage, "gitlab"))
		stageName := firstNonEmptyString(job.Stage, "GitLab")
		stageStatus := gitLabStatusToRunStatus(job.Status)
		stage := s.upsertStageRun(ctx, run.ID, stageID, stageName, stageStatus, job.StartedAt, job.FinishedAt)
		if stage == nil {
			continue
		}
		trace, traceErr := s.ciProvider.gitLabJobTrace(ctx, &repo, run.GitBranch, job.ID)
		if traceErr != nil {
			trace = traceErr.Error()
		}
		if strings.TrimSpace(trace) == "" {
			trace = fmt.Sprintf("GitLab Job #%d (%s) 当前暂无 trace，可打开 GitLab 查看：%s", job.ID, job.Name, job.WebURL)
		}
		stepID := fmt.Sprintf("gitlab-job-%d", job.ID)
		s.upsertStepRun(ctx, stage.ID, stepID, firstNonEmptyString(job.Name, stepID), "gitlab_job", stageStatus, trace, job.StartedAt, job.FinishedAt)
	}
}

func (s *RunService) ensureSyntheticGitLabLog(ctx context.Context, run *models.PipelineRun, info gitLabPipelineInfo, msg string) {
	stage := s.upsertStageRun(ctx, run.ID, "gitlab", "GitLab Runner", run.Status, run.StartedAt, run.FinishedAt)
	if stage == nil {
		return
	}
	logs := strings.TrimSpace(msg)
	if logs == "" {
		logs = "GitLab Runner 日志尚未同步"
	}
	if info.WebURL != "" {
		logs += "\nGitLab Pipeline: " + info.WebURL
	}
	s.upsertStepRun(ctx, stage.ID, "gitlab-pipeline", "GitLab Pipeline", "gitlab_pipeline", run.Status, logs, run.StartedAt, run.FinishedAt)
}

func (s *RunService) upsertStageRun(ctx context.Context, runID uint, stageID, stageName, status string, startedAt, finishedAt *time.Time) *models.StageRun {
	var stage models.StageRun
	err := s.db.WithContext(ctx).Where("pipeline_run_id = ? AND stage_id = ?", runID, stageID).First(&stage).Error
	if err != nil {
		stage = models.StageRun{
			PipelineRunID: runID,
			StageID:       stageID,
			StageName:     stageName,
			Status:        status,
			StartedAt:     startedAt,
			FinishedAt:    finishedAt,
		}
		if createErr := s.db.WithContext(ctx).Create(&stage).Error; createErr != nil {
			logger.L().WithField("run_id", runID).WithError(createErr).Warn("创建 GitLab Runner 阶段日志失败")
			return nil
		}
		return &stage
	}
	updates := map[string]interface{}{
		"stage_name":  stageName,
		"status":      status,
		"started_at":  startedAt,
		"finished_at": finishedAt,
	}
	if err := s.db.WithContext(ctx).Model(&stage).Updates(updates).Error; err != nil {
		logger.L().WithField("stage_run_id", stage.ID).WithError(err).Warn("更新 GitLab Runner 阶段日志失败")
	}
	return &stage
}

func (s *RunService) upsertStepRun(ctx context.Context, stageRunID uint, stepID, stepName, stepType, status, logs string, startedAt, finishedAt *time.Time) {
	var step models.StepRun
	err := s.db.WithContext(ctx).Where("stage_run_id = ? AND step_id = ?", stageRunID, stepID).First(&step).Error
	updates := map[string]interface{}{
		"step_name":   stepName,
		"step_type":   stepType,
		"status":      status,
		"logs":        logs,
		"started_at":  startedAt,
		"finished_at": finishedAt,
	}
	if err != nil {
		step = models.StepRun{
			StageRunID: stageRunID,
			StepID:     stepID,
			StepName:   stepName,
			StepType:   stepType,
			Status:     status,
			Logs:       logs,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
		}
		if createErr := s.db.WithContext(ctx).Create(&step).Error; createErr != nil {
			logger.L().WithField("stage_run_id", stageRunID).WithError(createErr).Warn("创建 GitLab Runner 步骤日志失败")
		}
		return
	}
	if err := s.db.WithContext(ctx).Model(&step).Updates(updates).Error; err != nil {
		logger.L().WithField("step_run_id", step.ID).WithError(err).Warn("更新 GitLab Runner 步骤日志失败")
	}
}

func (s *RunService) refreshGitLabRunStatus(ctx context.Context, run *models.PipelineRun) (*models.PipelineRun, bool) {
	if run == nil || run.Status == "success" || run.Status == "failed" || run.Status == "cancelled" {
		return run, false
	}
	info, ok := gitLabPipelineInfoFromRun(run)
	if !ok {
		info, ok = decodeGitLabExternalRef(run.GitCommit)
	}
	if !ok || info.ID <= 0 {
		return run, false
	}

	var pipeline models.Pipeline
	if err := s.db.WithContext(ctx).First(&pipeline, run.PipelineID).Error; err != nil || pipeline.GitRepoID == nil {
		return run, false
	}
	var repo models.GitRepository
	if err := s.db.WithContext(ctx).First(&repo, *pipeline.GitRepoID).Error; err != nil {
		return run, false
	}

	latest, err := s.ciProvider.gitLabPipelineStatus(ctx, &repo, run.GitBranch, info.ID)
	if err != nil {
		logger.L().WithField("run_id", run.ID).WithError(err).Warn("刷新 GitLab Pipeline 状态失败")
		return run, false
	}

	status := gitLabStatusToRunStatus(latest.Status)
	updates := map[string]interface{}{"status": status}
	if latest.SHA != "" {
		updates["git_message"] = firstNonEmptyString(latest.WebURL, run.GitMessage)
	}
	if gitLabStatusIsTerminal(latest.Status) && run.FinishedAt == nil {
		now := time.Now()
		updates["finished_at"] = &now
		if run.StartedAt != nil {
			updates["duration"] = int(now.Sub(*run.StartedAt).Seconds())
		}
	}

	if err := s.db.WithContext(ctx).Model(run).Updates(updates).Error; err != nil {
		logger.L().WithField("run_id", run.ID).WithError(err).Warn("保存 GitLab Pipeline 状态失败")
		return run, false
	}
	_ = s.db.WithContext(ctx).Model(&pipeline).Updates(map[string]interface{}{"last_run_status": status}).Error
	_ = s.db.WithContext(ctx).First(run, run.ID).Error
	if status == "success" {
		// GitOps handoff calls ArgoCD / Git APIs and can block for a long time; do not hold GET /pipelines/runs/:id.
		runID := run.ID
		pipelineID := pipeline.ID
		go func() {
			ctxBg := context.Background()
			var r models.PipelineRun
			var p models.Pipeline
			if err := s.db.WithContext(ctxBg).First(&r, runID).Error; err != nil {
				return
			}
			if err := s.db.WithContext(ctxBg).First(&p, pipelineID).Error; err != nil {
				return
			}
			s.handleGitLabRunnerGitOpsHandoff(ctxBg, &r, &p)
		}()
	}
	return run, true
}

func (s *RunService) handleGitLabRunnerGitOpsHandoff(ctx context.Context, run *models.PipelineRun, pipeline *models.Pipeline) {
	if s == nil || s.db == nil || run == nil || pipeline == nil {
		return
	}
	if run.GitOpsChangeRequestID != nil && *run.GitOpsChangeRequestID > 0 {
		return
	}
	if strings.TrimSpace(run.GitOpsHandoffStatus) == "failed" {
		return
	}

	env := make(map[string]string)
	var config struct {
		Variables []dto.Variable `json:"variables"`
	}
	if strings.TrimSpace(pipeline.ConfigJSON) != "" {
		_ = json.Unmarshal([]byte(pipeline.ConfigJSON), &config)
	}
	for _, variable := range config.Variables {
		name := strings.TrimSpace(variable.Name)
		if name == "" || variable.IsSecret {
			continue
		}
		env[name] = strings.TrimSpace(variable.Value)
	}
	if run.ParametersJSON != "" {
		params := make(map[string]string)
		if err := json.Unmarshal([]byte(run.ParametersJSON), &params); err == nil {
			for key, value := range params {
				env[key] = value
			}
		}
	}
	if strings.TrimSpace(pipeline.ApplicationName) != "" {
		env["APP_NAME"] = strings.TrimSpace(pipeline.ApplicationName)
		env["APPLICATION_NAME"] = strings.TrimSpace(pipeline.ApplicationName)
	}
	if strings.TrimSpace(pipeline.Env) != "" {
		env["DEPLOY_ENV"] = strings.TrimSpace(pipeline.Env)
	}
	if _, ok := env["IMAGE_TAG"]; !ok && strings.TrimSpace(run.GitCommit) != "" {
		env["IMAGE_TAG"] = shortGitCommit(run.GitCommit)
	}
	if _, ok := env["GITOPS_IMAGE_TAG"]; !ok && strings.TrimSpace(env["IMAGE_TAG"]) != "" {
		env["GITOPS_IMAGE_TAG"] = strings.TrimSpace(env["IMAGE_TAG"])
	}
	if strings.TrimSpace(env["GITOPS_IMAGE_REPOSITORY"]) == "" && strings.TrimSpace(env["IMAGE_NAME"]) != "" {
		env["GITOPS_IMAGE_REPOSITORY"] = strings.TrimSpace(env["IMAGE_NAME"])
	}
	if strings.TrimSpace(env["AUTO_GITOPS_HANDOFF"]) == "" && strings.TrimSpace(env["GITOPS_REPO_ID"]) != "" {
		env["AUTO_GITOPS_HANDOFF"] = "true"
	}
	if !isTruthy(env["AUTO_GITOPS_HANDOFF"]) {
		return
	}

	change, err := NewGitOpsHandoffService(s.db).HandleSuccessfulRun(ctx, run, pipeline, env)
	if err != nil {
		logger.L().WithField("run_id", run.ID).WithError(err).Warn("GitLab Runner 成功后 GitOps 自动交接失败")
		return
	}
	if change != nil {
		logger.L().WithField("run_id", run.ID).WithField("change_request_id", change.ID).Info("GitLab Runner 成功后已自动创建 GitOps 变更请求")
	}
}

func runExternalURL(run *models.PipelineRun) string {
	if run == nil {
		return ""
	}
	if info, ok := gitLabPipelineInfoFromRun(run); ok {
		return firstNonEmptyString(info.WebURL, run.GitMessage)
	}
	if info, ok := decodeGitLabExternalRef(run.GitCommit); ok {
		return firstNonEmptyString(info.WebURL, run.GitMessage)
	}
	return ""
}

func gitLabPipelineInfoFromRun(run *models.PipelineRun) (gitLabPipelineInfo, bool) {
	if run == nil || run.ParametersJSON == "" {
		return gitLabPipelineInfo{}, false
	}
	var params map[string]string
	if err := json.Unmarshal([]byte(run.ParametersJSON), &params); err != nil {
		return gitLabPipelineInfo{}, false
	}
	id, err := strconv.Atoi(params["GITLAB_PIPELINE_ID"])
	if err != nil || id <= 0 {
		return gitLabPipelineInfo{}, false
	}
	iid, _ := strconv.Atoi(params["GITLAB_PIPELINE_IID"])
	return gitLabPipelineInfo{
		ID:     id,
		IID:    iid,
		WebURL: firstNonEmptyString(params["GITLAB_PIPELINE_URL"], run.GitMessage),
		SHA:    run.GitCommit,
	}, true
}

// GetStepLogs 获取步骤日志
func (s *RunService) GetStepLogs(ctx context.Context, stepRunID uint) (*dto.StepLogsResponse, error) {
	var stepRun models.StepRun
	if err := s.db.First(&stepRun, stepRunID).Error; err != nil {
		return nil, err
	}

	return &dto.StepLogsResponse{
		StepID:   stepRun.StepID,
		StepName: stepRun.StepName,
		Logs:     stepRun.Logs,
		Status:   stepRun.Status,
	}, nil
}

// GetActiveBuilderPods 获取活跃的构建 Pod 列表
func (s *RunService) GetActiveBuilderPods() []map[string]interface{} {
	return s.engine.GetActiveBuilderPods()
}

// SetBuilderIdleTimeout 设置构建 Pod 空闲超时时间
func (s *RunService) SetBuilderIdleTimeout(timeout time.Duration) {
	s.engine.SetBuilderIdleTimeout(timeout)
}

// BuilderConfig 构建器配置（别名）
type BuilderConfig = BuilderPodConfig

// GetBuilderConfig 获取构建器配置
func (s *RunService) GetBuilderConfig() *BuilderConfig {
	// 先从数据库加载
	var sysConfig models.SystemConfig
	if err := s.db.Where("`key` = ?", builderConfigKey).First(&sysConfig).Error; err == nil {
		var cfg BuilderConfig
		if json.Unmarshal([]byte(sysConfig.Value), &cfg) == nil {
			// 同步到内存
			s.engine.SetBuilderConfig(&cfg)
			return &cfg
		}
	}
	return s.engine.GetBuilderConfig()
}

// SetBuilderConfig 设置构建器配置
func (s *RunService) SetBuilderConfig(cfg *BuilderConfig) {
	// 保存到内存
	s.engine.SetBuilderConfig(cfg)
	cfg = s.engine.GetBuilderConfig()

	// 持久化到数据库
	cfgJSON, _ := json.Marshal(cfg)
	var sysConfig models.SystemConfig
	err := s.db.Where("`key` = ?", builderConfigKey).First(&sysConfig).Error
	if err != nil {
		// 不存在，创建
		sysConfig = models.SystemConfig{
			Key:         builderConfigKey,
			Value:       string(cfgJSON),
			Description: "构建 Pod 配置",
		}
		if createErr := s.db.Create(&sysConfig).Error; createErr != nil {
			logger.L().WithError(createErr).Error("保存构建器配置失败")
		} else {
			logger.L().Info("构建器配置已保存到数据库")
		}
	} else {
		// 存在，更新
		if updateErr := s.db.Model(&sysConfig).Update("value", string(cfgJSON)).Error; updateErr != nil {
			logger.L().WithError(updateErr).Error("更新构建器配置失败")
		} else {
			logger.L().Info("构建器配置已更新")
		}
	}
}

// DeleteBuilderPod 删除指定的构建 Pod
func (s *RunService) DeleteBuilderPod(ctx context.Context, clusterID uint, namespace, podName string) error {
	return s.engine.DeleteBuilderPod(ctx, clusterID, namespace, podName)
}

// GetStats 获取流水线执行统计
func (s *RunService) GetStats(ctx context.Context, req *dto.PipelineStatsRequest) (*dto.PipelineStatsResponse, error) {
	log := logger.L().WithField("method", "GetStats")

	// 解析日期范围
	startDate, err := time.Parse("2006-01-02", req.StartDate)
	if err != nil {
		startDate = time.Now().AddDate(0, 0, -7) // 默认最近7天
	}
	endDate, err := time.Parse("2006-01-02", req.EndDate)
	if err != nil {
		endDate = time.Now()
	}
	endDate = endDate.Add(24*time.Hour - time.Second) // 包含当天

	log.WithField("start", startDate).WithField("end", endDate).Info("获取流水线统计")

	result := &dto.PipelineStatsResponse{
		StatusDistribution: make(map[string]int),
	}

	// 1. 概览统计
	var total, success, failed int64
	s.db.Model(&models.PipelineRun{}).
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Count(&total)
	s.db.Model(&models.PipelineRun{}).
		Where("created_at BETWEEN ? AND ? AND status = ?", startDate, endDate, "success").
		Count(&success)
	s.db.Model(&models.PipelineRun{}).
		Where("created_at BETWEEN ? AND ? AND status = ?", startDate, endDate, "failed").
		Count(&failed)

	successRate := float64(0)
	if total > 0 {
		successRate = float64(success) / float64(total) * 100
	}
	result.Overview = dto.PipelineStatsOverview{
		Total:       int(total),
		Success:     int(success),
		Failed:      int(failed),
		SuccessRate: successRate,
	}

	// 2. 状态分布
	type StatusCount struct {
		Status string
		Count  int
	}
	var statusCounts []StatusCount
	s.db.Model(&models.PipelineRun{}).
		Select("status, COUNT(*) as count").
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("status").
		Scan(&statusCounts)
	for _, sc := range statusCounts {
		result.StatusDistribution[sc.Status] = sc.Count
	}

	// 3. 每日趋势
	type DailyStats struct {
		Date    string
		Success int
		Failed  int
	}
	var dailyStats []DailyStats
	s.db.Model(&models.PipelineRun{}).
		Select("DATE(created_at) as date, "+
			"SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, "+
			"SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END) as failed").
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("DATE(created_at)").
		Order("date").
		Scan(&dailyStats)

	for _, ds := range dailyStats {
		result.Trend = append(result.Trend, dto.PipelineStatsTrendItem{
			Date:    ds.Date,
			Success: ds.Success,
			Failed:  ds.Failed,
		})
	}

	// 4. 平均耗时趋势
	type DailyDuration struct {
		Date        string
		AvgDuration float64
	}
	var dailyDurations []DailyDuration
	s.db.Model(&models.PipelineRun{}).
		Select("DATE(created_at) as date, AVG(duration) as avg_duration").
		Where("created_at BETWEEN ? AND ? AND duration > 0", startDate, endDate).
		Group("DATE(created_at)").
		Order("date").
		Scan(&dailyDurations)

	for _, dd := range dailyDurations {
		result.DurationTrend = append(result.DurationTrend, dto.PipelineDurationItem{
			Date:     dd.Date,
			Duration: int(dd.AvgDuration),
		})
	}

	// 5. 流水线排行
	type PipelineStats struct {
		PipelineID   uint
		PipelineName string
		Total        int
		Success      int
		AvgDuration  float64
	}
	var pipelineStats []PipelineStats
	s.db.Model(&models.PipelineRun{}).
		Select("pipeline_id, pipeline_name, COUNT(*) as total, "+
			"SUM(CASE WHEN status = 'success' THEN 1 ELSE 0 END) as success, "+
			"AVG(duration) as avg_duration").
		Where("created_at BETWEEN ? AND ?", startDate, endDate).
		Group("pipeline_id, pipeline_name").
		Order("total DESC").
		Limit(10).
		Scan(&pipelineStats)

	for _, ps := range pipelineStats {
		rate := float64(0)
		if ps.Total > 0 {
			rate = float64(ps.Success) / float64(ps.Total) * 100
		}
		avgDur := formatDuration(int(ps.AvgDuration))
		result.Rank = append(result.Rank, dto.PipelineRankItem{
			ID:          ps.PipelineID,
			Name:        ps.PipelineName,
			Total:       ps.Total,
			SuccessRate: rate,
			AvgDuration: avgDur,
		})
	}

	// 6. 最近失败的执行
	var failedRuns []models.PipelineRun
	s.db.Where("status = ? AND created_at BETWEEN ? AND ?", "failed", startDate, endDate).
		Order("created_at DESC").
		Limit(10).
		Find(&failedRuns)

	for _, run := range failedRuns {
		// 获取错误信息（从最后一个失败的步骤）
		var stepRun models.StepRun
		var errorMsg string
		if err := s.db.Joins("JOIN stage_runs ON step_runs.stage_run_id = stage_runs.id").
			Where("stage_runs.pipeline_run_id = ? AND step_runs.status = ?", run.ID, "failed").
			Order("step_runs.id DESC").
			First(&stepRun).Error; err == nil {
			if len(stepRun.Logs) > 200 {
				errorMsg = stepRun.Logs[len(stepRun.Logs)-200:]
			} else {
				errorMsg = stepRun.Logs
			}
		}

		result.RecentFailed = append(result.RecentFailed, dto.PipelineRecentFailedRun{
			ID:           run.ID,
			PipelineID:   run.PipelineID,
			PipelineName: run.PipelineName,
			RunNumber:    int(run.ID),
			Status:       run.Status,
			ErrorMessage: errorMsg,
			CreatedAt:    run.CreatedAt,
		})
	}

	return result, nil
}

// formatDuration 格式化时长
func formatDuration(seconds int) string {
	if seconds < 60 {
		return fmt.Sprintf("%ds", seconds)
	}
	m := seconds / 60
	s := seconds % 60
	if m < 60 {
		return fmt.Sprintf("%dm %ds", m, s)
	}
	h := m / 60
	m = m % 60
	return fmt.Sprintf("%dh %dm", h, m)
}
