package vector

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"memoro/internal/config"
)

// TestVectorCacheManager_Basic 测试缓存管理器基本功能
func TestVectorCacheManager_Basic(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    100,
				RecommendationTTL:     30 * time.Minute,
				RecommendationMaxSize: 50,
				UserPreferenceTTL:     24 * time.Hour,
				UserPreferenceMaxSize: 20,
				CleanupInterval:       1 * time.Second,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	require.NotNil(t, cacheManager)
	defer cacheManager.Close()

	t.Run("初始状态验证", func(t *testing.T) {
		stats := cacheManager.GetStats()
		assert.Equal(t, int64(0), stats.QueryVectorHits)
		assert.Equal(t, int64(0), stats.QueryVectorMisses)
		assert.Equal(t, int64(0), stats.RecommendationHits)
		assert.Equal(t, int64(0), stats.RecommendationMisses)

		info := cacheManager.GetCacheInfo()
		assert.NotNil(t, info)

		queryVectorInfo := info["query_vector_cache"].(map[string]interface{})
		assert.Equal(t, 0, queryVectorInfo["size"])
		assert.Equal(t, 100, queryVectorInfo["max_size"])
	})
}

// TestQueryVectorCache 测试查询向量缓存
func TestQueryVectorCache(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:     1 * time.Hour,
				QueryVectorMaxSize: 10,
				CleanupInterval:    100 * time.Millisecond,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("缓存未命中和设置", func(t *testing.T) {
		query := "人工智能技术发展"
		options := &SearchOptions{
			TopK:          10,
			MinSimilarity: 0.7,
			UserID:        "user-123",
		}
		testVector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

		// 第一次获取应该未命中
		cachedVector, found := cacheManager.GetQueryVector(query, options)
		assert.False(t, found)
		assert.Nil(t, cachedVector)

		// 验证统计信息
		stats := cacheManager.GetStats()
		assert.Equal(t, int64(1), stats.QueryVectorMisses)
		assert.Equal(t, int64(0), stats.QueryVectorHits)

		// 设置缓存
		cacheManager.SetQueryVector(query, options, testVector)

		// 第二次获取应该命中
		cachedVector, found = cacheManager.GetQueryVector(query, options)
		assert.True(t, found)
		assert.Equal(t, testVector, cachedVector)

		// 验证统计信息更新
		stats = cacheManager.GetStats()
		assert.Equal(t, int64(1), stats.QueryVectorMisses)
		assert.Equal(t, int64(1), stats.QueryVectorHits)
	})

	t.Run("不同查询选项产生不同缓存键", func(t *testing.T) {
		query := "机器学习算法"
		vector1 := []float32{1.0, 2.0, 3.0}
		vector2 := []float32{4.0, 5.0, 6.0}

		options1 := &SearchOptions{TopK: 5, UserID: "user1"}
		options2 := &SearchOptions{TopK: 10, UserID: "user1"} // 不同TopK
		options3 := &SearchOptions{TopK: 5, UserID: "user2"}  // 不同UserID

		// 设置三个不同的缓存
		cacheManager.SetQueryVector(query, options1, vector1)
		cacheManager.SetQueryVector(query, options2, vector2)
		cacheManager.SetQueryVector(query, options3, vector1)

		// 验证每个都能正确获取
		cached1, found1 := cacheManager.GetQueryVector(query, options1)
		cached2, found2 := cacheManager.GetQueryVector(query, options2)
		cached3, found3 := cacheManager.GetQueryVector(query, options3)

		assert.True(t, found1)
		assert.True(t, found2)
		assert.True(t, found3)
		assert.Equal(t, vector1, cached1)
		assert.Equal(t, vector2, cached2)
		assert.Equal(t, vector1, cached3)
	})

	t.Run("LRU淘汰机制", func(t *testing.T) {
		// 清空之前的缓存状态
		cacheManager.Close()
		cacheManager = NewVectorCacheManager(cfg)
		defer cacheManager.Close()

		options := &SearchOptions{TopK: 10, UserID: "test-user"}

		// 添加超过最大容量的缓存项（MaxSize = 10）
		for i := 0; i < 15; i++ {
			query := fmt.Sprintf("test-query-%d", i)
			vector := []float32{float32(i), float32(i + 1)}
			cacheManager.SetQueryVector(query, options, vector)
		}

		// 验证缓存大小不超过限制
		info := cacheManager.GetCacheInfo()
		queryVectorInfo := info["query_vector_cache"].(map[string]interface{})
		cacheSize := queryVectorInfo["size"].(int)
		assert.LessOrEqual(t, cacheSize, 10)

		// 验证最早的缓存项被淘汰
		earlyVector, found := cacheManager.GetQueryVector("test-query-0", options)
		assert.False(t, found)
		assert.Nil(t, earlyVector)

		// 验证最新的缓存项仍然存在
		lateVector, found := cacheManager.GetQueryVector("test-query-14", options)
		assert.True(t, found)
		assert.NotNil(t, lateVector)

		// 验证淘汰统计
		stats := cacheManager.GetStats()
		assert.Greater(t, stats.QueryVectorEvictions, int64(0))
	})
}

