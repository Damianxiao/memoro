package handlers

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"memoro/internal/logger"
	"memoro/internal/models"
	"memoro/internal/services/vector"
)

// SearchHandler 搜索API处理器
type SearchHandler struct {
	searchEngine SearchEngineInterface
	logger       *logger.Logger
}

// SearchEngineInterface 搜索引擎接口
type SearchEngineInterface interface {
	Search(ctx context.Context, options *vector.SearchOptions) (*vector.SearchResponse, error)
	GetSearchStats(ctx context.Context) (map[string]interface{}, error)
	Close() error
}

// NewSearchHandler 创建搜索处理器
func NewSearchHandler(searchEngine SearchEngineInterface) *SearchHandler {
	return &SearchHandler{
		searchEngine: searchEngine,
		logger:       logger.NewLogger("search-handler"),
	}
}

// Search 执行语义搜索
// @Summary 语义搜索
// @Description 基于向量相似度的智能内容搜索
// @Tags search
// @Accept json
// @Produce json
// @Param request body SearchRequest true "搜索请求"
// @Success 200 {object} SearchResponse "搜索成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/search [post]
func (h *SearchHandler) Search(c *gin.Context) {
	startTime := time.Now()

	// 解析请求
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid search request", logger.Fields{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request parameters: " + err.Error(),
		})
		return
	}

	// 验证必需参数
	if req.Query == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Query parameter is required",
		})
		return
	}

	// 设置默认值
	if req.TopK <= 0 {
		req.TopK = 10
	}
	if req.MinSimilarity <= 0 {
		req.MinSimilarity = 0.7
	}

	// 构建搜索选项
	searchOptions := &vector.SearchOptions{
		Query:           req.Query,
		TopK:            req.TopK,
		MinSimilarity:   float32(req.MinSimilarity),
		ContentTypes:    stringSliceToContentTypes(req.ContentTypes),
		UserID:          req.UserID,
		IncludeContent:  true,
		SimilarityType:  vector.SimilarityTypeCosine,
	}

	// 执行搜索
	if h.searchEngine == nil {
		h.logger.Error("Search engine is not initialized")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Search engine is not available",
		})
		return
	}
	
	response, err := h.searchEngine.Search(c.Request.Context(), searchOptions)
	if err != nil {
		h.logger.Error("Search failed", logger.Fields{
			"error":   err.Error(),
			"query":   req.Query,
			"user_id": req.UserID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Search failed: " + err.Error(),
		})
		return
	}

	processTime := time.Since(startTime)

	// 记录搜索日志
	h.logger.Info("Search completed", logger.Fields{
		"query":        req.Query,
		"results":      len(response.Results),
		"process_time": processTime,
		"user_id":      req.UserID,
	})

	// 返回搜索结果
	apiResponse := SearchResponse{
		Success:     true,
		Results:     response.Results,
		Total:       len(response.Results),
		ProcessTime: processTime,
		Timestamp:   time.Now(),
	}

	c.JSON(http.StatusOK, apiResponse)
}

// GetStats 获取搜索统计信息
// @Summary 获取搜索统计
// @Description 获取搜索引擎的统计信息和性能指标
// @Tags search
// @Produce json
// @Success 200 {object} map[string]interface{} "统计信息"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/search/stats [get]
func (h *SearchHandler) GetStats(c *gin.Context) {
	if h.searchEngine == nil {
		h.logger.Error("Search engine is not initialized")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Search engine is not available",
		})
		return
	}
	
	stats, err := h.searchEngine.GetSearchStats(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to get search stats", logger.Fields{
			"error": err.Error(),
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Failed to get search statistics: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// SearchRequest 搜索请求结构
type SearchRequest struct {
	Query         string   `json:"query" binding:"required"`
	TopK          int      `json:"top_k,omitempty"`
	MinSimilarity float64  `json:"min_similarity,omitempty"`
	ContentTypes  []string `json:"content_types,omitempty"`
	UserID        string   `json:"user_id,omitempty"`
}

// SearchResponse 搜索响应结构
type SearchResponse struct {
	Success     bool                        `json:"success"`
	Message     string                      `json:"message,omitempty"`
	Results     []*vector.SearchResultItem  `json:"results,omitempty"`
	Total       int                         `json:"total"`
	ProcessTime time.Duration               `json:"process_time"`
	Timestamp   time.Time                   `json:"timestamp"`
}

// ErrorResponse 错误响应结构
type ErrorResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// stringSliceToContentTypes 将字符串切片转换为ContentType切片
func stringSliceToContentTypes(strs []string) []models.ContentType {
	if len(strs) == 0 {
		return nil
	}
	
	contentTypes := make([]models.ContentType, len(strs))
	for i, str := range strs {
		contentTypes[i] = models.ContentType(str)
	}
	return contentTypes
}