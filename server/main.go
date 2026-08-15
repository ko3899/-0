package main

import (
	"log"
	"net/http"

	"hotel-management/server/internal/router"
)

func main() {
	r := router.New()

	addr := ":8080"
	log.Printf("酒店管理系统后端启动，监听 %s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatalf("服务启动失败: %v", err)
	}
}
