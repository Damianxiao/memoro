package vector

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// SearchEngine 智能搜索引擎
type SearchEngine struct {
	chromaClient        *ChromaClient
	embeddingService    *EmbeddingService
	similarityCalc      *SimilarityCalculator
	config              config.VectorDBConfig
	logger              *logger.Logger
}

// SearchOptions 搜索选项
type SearchOptions struct {
	Query               string                 `json:"query"`                          // 查询文本
	ContentTypes        []models.ContentType  `json:"content_types,omitempty"`       // 内容类型过滤
	UserID              string                 `json:"user_id,omitempty"`             // 用户ID过滤
	TopK                int                    `json:"top_k"`                         // 返回结果数量
	MinSimilarity       float32                `json:"min_similarity"`                // 最小相似度阈值
	IncludeContent      bool                   `json:"include_content"`               // 是否包含原文
	SimilarityType      SimilarityType         `json:"similarity_type,omitempty"`     // 相似度计算类型
	TimeRange           *TimeRange             `json:"time_range,omitempty"`          // 时间范围过滤
	Tags                []string               `json:"tags,omitempty"`                // 标签过滤
	ImportanceThreshold float64                `json:"importance_threshold,omitempty"` // 重要性阈值
	EnableReranking     bool                   `json:"enable_reranking"`              // 启用重排序
	MaxResults          int                    `json:"max_results"`                   // 最大结果数量限制
}

// TimeRange 时间范围
type TimeRange struct {
	StartTime time.Time `json:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time"`   // 结束时间
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results         []*SearchResultItem `json:"results"`          // 搜索结果
	TotalResults    int                 `json:"total_results"`    // 总结果数
	QueryTime       time.Duration       `json:"query_time"`       // 查询耗时
	ProcessedQuery  string              `json:"processed_query"`  // 处理后的查询
	SimilarityType  SimilarityType      `json:"similarity_type"`  // 使用的相似度类型
	VectorDimension int                 `json:"vector_dimension"` // 向量维度
	Metadata        map[string]interface{} `json:"metadata"`      // 元数据信息
}

// SearchResultItem 搜索结果项
type SearchResultItem struct {
	DocumentID       string                 `json:"document_id"`       // 文档ID
	Content          string                 `json:"content,omitempty"` // 文档内容
	Similarity       float64                `json:"similarity"`        // 相似度分数
	Distance         float32                `json:"distance"`          // 向量距离
	Rank             int                    `json:"rank"`              // 排名
	Metadata         map[string]interface{} `json:"metadata"`          // 文档元数据
	RelevanceScore   float64                `json:"relevance_score"`   // 综合相关性分数
	MatchedKeywords  []string               `json:"matched_keywords"`  // 匹配的关键词
	ContentSummary   string                 `json:"content_summary"`   // 内容摘要
	CreatedAt        time.Time              `json:"created_at"`        // 创建时间
}

// NewSearchEngine 创建搜索引擎
func NewSearchEngine() (*SearchEngine, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("search engine config")
	}

	searchLogger := logger.NewLogger("search-engine")

	// 初始化Chroma客户端
	chromaClient, err := NewChromaClient()
	if err != nil {
		return nil, err
	}

	// 初始化embedding服务
	embeddingService, err := NewEmbeddingService()
	if err != nil {
		return nil, err
	}

	// 初始化相似度计算器
	similarityCalc := NewSimilarityCalculator()

	engine := &SearchEngine{
		chromaClient:     chromaClient,
		embeddingService: embeddingService,
		similarityCalc:   similarityCalc,
		config:           cfg.VectorDB,
		logger:           searchLogger,
	}

	searchLogger.Info("Search engine initialized", logger.Fields{
		"vector_db_host": cfg.VectorDB.Host,
		"vector_db_port": cfg.VectorDB.Port,
		"collection":     cfg.VectorDB.Collection,
	})

	return engine, nil
}

