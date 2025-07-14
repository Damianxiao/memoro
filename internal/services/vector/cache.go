package vector

import (
	"crypto/md5"
	"fmt"
	"sync"
	"time"

	"memoro/internal/config"
	"memoro/internal/logger"
)

// CacheConfig 缓存配置
type CacheConfig struct {
	QueryVectorTTL        time.Duration `yaml:"query_vector_ttl"`
	QueryVectorMaxSize    int           `yaml:"query_vector_max_size"`
	RecommendationTTL     time.Duration `yaml:"recommendation_ttl"`
	RecommendationMaxSize int           `yaml:"recommendation_max_size"`
	UserPreferenceTTL     time.Duration `yaml:"user_preference_ttl"`
	UserPreferenceMaxSize int           `yaml:"user_preference_max_size"`
	CleanupInterval       time.Duration `yaml:"cleanup_interval"`
}

// DefaultCacheConfig 默认缓存配置
func DefaultCacheConfig() *CacheConfig {
	return &CacheConfig{
		QueryVectorTTL:        60 * time.Minute,  // 1小时
		QueryVectorMaxSize:    10000,             // 最多缓存10k个查询向量
		RecommendationTTL:     30 * time.Minute,  // 30分钟
		RecommendationMaxSize: 5000,              // 最多缓存5k个推荐结果
		UserPreferenceTTL:     24 * time.Hour,    // 24小时
		UserPreferenceMaxSize: 1000,              // 最多缓存1k个用户偏好
		CleanupInterval:       10 * time.Minute,  // 10分钟清理一次
	}
}

// CachedQueryVector 缓存的查询向量
type CachedQueryVector struct {
	Vector    []float32              `json:"vector"`
	QueryHash string                 `json:"query_hash"`
	Options   map[string]interface{} `json:"options"`
	CachedAt  time.Time              `json:"cached_at"`
	AccessCount int64                `json:"access_count"`
	LastAccess  time.Time            `json:"last_access"`
}

// CachedRecommendation 缓存的推荐结果
type CachedRecommendation struct {
	Recommendations []*RecommendationItem `json:"recommendations"`
	RequestHash     string                `json:"request_hash"`
	CachedAt        time.Time             `json:"cached_at"`
	AccessCount     int64                 `json:"access_count"`
	LastAccess      time.Time             `json:"last_access"`
}

// CachedUserPreference 缓存的用户偏好
type CachedUserPreference struct {
	UserID          string                 `json:"user_id"`
	Preferences     map[string]interface{} `json:"preferences"`
	InteractionHist []string               `json:"interaction_history"`
	CachedAt        time.Time              `json:"cached_at"`
	LastAccess      time.Time              `json:"last_access"`
}

// VectorCacheManager 向量缓存管理器
type VectorCacheManager struct {
	config *CacheConfig
	logger *logger.Logger

	// 查询向量缓存
	queryVectorCache map[string]*CachedQueryVector
	queryVectorMutex sync.RWMutex

	// 推荐结果缓存
	recommendationCache map[string]*CachedRecommendation
	recommendationMutex sync.RWMutex

	// 用户偏好缓存
	userPreferenceCache map[string]*CachedUserPreference
	userPreferenceMutex sync.RWMutex

	// 统计信息
	stats      *CacheStats
	statsMutex sync.RWMutex

	// 清理控制
	stopCleanup chan struct{}
	cleanupWg   sync.WaitGroup
}

// CacheStats 缓存统计信息
type CacheStats struct {
	QueryVectorHits   int64 `json:"query_vector_hits"`
	QueryVectorMisses int64 `json:"query_vector_misses"`
	QueryVectorEvictions int64 `json:"query_vector_evictions"`

	RecommendationHits   int64 `json:"recommendation_hits"`
	RecommendationMisses int64 `json:"recommendation_misses"`
	RecommendationEvictions int64 `json:"recommendation_evictions"`

	UserPreferenceHits   int64 `json:"user_preference_hits"`
	UserPreferenceMisses int64 `json:"user_preference_misses"`
	UserPreferenceEvictions int64 `json:"user_preference_evictions"`
}

