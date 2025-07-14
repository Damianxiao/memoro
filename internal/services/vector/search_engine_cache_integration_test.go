package vector

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"memoro/internal/config"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// MockEmbeddingService 模拟嵌入服务
type MockEmbeddingService struct {
	mock.Mock
}

func (m *MockEmbeddingService) GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResult, error) {
	args := m.Called(ctx, req)
	return args.Get(0).(*EmbeddingResult), args.Error(1)
}

func (m *MockEmbeddingService) CreateContentVector(ctx context.Context, contentItem *models.ContentItem) (*VectorDocument, error) {
	args := m.Called(ctx, contentItem)
	return args.Get(0).(*VectorDocument), args.Error(1)
}

func (m *MockEmbeddingService) Close() error {
	args := m.Called()
	return args.Error(0)
}

// TestSearchEngine_CacheIntegration 测试SearchEngine的缓存集成
func TestSearchEngine_CacheIntegration(t *testing.T) {
	// 跳过需要真实Chroma连接的测试
	if testing.Short() {
		t.Skip("跳过需要外部依赖的集成测试")
	}

	searchEngine, err := NewSearchEngine()
	if err != nil {
		t.Skipf("跳过SearchEngine集成测试，无法连接到Chroma: %v", err)
		return
	}
	defer searchEngine.Close()

	t.Run("查询向量缓存集成测试", func(t *testing.T) {
		query := "人工智能技术在医疗领域的应用"
		options := &SearchOptions{
			Query:         query,
			TopK:          10,
			MinSimilarity: 0.7,
			UserID:        "cache-test-user",
		}

		// 模拟第一次查询 - 应该调用embedding服务
		vector1, err := searchEngine.generateQueryVector(context.Background(), query, options)
		require.NoError(t, err)
		require.NotNil(t, vector1)
		require.Greater(t, len(vector1), 0)

		// 第二次相同查询 - 应该从缓存获取
		startTime := time.Now()
		vector2, err := searchEngine.generateQueryVector(context.Background(), query, options)
		cacheLatency := time.Since(startTime)

		require.NoError(t, err)
		assert.Equal(t, vector1, vector2)

		// 缓存命中应该更快（小于10ms）
		assert.Less(t, cacheLatency, 10*time.Millisecond, "缓存命中应该很快")

		// 验证缓存统计
		stats, err := searchEngine.GetSearchStats(context.Background())
		require.NoError(t, err)

		cacheInfo := stats["cache_info"].(map[string]interface{})
		qvCache := cacheInfo["query_vector_cache"].(map[string]interface{})

		hits := qvCache["hits"].(int64)
		assert.Greater(t, hits, int64(0), "应该有缓存命中")
	})

	t.Run("不同查询参数生成不同缓存", func(t *testing.T) {
		baseQuery := "深度学习神经网络"

		options1 := &SearchOptions{Query: baseQuery, TopK: 5, UserID: "user1"}
		options2 := &SearchOptions{Query: baseQuery, TopK: 10, UserID: "user1"} // 不同TopK
		options3 := &SearchOptions{Query: baseQuery, TopK: 5, UserID: "user2"}  // 不同UserID

		// 生成三个不同的查询向量
		vector1, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options1)
		require.NoError(t, err)

		vector2, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options2)
		require.NoError(t, err)

		vector3, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options3)
		require.NoError(t, err)

		// 验证向量都存在且被正确缓存
		assert.NotNil(t, vector1)
		assert.NotNil(t, vector2)
		assert.NotNil(t, vector3)

		// 再次查询验证缓存命中
		cachedVector1, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options1)
		require.NoError(t, err)
		assert.Equal(t, vector1, cachedVector1)

		cachedVector2, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options2)
		require.NoError(t, err)
		assert.Equal(t, vector2, cachedVector2)

		cachedVector3, err := searchEngine.generateQueryVector(context.Background(), baseQuery, options3)
		require.NoError(t, err)
		assert.Equal(t, vector3, cachedVector3)
	})
}

