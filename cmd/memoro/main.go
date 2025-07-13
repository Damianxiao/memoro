package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
)

func main() {
	// 设置Gin模式
	gin.SetMode(gin.ReleaseMode)

	// 创建Gin实例
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 基础健康检查路由
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status": "ok",
			"timestamp": time.Now().Unix(),
		})
	})

	// 启动服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: r,
	}

	// 在goroutine中启动服务器
	go func() {
		log.Printf("Server starting on :8080")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	// 5秒超时关闭
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Fatal("Server forced to shutdown:", err)
	}

	log.Println("Server exited")
}