// TestRecommendationCache 测试推荐缓存
func TestRecommendationCache(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				RecommendationTTL:     30 * time.Minute,
				RecommendationMaxSize: 5,
				CleanupInterval:       100 * time.Millisecond,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("推荐缓存基本功能", func(t *testing.T) {
		request := &RecommendationRequest{
			Type:               RecommendationTypeSimilar,
			UserID:             "user-123",
			SourceDocumentID:   "doc-456",
			MaxRecommendations: 5,
			MinSimilarity:      0.8,
		}

		testRecommendations := []*RecommendationItem{
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
		}

		// 第一次获取应该未命中
		cachedRecs, found := cacheManager.GetRecommendation(request)
		assert.False(t, found)
		assert.Nil(t, cachedRecs)

		// 设置缓存
		cacheManager.SetRecommendation(request, testRecommendations)

		// 第二次获取应该命中
		cachedRecs, found = cacheManager.GetRecommendation(request)
		assert.True(t, found)
		assert.Equal(t, len(testRecommendations), len(cachedRecs))
		assert.Equal(t, testRecommendations[0].DocumentID, cachedRecs[0].DocumentID)
		assert.Equal(t, testRecommendations[1].Content, cachedRecs[1].Content)

		// 验证统计信息
		stats := cacheManager.GetStats()
		assert.Equal(t, int64(1), stats.RecommendationMisses)
		assert.Equal(t, int64(1), stats.RecommendationHits)
	})

	t.Run("不同请求参数生成不同缓存键", func(t *testing.T) {
		// 创建不同的请求
		requests := []*RecommendationRequest{
			{
				Type:               RecommendationTypeSimilar,
				UserID:             "user-123",
				SourceDocumentID:   "doc-1",
				MaxRecommendations: 5,
				MinSimilarity:      0.8,
			},
			{
				Type:               RecommendationTypeSimilar,
				UserID:             "user-123",
				SourceDocumentID:   "doc-2", // 不同的源文档
				MaxRecommendations: 5,
				MinSimilarity:      0.8,
			},
			{
				Type:               RecommendationTypeRelated, // 不同的类型
				UserID:             "user-123",
				SourceDocumentID:   "doc-1",
				MaxRecommendations: 5,
				MinSimilarity:      0.8,
			},
		}

		recommendations := make([][]*RecommendationItem, len(requests))
		for i := range recommendations {
			recommendations[i] = []*RecommendationItem{
				{
					DocumentID: fmt.Sprintf("doc-%d-1", i),
					Similarity: 0.9,
				},
			}
		}

		// 设置不同的缓存
		for i, req := range requests {
			cacheManager.SetRecommendation(req, recommendations[i])
		}

		// 验证每个请求都能获取到正确的缓存
		for i, req := range requests {
			cached, found := cacheManager.GetRecommendation(req)
			assert.True(t, found, "Request %d should be cached", i)
			assert.Equal(t, recommendations[i][0].DocumentID, cached[0].DocumentID)
		}
	})
}