// TestSearchEngine_CachePerformance 测试SearchEngine缓存性能
func TestSearchEngine_CachePerformance(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过性能测试")
	}

	// 使用模拟服务来避免外部依赖
	t.Run("缓存性能改善验证", func(t *testing.T) {
		// 创建带缓存的配置
		cfg := &config.Config{
			VectorDB: config.VectorDBConfig{
				CacheConfig: &config.VectorCacheConfig{
					QueryVectorTTL:     1 * time.Hour,
					QueryVectorMaxSize: 1000,
					CleanupInterval:    1 * time.Minute,
				},
			},
		}

		cacheManager := NewVectorCacheManager(cfg)
		defer cacheManager.Close()

		// 模拟向量生成（耗时操作）
		simulateVectorGeneration := func(query string) ([]float32, time.Duration) {
			// 模拟LLM API调用延迟（100ms）
			time.Sleep(100 * time.Millisecond)
			return []float32{1.0, 2.0, 3.0, 4.0, 5.0}, 100 * time.Millisecond
		}

		query := "性能测试查询"
		options := &SearchOptions{TopK: 10, UserID: "perf-user"}

		// 第一次查询 - 模拟未命中，需要生成向量
		start := time.Now()
		vector, _ := simulateVectorGeneration(query)
		firstCallDuration := time.Since(start)

		// 缓存结果
		cacheManager.SetQueryVector(query, options, vector)

		// 第二次查询 - 从缓存获取
		start = time.Now()
		cachedVector, found := cacheManager.GetQueryVector(query, options)
		cacheCallDuration := time.Since(start)

		// 验证缓存命中
		assert.True(t, found)
		assert.Equal(t, vector, cachedVector)

		// 验证性能改善
		assert.Greater(t, firstCallDuration, 50*time.Millisecond, "第一次调用应该较慢")
		assert.Less(t, cacheCallDuration, 1*time.Millisecond, "缓存调用应该很快")

		speedupRatio := firstCallDuration.Nanoseconds() / cacheCallDuration.Nanoseconds()
		assert.Greater(t, speedupRatio, int64(50), "缓存应该提供至少50倍的速度提升")

		t.Logf("性能改善:")
		t.Logf("  第一次调用: %v", firstCallDuration)
		t.Logf("  缓存调用: %v", cacheCallDuration)
		t.Logf("  速度提升: %dx", speedupRatio)
	})

	t.Run("并发缓存访问性能", func(t *testing.T) {
		cfg := &config.Config{
			VectorDB: config.VectorDBConfig{
				CacheConfig: &config.VectorCacheConfig{
					QueryVectorTTL:     1 * time.Hour,
					QueryVectorMaxSize: 1000,
					CleanupInterval:    1 * time.Minute,
				},
			},
		}

		cacheManager := NewVectorCacheManager(cfg)
		defer cacheManager.Close()

		// 预填充缓存
		options := &SearchOptions{TopK: 10, UserID: "concurrent-user"}
		for i := 0; i < 100; i++ {
			query := fmt.Sprintf("concurrent-query-%d", i)
			vector := make([]float32, 1536) // OpenAI embedding size
			for j := range vector {
				vector[j] = float32(i*j) / 1000.0
			}
			cacheManager.SetQueryVector(query, options, vector)
		}

		// 并发性能测试
		const numGoroutines = 50
		const numQueries = 100

		start := time.Now()
		done := make(chan bool, numGoroutines)

		for g := 0; g < numGoroutines; g++ {
			go func(goroutineID int) {
				defer func() { done <- true }()

				for i := 0; i < numQueries; i++ {
					query := fmt.Sprintf("concurrent-query-%d", i%100) // 访问预填充的查询
					_, found := cacheManager.GetQueryVector(query, options)
					assert.True(t, found, "所有查询都应该命中缓存")
				}
			}(g)
		}

		// 等待所有goroutine完成
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		totalDuration := time.Since(start)
		totalQueries := numGoroutines * numQueries
		avgLatency := totalDuration / time.Duration(totalQueries)

		t.Logf("并发性能测试结果:")
		t.Logf("  总查询数: %d", totalQueries)
		t.Logf("  总耗时: %v", totalDuration)
		t.Logf("  平均延迟: %v", avgLatency)
		t.Logf("  QPS: %.2f", float64(totalQueries)/totalDuration.Seconds())

		// 验证性能目标
		assert.Less(t, avgLatency, 1*time.Millisecond, "平均缓存访问延迟应该小于1ms")

		qps := float64(totalQueries) / totalDuration.Seconds()
		assert.Greater(t, qps, 10000.0, "缓存QPS应该大于10,000")
	})
}

