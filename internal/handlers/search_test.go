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

// TestSearchHandler_Search 测试搜索API端点
func TestSearchHandler_Search(t *testing.T) {
	// 设置Gin为测试模式
	gin.SetMode(gin.TestMode)

	t.Run("有效搜索请求", func(t *testing.T) {
		// 创建测试路由
		router := gin.New()
		searchHandler := NewSearchHandler(nil) // 使用构造函数
		router.POST("/api/v1/search", searchHandler.Search)

		// 创建搜索请求
		searchReq := TestSearchRequest{
			Query:         "人工智能技术发展",
			TopK:          10,
			MinSimilarity: 0.7,
			ContentTypes:  []string{"text", "document"},
			UserID:        "test-user",
		}

		body, _ := json.Marshal(searchReq)
		req, _ := http.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// 执行请求
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应（暂时期望500，因为searchEngine为nil）
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})

	t.Run("无效请求参数", func(t *testing.T) {
		router := gin.New()
		searchHandler := NewSearchHandler(nil) // 使用构造函数
		router.POST("/api/v1/search", searchHandler.Search)

		// 空的查询请求
		searchReq := TestSearchRequest{
			Query: "", // 空查询应该返回错误
		}

		body, _ := json.Marshal(searchReq)
		req, _ := http.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回400错误
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})

	t.Run("JSON格式错误", func(t *testing.T) {
		router := gin.New()
		searchHandler := NewSearchHandler(nil) // 使用构造函数
		router.POST("/api/v1/search", searchHandler.Search)

		// 发送无效JSON
		req, _ := http.NewRequest("POST", "/api/v1/search", bytes.NewBuffer([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 应该返回400错误
		assert.Equal(t, http.StatusBadRequest, w.Code)
	})
}

// TestSearchHandler_GetStats 测试搜索统计API端点
func TestSearchHandler_GetStats(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("获取搜索统计", func(t *testing.T) {
		router := gin.New()
		searchHandler := NewSearchHandler(nil) // 使用构造函数
		router.GET("/api/v1/search/stats", searchHandler.GetStats)

		req, _ := http.NewRequest("GET", "/api/v1/search/stats", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 暂时期望500，因为searchEngine为nil
		assert.Equal(t, http.StatusInternalServerError, w.Code)
	})
}

// TestSearchRequest 测试用搜索请求结构
type TestSearchRequest struct {
	Query         string   `json:"query" binding:"required"`
	TopK          int      `json:"top_k,omitempty"`
	MinSimilarity float64  `json:"min_similarity,omitempty"`
	ContentTypes  []string `json:"content_types,omitempty"`
	UserID        string   `json:"user_id,omitempty"`
}

// TestSearchResponse 测试用搜索响应结构
type TestSearchResponse struct {
	Success     bool                        `json:"success"`
	Message     string                      `json:"message,omitempty"`
	Results     []*vector.SearchResultItem  `json:"results,omitempty"`
	Total       int                         `json:"total"`
	ProcessTime time.Duration               `json:"process_time"`
	Timestamp   time.Time                   `json:"timestamp"`
}

// MockSearchEngine 模拟搜索引擎（用于测试）
type MockSearchEngine struct {
	SearchFunc    func(ctx context.Context, options *vector.SearchOptions) ([]*vector.SearchResultItem, error)
	GetStatsFunc  func(ctx context.Context) (map[string]interface{}, error)
}

func (m *MockSearchEngine) Search(ctx context.Context, options *vector.SearchOptions) ([]*vector.SearchResultItem, error) {
	if m.SearchFunc != nil {
		return m.SearchFunc(ctx, options)
	}
	return nil, nil
}

func (m *MockSearchEngine) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	if m.GetStatsFunc != nil {
		return m.GetStatsFunc(ctx)
	}
	return map[string]interface{}{
		"total_searches": 100,
		"cache_hits":     80,
		"avg_latency":    "150ms",
	}, nil
}

func (m *MockSearchEngine) Close() error {
	return nil
}

// TestSearchHandler_WithMockEngine 使用Mock引擎进行完整测试
func TestSearchHandler_WithMockEngine(t *testing.T) {
	gin.SetMode(gin.TestMode)

	t.Run("成功搜索请求", func(t *testing.T) {
		// 创建Mock搜索引擎
		mockEngine := &MockSearchEngine{
			SearchFunc: func(ctx context.Context, options *vector.SearchOptions) ([]*vector.SearchResultItem, error) {
				// 返回模拟搜索结果
				return []*vector.SearchResultItem{
					{
						DocumentID:     "doc-1",
						Content:        "人工智能技术在医疗领域的应用",
						Similarity:     0.95,
						Rank:           1,
						RelevanceScore: 0.95,
					},
					{
						DocumentID:     "doc-2", 
						Content:        "机器学习算法的最新发展",
						Similarity:     0.88,
						Rank:           2,
						RelevanceScore: 0.88,
					},
				}, nil
			},
		}

		// 创建搜索处理器
		searchHandler := NewSearchHandler(mockEngine)

		// 创建路由
		router := gin.New()
		router.POST("/api/v1/search", searchHandler.Search)

		// 创建搜索请求
		searchReq := TestSearchRequest{
			Query:         "人工智能技术",
			TopK:          10,
			MinSimilarity: 0.7,
			UserID:        "test-user",
		}

		body, _ := json.Marshal(searchReq)
		req, _ := http.NewRequest("POST", "/api/v1/search", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")

		// 执行请求
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// 验证响应
		assert.Equal(t, http.StatusOK, w.Code)

		var response TestSearchResponse
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.True(t, response.Success)
		assert.Equal(t, 2, response.Total)
		assert.Len(t, response.Results, 2)
		assert.Equal(t, "doc-1", response.Results[0].DocumentID)
		assert.Greater(t, response.Results[0].Similarity, 0.9)
	})

	t.Run("获取搜索统计", func(t *testing.T) {
		mockEngine := &MockSearchEngine{
			GetStatsFunc: func(ctx context.Context) (map[string]interface{}, error) {
				return map[string]interface{}{
					"total_searches": 150,
					"cache_hits":     120,
					"avg_latency":    "125ms",
				}, nil
			},
		}

		searchHandler := NewSearchHandler(mockEngine)

		router := gin.New()
		router.GET("/api/v1/search/stats", searchHandler.GetStats)

		req, _ := http.NewRequest("GET", "/api/v1/search/stats", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)

		var response map[string]interface{}
		err := json.Unmarshal(w.Body.Bytes(), &response)
		require.NoError(t, err)

		assert.Equal(t, float64(150), response["total_searches"])
		assert.Equal(t, float64(120), response["cache_hits"])
	})
}