package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse 健康检查响应结构
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp int64  `json:"timestamp"`
}

// HealthHandler 健康检查处理器
func HealthHandler(c *gin.Context) {
	response := HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().Unix(),
	}
	
	c.JSON(http.StatusOK, response)
}

// RegisterHealthRoutes 注册健康检查路由
func RegisterHealthRoutes(r *gin.Engine) {
	r.GET("/health", HealthHandler)
}