// NewVectorCacheManager 创建向量缓存管理器
func NewVectorCacheManager(cfg *config.Config) *VectorCacheManager {
	cacheConfig := DefaultCacheConfig()
	if cfg != nil && cfg.VectorDB.CacheConfig != nil {
		// 从配置文件加载缓存配置
		if cfg.VectorDB.CacheConfig.QueryVectorTTL > 0 {
			cacheConfig.QueryVectorTTL = cfg.VectorDB.CacheConfig.QueryVectorTTL
		}
		if cfg.VectorDB.CacheConfig.QueryVectorMaxSize > 0 {
			cacheConfig.QueryVectorMaxSize = cfg.VectorDB.CacheConfig.QueryVectorMaxSize
		}
		// 其他配置项...
	}

	manager := &VectorCacheManager{
		config:              cacheConfig,
		logger:              logger.NewLogger("vector-cache"),
		queryVectorCache:    make(map[string]*CachedQueryVector),
		recommendationCache: make(map[string]*CachedRecommendation),
		userPreferenceCache: make(map[string]*CachedUserPreference),
		stats:               &CacheStats{},
		stopCleanup:         make(chan struct{}),
	}

	// 启动定期清理
	manager.startCleanupRoutine()

	manager.logger.Info("Vector cache manager initialized", logger.Fields{
		"query_vector_ttl":        cacheConfig.QueryVectorTTL,
		"query_vector_max_size":   cacheConfig.QueryVectorMaxSize,
		"recommendation_ttl":      cacheConfig.RecommendationTTL,
		"recommendation_max_size": cacheConfig.RecommendationMaxSize,
		"cleanup_interval":        cacheConfig.CleanupInterval,
	})

	return manager
}

// GetQueryVector 获取查询向量
func (cm *VectorCacheManager) GetQueryVector(query string, options *SearchOptions) ([]float32, bool) {
	key := cm.generateQueryVectorKey(query, options)
	
	cm.queryVectorMutex.RLock()
	cached, exists := cm.queryVectorCache[key]
	cm.queryVectorMutex.RUnlock()

	if !exists {
		cm.incrementStat("query_vector_misses")
		return nil, false
	}

	// 检查是否过期
	if time.Since(cached.CachedAt) > cm.config.QueryVectorTTL {
		cm.evictQueryVector(key)
		cm.incrementStat("query_vector_misses")
		return nil, false
	}

	// 更新访问统计
	cm.queryVectorMutex.Lock()
	cached.AccessCount++
	cached.LastAccess = time.Now()
	cm.queryVectorMutex.Unlock()

	cm.incrementStat("query_vector_hits")
	cm.logger.Debug("Query vector cache hit", logger.Fields{
		"query_hash":   cached.QueryHash,
		"access_count": cached.AccessCount,
	})

	return cached.Vector, true
}

// SetQueryVector 设置查询向量
func (cm *VectorCacheManager) SetQueryVector(query string, options *SearchOptions, vector []float32) {
	key := cm.generateQueryVectorKey(query, options)
	
	cached := &CachedQueryVector{
		Vector:      make([]float32, len(vector)),
		QueryHash:   key,
		Options:     cm.serializeOptions(options),
		CachedAt:    time.Now(),
		AccessCount: 1,
		LastAccess:  time.Now(),
	}
	copy(cached.Vector, vector)

	cm.queryVectorMutex.Lock()
	defer cm.queryVectorMutex.Unlock()

	// 检查缓存大小限制
	if len(cm.queryVectorCache) >= cm.config.QueryVectorMaxSize {
		cm.evictLRUQueryVector()
	}

	cm.queryVectorCache[key] = cached
	cm.logger.Debug("Query vector cached", logger.Fields{
		"query_hash":    key,
		"vector_length": len(vector),
	})
}

