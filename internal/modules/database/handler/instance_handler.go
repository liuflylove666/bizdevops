package handler

import (
	"net/http"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"devops/internal/config"
	"devops/internal/models"
	"devops/internal/repository"
	approvalsvc "devops/internal/service/approval"
	"devops/internal/service/database"
	"devops/pkg/ioc"
	"devops/pkg/middleware"
)

func init() {
	ioc.Api.RegisterContainer("DatabaseHandler", &DatabaseApiHandler{})
}

type DatabaseApiHandler struct {
	instance  *InstanceHandler
	console   *ConsoleHandler
	logs      *QueryLogHandler
	ticket    *TicketHandler
	rule      *RuleHandler
	acl       *ACLHandler
	statement *StatementHandler
}

func (h *DatabaseApiHandler) Init() error {
	cfg, _ := config.LoadConfig()
	db := cfg.GetDB()

	instRepo := repository.NewDBInstanceRepository(db)
	logRepo := repository.NewDBQueryLogRepository(db)
	ticketRepo := repository.NewSQLChangeTicketRepository(db)
	stmtRepo := repository.NewSQLChangeStatementRepository(db)
	wfRepo := repository.NewSQLChangeWorkflowRepository(db)
	ruleRepo := repository.NewSQLAuditRuleRepository(db)
	rollbackRepo := repository.NewSQLRollbackRepository(db)
	approvalInstanceRepo := repository.NewApprovalInstanceRepository(db)
	approvalNodeInstanceRepo := repository.NewApprovalNodeInstanceRepository(db)
	connector := database.NewConnector(instRepo)
	instSvc := database.NewInstanceService(instRepo, connector)
	inspector := database.NewSchemaInspector(connector)
	console := database.NewConsole(connector, logRepo)
	rollbackBuilder := database.NewRollbackBuilder(rollbackRepo)
	executor := database.NewExecutor(connector, ticketRepo, stmtRepo, rollbackBuilder)
	if os.Getenv("GH_OST_ENABLED") == "1" {
		executor.SetGhost(database.NewGhostExecutor(database.GhostConfig{
			Enabled:  true,
			BinPath:  os.Getenv("GH_OST_BIN"),
			ExtraArg: os.Getenv("GH_OST_EXTRA"),
		}, stmtRepo))
	}
	builtin := database.NewBuiltinAuditor()
	var auditor database.Auditor = builtin
	if addr := os.Getenv("YEARNING_ENGINE_RPC"); addr != "" {
		auditor = database.NewEngineRPCAuditor(database.EngineAuditorConfig{Addr: addr, Fallback: builtin})
	}
	ticketSvc := database.NewTicketService(ticketRepo, stmtRepo, wfRepo, ruleRepo, auditor, executor)
	ticketSvc.SetRollbackRepo(rollbackRepo)
	ticketSvc.SetApprovalFlow(
		approvalInstanceRepo,
		approvalNodeInstanceRepo,
		approvalsvc.NewApproverResolver(db),
	)
	ruleSvc := database.NewRuleService(ruleRepo)
	aclRepo := repository.NewDBInstanceACLRepository(db)
	aclSvc := database.NewACLService(aclRepo, instRepo)

	database.NewTicketScheduler(db, ticketSvc).Start()

	h.instance = &InstanceHandler{svc: instSvc, inspector: inspector, db: db, acl: aclSvc}
	h.console = &ConsoleHandler{console: console}
	h.logs = &QueryLogHandler{repo: logRepo}
	h.ticket = &TicketHandler{svc: ticketSvc}
	h.rule = &RuleHandler{svc: ruleSvc}
	h.acl = &ACLHandler{svc: aclSvc}
	h.statement = &StatementHandler{repo: stmtRepo}

	root := cfg.Application.GinRootRouter().Group("database")
	root.Use(middleware.AuthMiddleware())
	h.Register(root)
	return nil
}

func (h *DatabaseApiHandler) Register(r gin.IRouter) {
	inst := r.Group("/instance")
	{
		inst.GET("", h.instance.List)
		inst.GET("/all", h.instance.ListAll)
		inst.GET("/:id", h.instance.Get)
		inst.POST("", h.instance.Create)
		inst.PUT("/:id", h.instance.Update)
		inst.DELETE("/:id", h.instance.Delete)
		inst.POST("/test", h.instance.Test)
		inst.POST("/:id/test", h.instance.TestExisting)
		inst.GET("/:id/databases", h.instance.ListDatabases)
		inst.GET("/:id/tables", h.instance.ListTables)
		inst.GET("/:id/columns", h.instance.ListColumns)
		inst.GET("/:id/indexes", h.instance.ListIndexes)
	}
	r.POST("/query/execute", h.console.Execute)
	r.GET("/logs", h.logs.List)
	h.ticket.Register(r)
	h.rule.Register(r)
	h.acl.Register(r)
	h.statement.Register(r)
}

// =========================

type InstanceHandler struct {
	svc       *database.InstanceService
	inspector *database.SchemaInspector
	db        *gorm.DB
	acl       *database.ACLService
}

type instanceReq struct {
	models.DBInstance
	PlainPassword string `json:"plain_password"`
}

