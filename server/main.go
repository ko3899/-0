package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"hotel-management/server/internal/db"
	"hotel-management/server/internal/router"
)

func main() {
	// 数据库初始化：失败不阻断服务启动（仅记录警告），便于无库环境做健康检查
	initDB()

	r := router.New()

	addr := ":8080"
	if p := os.Getenv("PORT"); p != "" {
		addr = ":" + p
	}
	log.Printf("酒店管理系统后端启动，监听 %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}

// initDB 初始化连接池并执行数据库迁移。
// 连接/迁移失败仅记录警告，不影响服务启动。
func initDB() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := db.Init(ctx, db.ConfigFromEnv())
	if err != nil {
		log.Printf("警告：数据库连接失败（业务接口暂不可用）: %v", err)
		return
	}
	log.Printf("数据库连接成功")

	migrateCtx, migrateCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer migrateCancel()

	dir := os.Getenv("MIGRATIONS_DIR")
	if dir == "" {
		dir = "migrations"
	}
	if err := db.RunMigrations(migrateCtx, pool, dir); err != nil {
		log.Printf("警告：数据库迁移失败: %v", err)
		return
	}
	log.Printf("数据库迁移完成")

	seedCtx, seedCancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer seedCancel()
	if err := db.Seed(seedCtx, pool); err != nil {
		log.Printf("警告：演示数据注入失败: %v", err)
	}
}