// Search 执行智能搜索
func (se *SearchEngine) Search(ctx context.Context, options *SearchOptions) (*SearchResponse, error) {
	if options == nil {
		return nil, errors.ErrValidationFailed("search_options", "cannot be nil")
	}

	if strings.TrimSpace(options.Query) == "" {
		return nil, errors.ErrValidationFailed("query", "cannot be empty")
	}

	startTime := time.Now()

	se.logger.Info("Executing intelligent search", logger.Fields{
		"query":           options.Query,
		"top_k":           options.TopK,
		"min_similarity":  options.MinSimilarity,
		"content_types":   options.ContentTypes,
		"user_id":         options.UserID,
		"enable_reranking": options.EnableReranking,
	})

	// 设置默认值
	if options.TopK <= 0 {
		options.TopK = 10
	}
	if options.MaxResults <= 0 {
		options.MaxResults = 100
	}
	if options.SimilarityType == "" {
		options.SimilarityType = SimilarityTypeCosine
	}

	// 1. 预处理查询文本
	processedQuery := se.preprocessQuery(options.Query)

	// 2. 生成查询向量
	queryVector, err := se.generateQueryVector(ctx, processedQuery, options)
	if err != nil {
		return nil, err
	}

	// 3. 构建过滤条件
	filter := se.buildFilter(options)

	// 4. 执行向量搜索
	searchQuery := &SearchQuery{
		QueryText:     processedQuery,
		QueryVector:   queryVector,
		TopK:          options.MaxResults, // 先获取更多结果用于重排序
		Filter:        filter,
		IncludeText:   options.IncludeContent,
		MinSimilarity: options.MinSimilarity,
	}

	vectorResults, err := se.chromaClient.Search(ctx, searchQuery)
	if err != nil {
		return nil, err
	}

	// 5. 转换为搜索结果项
	resultItems, err := se.convertToSearchResults(ctx, vectorResults, options, queryVector)
	if err != nil {
		return nil, err
	}

	// 6. 执行重排序（如果启用）
	if options.EnableReranking && len(resultItems) > 1 {
		resultItems = se.rerankResults(ctx, resultItems, options)
	}

	// 7. 应用最终过滤和限制
	finalResults := se.applyFinalFiltering(resultItems, options)

	// 8. 设置排名
	for i, result := range finalResults {
		result.Rank = i + 1
	}

	queryTime := time.Since(startTime)

	response := &SearchResponse{
		Results:         finalResults,
		TotalResults:    len(finalResults),
		QueryTime:       queryTime,
		ProcessedQuery:  processedQuery,
		SimilarityType:  options.SimilarityType,
		VectorDimension: len(queryVector),
		Metadata: map[string]interface{}{
			"original_results": len(vectorResults.Documents),
			"after_filtering":  len(resultItems),
			"final_count":      len(finalResults),
			"reranking_enabled": options.EnableReranking,
		},
	}

	se.logger.Info("Search completed", logger.Fields{
		"query_time":      queryTime,
		"total_results":   len(finalResults),
		"vector_results":  len(vectorResults.Documents),
		"processed_query": processedQuery,
	})

	return response, nil
}

// preprocessQuery 预处理查询文本
func (se *SearchEngine) preprocessQuery(query string) string {
	// 清理查询文本
	processed := strings.TrimSpace(query)
	
	// 移除多余的空白字符
	processed = strings.ReplaceAll(processed, "\n", " ")
	processed = strings.ReplaceAll(processed, "\t", " ")
	
	// 移除重复空格
	for strings.Contains(processed, "  ") {
		processed = strings.ReplaceAll(processed, "  ", " ")
	}

	// 基本的查询扩展（可以根据需要添加更复杂的逻辑）
	if len(processed) < 10 && !strings.Contains(processed, " ") {
		// 对于短查询，添加通配符支持
		processed = strings.ToLower(processed)
	}

	return processed
}

// generateQueryVector 生成查询向量
func (se *SearchEngine) generateQueryVector(ctx context.Context, query string, options *SearchOptions) ([]float32, error) {
	se.logger.Debug("Generating query vector", logger.Fields{
		"query_length": len(query),
	})

	// 创建embedding请求
	embeddingReq := &EmbeddingRequest{
		Text:        query,
		ContentType: models.ContentTypeText, // 查询默认为文本类型
		MaxTokens:   1000,                   // 查询向量的token限制
		Metadata: map[string]interface{}{
			"query_type": "search",
			"user_id":    options.UserID,
		},
	}

	// 生成embedding
	result, err := se.embeddingService.GenerateEmbedding(ctx, embeddingReq)
	if err != nil {
		return nil, err
	}

	se.logger.Debug("Query vector generated", logger.Fields{
		"dimension":   result.Dimension,
		"tokens_used": result.TokensUsed,
	})

	return result.Vector, nil
}

