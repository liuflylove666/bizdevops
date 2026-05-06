package handler

import (
	"strconv"

	"github.com/gin-gonic/gin"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/repository"
	"devops/internal/service/jira"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
	"devops/pkg/response"
)

func init() {
	ioc.Api.RegisterContainer("JiraHandler", &JiraApiHandler{})
}

type JiraApiHandler struct {
	handler *JiraHandler
}

func (h *JiraApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	instRepo := repository.NewJiraInstanceRepository(db)
	mappingRepo := repository.NewJiraProjectMappingRepository(db)
	svc := jira.NewService(instRepo, mappingRepo)

	h.handler = &JiraHandler{svc: svc}

	root := cfg.Application.GinRootRouter().Group("jira")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *JiraApiHandler) Register(r gin.IRouter) {
	// 实例管理
	r.GET("/instances", h.handler.ListInstances)
	r.POST("/instances", middleware.RequireAdmin(), h.handler.CreateInstance)
	r.PUT("/instances/:id", middleware.RequireAdmin(), h.handler.UpdateInstance)
	r.DELETE("/instances/:id", middleware.RequireAdmin(), h.handler.DeleteInstance)
	r.POST("/instances/:id/test", h.handler.TestConnection)

	// 项目映射
	r.GET("/instances/:id/mappings", h.handler.ListMappings)
	r.POST("/instances/:id/mappings", middleware.RequireAdmin(), h.handler.CreateMapping)
	r.PUT("/mappings/:id", middleware.RequireAdmin(), h.handler.UpdateMapping)
	r.DELETE("/mappings/:id", middleware.RequireAdmin(), h.handler.DeleteMapping)

	// Jira API 代理
	r.GET("/instances/:id/projects", h.handler.ListProjects)
	r.GET("/instances/:id/issues", h.handler.SearchIssues)
	r.GET("/instances/:id/issues/:key", h.handler.GetIssue)
	r.GET("/instances/:id/boards", h.handler.GetBoards)
	r.GET("/instances/:id/boards/:boardId/sprints", h.handler.GetSprints)
	r.GET("/instances/:id/sprints/:sprintId/issues", h.handler.GetSprintIssues)
	r.POST("/instances/:id/issues/:key/comment", h.handler.AddComment)
	r.POST("/instances/:id/issues/:key/transition", h.handler.TransitionIssue)
	r.GET("/instances/:id/issues/:key/transitions", h.handler.GetTransitions)
}

type JiraHandler struct {
	svc *jira.Service
}

func parseID(c *gin.Context) uint {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	return uint(id)
}

// --- Instance CRUD ---

func (h *JiraHandler) ListInstances(c *gin.Context) {
	list, err := h.svc.ListInstances()
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *JiraHandler) CreateInstance(c *gin.Context) {
	var inst models.JiraInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = 0
	if err := h.svc.CreateInstance(&inst); err != nil {
		response.InternalError(c, "创建失败: "+err.Error())
		return
	}
	inst.Token = "******"
	response.Success(c, inst)
}

func (h *JiraHandler) UpdateInstance(c *gin.Context) {
	var inst models.JiraInstance
	if err := c.ShouldBindJSON(&inst); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	inst.ID = parseID(c)
	if err := h.svc.UpdateInstance(&inst); err != nil {
		response.InternalError(c, "更新失败: "+err.Error())
		return
	}
	inst.Token = "******"
	response.Success(c, inst)
}

func (h *JiraHandler) DeleteInstance(c *gin.Context) {
	if err := h.svc.DeleteInstance(parseID(c)); err != nil {
		response.InternalError(c, "删除失败: "+err.Error())
		return
	}
	response.OK(c)
}

func (h *JiraHandler) TestConnection(c *gin.Context) {
	if err := h.svc.TestConnection(parseID(c)); err != nil {
		response.InternalError(c, "连接失败: "+err.Error())
		return
	}
	response.SuccessWithMessage(c, "连接成功", nil)
}

// --- Mapping CRUD ---

func (h *JiraHandler) ListMappings(c *gin.Context) {
	list, err := h.svc.ListMappings(parseID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, list)
}

func (h *JiraHandler) CreateMapping(c *gin.Context) {
	var m models.JiraProjectMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	m.ID = 0
	m.JiraInstanceID = parseID(c)
	if err := h.svc.CreateMapping(&m); err != nil {
		response.InternalError(c, "创建失败: "+err.Error())
		return
	}
	response.Success(c, m)
}

func (h *JiraHandler) UpdateMapping(c *gin.Context) {
	var m models.JiraProjectMapping
	if err := c.ShouldBindJSON(&m); err != nil {
		response.BadRequest(c, "参数错误: "+err.Error())
		return
	}
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m.ID = uint(id)
	if err := h.svc.UpdateMapping(&m); err != nil {
		response.InternalError(c, "更新失败: "+err.Error())
		return
	}
	response.Success(c, m)
}

func (h *JiraHandler) DeleteMapping(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.DeleteMapping(uint(id)); err != nil {
		response.InternalError(c, "删除失败: "+err.Error())
		return
	}
	response.OK(c)
}

// --- Jira API proxy ---

func (h *JiraHandler) ListProjects(c *gin.Context) {
	projects, err := h.svc.ListProjects(parseID(c))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, projects)
}

func (h *JiraHandler) SearchIssues(c *gin.Context) {
	jql := c.Query("jql")
	startAt, _ := strconv.Atoi(c.DefaultQuery("start_at", "0"))
	maxResults, _ := strconv.Atoi(c.DefaultQuery("max_results", "20"))
	result, err := h.svc.SearchIssues(parseID(c), jql, startAt, maxResults)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) GetIssue(c *gin.Context) {
	result, err := h.svc.GetIssue(parseID(c), c.Param("key"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) GetBoards(c *gin.Context) {
	result, err := h.svc.GetBoards(parseID(c), c.Query("project_key"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) GetSprints(c *gin.Context) {
	boardID, _ := strconv.Atoi(c.Param("boardId"))
	result, err := h.svc.GetSprints(parseID(c), boardID, c.Query("state"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) GetSprintIssues(c *gin.Context) {
	sprintID, _ := strconv.Atoi(c.Param("sprintId"))
	startAt, _ := strconv.Atoi(c.DefaultQuery("start_at", "0"))
	maxResults, _ := strconv.Atoi(c.DefaultQuery("max_results", "50"))
	result, err := h.svc.GetSprintIssues(parseID(c), sprintID, startAt, maxResults)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) AddComment(c *gin.Context) {
	var body struct {
		Comment string `json:"comment"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.Comment == "" {
		response.BadRequest(c, "请输入评论内容")
		return
	}
	result, err := h.svc.AddComment(parseID(c), c.Param("key"), body.Comment)
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}

func (h *JiraHandler) TransitionIssue(c *gin.Context) {
	var body struct {
		TransitionID string `json:"transition_id"`
	}
	if err := c.ShouldBindJSON(&body); err != nil || body.TransitionID == "" {
		response.BadRequest(c, "请指定 transition_id")
		return
	}
	if err := h.svc.TransitionIssue(parseID(c), c.Param("key"), body.TransitionID); err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.OK(c)
}

func (h *JiraHandler) GetTransitions(c *gin.Context) {
	result, err := h.svc.GetTransitions(parseID(c), c.Param("key"))
	if err != nil {
		response.InternalError(c, err.Error())
		return
	}
	response.Success(c, result)
}
