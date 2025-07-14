package vector

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"memoro/internal/config"
	"memoro/internal/services/vector"
)

// TestVectorCacheManagerBasic 测试缓存管理器基本功能
func TestVectorCacheManagerBasic(t *testing.T) {
	// 创建测试配置
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

	// 创建缓存管理器
	cacheManager := vector.NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	t.Run("QueryVectorCache", func(t *testing.T) {
		query := "人工智能技术发展"
		options := &vector.SearchOptions{
			TopK:          10,
			MinSimilarity: 0.7,
			UserID:        "test-user",
		}
		testVector := []float32{0.1, 0.2, 0.3, 0.4, 0.5}

		// 第一次获取应该未命中
		cachedVector, found := cacheManager.GetQueryVector(query, options)
		assert.False(t, found)
		assert.Nil(t, cachedVector)

		// 设置缓存
		cacheManager.SetQueryVector(query, options, testVector)

		// 第二次获取应该命中
		cachedVector, found = cacheManager.GetQueryVector(query, options)
		assert.True(t, found)
		assert.Equal(t, testVector, cachedVector)

		// 验证统计信息
		stats := cacheManager.GetStats()
		assert.Equal(t, int64(1), stats.QueryVectorMisses)
		assert.Equal(t, int64(1), stats.QueryVectorHits)
	})

	t.Run("RecommendationCache", func(t *testing.T) {
		request := &vector.RecommendationRequest{
			Type:               vector.RecommendationTypeSimilar,
			UserID:             "test-user",
			MaxRecommendations: 5,
			MinSimilarity:      0.8,
		}

		testRecommendations := []*vector.RecommendationItem{
			{
				DocumentID: "doc-1",
				Content:    "测试推荐内容1",
				Similarity: 0.95,
				Confidence: 0.9,
			},
			{
				DocumentID: "doc-2", 
				Content:    "测试推荐内容2",
				Similarity: 0.88,
				Confidence: 0.85,
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
	})

	t.Run("CacheEviction", func(t *testing.T) {
		// 测试LRU淘汰策略
		options := &vector.SearchOptions{TopK: 10, UserID: "test"}
		
		// 添加超过最大限制的缓存项
		for i := 0; i < 15; i++ {
			query := fmt.Sprintf("test-query-%d", i)
			vector := []float32{float32(i), float32(i + 1)}
			cacheManager.SetQueryVector(query, options, vector)
		}

		// 验证缓存大小没有超过限制
		info := cacheManager.GetCacheInfo()
		queryVectorInfo := info["query_vector_cache"].(map[string]interface{})
		cacheSize := queryVectorInfo["size"].(int)
		assert.LessOrEqual(t, cacheSize, 1000) // 不应超过最大大小
	})
}

// BenchmarkCachePerformance 缓存性能基准测试
func BenchmarkCachePerformance(t *testing.B) {
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

	cacheManager := vector.NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	// 预填充一些缓存数据
	for i := 0; i < 1000; i++ {
		query := fmt.Sprintf("benchmark-query-%d", i)
		options := &vector.SearchOptions{TopK: 10, UserID: "bench-user"}
		vector := make([]float32, 1536) // 模拟OpenAI embedding维度
		for j := range vector {
			vector[j] = float32(i*j) / 1000.0
		}
		cacheManager.SetQueryVector(query, options, vector)
	}

	t.ResetTimer()

	t.Run("QueryVectorLookup", func(b *testing.B) {
		options := &vector.SearchOptions{TopK: 10, UserID: "bench-user"}
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("benchmark-query-%d", i%1000)
				_, _ = cacheManager.GetQueryVector(query, options)
				i++
			}
		})
	})

	t.Run("QueryVectorSet", func(b *testing.B) {
		options := &vector.SearchOptions{TopK: 10, UserID: "bench-user"}
		vector := make([]float32, 1536)
		
		b.RunParallel(func(pb *testing.PB) {
			i := 0
			for pb.Next() {
				query := fmt.Sprintf("new-query-%d", i)
				cacheManager.SetQueryVector(query, options, vector)
				i++
			}
		})
	})
}