// buildFilter 构建过滤条件
func (se *SearchEngine) buildFilter(options *SearchOptions) map[string]interface{} {
	filter := make(map[string]interface{})

	// 用户ID过滤
	if options.UserID != "" {
		filter["user_id"] = options.UserID
	}

	// 内容类型过滤
	if len(options.ContentTypes) > 0 {
		contentTypeStrs := make([]string, len(options.ContentTypes))
		for i, ct := range options.ContentTypes {
			contentTypeStrs[i] = string(ct)
		}
		filter["content_type"] = map[string]interface{}{
			"$in": contentTypeStrs,
		}
	}

	// 时间范围过滤
	if options.TimeRange != nil {
		timeFilter := make(map[string]interface{})
		if !options.TimeRange.StartTime.IsZero() {
			timeFilter["$gte"] = options.TimeRange.StartTime.Unix()
		}
		if !options.TimeRange.EndTime.IsZero() {
			timeFilter["$lte"] = options.TimeRange.EndTime.Unix()
		}
		if len(timeFilter) > 0 {
			filter["created_at"] = timeFilter
		}
	}

	// 重要性阈值过滤
	if options.ImportanceThreshold > 0 {
		filter["importance_score"] = map[string]interface{}{
			"$gte": options.ImportanceThreshold,
		}
	}

	// 标签过滤
	if len(options.Tags) > 0 {
		filter["tags"] = map[string]interface{}{
			"$in": options.Tags,
		}
	}

	return filter
}

// convertToSearchResults 转换为搜索结果项
func (se *SearchEngine) convertToSearchResults(ctx context.Context, vectorResults *SearchResult, options *SearchOptions, queryVector []float32) ([]*SearchResultItem, error) {
	results := make([]*SearchResultItem, 0, len(vectorResults.Documents))

	for _, doc := range vectorResults.Documents {
		// 计算相似度分数
		similarity := float64(0)
		if len(doc.Embedding) > 0 {
			sim, err := se.similarityCalc.CalculateSimilarity(queryVector, doc.Embedding, options.SimilarityType)
			if err != nil {
				se.logger.Warn("Failed to calculate similarity", logger.Fields{
					"document_id": doc.ID,
					"error":       err.Error(),
				})
				continue
			}
			similarity = sim
		}

		// 提取关键词匹配
		matchedKeywords := se.extractMatchedKeywords(options.Query, doc.Content, doc.Metadata)

		// 生成内容摘要
		contentSummary := se.generateContentSummary(doc.Content, options.Query)

		// 计算综合相关性分数
		relevanceScore := se.calculateRelevanceScore(similarity, matchedKeywords, doc.Metadata, options)

		resultItem := &SearchResultItem{
			DocumentID:      doc.ID,
			Content:         doc.Content,
			Similarity:      similarity,
			Distance:        doc.Distance,
			Metadata:        doc.Metadata,
			RelevanceScore:  relevanceScore,
			MatchedKeywords: matchedKeywords,
			ContentSummary:  contentSummary,
			CreatedAt:       doc.CreatedAt,
		}

		results = append(results, resultItem)
	}

	se.logger.Debug("Converted vector results to search results", logger.Fields{
		"vector_count": len(vectorResults.Documents),
		"result_count": len(results),
	})

	return results, nil
}

// extractMatchedKeywords 提取匹配的关键词
func (se *SearchEngine) extractMatchedKeywords(query string, content string, metadata map[string]interface{}) []string {
	queryWords := strings.Fields(strings.ToLower(query))
	contentLower := strings.ToLower(content)
	
	matched := make([]string, 0)
	for _, word := range queryWords {
		if len(word) > 2 && strings.Contains(contentLower, word) {
			matched = append(matched, word)
		}
	}

	// 从元数据中提取关键词匹配
	if keywords, exists := metadata["keywords"]; exists {
		if keywordList, ok := keywords.([]interface{}); ok {
			for _, kw := range keywordList {
				if kwStr, ok := kw.(string); ok {
					kwLower := strings.ToLower(kwStr)
					for _, queryWord := range queryWords {
						if strings.Contains(kwLower, queryWord) || strings.Contains(queryWord, kwLower) {
							matched = append(matched, kwStr)
						}
					}
				}
			}
		}
	}

	// 去重
	uniqueMatched := make([]string, 0)
	seen := make(map[string]bool)
	for _, word := range matched {
		if !seen[word] {
			uniqueMatched = append(uniqueMatched, word)
			seen[word] = true
		}
	}

	return uniqueMatched
}

