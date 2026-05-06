package database

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"devops/internal/domain/database/model"
	dbrepo "devops/internal/domain/database/repository"
	"devops/pkg/logger"
)

// GhostConfig gh-ost 集成配置
type GhostConfig struct {
	Enabled  bool   // 未开启时直接回退
	BinPath  string // gh-ost 可执行路径，默认 "gh-ost"
	ExtraArg string // 额外追加的参数，如 "--chunk-size=1000 --max-load=Threads_running=25"
}

// GhostExecutor 把对大表的 ALTER TABLE 改为 gh-ost 执行
type GhostExecutor struct {
	cfg     GhostConfig
	stmtR   *dbrepo.SQLChangeStatementRepository
}

func NewGhostExecutor(cfg GhostConfig, stmtR *dbrepo.SQLChangeStatementRepository) *GhostExecutor {
	if cfg.BinPath == "" {
		cfg.BinPath = "gh-ost"
	}
	return &GhostExecutor{cfg: cfg, stmtR: stmtR}
}

// CanHandle 粗判 SQL 是否是 ALTER TABLE
func (e *GhostExecutor) CanHandle(sql string) bool {
	if !e.cfg.Enabled {
		return false
	}
	up := strings.ToUpper(strings.TrimSpace(sql))
	return strings.HasPrefix(up, "ALTER TABLE")
}

var reAlterTable = regexp.MustCompile(`(?is)^\s*ALTER\s+TABLE\s+` +
	"`?([A-Za-z0-9_]+)`?" + `(?:\.` + "`?([A-Za-z0-9_]+)`?" + `)?\s+(.+)$`)

// Run 执行单条 ALTER TABLE。返回的 err 为 nil 即代表成功。
// gh-ost 需要目标实例开启 row binlog 并授予副本账户相关权限；失败即视作执行失败。
func (e *GhostExecutor) Run(
	ctx context.Context,
	inst *model.DBInstance,
	plainPassword string,
	schema string,
	stmt *model.SQLChangeStatement,
) error {
	m := reAlterTable.FindStringSubmatch(stmt.SQLText)
	if len(m) < 4 {
		return fmt.Errorf("ALTER TABLE 无法解析")
	}
	db := schema
	table := trimIdent(m[1])
	if m[2] != "" {
		db = trimIdent(m[1])
		table = trimIdent(m[2])
	}
	alter := strings.TrimSuffix(strings.TrimSpace(m[3]), ";")

	args := []string{
		"--execute",
		"--allow-on-master",
		"--host=" + inst.Host,
		fmt.Sprintf("--port=%d", inst.Port),
		"--user=" + inst.Username,
		"--password=" + plainPassword,
		"--database=" + db,
		"--table=" + table,
		"--alter=" + alter,
		"--assume-rbr",
		"--switch-to-rbr=false",
	}
	if e.cfg.ExtraArg != "" {
		args = append(args, strings.Fields(e.cfg.ExtraArg)...)
	}

	log := logger.L().WithField("service", "gh-ost").WithField("table", db+"."+table)
	log.Info("启动 gh-ost")
	start := time.Now()

	cmd := exec.CommandContext(ctx, e.cfg.BinPath, args...)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	cmd.Stderr = cmd.Stdout
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("gh-ost 启动失败: %w", err)
	}

	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	var last string
	for scanner.Scan() {
		last = scanner.Text()
		log.Debug(last)
	}

	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("gh-ost 执行失败: %w (最后一行: %s)", err, last)
	}

	elapsed := int(time.Since(start) / time.Millisecond)
	executedAt := time.Now()
	return e.stmtR.UpdateFields(ctx, stmt.ID, map[string]any{
		"state":       "success",
		"exec_ms":     elapsed,
		"executed_at": executedAt,
	})
}
