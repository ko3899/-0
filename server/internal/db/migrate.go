package db

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// RunMigrations 执行 dir 目录下所有未应用的 SQL 迁移（按文件名排序）。
// 用 schema_migrations 表跟踪已应用版本，每个迁移在一个事务内执行并记录，保证原子性。
func RunMigrations(ctx context.Context, p *pgxpool.Pool, dir string) error {
	// 确保迁移记录表存在
	if _, err := p.Exec(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
	)`); err != nil {
		return fmt.Errorf("创建迁移记录表失败: %w", err)
	}

	// 收集迁移文件
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("读取迁移目录失败: %w", err)
	}
	var files []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)

	for _, f := range files {
		// 已应用则跳过
		var exists bool
		if err := p.QueryRow(ctx,
			`SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, f,
		).Scan(&exists); err != nil {
			return fmt.Errorf("查询迁移状态失败 %s: %w", f, err)
		}
		if exists {
			continue
		}

		sqlBytes, err := os.ReadFile(filepath.Join(dir, f))
		if err != nil {
			return fmt.Errorf("读取迁移文件失败 %s: %w", f, err)
		}

		// 迁移执行 + 记录在同一事务内
		tx, err := p.Begin(ctx)
		if err != nil {
			return fmt.Errorf("开启迁移事务失败 %s: %w", f, err)
		}
		if _, err := tx.Exec(ctx, string(sqlBytes)); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("执行迁移失败 %s: %w", f, err)
		}
		if _, err := tx.Exec(ctx,
			`INSERT INTO schema_migrations(version) VALUES($1)`, f,
		); err != nil {
			_ = tx.Rollback(ctx)
			return fmt.Errorf("记录迁移失败 %s: %w", f, err)
		}
		if err := tx.Commit(ctx); err != nil {
			return fmt.Errorf("提交迁移失败 %s: %w", f, err)
		}
	}
	return nil
}