// generateContentSummary 生成内容摘要
func (se *SearchEngine) generateContentSummary(content string, query string) string {
	maxLength := 200
	
	if len(content) <= maxLength {
		return content
	}

	// 尝试找到包含查询词的段落
	queryWords := strings.Fields(strings.ToLower(query))
	contentLower := strings.ToLower(content)
	
	bestStart := 0
	maxMatches := 0
	
	// 滑动窗口寻找最相关的片段
	windowSize := maxLength
	for i := 0; i <= len(content)-windowSize; i += 50 {
		windowText := contentLower[i:i+windowSize]
		matches := 0
		for _, word := range queryWords {
			if strings.Contains(windowText, word) {
				matches++
			}
		}
		if matches > maxMatches {
			maxMatches = matches
			bestStart = i
		}
	}

	// 截取最相关的片段
	end := bestStart + maxLength
	if end > len(content) {
		end = len(content)
	}
	
	summary := content[bestStart:end]
	
	// 确保不在单词中间截断
	if bestStart > 0 {
		if spaceIdx := strings.Index(summary, " "); spaceIdx > 0 {
			summary = summary[spaceIdx+1:]
		}
		summary = "..." + summary
	}
	
	if end < len(content) {
		if spaceIdx := strings.LastIndex(summary, " "); spaceIdx > 0 {
			summary = summary[:spaceIdx]
		}
		summary = summary + "..."
	}

	return summary
}

// calculateRelevanceScore 计算综合相关性分数
func (se *SearchEngine) calculateRelevanceScore(similarity float64, matchedKeywords []string, metadata map[string]interface{}, options *SearchOptions) float64 {
	// 基础相似度分数 (权重: 0.6)
	relevanceScore := similarity * 0.6

	// 关键词匹配分数 (权重: 0.2)
	keywordScore := float64(len(matchedKeywords)) / float64(len(strings.Fields(options.Query)))
	if keywordScore > 1.0 {
		keywordScore = 1.0
	}
	relevanceScore += keywordScore * 0.2

	// 重要性分数 (权重: 0.1)
	if importanceVal, exists := metadata["importance_score"]; exists {
		if importance, ok := importanceVal.(float64); ok {
			relevanceScore += (importance / 10.0) * 0.1 // 假设重要性分数0-10
		}
	}

	// 新鲜度分数 (权重: 0.1)
	if createdAtVal, exists := metadata["created_at"]; exists {
		if createdAtInt, ok := createdAtVal.(int64); ok {
			createdAt := time.Unix(createdAtInt, 0)
			daysSinceCreation := time.Since(createdAt).Hours() / 24
			freshnessScore := 1.0 / (1.0 + daysSinceCreation/365.0) // 一年后降到50%
			relevanceScore += freshnessScore * 0.1
		}
	}

	// 确保分数在[0,1]范围内
	if relevanceScore > 1.0 {
		relevanceScore = 1.0
	}
	if relevanceScore < 0.0 {
		relevanceScore = 0.0
	}

	return relevanceScore
}

// rerankResults 重排序结果
func (se *SearchEngine) rerankResults(ctx context.Context, results []*SearchResultItem, options *SearchOptions) []*SearchResultItem {
	se.logger.Debug("Reranking search results", logger.Fields{
		"result_count": len(results),
	})

	// 按照综合相关性分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results
}

// applyFinalFiltering 应用最终过滤
func (se *SearchEngine) applyFinalFiltering(results []*SearchResultItem, options *SearchOptions) []*SearchResultItem {
	filtered := make([]*SearchResultItem, 0)

	for _, result := range results {
		// 应用最小相似度过滤
		if result.Similarity < float64(options.MinSimilarity) {
			continue
		}

		// 如果不需要内容，清空内容字段
		if !options.IncludeContent {
			result.Content = ""
		}

		filtered = append(filtered, result)

		// 限制结果数量
		if len(filtered) >= options.TopK {
			break
		}
	}

	return filtered
}

// IndexDocument 索引文档到向量数据库
func (se *SearchEngine) IndexDocument(ctx context.Context, contentItem *models.ContentItem) error {
	if contentItem == nil {
		return errors.ErrValidationFailed("content_item", "cannot be nil")
	}

	se.logger.Info("Indexing document", logger.Fields{
		"content_id":   contentItem.ID,
		"content_type": string(contentItem.Type),
		"user_id":      contentItem.UserID,
	})

	// 创建向量文档
	vectorDoc, err := se.embeddingService.CreateContentVector(ctx, contentItem)
	if err != nil {
		return err
	}

	// 添加到向量数据库
	if err := se.chromaClient.AddDocument(ctx, vectorDoc); err != nil {
		return err
	}

	se.logger.Info("Document indexed successfully", logger.Fields{
		"content_id": contentItem.ID,
		"vector_dim": len(vectorDoc.Embedding),
	})

	return nil
}