// GetRecommendation 获取推荐结果
func (cm *VectorCacheManager) GetRecommendation(request *RecommendationRequest) ([]*RecommendationItem, bool) {
	key := cm.generateRecommendationKey(request)
	
	cm.recommendationMutex.RLock()
	cached, exists := cm.recommendationCache[key]
	cm.recommendationMutex.RUnlock()

	if !exists {
		cm.incrementStat("recommendation_misses")
		return nil, false
	}

	// 检查是否过期
	if time.Since(cached.CachedAt) > cm.config.RecommendationTTL {
		cm.evictRecommendation(key)
		cm.incrementStat("recommendation_misses")
		return nil, false
	}

	// 更新访问统计
	cm.recommendationMutex.Lock()
	cached.AccessCount++
	cached.LastAccess = time.Now()
	cm.recommendationMutex.Unlock()

	cm.incrementStat("recommendation_hits")
	cm.logger.Debug("Recommendation cache hit", logger.Fields{
		"request_hash":        cached.RequestHash,
		"recommendations":     len(cached.Recommendations),
		"access_count":        cached.AccessCount,
	})

	return cached.Recommendations, true
}

// SetRecommendation 设置推荐结果
func (cm *VectorCacheManager) SetRecommendation(request *RecommendationRequest, recommendations []*RecommendationItem) {
	key := cm.generateRecommendationKey(request)
	
	cached := &CachedRecommendation{
		Recommendations: make([]*RecommendationItem, len(recommendations)),
		RequestHash:     key,
		CachedAt:        time.Now(),
		AccessCount:     1,
		LastAccess:      time.Now(),
	}
	copy(cached.Recommendations, recommendations)

	cm.recommendationMutex.Lock()
	defer cm.recommendationMutex.Unlock()

	// 检查缓存大小限制
	if len(cm.recommendationCache) >= cm.config.RecommendationMaxSize {
		cm.evictLRURecommendation()
	}

	cm.recommendationCache[key] = cached
	cm.logger.Debug("Recommendation cached", logger.Fields{
		"request_hash":       key,
		"recommendations":    len(recommendations),
	})
}

// generateQueryVectorKey 生成查询向量缓存键
func (cm *VectorCacheManager) generateQueryVectorKey(query string, options *SearchOptions) string {
	data := fmt.Sprintf("%s|%v", query, options)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("qv:%x", hash)
}

// generateRecommendationKey 生成推荐结果缓存键
func (cm *VectorCacheManager) generateRecommendationKey(request *RecommendationRequest) string {
	data := fmt.Sprintf("%s|%s|%s|%d|%f", 
		request.Type, 
		request.UserID, 
		request.SourceDocumentID, 
		request.MaxRecommendations,
		request.MinSimilarity)
	hash := md5.Sum([]byte(data))
	return fmt.Sprintf("rec:%x", hash)
}

// serializeOptions 序列化搜索选项
func (cm *VectorCacheManager) serializeOptions(options *SearchOptions) map[string]interface{} {
	if options == nil {
		return nil
	}
	
	return map[string]interface{}{
		"content_types":        options.ContentTypes,
		"user_id":             options.UserID,
		"top_k":               options.TopK,
		"min_similarity":      options.MinSimilarity,
		"include_content":     options.IncludeContent,
		"similarity_type":     options.SimilarityType,
	}
}

// evictQueryVector 驱逐查询向量
func (cm *VectorCacheManager) evictQueryVector(key string) {
	cm.queryVectorMutex.Lock()
	delete(cm.queryVectorCache, key)
	cm.queryVectorMutex.Unlock()
	cm.incrementStat("query_vector_evictions")
}

// evictLRUQueryVector 驱逐最少使用的查询向量
func (cm *VectorCacheManager) evictLRUQueryVector() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, cached := range cm.queryVectorCache {
		if oldestKey == "" || cached.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.LastAccess
		}
	}
	
	if oldestKey != "" {
		delete(cm.queryVectorCache, oldestKey)
		cm.incrementStat("query_vector_evictions")
	}
}