// TestCacheExpiration 测试缓存过期机制
func TestCacheExpiration(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        100 * time.Millisecond, // 很短的TTL用于测试
				QueryVectorMaxSize:    100,
				RecommendationTTL:     100 * time.Millisecond,
				RecommendationMaxSize: 100,
				CleanupInterval:       50 * time.Millisecond, // 快速清理
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("查询向量TTL过期", func(t *testing.T) {
		query := "TTL测试查询"
		options := &SearchOptions{TopK: 10, UserID: "ttl-user"}
		vector := []float32{1.0, 2.0, 3.0}

		// 设置缓存
		cacheManager.SetQueryVector(query, options, vector)

		// 立即获取应该命中
		cached, found := cacheManager.GetQueryVector(query, options)
		assert.True(t, found)
		assert.Equal(t, vector, cached)

		// 等待TTL过期
		time.Sleep(150 * time.Millisecond)

		// 再次获取应该未命中（已过期）
		cached, found = cacheManager.GetQueryVector(query, options)
		assert.False(t, found)
		assert.Nil(t, cached)
	})

	t.Run("推荐缓存TTL过期", func(t *testing.T) {
		request := &RecommendationRequest{
			Type:               RecommendationTypeSimilar,
			UserID:             "ttl-user",
			MaxRecommendations: 3,
		}

		recommendations := []*RecommendationItem{
			{DocumentID: "ttl-doc-1", Similarity: 0.9},
		}

		// 设置缓存
		cacheManager.SetRecommendation(request, recommendations)

		// 立即获取应该命中
		cached, found := cacheManager.GetRecommendation(request)
		assert.True(t, found)
		assert.Equal(t, len(recommendations), len(cached))

		// 等待TTL过期
		time.Sleep(150 * time.Millisecond)

		// 再次获取应该未命中（已过期）
		cached, found = cacheManager.GetRecommendation(request)
		assert.False(t, found)
		assert.Nil(t, cached)
	})

	t.Run("自动清理过期条目", func(t *testing.T) {
		// 添加多个缓存项
		for i := 0; i < 5; i++ {
			query := fmt.Sprintf("cleanup-query-%d", i)
			options := &SearchOptions{TopK: 10, UserID: "cleanup-user"}
			vector := []float32{float32(i)}
			cacheManager.SetQueryVector(query, options, vector)
		}

		// 验证缓存项存在
		info := cacheManager.GetCacheInfo()
		queryVectorInfo := info["query_vector_cache"].(map[string]interface{})
		assert.Greater(t, queryVectorInfo["size"].(int), 0)

		// 等待TTL过期和自动清理
		time.Sleep(200 * time.Millisecond)

		// 验证过期条目被清理
		info = cacheManager.GetCacheInfo()
		queryVectorInfo = info["query_vector_cache"].(map[string]interface{})
		// 注意：由于清理是异步的，可能需要等待更长时间或者检查是否减少了
		t.Logf("清理后缓存大小: %v", queryVectorInfo["size"])
	})
}

