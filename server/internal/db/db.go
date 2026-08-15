// Package db 提供 PostgreSQL 连接池管理与迁移执行。
package db

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

var pool *pgxpool.Pool

// Config 数据库连接配置。
type Config struct {
	Host     string
	Port     string
	User     string
	Password string
	Name     string
}

// ConfigFromEnv 从环境变量读取配置，缺省用本地开发默认值。
func ConfigFromEnv() Config {
	return Config{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnv("DB_PORT", "5432"),
		User:     getEnv("DB_USER", "hotel"),
		Password: getEnv("DB_PASSWORD", "hotel123"),
		Name:     getEnv("DB_NAME", "hotel_management"),
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// Init 建立 pgx 连接池并验证连通性。
func Init(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	dsn := fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Name)

	config, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("解析数据库配置失败: %w", err)
	}
	config.MaxConns = 10
	config.MinConns = 1
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err = pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, fmt.Errorf("创建连接池失败: %w", err)
	}

	// 验证连接
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("数据库连接失败: %w", err)
	}
	return pool, nil
}

// Pool 返回全局连接池（未初始化时为 nil）。
func Pool() *pgxpool.Pool {
	return pool
}

// Close 关闭全局连接池。
func Close() {
	if pool != nil {
		pool.Close()
		pool = nil
	}
}