func (h *InstanceHandler) resolveRoleIDs(c *gin.Context) []uint {
	userID := c.GetUint("user_id")
	if userID == 0 {
		return nil
	}
	var ids []uint
	h.db.Table("user_roles").Where("user_id = ?", userID).Pluck("role_id", &ids)
	return ids
}

func (h *InstanceHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	f := repository.DBInstanceFilter{
		Env:    c.Query("env"),
		DBType: c.Query("db_type"),
		Status: c.Query("status"),
		Name:   c.Query("name"),
	}
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	roleIDs := h.resolveRoleIDs(c)
	allowedIDs, isAll := h.acl.AccessibleInstanceIDs(c.Request.Context(), userID, role, roleIDs)
	if isAll {
		list, total, err := h.svc.List(c.Request.Context(), f, page, pageSize)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
		return
	}
	if len(allowedIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": []any{}, "total": 0}})
		return
	}
	list, total, err := h.svc.ListFiltered(c.Request.Context(), f, allowedIDs, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}

func (h *InstanceHandler) ListAll(c *gin.Context) {
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	roleIDs := h.resolveRoleIDs(c)
	allowedIDs, isAll := h.acl.AccessibleInstanceIDs(c.Request.Context(), userID, role, roleIDs)
	if isAll {
		list, err := h.svc.ListAll(c.Request.Context())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
		return
	}
	if len(allowedIDs) == 0 {
		c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": []any{}})
		return
	}
	list, err := h.svc.ListAllFiltered(c.Request.Context(), allowedIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

func (h *InstanceHandler) Get(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m, err := h.svc.Get(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": 404, "message": "Not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": m})
}

func (h *InstanceHandler) Create(c *gin.Context) {
	var req instanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	if uid := c.GetUint("user_id"); uid > 0 {
		req.DBInstance.CreatedBy = &uid
	}
	m := req.DBInstance
	m.ID = 0
	if err := h.svc.Create(c.Request.Context(), &m, req.PlainPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": m})
}

func (h *InstanceHandler) Update(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	var req instanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	m := req.DBInstance
	m.ID = uint(id)
	if err := h.svc.Update(c.Request.Context(), &m, req.PlainPassword); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": m})
}

func (h *InstanceHandler) Delete(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	if err := h.svc.Delete(c.Request.Context(), uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success"})
}

func (h *InstanceHandler) Test(c *gin.Context) {
	var req instanceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	m := req.DBInstance
	if err := h.svc.Test(c.Request.Context(), &m, req.PlainPassword); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "连接成功"})
}

func (h *InstanceHandler) TestExisting(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	m := &models.DBInstance{}
	m.ID = uint(id)
	if err := h.svc.Test(c.Request.Context(), m, ""); err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "连接成功"})
}

func (h *InstanceHandler) ListDatabases(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	list, err := h.inspector.Databases(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	userID := c.GetUint("user_id")
	role := c.GetString("role")
	roleIDs := h.resolveRoleIDs(c)
	allowed, isAll := h.acl.AccessibleSchemas(c.Request.Context(), userID, role, roleIDs, uint(id))
	if !isAll && allowed != nil {
		set := make(map[string]struct{}, len(allowed))
		for _, s := range allowed {
			set[s] = struct{}{}
		}
		filtered := make([]string, 0, len(allowed))
		for _, name := range list {
			if _, ok := set[name]; ok {
				filtered = append(filtered, name)
			}
		}
		list = filtered
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

func (h *InstanceHandler) ListTables(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	schema := c.Query("schema")
	if schema == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "schema 参数必填"})
		return
	}
	list, err := h.inspector.Tables(c.Request.Context(), uint(id), schema)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

func (h *InstanceHandler) ListColumns(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	schema := c.Query("schema")
	table := c.Query("table")
	if schema == "" || table == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "schema/table 参数必填"})
		return
	}
	list, err := h.inspector.Columns(c.Request.Context(), uint(id), schema, table)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

func (h *InstanceHandler) ListIndexes(c *gin.Context) {
	id, _ := strconv.ParseUint(c.Param("id"), 10, 64)
	schema := c.Query("schema")
	table := c.Query("table")
	if schema == "" || table == "" {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": "schema/table 参数必填"})
		return
	}
	list, err := h.inspector.Indexes(c.Request.Context(), uint(id), schema, table)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": list})
}

// =========================

type ConsoleHandler struct {
	console *database.Console
}

func (h *ConsoleHandler) Execute(c *gin.Context) {
	var req database.QueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": 400, "message": err.Error()})
		return
	}
	user := c.GetString("username")
	if user == "" {
		user = "unknown"
	}
	res, err := h.console.Execute(c.Request.Context(), user, &req)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": res})
}

// =========================

type QueryLogHandler struct {
	repo *repository.DBQueryLogRepository
}

func (h *QueryLogHandler) List(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	instanceID, _ := strconv.ParseUint(c.Query("instance_id"), 10, 64)
	f := repository.DBQueryLogFilter{
		InstanceID: uint(instanceID),
		Username:   c.Query("username"),
		Status:     c.Query("status"),
	}
	list, total, err := h.repo.List(c.Request.Context(), f, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "Success", "data": gin.H{"list": list, "total": total}})
}