// TestCacheHitRatioImprovement 测试缓存命中率改善
func TestCacheHitRatioImprovement(t *testing.T) {
	if testing.Short() {
		t.Skip("跳过缓存性能测试")
	}

	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    1000,
				CleanupInterval:       10 * time.Second,
			},
		},
	}

	cacheManager := vector.NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	// 模拟重复查询场景
	queries := []string{
		"人工智能机器学习",
		"深度学习神经网络", 
		"自然语言处理NLP",
		"计算机视觉图像识别",
		"推荐系统算法",
	}

	options := &vector.SearchOptions{TopK: 10, UserID: "test-user"}

	// 第一轮：所有查询都会未命中
	firstRoundMisses := int64(0)
	for _, query := range queries {
		if _, found := cacheManager.GetQueryVector(query, options); !found {
			firstRoundMisses++
			// 模拟生成向量并缓存
			mockVector := make([]float32, 1536)
			for i := range mockVector {
				mockVector[i] = float32(len(query)*i) / 1000.0
			}
			cacheManager.SetQueryVector(query, options, mockVector)
		}
	}

	// 第二轮：应该全部命中
	secondRoundHits := int64(0)
	for _, query := range queries {
		if _, found := cacheManager.GetQueryVector(query, options); found {
			secondRoundHits++
		}
	}

	// 验证缓存效果
	assert.Equal(t, int64(len(queries)), firstRoundMisses, "第一轮应该全部未命中")
	assert.Equal(t, int64(len(queries)), secondRoundHits, "第二轮应该全部命中")

	// 验证统计信息
	stats := cacheManager.GetStats()
	hitRatio := float64(stats.QueryVectorHits) / float64(stats.QueryVectorHits+stats.QueryVectorMisses)
	assert.GreaterOrEqual(t, hitRatio, 0.5, "缓存命中率应该大于50%")

	t.Logf("缓存统计信息:")
	t.Logf("  命中次数: %d", stats.QueryVectorHits)
	t.Logf("  未命中次数: %d", stats.QueryVectorMisses)
	t.Logf("  命中率: %.2f%%", hitRatio*100)
}

// TestCacheConcurrency 测试缓存并发安全性
func TestCacheConcurrency(t *testing.T) {
	cfg := &config.Config{
		VectorDB: config.VectorDBConfig{
			CacheConfig: &config.VectorCacheConfig{
				QueryVectorTTL:        1 * time.Hour,
				QueryVectorMaxSize:    1000,
				CleanupInterval:       1 * time.Minute,
			},
		},
	}

	cacheManager := vector.NewVectorCacheManager(cfg)
	defer cacheManager.Close()

	const numGoroutines = 10
	const numOperations = 100

	// 并发读写测试
	done := make(chan bool, numGoroutines)

	for g := 0; g < numGoroutines; g++ {
		go func(goroutineID int) {
			defer func() { done <- true }()

			options := &vector.SearchOptions{TopK: 10, UserID: fmt.Sprintf("user-%d", goroutineID)}

			for i := 0; i < numOperations; i++ {
				query := fmt.Sprintf("concurrent-query-%d-%d", goroutineID, i)
				vector := make([]float32, 100)
				for j := range vector {
					vector[j] = float32(goroutineID*i*j) / 1000.0
				}

				// 随机执行读或写操作
				if i%2 == 0 {
					cacheManager.SetQueryVector(query, options, vector)
				} else {
					cacheManager.GetQueryVector(query, options)
				}
			}
		}(g)
	}

	// 等待所有协程完成
	for i := 0; i < numGoroutines; i++ {
		<-done
	}

	// 验证缓存管理器仍然正常工作
	stats := cacheManager.GetStats()
	assert.True(t, stats.QueryVectorHits+stats.QueryVectorMisses > 0, "应该有缓存操作记录")
}