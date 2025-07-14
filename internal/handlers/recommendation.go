package handlers

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"memoro/internal/logger"
	"memoro/internal/services/vector"
)

// RecommendationHandler 推荐API处理器
type RecommendationHandler struct {
	recommender RecommenderInterface
	logger      *logger.Logger
}

// RecommenderInterface 推荐引擎接口
type RecommenderInterface interface {
	GetRecommendations(ctx context.Context, request *vector.RecommendationRequest) (*vector.RecommendationResponse, error)
	Close() error
}

// NewRecommendationHandler 创建推荐处理器
func NewRecommendationHandler(recommender RecommenderInterface) *RecommendationHandler {
	return &RecommendationHandler{
		recommender: recommender,
		logger:      logger.NewLogger("recommendation-handler"),
	}
}

// GetRecommendations 获取推荐内容
// @Summary 获取推荐内容
// @Description 基于用户行为和内容相似度的智能推荐
// @Tags recommendations
// @Accept json
// @Produce json
// @Param request body RecommendationRequest true "推荐请求"
// @Success 200 {object} RecommendationResponse "推荐成功"
// @Failure 400 {object} ErrorResponse "请求参数错误"
// @Failure 500 {object} ErrorResponse "服务器内部错误"
// @Router /api/v1/recommendations [post]
func (h *RecommendationHandler) GetRecommendations(c *gin.Context) {
	startTime := time.Now()

	// 解析请求
	var req RecommendationRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		h.logger.Error("Invalid recommendation request", logger.Fields{
			"error": err.Error(),
		})
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid request parameters: " + err.Error(),
		})
		return
	}

	// 验证必需参数
	if req.Type == "" {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Recommendation type is required",
		})
		return
	}

	// 验证推荐类型
	if !isValidRecommendationType(req.Type) {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Success: false,
			Message: "Invalid recommendation type: " + req.Type,
		})
		return
	}

	// 设置默认值
	if req.MaxRecommendations <= 0 {
		req.MaxRecommendations = 5
	}
	if req.MinSimilarity <= 0 {
		req.MinSimilarity = 0.7
	}

	// 构建推荐请求
	recommendationReq := &vector.RecommendationRequest{
		Type:               vector.RecommendationType(req.Type),
		UserID:             req.UserID,
		SourceDocumentID:   req.SourceDocumentID,
		MaxRecommendations: req.MaxRecommendations,
		MinSimilarity:      float32(req.MinSimilarity),
		ContentTypes:       stringSliceToContentTypes(req.ContentTypes),
	}

	// 执行推荐
	if h.recommender == nil {
		h.logger.Error("Recommender is not initialized")
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Recommendation service is not available",
		})
		return
	}

	response, err := h.recommender.GetRecommendations(c.Request.Context(), recommendationReq)
	if err != nil {
		h.logger.Error("Recommendation failed", logger.Fields{
			"error":   err.Error(),
			"type":    req.Type,
			"user_id": req.UserID,
		})
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Success: false,
			Message: "Recommendation failed: " + err.Error(),
		})
		return
	}

	processTime := time.Since(startTime)

	// 记录推荐日志
	h.logger.Info("Recommendation completed", logger.Fields{
		"type":         req.Type,
		"results":      len(response.Recommendations),
		"process_time": processTime,
		"user_id":      req.UserID,
	})

	// 返回推荐结果
	apiResponse := RecommendationResponse{
		Success:         true,
		Recommendations: response.Recommendations,
		Total:           len(response.Recommendations),
		ProcessTime:     processTime,
		Timestamp:       time.Now(),
		AlgorithmUsed:   string(response.RecommendationType),
	}

	c.JSON(http.StatusOK, apiResponse)
}

// RecommendationRequest 推荐请求结构
type RecommendationRequest struct {
	Type               string   `json:"type" binding:"required"`
	UserID             string   `json:"user_id,omitempty"`
	SourceDocumentID   string   `json:"source_document_id,omitempty"`
	MaxRecommendations int      `json:"max_recommendations,omitempty"`
	MinSimilarity      float64  `json:"min_similarity,omitempty"`
	ContentTypes       []string `json:"content_types,omitempty"`
}

// RecommendationResponse 推荐响应结构
type RecommendationResponse struct {
	Success         bool                         `json:"success"`
	Message         string                       `json:"message,omitempty"`
	Recommendations []*vector.RecommendationItem `json:"recommendations,omitempty"`
	Total           int                          `json:"total"`
	ProcessTime     time.Duration                `json:"process_time"`
	Timestamp       time.Time                    `json:"timestamp"`
	AlgorithmUsed   string                       `json:"algorithm_used,omitempty"`
}

// isValidRecommendationType 验证推荐类型是否有效
func isValidRecommendationType(recType string) bool {
	validTypes := []string{
		"similar",     // 相似内容推荐
		"related",     // 相关内容推荐
		"popular",     // 热门内容推荐
		"trending",    // 趋势内容推荐
		"personalized", // 个性化推荐
	}

	recType = strings.ToLower(recType)
	for _, valid := range validTypes {
		if recType == valid {
			return true
		}
	}
	return false
}