// BatchIndexDocuments 批量索引文档
func (se *SearchEngine) BatchIndexDocuments(ctx context.Context, contentItems []*models.ContentItem) error {
	if len(contentItems) == 0 {
		return errors.ErrValidationFailed("content_items", "cannot be empty")
	}

	se.logger.Info("Batch indexing documents", logger.Fields{
		"batch_size": len(contentItems),
	})

	vectorDocs := make([]*VectorDocument, 0, len(contentItems))

	// 生成向量文档
	for _, item := range contentItems {
		vectorDoc, err := se.embeddingService.CreateContentVector(ctx, item)
		if err != nil {
			se.logger.Error("Failed to create vector for content item", logger.Fields{
				"content_id": item.ID,
				"error":      err.Error(),
			})
			continue
		}
		vectorDocs = append(vectorDocs, vectorDoc)
	}

	// 批量添加到向量数据库
	if len(vectorDocs) > 0 {
		if err := se.chromaClient.AddDocuments(ctx, vectorDocs); err != nil {
			return err
		}
	}

	se.logger.Info("Batch indexing completed", logger.Fields{
		"total_items":    len(contentItems),
		"indexed_items":  len(vectorDocs),
		"failed_items":   len(contentItems) - len(vectorDocs),
	})

	return nil
}

// DeleteDocument 从索引中删除文档
func (se *SearchEngine) DeleteDocument(ctx context.Context, documentID string) error {
	if documentID == "" {
		return errors.ErrValidationFailed("document_id", "cannot be empty")
	}

	se.logger.Info("Deleting document from index", logger.Fields{
		"document_id": documentID,
	})

	return se.chromaClient.DeleteDocument(ctx, documentID)
}

// UpdateDocument 更新索引中的文档
func (se *SearchEngine) UpdateDocument(ctx context.Context, contentItem *models.ContentItem) error {
	if contentItem == nil {
		return errors.ErrValidationFailed("content_item", "cannot be nil")
	}

	se.logger.Info("Updating document in index", logger.Fields{
		"content_id": contentItem.ID,
	})

	// 重新生成向量文档
	vectorDoc, err := se.embeddingService.CreateContentVector(ctx, contentItem)
	if err != nil {
		return err
	}

	// 更新向量数据库
	return se.chromaClient.UpdateDocument(ctx, vectorDoc)
}

// GetSearchStats 获取搜索统计信息
func (se *SearchEngine) GetSearchStats(ctx context.Context) (map[string]interface{}, error) {
	// 获取Chroma集合信息
	collectionInfo, err := se.chromaClient.GetCollectionInfo(ctx)
	if err != nil {
		return nil, err
	}

	stats := map[string]interface{}{
		"collection_info":    collectionInfo,
		"engine_type":        "semantic_search",
		"similarity_types":   []string{"cosine", "euclidean", "dot", "manhattan"},
		"supported_features": []string{"vector_search", "metadata_filtering", "reranking", "batch_operations"},
	}

	return stats, nil
}

// HealthCheck 健康检查
func (se *SearchEngine) HealthCheck(ctx context.Context) error {
	se.logger.Debug("Performing search engine health check")

	// 检查Chroma客户端
	if err := se.chromaClient.HealthCheck(ctx); err != nil {
		return fmt.Errorf("chroma client health check failed: %w", err)
	}

	// 检查embedding服务（通过生成一个简单的测试向量）
	testReq := &EmbeddingRequest{
		Text:        "health check test",
		ContentType: models.ContentTypeText,
		MaxTokens:   50,
	}
	_, err := se.embeddingService.GenerateEmbedding(ctx, testReq)
	if err != nil {
		return fmt.Errorf("embedding service health check failed: %w", err)
	}

	se.logger.Debug("Search engine health check passed")
	return nil
}

// GetRecommendations 获取推荐内容
func (se *SearchEngine) GetRecommendations(ctx context.Context, request *RecommendationRequest) (*RecommendationResponse, error) {
	// 创建推荐系统实例
	recommender, err := NewRecommender()
	if err != nil {
		return nil, err
	}
	defer recommender.Close()

	// 调用推荐系统
	return recommender.GetRecommendations(ctx, request)
}

// Close 关闭搜索引擎
func (se *SearchEngine) Close() error {
	se.logger.Info("Closing search engine")

	var err error

	// 关闭Chroma客户端
	if closeErr := se.chromaClient.Close(); closeErr != nil {
		se.logger.Error("Failed to close Chroma client", logger.Fields{"error": closeErr.Error()})
		err = closeErr
	}

	// 关闭embedding服务
	if closeErr := se.embeddingService.Close(); closeErr != nil {
		se.logger.Error("Failed to close embedding service", logger.Fields{"error": closeErr.Error()})
		err = closeErr
	}

	se.logger.Info("Search engine closed")
	return err
}