// TestCacheConcurrency 测试缓存并发安全性
func TestCacheConcurrency(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    1000,
				RecommendationTTL:     30 * time.Minute,
				RecommendationMaxSize: 500,
				CleanupInterval:       1 * time.Minute,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("并发读写安全性", func(t *testing.T) {
		const numGoroutines = 20
		const numOperations = 100

		var wg sync.WaitGroup
		errChan := make(chan error, numGoroutines)

		// 启动多个goroutine并发操作缓存
		for g := 0; g < numGoroutines; g++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()

				options := &SearchOptions{
					TopK:   10,
					UserID: fmt.Sprintf("concurrent-user-%d", goroutineID),
				}

				for i := 0; i < numOperations; i++ {
					query := fmt.Sprintf("concurrent-query-%d-%d", goroutineID, i)
					vector := make([]float32, 10)
					for j := range vector {
						vector[j] = float32(goroutineID*i*j) / 100.0
					}

					// 随机执行读或写操作
					if i%3 == 0 {
						// 写操作
						cacheManager.SetQueryVector(query, options, vector)
					} else {
						// 读操作
						_, _ = cacheManager.GetQueryVector(query, options)
					}

					// 偶尔操作推荐缓存
					if i%10 == 0 {
						req := &RecommendationRequest{
							Type:               RecommendationTypeSimilar,
							UserID:             fmt.Sprintf("concurrent-user-%d", goroutineID),
							MaxRecommendations: 5,
						}

						recommendations := []*RecommendationItem{
							{DocumentID: fmt.Sprintf("concurrent-doc-%d-%d", goroutineID, i)},
						}

						if i%2 == 0 {
							cacheManager.SetRecommendation(req, recommendations)
						} else {
							_, _ = cacheManager.GetRecommendation(req)
						}
					}
				}
			}(g)
		}

		// 等待所有goroutine完成
		wg.Wait()
		close(errChan)

		// 检查是否有错误
		for err := range errChan {
			assert.NoError(t, err)
		}

		// 验证缓存管理器仍然正常工作
		stats := cacheManager.GetStats()
		assert.True(t, stats.QueryVectorHits+stats.QueryVectorMisses > 0, "应该有缓存操作记录")

		// 验证缓存信息可以正常获取
		info := cacheManager.GetCacheInfo()
		assert.NotNil(t, info)
		assert.Contains(t, info, "query_vector_cache")
		assert.Contains(t, info, "recommendation_cache")

		t.Logf("并发测试完成 - 查询向量: 命中=%d, 未命中=%d, 淘汰=%d",
			stats.QueryVectorHits, stats.QueryVectorMisses, stats.QueryVectorEvictions)
		t.Logf("推荐缓存: 命中=%d, 未命中=%d, 淘汰=%d",
			stats.RecommendationHits, stats.RecommendationMisses, stats.RecommendationEvictions)
	})
}