// evictRecommendation 驱逐推荐结果
func (cm *VectorCacheManager) evictRecommendation(key string) {
	cm.recommendationMutex.Lock()
	delete(cm.recommendationCache, key)
	cm.recommendationMutex.Unlock()
	cm.incrementStat("recommendation_evictions")
}

// evictLRURecommendation 驱逐最少使用的推荐结果
func (cm *VectorCacheManager) evictLRURecommendation() {
	var oldestKey string
	var oldestTime time.Time
	
	for key, cached := range cm.recommendationCache {
		if oldestKey == "" || cached.LastAccess.Before(oldestTime) {
			oldestKey = key
			oldestTime = cached.LastAccess
		}
	}
	
	if oldestKey != "" {
		delete(cm.recommendationCache, oldestKey)
		cm.incrementStat("recommendation_evictions")
	}
}

// incrementStat 增加统计计数
func (cm *VectorCacheManager) incrementStat(statName string) {
	cm.statsMutex.Lock()
	defer cm.statsMutex.Unlock()
	
	switch statName {
	case "query_vector_hits":
		cm.stats.QueryVectorHits++
	case "query_vector_misses":
		cm.stats.QueryVectorMisses++
	case "query_vector_evictions":
		cm.stats.QueryVectorEvictions++
	case "recommendation_hits":
		cm.stats.RecommendationHits++
	case "recommendation_misses":
		cm.stats.RecommendationMisses++
	case "recommendation_evictions":
		cm.stats.RecommendationEvictions++
	case "user_preference_hits":
		cm.stats.UserPreferenceHits++
	case "user_preference_misses":
		cm.stats.UserPreferenceMisses++
	case "user_preference_evictions":
		cm.stats.UserPreferenceEvictions++
	}
}

// GetStats 获取缓存统计信息
func (cm *VectorCacheManager) GetStats() *CacheStats {
	cm.statsMutex.RLock()
	defer cm.statsMutex.RUnlock()
	
	// 返回统计信息的副本
	return &CacheStats{
		QueryVectorHits:          cm.stats.QueryVectorHits,
		QueryVectorMisses:        cm.stats.QueryVectorMisses,
		QueryVectorEvictions:     cm.stats.QueryVectorEvictions,
		RecommendationHits:       cm.stats.RecommendationHits,
		RecommendationMisses:     cm.stats.RecommendationMisses,
		RecommendationEvictions:  cm.stats.RecommendationEvictions,
		UserPreferenceHits:       cm.stats.UserPreferenceHits,
		UserPreferenceMisses:     cm.stats.UserPreferenceMisses,
		UserPreferenceEvictions:  cm.stats.UserPreferenceEvictions,
	}
}

// GetCacheInfo 获取缓存信息
func (cm *VectorCacheManager) GetCacheInfo() map[string]interface{} {
	cm.queryVectorMutex.RLock()
	queryVectorSize := len(cm.queryVectorCache)
	cm.queryVectorMutex.RUnlock()

	cm.recommendationMutex.RLock()
	recommendationSize := len(cm.recommendationCache)
	cm.recommendationMutex.RUnlock()

	cm.userPreferenceMutex.RLock()
	userPreferenceSize := len(cm.userPreferenceCache)
	cm.userPreferenceMutex.RUnlock()

	stats := cm.GetStats()

	return map[string]interface{}{
		"query_vector_cache": map[string]interface{}{
			"size":          queryVectorSize,
			"max_size":      cm.config.QueryVectorMaxSize,
			"ttl":           cm.config.QueryVectorTTL,
			"hits":          stats.QueryVectorHits,
			"misses":        stats.QueryVectorMisses,
			"evictions":     stats.QueryVectorEvictions,
			"hit_ratio":     cm.calculateHitRatio(stats.QueryVectorHits, stats.QueryVectorMisses),
		},
		"recommendation_cache": map[string]interface{}{
			"size":          recommendationSize,
			"max_size":      cm.config.RecommendationMaxSize,
			"ttl":           cm.config.RecommendationTTL,
			"hits":          stats.RecommendationHits,
			"misses":        stats.RecommendationMisses,
			"evictions":     stats.RecommendationEvictions,
			"hit_ratio":     cm.calculateHitRatio(stats.RecommendationHits, stats.RecommendationMisses),
		},
		"user_preference_cache": map[string]interface{}{
			"size":          userPreferenceSize,
			"max_size":      cm.config.UserPreferenceMaxSize,
			"ttl":           cm.config.UserPreferenceTTL,
			"hits":          stats.UserPreferenceHits,
			"misses":        stats.UserPreferenceMisses,
			"evictions":     stats.UserPreferenceEvictions,
			"hit_ratio":     cm.calculateHitRatio(stats.UserPreferenceHits, stats.UserPreferenceMisses),
		},
	}
}

