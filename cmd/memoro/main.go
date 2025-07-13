package main

import (
	"context"
	"flag"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/gin-gonic/gin"
	"memoro/internal/config"
	"memoro/internal/handlers"
	"memoro/internal/logger"
)

func main() {
	// 解析命令行参数
	configPath := flag.String("config", "./config/app.yaml", "Configuration file path")
	flag.Parse()

	// 初始化日志器
	mainLogger := logger.NewLogger("main")

	// 加载配置
	cfg, err := config.Load(*configPath)
	if err != nil {
		mainLogger.Error("Failed to load configuration", logger.Fields{
			"error": err.Error(),
			"config_path": *configPath,
		})
		os.Exit(1)
	}

	mainLogger.Info("Application starting", logger.Fields{
		"version": "v0.1.0",
		"mode": cfg.Server.Mode,
		"port": cfg.Server.Port,
		"config_path": *configPath,
	})

	// 设置Gin模式
	if config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// 创建Gin实例
	r := gin.New()
	
	// 添加中间件
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	// 注册路由
	setupRoutes(r)

	// 创建HTTP服务器
	serverAddr := config.GetServerAddress()
	srv := &http.Server{
		Addr:         serverAddr,
		Handler:      r,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
	}

	// 在goroutine中启动服务器
	go func() {
		mainLogger.Info("HTTP server starting", logger.Fields{
			"address": serverAddr,
			"read_timeout": cfg.Server.ReadTimeout,
			"write_timeout": cfg.Server.WriteTimeout,
		})
		
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			mainLogger.Error("Failed to start HTTP server", logger.Fields{
				"error": err.Error(),
				"address": serverAddr,
			})
			os.Exit(1)
		}
	}()

	// 等待中断信号优雅关闭服务器
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	
	mainLogger.Info("Shutting down server...")

	// 使用配置的超时时间进行优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Server.ShutdownTimeout)
	defer cancel()
	
	if err := srv.Shutdown(ctx); err != nil {
		mainLogger.Error("Server forced to shutdown", logger.Fields{
			"error": err.Error(),
			"timeout": cfg.Server.ShutdownTimeout,
		})
		os.Exit(1)
	}

	mainLogger.Info("Server exited gracefully")
}

// setupRoutes 设置路由
func setupRoutes(r *gin.Engine) {
	// API v1 路由组
	v1 := r.Group("/api/v1")
	{
		// 健康检查
		v1.GET("/health", handlers.HealthHandler)
		
		// 预留其他API端点
		// TODO: 添加内容管理API
		// TODO: 添加搜索API
		// TODO: 添加WebHook API
	}

	// 直接健康检查路由 (向后兼容)
	r.GET("/health", handlers.HealthHandler)
}