// BenchmarkCacheIntegration 缓存集成基准测试
func BenchmarkCacheIntegration(b *testing.B) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    10000,
				RecommendationTTL:     30 * time.Minute,
				RecommendationMaxSize: 5000,
				CleanupInterval:       10 * time.Minute,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	// 预填充缓存
	options := &SearchOptions{TopK: 10, UserID: "bench-user"}
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("benchmark-query-%d", i)
		vector := make([]float32, 1536)
		for j := range vector {
			vector[j] = float32(i*j) / 1000.0
		}
		cacheManager.SetQueryVector(query, options, vector)
	}

	b.ResetTimer()

	b.Run("CacheHit", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("benchmark-query-%d", i%1000)
				_, _ = cacheManager.GetQueryVector(query, options)
				i++
			}
		})
	})

	b.Run("CacheMiss", func(b *testing.B) {
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("miss-query-%d", i)
				_, _ = cacheManager.GetQueryVector(query, options)
				i++
			}
		})
	})

	b.Run("CacheSet", func(b *testing.B) {
		vector := make([]float32, 1536)
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("set-query-%d", i)
				cacheManager.SetQueryVector(query, options, vector)
				i++
			}
		})
	})
}

// TestRecommenderCacheIntegration 测试Recommender的缓存集成
func TestRecommenderCacheIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过Recommender集成测试")
	}

	// 这个测试需要完整的SearchEngine，在实际环境中会跳过
	searchEngine, err := NewSearchEngine()
	if err != nil {
		t.Skipf("跳过Recommender缓存测试，无法创建SearchEngine: %v", err)
		return
	}
	defer searchEngine.Close()

	recommender := &Recommender{
		searchEngine:   searchEngine,
		similarityCalc: NewSimilarityCalculator(),
		ranker:         NewRanker(),
		logger:         logger.NewLogger("test-recommender"),
	}

	t.Run("推荐缓存基本功能", func(t *testing.T) {
		request := &RecommendationRequest{
			Type:               RecommendationTypeSimilar,
			UserID:             "cache-test-user",
			SourceDocumentID:   "test-doc-123",
			MaxRecommendations: 5,
			MinSimilarity:      0.8,
		}

		// 第一次请求 - 应该生成推荐并缓存
		response1, err := recommender.GetRecommendations(context.Background(), request)
		require.NoError(t, err)
		require.NotNil(t, response1)

		firstCallTime := response1.ProcessTime

		// 第二次相同请求 - 应该从缓存返回
		start := time.Now()
		response2, err := recommender.GetRecommendations(context.Background(), request)
		cacheCallTime := time.Since(start)

		require.NoError(t, err)
		require.NotNil(t, response2)

		// 验证结果一致性
		assert.Equal(t, len(response1.Recommendations), len(response2.Recommendations))
		if len(response1.Recommendations) > 0 && len(response2.Recommendations) > 0 {
			assert.Equal(t, response1.Recommendations[0].DocumentID, response2.Recommendations[0].DocumentID)
		}

		// 验证缓存性能改善
		assert.Less(t, cacheCallTime, firstCallTime/2, "缓存调用应该明显更快")

		// 验证缓存统计
		stats, err := searchEngine.GetSearchStats(context.Background())
		require.NoError(t, err)

		cacheInfo := stats["cache_info"].(map[string]interface{})
		recCache := cacheInfo["recommendation_cache"].(map[string]interface{})

		hits := recCache["hits"].(int64)
		assert.Greater(t, hits, int64(0), "应该有推荐缓存命中")
	})
}