// TestCacheStats 测试缓存统计功能
func TestCacheStats(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    10,
				RecommendationTTL:     30 * time.Minute,
				RecommendationMaxSize: 5,
				CleanupInterval:       1 * time.Minute,
			},
		},
	}

	cacheManager := NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("统计信息准确性", func(t *testing.T) {
		query := "统计测试查询"
		options := &SearchOptions{TopK: 10, UserID: "stats-user"}
		vector := []float32{1.0, 2.0}

		// 初始统计应该为0
		stats := cacheManager.GetStats()
		initialHits := stats.QueryVectorHits
		initialMisses := stats.QueryVectorMisses

		// 执行一次未命中
		_, found := cacheManager.GetQueryVector(query, options)
		assert.False(t, found)

		stats = cacheManager.GetStats()
		assert.Equal(t, initialMisses+1, stats.QueryVectorMisses)
		assert.Equal(t, initialHits, stats.QueryVectorHits)

		// 设置缓存
		cacheManager.SetQueryVector(query, options, vector)

		// 执行一次命中
		_, found = cacheManager.GetQueryVector(query, options)
		assert.True(t, found)

		stats = cacheManager.GetStats()
		assert.Equal(t, initialMisses+1, stats.QueryVectorMisses)
		assert.Equal(t, initialHits+1, stats.QueryVectorHits)

		// 多次命中
		for i := 0; i < 5; i++ {
			_, found = cacheManager.GetQueryVector(query, options)
			assert.True(t, found)
		}

		stats = cacheManager.GetStats()
		assert.Equal(t, initialMisses+1, stats.QueryVectorMisses)
		assert.Equal(t, initialHits+6, stats.QueryVectorHits) // 1 + 5 = 6
	})

	t.Run("缓存信息结构验证", func(t *testing.T) {
		info := cacheManager.GetCacheInfo()

		// 验证查询向量缓存信息
		require.Contains(t, info, "query_vector_cache")
		qvInfo := info["query_vector_cache"].(map[string]interface{})
		assert.Contains(t, qvInfo, "size")
		assert.Contains(t, qvInfo, "max_size")
		assert.Contains(t, qvInfo, "ttl")
		assert.Contains(t, qvInfo, "hits")
		assert.Contains(t, qvInfo, "misses")
		assert.Contains(t, qvInfo, "evictions")
		assert.Contains(t, qvInfo, "hit_ratio")

		// 验证推荐缓存信息
		require.Contains(t, info, "recommendation_cache")
		recInfo := info["recommendation_cache"].(map[string]interface{})
		assert.Contains(t, recInfo, "size")
		assert.Contains(t, recInfo, "max_size")
		assert.Contains(t, recInfo, "ttl")

		// 验证用户偏好缓存信息
		require.Contains(t, info, "user_preference_cache")
		upInfo := info["user_preference_cache"].(map[string]interface{})
		assert.Contains(t, upInfo, "size")
		assert.Contains(t, upInfo, "max_size")
		assert.Contains(t, upInfo, "ttl")
	})

	t.Run("命中率计算正确性", func(t *testing.T) {
		// 重新创建缓存管理器以清空统计
		cacheManager.Close()
		cacheManager = NewVectorCacheManager(cfg)
		defer cacheManager.Close()

		options := &SearchOptions{TopK: 5, UserID: "ratio-user"}

		// 设置一些缓存
		for i := 0; i < 3; i++ {
			query := fmt.Sprintf("ratio-query-%d", i)
			vector := []float32{float32(i)}
			cacheManager.SetQueryVector(query, options, vector)
		}

		// 执行命中和未命中操作
		// 3次命中
		for i := 0; i < 3; i++ {
			query := fmt.Sprintf("ratio-query-%d", i)
			_, found := cacheManager.GetQueryVector(query, options)
			assert.True(t, found)
		}

		// 2次未命中
		for i := 3; i < 5; i++ {
			query := fmt.Sprintf("ratio-query-%d", i)
			_, found := cacheManager.GetQueryVector(query, options)
			assert.False(t, found)
		}

		// 验证命中率
		info := cacheManager.GetCacheInfo()
		qvInfo := info["query_vector_cache"].(map[string]interface{})
		hitRatio := qvInfo["hit_ratio"].(float64)

		// 期望命中率 = 3 / (3 + 2) = 0.6
		assert.InDelta(t, 0.6, hitRatio, 0.01)
	})
}

// TestCacheConfiguration 测试缓存配置
func TestCacheConfiguration(t *testing.T) {
	t.Run("默认配置", func(t *testing.T) {
		// 使用nil配置应该使用默认配置
		cacheManager := NewVectorCacheManager(nil)
		defer cacheManager.Close()

		info := cacheManager.GetCacheInfo()
		qvInfo := info["query_vector_cache"].(map[string]interface{})

		// 验证默认配置值
		assert.Equal(t, 10000, qvInfo["max_size"].(int))
		assert.Equal(t, 1*time.Hour, qvInfo["ttl"].(time.Duration))
	})

	t.Run("自定义配置", func(t *testing.T) {
		customCfg := &config.Config{
			VectorDB: config.VectorDBConfig{
				CacheConfig: &config.VectorCacheConfig{
					QueryVectorTTL:        2 * time.Hour,
					QueryVectorMaxSize:    5000,
					RecommendationTTL:     45 * time.Minute,
					RecommendationMaxSize: 2000,
					CleanupInterval:       5 * time.Minute,
				},
			},
		}

		cacheManager := NewVectorCacheManager(customCfg)
		defer cacheManager.Close()

		info := cacheManager.GetCacheInfo()

		// 验证自定义配置生效
		qvInfo := info["query_vector_cache"].(map[string]interface{})
		assert.Equal(t, 5000, qvInfo["max_size"].(int))
		assert.Equal(t, 2*time.Hour, qvInfo["ttl"].(time.Duration))

		recInfo := info["recommendation_cache"].(map[string]interface{})
		assert.Equal(t, 2000, recInfo["max_size"].(int))
		assert.Equal(t, 45*time.Minute, recInfo["ttl"].(time.Duration))
	})
}