// calculateHitRatio 计算命中率
func (cm *VectorCacheManager) calculateHitRatio(hits, misses int64) float64 {
	total := hits + misses
	if total == 0 {
		return 0.0
	}
	return float64(hits) / float64(total)
}

// startCleanupRoutine 启动清理协程
func (cm *VectorCacheManager) startCleanupRoutine() {
	cm.cleanupWg.Add(1)
	go func() {
		defer cm.cleanupWg.Done()
		ticker := time.NewTicker(cm.config.CleanupInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				cm.cleanupExpiredEntries()
			case <-cm.stopCleanup:
				return
			}
		}
	}()
}

// cleanupExpiredEntries 清理过期条目
func (cm *VectorCacheManager) cleanupExpiredEntries() {
	now := time.Now()
	cleanedQuery := 0
	cleanedRec := 0
	cleanedUser := 0

	// 清理过期的查询向量
	cm.queryVectorMutex.Lock()
	for key, cached := range cm.queryVectorCache {
		if now.Sub(cached.CachedAt) > cm.config.QueryVectorTTL {
			delete(cm.queryVectorCache, key)
			cleanedQuery++
		}
	}
	cm.queryVectorMutex.Unlock()

	// 清理过期的推荐结果
	cm.recommendationMutex.Lock()
	for key, cached := range cm.recommendationCache {
		if now.Sub(cached.CachedAt) > cm.config.RecommendationTTL {
			delete(cm.recommendationCache, key)
			cleanedRec++
		}
	}
	cm.recommendationMutex.Unlock()

	// 清理过期的用户偏好
	cm.userPreferenceMutex.Lock()
	for key, cached := range cm.userPreferenceCache {
		if now.Sub(cached.CachedAt) > cm.config.UserPreferenceTTL {
			delete(cm.userPreferenceCache, key)
			cleanedUser++
		}
	}
	cm.userPreferenceMutex.Unlock()

	if cleanedQuery > 0 || cleanedRec > 0 || cleanedUser > 0 {
		cm.logger.Debug("Cache cleanup completed", logger.Fields{
			"cleaned_query_vectors":   cleanedQuery,
			"cleaned_recommendations": cleanedRec,
			"cleaned_user_preferences": cleanedUser,
		})
	}
}

// Close 关闭缓存管理器
func (cm *VectorCacheManager) Close() error {
	cm.logger.Info("Shutting down vector cache manager")
	
	// 停止清理协程
	close(cm.stopCleanup)
	cm.cleanupWg.Wait()
	
	// 清理所有缓存
	cm.queryVectorMutex.Lock()
	cm.queryVectorCache = make(map[string]*CachedQueryVector)
	cm.queryVectorMutex.Unlock()

	cm.recommendationMutex.Lock()
	cm.recommendationCache = make(map[string]*CachedRecommendation)
	cm.recommendationMutex.Unlock()

	cm.userPreferenceMutex.Lock()
	cm.userPreferenceCache = make(map[string]*CachedUserPreference)
	cm.userPreferenceMutex.Unlock()

	cm.logger.Info("Vector cache manager shut down completed")
	return nil
}