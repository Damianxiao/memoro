package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"memoro/internal/services/vector"
)

// TestRecommendationHandler_GetRecommendations 测试推荐API端点
func TestRecommendationHandler_GetRecommendations(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("有效推荐请求", func(t *testing.T) {
		router := gin.New()
		handler := NewRecommendationHandler(nil) // 使用构造函数
		router.POST("/api/v1/recommendations", handler.GetRecommendations)

		// 创建推荐请求
		recReq := TestRecommendationRequest{
			Type:               "similar",
			UserID:             "test-user",
			SourceDocumentID:   "doc-123",
			MaxRecommendations: 5,
			MinSimilarity:      0.8,
		}

		body, _ := json.Marshal(recReq)
		req, _ := http.NewRequest("POST", "/api/v1/recommendations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应（期望500，因为recommender为nil）
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("无效请求参数", func(t *testing.T) {
		router := gin.New()
		handler := NewRecommendationHandler(nil)
		router.POST("/api/v1/recommendations", handler.GetRecommendations)

		// 空的类型请求
		recReq := TestRecommendationRequest{
			Type: "", // 空类型应该返回错误
		}

		body, _ := json.Marshal(recReq)
		req, _ := http.NewRequest("POST", "/api/v1/recommendations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回400错误
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("JSON格式错误", func(t *testing.T) {
		router := gin.New()
		handler := NewRecommendationHandler(nil)
		router.POST("/api/v1/recommendations", handler.GetRecommendations)

		// 发送无效JSON
		req, _ := http.NewRequest("POST", "/api/v1/recommendations", bytes.NewBuffer([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回400错误
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestRecommendationRequest 测试用推荐请求结构
type TestRecommendationRequest struct {
	Type               string  `json:"type" binding:"required"`
	UserID             string  `json:"user_id,omitempty"`
	SourceDocumentID   string  `json:"source_document_id,omitempty"`
	MaxRecommendations int     `json:"max_recommendations,omitempty"`
	MinSimilarity      float64 `json:"min_similarity,omitempty"`
	ContentTypes       []string `json:"content_types,omitempty"`
}

// TestRecommendationResponse 测试用推荐响应结构
type TestRecommendationResponse struct {
	Success         bool                         `json:"success"`
	Message         string                       `json:"message,omitempty"`
	Recommendations []*vector.RecommendationItem `json:"recommendations,omitempty"`
	Total           int                          `json:"total"`
	ProcessTime     time.Duration                `json:"process_time"`
	Timestamp       time.Time                    `json:"timestamp"`
}

// MockRecommender 模拟推荐引擎（用于测试）
type MockRecommender struct {
	GetRecommendationsFunc func(ctx context.Context, request *vector.RecommendationRequest) (*vector.RecommendationResponse, error)
}

func (m *MockRecommender) GetRecommendations(ctx context.Context, request *vector.RecommendationRequest) (*vector.RecommendationResponse, error) {
	if m.GetRecommendationsFunc != nil {
		return m.GetRecommendationsFunc(ctx, request)
	}
	return nil, nil
}

func (m *MockRecommender) Close() error {
	return nil
}

// TestRecommendationHandler_WithMockRecommender 使用Mock推荐器进行完整测试
func TestRecommendationHandler_WithMockRecommender(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功推荐请求", func(t *testing.T) {
		// 创建Mock推荐器
		mockRecommender := &MockRecommender{
			GetRecommendationsFunc: func(ctx context.Context, request *vector.RecommendationRequest) (*vector.RecommendationResponse, error) {
				// 返回模拟推荐结果
				return &vector.RecommendationResponse{
					Recommendations: []*vector.RecommendationItem{
						{
							DocumentID: "rec-doc-1",
							Content:    "推荐内容1",
							Similarity: 0.95,
							Confidence: 0.9,
							Rank:       1,
						},
						{
							DocumentID: "rec-doc-2",
							Content:    "推荐内容2",
							Similarity: 0.88,
							Confidence: 0.85,
							Rank:       2,
						},
					},
					TotalFound:         2,
					ProcessTime:        50 * time.Millisecond,
					RecommendationType: vector.RecommendationTypeSimilar,
				}, nil
			},
		}

		// 创建推荐处理器
		handler := NewRecommendationHandler(mockRecommender)

		// 创建路由
		router := gin.New()
		router.POST("/api/v1/recommendations", handler.GetRecommendations)

		// 创建推荐请求
		recReq := TestRecommendationRequest{
			Type:               "similar",
			UserID:             "test-user",
			SourceDocumentID:   "doc-123",
			MaxRecommendations: 5,
			MinSimilarity:      0.8,
		}

		body, _ := json.Marshal(recReq)
		req, _ := http.NewRequest("POST", "/api/v1/recommendations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// 执行请求
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)

		var response TestRecommendationResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, 2, response.Total)
		assert.Len(t, response.Recommendations, 2)
		assert.Equal(t, "rec-doc-1", response.Recommendations[0].DocumentID)
		assert.Greater(t, response.Recommendations[0].Similarity, 0.9)
	})

	t.Run("无效推荐类型", func(t *testing.T) {
		mockRecommender := &MockRecommender{}
		handler := NewRecommendationHandler(mockRecommender)

		router := gin.New()
		router.POST("/api/v1/recommendations", handler.GetRecommendations)

		// 创建无效类型的推荐请求
		recReq := TestRecommendationRequest{
			Type:               "invalid_type",
			UserID:             "test-user",
			MaxRecommendations: 5,
		}

		body, _ := json.Marshal(recReq)
		req, _ := http.NewRequest("POST", "/api/v1/recommendations", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回400错误
		assert.Equal(t, http.StatusBadRequest, w.Code)

		var response ErrorResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.False(t, response.Success)
		assert.Contains(t, response.Message, "Invalid recommendation type")
	})
}