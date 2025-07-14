package vector

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// Recommender 推荐系统
type Recommender struct {
	searchEngine       *SearchEngine
	similarityCalc     *SimilarityCalculator
	ranker             *Ranker
	logger             *logger.Logger
}

// RecommendationType 推荐类型
type RecommendationType string

const (
	RecommendationTypeSimilar      RecommendationType = "similar"       // 相似内容推荐
	RecommendationTypeRelated      RecommendationType = "related"       // 相关内容推荐
	RecommendationTypePersonalized RecommendationType = "personalized"  // 个性化推荐
	RecommendationTypeTrending     RecommendationType = "trending"      // 热门推荐
	RecommendationTypeCollaborative RecommendationType = "collaborative" // 协同过滤推荐
	RecommendationTypeHybrid       RecommendationType = "hybrid"        // 混合推荐
)

// RecommendationRequest 推荐请求
type RecommendationRequest struct {
	Type                RecommendationType    `json:"type"`                          // 推荐类型
	UserID              string                `json:"user_id"`                       // 用户ID
	SourceDocumentID    string                `json:"source_document_id,omitempty"` // 源文档ID
	SourceQuery         string                `json:"source_query,omitempty"`       // 源查询
	MaxRecommendations  int                   `json:"max_recommendations"`          // 最大推荐数量
	ContentTypes        []models.ContentType  `json:"content_types,omitempty"`      // 内容类型过滤
	ExcludeDocuments    []string              `json:"exclude_documents,omitempty"`  // 排除的文档ID
	TimeRange           *TimeRange            `json:"time_range,omitempty"`         // 时间范围
	MinSimilarity       float32               `json:"min_similarity"`               // 最小相似度
	DiversityEnabled    bool                  `json:"diversity_enabled"`            // 启用多样性
	PersonalizationCtx  *PersonalizationContext `json:"personalization,omitempty"`  // 个性化上下文
	IncludeExplanations bool                  `json:"include_explanations"`         // 包含推荐解释
}

// RecommendationResponse 推荐响应
type RecommendationResponse struct {
	Recommendations []*RecommendationItem `json:"recommendations"`    // 推荐结果
	TotalFound      int                   `json:"total_found"`        // 总发现数量
	ProcessTime     time.Duration         `json:"process_time"`       // 处理时间
	RecommendationType RecommendationType `json:"recommendation_type"` // 推荐类型
	Metadata        map[string]interface{} `json:"metadata"`          // 元数据
}

// RecommendationItem 推荐项
type RecommendationItem struct {
	DocumentID        string                 `json:"document_id"`        // 文档ID
	Content           string                 `json:"content,omitempty"`  // 文档内容
	Similarity        float64                `json:"similarity"`         // 相似度分数
	Confidence        float64                `json:"confidence"`         // 推荐置信度
	Rank              int                    `json:"rank"`               // 排名
	Metadata          map[string]interface{} `json:"metadata"`           // 文档元数据
	RecommendationScore float64              `json:"recommendation_score"` // 推荐分数
	Explanation       *RecommendationExplanation `json:"explanation,omitempty"` // 推荐解释
	RelatedKeywords   []string               `json:"related_keywords"`   // 相关关键词
	CreatedAt         time.Time              `json:"created_at"`         // 创建时间
}

// RecommendationExplanation 推荐解释
type RecommendationExplanation struct {
	Reason          string            `json:"reason"`                     // 推荐原因
	SimilarityScore float64           `json:"similarity_score"`           // 相似度分数
	FactorBreakdown map[string]float64 `json:"factor_breakdown"`          // 因子分解
	MatchedFeatures []string          `json:"matched_features"`           // 匹配的特征
	UserPreferences []string          `json:"user_preferences,omitempty"` // 用户偏好匹配
}

// CollaborativeFilteringData 协同过滤数据
type CollaborativeFilteringData struct {
	UserInteractions   map[string][]string    `json:"user_interactions"`   // 用户交互数据
	DocumentSimilarity map[string][]string    `json:"document_similarity"`  // 文档相似性
	UserSimilarity     map[string]float64     `json:"user_similarity"`      // 用户相似性
}

// TrendingAnalysis 热门分析
type TrendingAnalysis struct {
	DocumentScores  map[string]float64 `json:"document_scores"`  // 文档热门分数
	TimeWindow      time.Duration      `json:"time_window"`      // 时间窗口
	InteractionCount map[string]int    `json:"interaction_count"` // 交互次数
}

// NewRecommender 创建推荐系统
func NewRecommender() (*Recommender, error) {
	searchEngine, err := NewSearchEngine()
	if err != nil {
		return nil, err
	}

	similarityCalc := NewSimilarityCalculator()
	ranker := NewRanker()

	recommender := &Recommender{
		searchEngine:   searchEngine,
		similarityCalc: similarityCalc,
		ranker:         ranker,
		logger:         logger.NewLogger("recommender"),
	}

	recommender.logger.Info("Recommender system initialized")

	return recommender, nil
}

// GetRecommendations 获取推荐
func (r *Recommender) GetRecommendations(ctx context.Context, req *RecommendationRequest) (*RecommendationResponse, error) {
	if req == nil {
		return nil, errors.ErrValidationFailed("recommendation_request", "cannot be nil")
	}

	startTime := time.Now()

	r.logger.Info("Generating recommendations", logger.Fields{
		"type":                string(req.Type),
		"user_id":             req.UserID,
		"source_document_id":  req.SourceDocumentID,
		"max_recommendations": req.MaxRecommendations,
	})

	// 尝试从缓存获取推荐结果
	if cachedRecommendations, found := r.searchEngine.cacheManager.GetRecommendation(req); found {
		r.logger.Debug("Recommendation cache hit", logger.Fields{
			"type":            string(req.Type),
			"user_id":         req.UserID,
			"recommendations": len(cachedRecommendations),
		})

		return &RecommendationResponse{
			Recommendations:    cachedRecommendations,
			TotalFound:         len(cachedRecommendations),
			ProcessTime:        time.Since(startTime),
			RecommendationType: req.Type,
		}, nil
	}

	// 缓存未命中，生成新的推荐
	r.logger.Debug("Recommendation cache miss, generating new recommendations")

	// 设置默认值
	if req.MaxRecommendations <= 0 {
		req.MaxRecommendations = 10
	}

	// 根据推荐类型执行相应的推荐算法
	var recommendations []*RecommendationItem
	var err error

	switch req.Type {
	case RecommendationTypeSimilar:
		recommendations, err = r.getSimilarRecommendations(ctx, req)
	case RecommendationTypeRelated:
		recommendations, err = r.getRelatedRecommendations(ctx, req)
	case RecommendationTypePersonalized:
		recommendations, err = r.getPersonalizedRecommendations(ctx, req)
	case RecommendationTypeTrending:
		recommendations, err = r.getTrendingRecommendations(ctx, req)
	case RecommendationTypeCollaborative:
		recommendations, err = r.getCollaborativeRecommendations(ctx, req)
	case RecommendationTypeHybrid:
		recommendations, err = r.getHybridRecommendations(ctx, req)
	default:
		return nil, errors.ErrValidationFailed("recommendation_type", "unsupported type")
	}

	if err != nil {
		return nil, err
	}

	// 应用过滤和排除
	recommendations = r.applyFiltering(recommendations, req)

	// 应用多样性处理
	if req.DiversityEnabled {
		recommendations = r.applyDiversity(recommendations, req)
	}

	// 限制推荐数量
	if len(recommendations) > req.MaxRecommendations {
		recommendations = recommendations[:req.MaxRecommendations]
	}

	// 设置排名
	for i, rec := range recommendations {
		rec.Rank = i + 1
	}

	// 缓存推荐结果
	r.searchEngine.cacheManager.SetRecommendation(req, recommendations)

	processTime := time.Since(startTime)

	response := &RecommendationResponse{
		Recommendations:    recommendations,
		TotalFound:         len(recommendations),
		ProcessTime:        processTime,
		RecommendationType: req.Type,
		Metadata: map[string]interface{}{
			"processing_time_ms": processTime.Milliseconds(),
			"diversity_enabled":  req.DiversityEnabled,
			"personalized":       req.PersonalizationCtx != nil,
		},
	}

	r.logger.Info("Recommendations generated and cached", logger.Fields{
		"type":         string(req.Type),
		"count":        len(recommendations),
		"process_time": processTime,
	})

	return response, nil
}

// getSimilarRecommendations 获取相似内容推荐
func (r *Recommender) getSimilarRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	if req.SourceDocumentID == "" {
		return nil, errors.ErrValidationFailed("source_document_id", "required for similar recommendations")
	}

	r.logger.Debug("Getting similar recommendations", logger.Fields{
		"source_document_id": req.SourceDocumentID,
	})

	// 获取源文档
	sourceDoc, err := r.searchEngine.chromaClient.GetDocument(ctx, req.SourceDocumentID)
	if err != nil {
		return nil, fmt.Errorf("failed to get source document: %w", err)
	}

	// 使用源文档的向量进行相似度搜索
	searchQuery := &SearchQuery{
		QueryVector:   sourceDoc.Embedding,
		TopK:          req.MaxRecommendations * 2, // 获取更多结果用于过滤
		IncludeText:   true,
		MinSimilarity: req.MinSimilarity,
		Filter:        r.buildSearchFilter(req),
	}

	searchResult, err := r.searchEngine.chromaClient.Search(ctx, searchQuery)
	if err != nil {
		return nil, err
	}

	recommendations := make([]*RecommendationItem, 0)
	for _, doc := range searchResult.Documents {
		// 排除源文档本身
		if doc.ID == req.SourceDocumentID {
			continue
		}

		// 计算相似度
		similarity, err := r.similarityCalc.CalculateCosineSimilarity(sourceDoc.Embedding, doc.Embedding)
		if err != nil {
			r.logger.Warn("Failed to calculate similarity", logger.Fields{
				"document_id": doc.ID,
				"error":       err.Error(),
			})
			continue
		}

		// 创建推荐项
		recItem := &RecommendationItem{
			DocumentID:          doc.ID,
			Content:             doc.Content,
			Similarity:          similarity,
			Confidence:          similarity, // 对于相似推荐，置信度等于相似度
			Metadata:            doc.Metadata,
			RecommendationScore: similarity,
			RelatedKeywords:     r.extractRelatedKeywords(sourceDoc, doc),
			CreatedAt:           doc.CreatedAt,
		}

		// 添加推荐解释
		if req.IncludeExplanations {
			recItem.Explanation = &RecommendationExplanation{
				Reason:          "Content similarity based on semantic vectors",
				SimilarityScore: similarity,
				FactorBreakdown: map[string]float64{
					"vector_similarity": similarity,
				},
				MatchedFeatures: r.extractMatchedFeatures(sourceDoc, doc),
			}
		}

		recommendations = append(recommendations, recItem)
	}

	// 按相似度排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].Similarity > recommendations[j].Similarity
	})

	return recommendations, nil
}

// getRelatedRecommendations 获取相关内容推荐
func (r *Recommender) getRelatedRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	var queryVector []float32
	var sourceKeywords []string

	// 根据输入生成查询向量
	if req.SourceDocumentID != "" {
		// 基于源文档
		sourceDoc, err := r.searchEngine.chromaClient.GetDocument(ctx, req.SourceDocumentID)
		if err != nil {
			return nil, err
		}
		queryVector = sourceDoc.Embedding
		sourceKeywords = r.extractKeywordsFromMetadata(sourceDoc.Metadata)
	} else if req.SourceQuery != "" {
		// 基于查询文本
		vector, err := r.searchEngine.generateQueryVector(ctx, req.SourceQuery, &SearchOptions{})
		if err != nil {
			return nil, err
		}
		queryVector = vector
		sourceKeywords = strings.Fields(strings.ToLower(req.SourceQuery))
	} else {
		return nil, errors.ErrValidationFailed("source", "either source_document_id or source_query is required")
	}

	// 执行向量搜索
	searchQuery := &SearchQuery{
		QueryVector:   queryVector,
		TopK:          req.MaxRecommendations * 3, // 获取更多结果
		IncludeText:   true,
		MinSimilarity: req.MinSimilarity * 0.7,    // 降低阈值以获取更多相关内容
		Filter:        r.buildSearchFilter(req),
	}

	searchResult, err := r.searchEngine.chromaClient.Search(ctx, searchQuery)
	if err != nil {
		return nil, err
	}

	recommendations := make([]*RecommendationItem, 0)
	for _, doc := range searchResult.Documents {
		// 排除源文档
		if doc.ID == req.SourceDocumentID {
			continue
		}

		// 计算相关性分数（结合向量相似度和关键词匹配）
		vectorSimilarity, _ := r.similarityCalc.CalculateCosineSimilarity(queryVector, doc.Embedding)
		keywordSimilarity := r.calculateKeywordSimilarity(sourceKeywords, doc)
		
		// 综合相关性分数
		relatednessScore := vectorSimilarity*0.7 + keywordSimilarity*0.3

		recItem := &RecommendationItem{
			DocumentID:          doc.ID,
			Content:             doc.Content,
			Similarity:          vectorSimilarity,
			Confidence:          relatednessScore,
			Metadata:            doc.Metadata,
			RecommendationScore: relatednessScore,
			RelatedKeywords:     r.extractSharedKeywords(sourceKeywords, doc),
			CreatedAt:           doc.CreatedAt,
		}

		// 添加推荐解释
		if req.IncludeExplanations {
			recItem.Explanation = &RecommendationExplanation{
				Reason:          "Related content based on semantic similarity and keyword matching",
				SimilarityScore: vectorSimilarity,
				FactorBreakdown: map[string]float64{
					"vector_similarity":  vectorSimilarity,
					"keyword_similarity": keywordSimilarity,
					"relatedness_score":  relatednessScore,
				},
				MatchedFeatures: recItem.RelatedKeywords,
			}
		}

		recommendations = append(recommendations, recItem)
	}

	// 按相关性分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].RecommendationScore > recommendations[j].RecommendationScore
	})

	return recommendations, nil
}

// getPersonalizedRecommendations 获取个性化推荐
func (r *Recommender) getPersonalizedRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	if req.PersonalizationCtx == nil {
		return nil, errors.ErrValidationFailed("personalization_context", "required for personalized recommendations")
	}

	r.logger.Debug("Getting personalized recommendations", logger.Fields{
		"user_id": req.UserID,
	})

	// 基于用户偏好构建查询
	var queryVector []float32
	var err error

	if len(req.PersonalizationCtx.RecentInteractions) > 0 {
		// 基于最近交互的内容生成查询向量
		queryVector, err = r.generatePersonalizedQueryVector(ctx, req.PersonalizationCtx)
		if err != nil {
			return nil, err
		}
	} else if req.SourceQuery != "" {
		// 基于查询文本
		queryVector, err = r.searchEngine.generateQueryVector(ctx, req.SourceQuery, &SearchOptions{})
		if err != nil {
			return nil, err
		}
	} else {
		// 使用用户偏好生成通用查询
		queryText := r.buildPreferenceQuery(req.PersonalizationCtx)
		queryVector, err = r.searchEngine.generateQueryVector(ctx, queryText, &SearchOptions{})
		if err != nil {
			return nil, err
		}
	}

	// 执行个性化搜索
	// Note: We use searchEngine.Search which will generate its own query vector from the source query
	// The manually generated queryVector from above is kept for potential future use in advanced personalization
	_ = queryVector // Acknowledge that we're not using it in this simplified version
	
	searchOptions := &SearchOptions{
		TopK:                req.MaxRecommendations * 2,
		MinSimilarity:       req.MinSimilarity * 0.8,
		ContentTypes:        req.ContentTypes,
		UserID:              req.UserID,
		IncludeContent:      true,
		EnableReranking:     true,
		SimilarityType:      SimilarityTypeCosine,
		MaxResults:          req.MaxRecommendations * 3,
	}

	searchResponse, err := r.searchEngine.Search(ctx, searchOptions)
	if err != nil {
		return nil, err
	}

	recommendations := make([]*RecommendationItem, 0)
	for _, searchItem := range searchResponse.Results {
		// 计算个性化分数
		personalizedScore := r.calculatePersonalizedScore(searchItem, req.PersonalizationCtx)
		
		recItem := &RecommendationItem{
			DocumentID:          searchItem.DocumentID,
			Content:             searchItem.Content,
			Similarity:          searchItem.Similarity,
			Confidence:          personalizedScore,
			Metadata:            searchItem.Metadata,
			RecommendationScore: personalizedScore,
			RelatedKeywords:     searchItem.MatchedKeywords,
			CreatedAt:           searchItem.CreatedAt,
		}

		// 添加推荐解释
		if req.IncludeExplanations {
			recItem.Explanation = r.createPersonalizedExplanation(searchItem, req.PersonalizationCtx)
		}

		recommendations = append(recommendations, recItem)
	}

	// 按个性化分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].RecommendationScore > recommendations[j].RecommendationScore
	})

	return recommendations, nil
}

// getTrendingRecommendations 获取热门推荐
func (r *Recommender) getTrendingRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	// 构建时间范围过滤（默认最近30天）
	timeRange := req.TimeRange
	if timeRange == nil {
		now := time.Now()
		timeRange = &TimeRange{
			StartTime: now.AddDate(0, 0, -30), // 30天前
			EndTime:   now,
		}
	}

	// 搜索最近的内容
	searchQuery := &SearchQuery{
		TopK:        req.MaxRecommendations * 5, // 获取更多结果用于热门分析
		IncludeText: true,
		Filter: map[string]interface{}{
			"created_at": map[string]interface{}{
				"$gte": timeRange.StartTime.Unix(),
				"$lte": timeRange.EndTime.Unix(),
			},
		},
	}

	// 如果有用户ID，添加用户过滤
	if req.UserID != "" {
		searchQuery.Filter["user_id"] = req.UserID
	}

	searchResult, err := r.searchEngine.chromaClient.Search(ctx, searchQuery)
	if err != nil {
		return nil, err
	}

	// 分析热门度
	trendingAnalysis := r.analyzeTrending(searchResult.Documents, timeRange)

	recommendations := make([]*RecommendationItem, 0)
	for _, doc := range searchResult.Documents {
		trendingScore, exists := trendingAnalysis.DocumentScores[doc.ID]
		if !exists {
			continue
		}

		recItem := &RecommendationItem{
			DocumentID:          doc.ID,
			Content:             doc.Content,
			Similarity:          0.5, // 热门推荐不基于相似度
			Confidence:          trendingScore,
			Metadata:            doc.Metadata,
			RecommendationScore: trendingScore,
			RelatedKeywords:     r.extractTrendingKeywords(doc),
			CreatedAt:           doc.CreatedAt,
		}

		// 添加推荐解释
		if req.IncludeExplanations {
			recItem.Explanation = &RecommendationExplanation{
				Reason:          "Trending content based on recent activity and engagement",
				SimilarityScore: 0,
				FactorBreakdown: map[string]float64{
					"trending_score": trendingScore,
					"recency_bonus":  r.calculateRecencyBonus(doc.CreatedAt),
				},
				MatchedFeatures: []string{"trending", "recent"},
			}
		}

		recommendations = append(recommendations, recItem)
	}

	// 按热门分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].RecommendationScore > recommendations[j].RecommendationScore
	})

	return recommendations, nil
}

// getCollaborativeRecommendations 获取协同过滤推荐
func (r *Recommender) getCollaborativeRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	// 简化的协同过滤实现
	// 在实际应用中，这里会使用用户行为数据和机器学习算法

	r.logger.Debug("Getting collaborative filtering recommendations", logger.Fields{
		"user_id": req.UserID,
	})

	// 获取用户历史交互数据（这里使用模拟数据）
	userInteractions := r.getUserInteractions(req.UserID)
	if len(userInteractions) == 0 {
		// 如果没有用户交互数据，回退到个性化推荐
		return r.getPersonalizedRecommendations(ctx, req)
	}

	// 基于用户交互的内容找相似用户
	similarUsers := r.findSimilarUsers(req.UserID, userInteractions)

	// 收集相似用户喜欢的内容
	recommendedDocs := r.collectRecommendationsFromSimilarUsers(similarUsers)

	recommendations := make([]*RecommendationItem, 0)
	for docID, score := range recommendedDocs {
		// 获取文档详情
		doc, err := r.searchEngine.chromaClient.GetDocument(ctx, docID)
		if err != nil {
			continue
		}

		recItem := &RecommendationItem{
			DocumentID:          doc.ID,
			Content:             doc.Content,
			Similarity:          0.5, // 协同过滤不基于内容相似度
			Confidence:          score,
			Metadata:            doc.Metadata,
			RecommendationScore: score,
			RelatedKeywords:     r.extractKeywordsFromMetadata(doc.Metadata),
			CreatedAt:           doc.CreatedAt,
		}

		// 添加推荐解释
		if req.IncludeExplanations {
			recItem.Explanation = &RecommendationExplanation{
				Reason:          "Recommended based on similar users' preferences",
				SimilarityScore: 0,
				FactorBreakdown: map[string]float64{
					"collaborative_score": score,
				},
				MatchedFeatures: []string{"collaborative_filtering"},
			}
		}

		recommendations = append(recommendations, recItem)
	}

	// 按协同过滤分数排序
	sort.Slice(recommendations, func(i, j int) bool {
		return recommendations[i].RecommendationScore > recommendations[j].RecommendationScore
	})

	return recommendations, nil
}

// getHybridRecommendations 获取混合推荐
func (r *Recommender) getHybridRecommendations(ctx context.Context, req *RecommendationRequest) ([]*RecommendationItem, error) {
	r.logger.Debug("Getting hybrid recommendations")

	// 获取不同类型的推荐
	var allRecommendations []*RecommendationItem

	// 1. 相似内容推荐 (权重: 0.3)
	if req.SourceDocumentID != "" {
		similarReq := *req
		similarReq.Type = RecommendationTypeSimilar
		similarReq.MaxRecommendations = req.MaxRecommendations / 2
		similarRecs, err := r.getSimilarRecommendations(ctx, &similarReq)
		if err == nil {
			for _, rec := range similarRecs {
				rec.RecommendationScore *= 0.3
				rec.Explanation = r.addHybridExplanation(rec.Explanation, "similar", 0.3)
			}
			allRecommendations = append(allRecommendations, similarRecs...)
		}
	}

	// 2. 个性化推荐 (权重: 0.4)
	if req.PersonalizationCtx != nil {
		personalizedReq := *req
		personalizedReq.Type = RecommendationTypePersonalized
		personalizedReq.MaxRecommendations = req.MaxRecommendations / 2
		personalizedRecs, err := r.getPersonalizedRecommendations(ctx, &personalizedReq)
		if err == nil {
			for _, rec := range personalizedRecs {
				rec.RecommendationScore *= 0.4
				rec.Explanation = r.addHybridExplanation(rec.Explanation, "personalized", 0.4)
			}
			allRecommendations = append(allRecommendations, personalizedRecs...)
		}
	}

	// 3. 热门推荐 (权重: 0.2)
	trendingReq := *req
	trendingReq.Type = RecommendationTypeTrending
	trendingReq.MaxRecommendations = req.MaxRecommendations / 3
	trendingRecs, err := r.getTrendingRecommendations(ctx, &trendingReq)
	if err == nil {
		for _, rec := range trendingRecs {
			rec.RecommendationScore *= 0.2
			rec.Explanation = r.addHybridExplanation(rec.Explanation, "trending", 0.2)
		}
		allRecommendations = append(allRecommendations, trendingRecs...)
	}

	// 4. 协同过滤推荐 (权重: 0.1)
	collaborativeReq := *req
	collaborativeReq.Type = RecommendationTypeCollaborative
	collaborativeReq.MaxRecommendations = req.MaxRecommendations / 4
	collaborativeRecs, err := r.getCollaborativeRecommendations(ctx, &collaborativeReq)
	if err == nil {
		for _, rec := range collaborativeRecs {
			rec.RecommendationScore *= 0.1
			rec.Explanation = r.addHybridExplanation(rec.Explanation, "collaborative", 0.1)
		}
		allRecommendations = append(allRecommendations, collaborativeRecs...)
	}

	// 合并和去重
	uniqueRecommendations := r.mergeAndDeduplicateRecommendations(allRecommendations)

	// 按混合分数排序
	sort.Slice(uniqueRecommendations, func(i, j int) bool {
		return uniqueRecommendations[i].RecommendationScore > uniqueRecommendations[j].RecommendationScore
	})

	return uniqueRecommendations, nil
}

// 辅助函数实现

func (r *Recommender) buildSearchFilter(req *RecommendationRequest) map[string]interface{} {
	filter := make(map[string]interface{})

	if req.UserID != "" {
		filter["user_id"] = req.UserID
	}

	if len(req.ContentTypes) > 0 {
		contentTypeStrs := make([]string, len(req.ContentTypes))
		for i, ct := range req.ContentTypes {
			contentTypeStrs[i] = string(ct)
		}
		filter["content_type"] = map[string]interface{}{
			"$in": contentTypeStrs,
		}
	}

	if req.TimeRange != nil {
		timeFilter := make(map[string]interface{})
		if !req.TimeRange.StartTime.IsZero() {
			timeFilter["$gte"] = req.TimeRange.StartTime.Unix()
		}
		if !req.TimeRange.EndTime.IsZero() {
			timeFilter["$lte"] = req.TimeRange.EndTime.Unix()
		}
		if len(timeFilter) > 0 {
			filter["created_at"] = timeFilter
		}
	}

	return filter
}

func (r *Recommender) extractRelatedKeywords(sourceDoc, targetDoc *VectorDocument) []string {
	sourceKeywords := r.extractKeywordsFromMetadata(sourceDoc.Metadata)
	targetKeywords := r.extractKeywordsFromMetadata(targetDoc.Metadata)

	// 找出共同关键词
	shared := make([]string, 0)
	for _, sk := range sourceKeywords {
		for _, tk := range targetKeywords {
			if strings.EqualFold(sk, tk) {
				shared = append(shared, sk)
			}
		}
	}

	return shared
}

func (r *Recommender) extractKeywordsFromMetadata(metadata map[string]interface{}) []string {
	if keywords, exists := metadata["keywords"]; exists {
		if keywordList, ok := keywords.([]interface{}); ok {
			result := make([]string, 0, len(keywordList))
			for _, kw := range keywordList {
				if kwStr, ok := kw.(string); ok {
					result = append(result, kwStr)
				}
			}
			return result
		}
	}
	return []string{}
}

func (r *Recommender) extractMatchedFeatures(sourceDoc, targetDoc *VectorDocument) []string {
	features := make([]string, 0)

	// 检查内容类型匹配
	if sourceType, exists := sourceDoc.Metadata["content_type"]; exists {
		if targetType, exists := targetDoc.Metadata["content_type"]; exists {
			if sourceType == targetType {
				features = append(features, fmt.Sprintf("content_type:%s", sourceType))
			}
		}
	}

	// 检查标签匹配
	sharedKeywords := r.extractRelatedKeywords(sourceDoc, targetDoc)
	features = append(features, sharedKeywords...)

	return features
}

func (r *Recommender) calculateKeywordSimilarity(sourceKeywords []string, doc *VectorDocument) float64 {
	if len(sourceKeywords) == 0 {
		return 0.0
	}

	docKeywords := r.extractKeywordsFromMetadata(doc.Metadata)
	if len(docKeywords) == 0 {
		return 0.0
	}

	// 计算关键词重叠度
	matches := 0
	for _, sk := range sourceKeywords {
		for _, dk := range docKeywords {
			if strings.EqualFold(sk, dk) {
				matches++
				break
			}
		}
	}

	return float64(matches) / float64(len(sourceKeywords))
}

func (r *Recommender) extractSharedKeywords(sourceKeywords []string, doc *VectorDocument) []string {
	docKeywords := r.extractKeywordsFromMetadata(doc.Metadata)
	shared := make([]string, 0)

	for _, sk := range sourceKeywords {
		for _, dk := range docKeywords {
			if strings.EqualFold(sk, dk) {
				shared = append(shared, sk)
				break
			}
		}
	}

	return shared
}

func (r *Recommender) generatePersonalizedQueryVector(ctx context.Context, personalCtx *PersonalizationContext) ([]float32, error) {
	// 基于用户最近交互的内容生成查询向量
	if len(personalCtx.RecentInteractions) == 0 {
		return nil, errors.ErrValidationFailed("recent_interactions", "no recent interactions found")
	}

	// 获取最近交互的文档
	var avgVector []float32
	validDocs := 0

	for _, docID := range personalCtx.RecentInteractions {
		doc, err := r.searchEngine.chromaClient.GetDocument(ctx, docID)
		if err != nil {
			continue
		}

		if len(doc.Embedding) == 0 {
			continue
		}

		if avgVector == nil {
			avgVector = make([]float32, len(doc.Embedding))
		}

		// 累加向量
		for i, val := range doc.Embedding {
			if i < len(avgVector) {
				avgVector[i] += val
			}
		}
		validDocs++
	}

	if validDocs == 0 {
		return nil, errors.ErrValidationFailed("valid_interactions", "no valid interaction documents found")
	}

	// 计算平均向量
	for i := range avgVector {
		avgVector[i] /= float32(validDocs)
	}

	return avgVector, nil
}

func (r *Recommender) buildPreferenceQuery(personalCtx *PersonalizationContext) string {
	// 基于用户偏好构建查询文本
	queryParts := make([]string, 0)

	// 添加偏好的标签
	for _, tag := range personalCtx.PreferredTags {
		queryParts = append(queryParts, tag)
	}

	// 添加偏好的内容类型
	for _, contentType := range personalCtx.PreferredContentTypes {
		queryParts = append(queryParts, string(contentType))
	}

	if len(queryParts) == 0 {
		return "general content" // 默认查询
	}

	return strings.Join(queryParts, " ")
}

func (r *Recommender) calculatePersonalizedScore(searchItem *SearchResultItem, personalCtx *PersonalizationContext) float64 {
	baseScore := searchItem.RelevanceScore

	// 个性化增强
	personalBonus := 0.0

	// 内容类型偏好
	if contentType, exists := searchItem.Metadata["content_type"]; exists {
		if contentTypeStr, ok := contentType.(string); ok {
			for _, preferredType := range personalCtx.PreferredContentTypes {
				if string(preferredType) == contentTypeStr {
					personalBonus += 0.2
					break
				}
			}
		}
	}

	// 标签偏好
	if tags, exists := searchItem.Metadata["tags"]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					for _, preferredTag := range personalCtx.PreferredTags {
						if strings.EqualFold(tagStr, preferredTag) {
							personalBonus += 0.1
							break
						}
					}
				}
			}
		}
	}

	// 用户交互历史
	if weight, exists := personalCtx.InteractionHistory[searchItem.DocumentID]; exists {
		personalBonus += weight * 0.3
	}

	return math.Min(baseScore+personalBonus, 1.0)
}

func (r *Recommender) createPersonalizedExplanation(searchItem *SearchResultItem, personalCtx *PersonalizationContext) *RecommendationExplanation {
	explanation := &RecommendationExplanation{
		Reason:          "Personalized recommendation based on your preferences and history",
		SimilarityScore: searchItem.Similarity,
		FactorBreakdown: make(map[string]float64),
		MatchedFeatures: make([]string, 0),
		UserPreferences: make([]string, 0),
	}

	// 分析匹配的特征
	if contentType, exists := searchItem.Metadata["content_type"]; exists {
		if contentTypeStr, ok := contentType.(string); ok {
			for _, preferredType := range personalCtx.PreferredContentTypes {
				if string(preferredType) == contentTypeStr {
					explanation.MatchedFeatures = append(explanation.MatchedFeatures, "preferred_content_type")
					explanation.UserPreferences = append(explanation.UserPreferences, contentTypeStr)
					break
				}
			}
		}
	}

	if tags, exists := searchItem.Metadata["tags"]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					for _, preferredTag := range personalCtx.PreferredTags {
						if strings.EqualFold(tagStr, preferredTag) {
							explanation.MatchedFeatures = append(explanation.MatchedFeatures, "preferred_tag")
							explanation.UserPreferences = append(explanation.UserPreferences, tagStr)
						}
					}
				}
			}
		}
	}

	return explanation
}

func (r *Recommender) analyzeTrending(documents []*VectorDocument, timeRange *TimeRange) *TrendingAnalysis {
	documentScores := make(map[string]float64)
	interactionCount := make(map[string]int)

	timeWindow := timeRange.EndTime.Sub(timeRange.StartTime)

	for _, doc := range documents {
		// 简单的热门分数计算
		// 在实际应用中，这里会考虑更多因素，如点击率、分享数、评论数等
		
		// 基于创建时间的新鲜度分数
		age := timeRange.EndTime.Sub(doc.CreatedAt)
		freshnessScore := 1.0 - (age.Hours() / timeWindow.Hours())
		if freshnessScore < 0 {
			freshnessScore = 0
		}

		// 基于重要性的分数
		importance := 0.5 // 默认重要性
		if importanceVal, exists := doc.Metadata["importance_score"]; exists {
			if score, ok := importanceVal.(float64); ok {
				importance = score / 10.0
			}
		}

		// 计算综合热门分数
		trendingScore := freshnessScore*0.6 + importance*0.4

		documentScores[doc.ID] = trendingScore
		interactionCount[doc.ID] = 1 // 简化的交互计数
	}

	return &TrendingAnalysis{
		DocumentScores:   documentScores,
		TimeWindow:       timeWindow,
		InteractionCount: interactionCount,
	}
}

func (r *Recommender) extractTrendingKeywords(doc *VectorDocument) []string {
	keywords := r.extractKeywordsFromMetadata(doc.Metadata)
	
	// 为热门内容添加特殊标记
	keywords = append(keywords, "trending", "popular")
	
	return keywords
}

func (r *Recommender) calculateRecencyBonus(createdAt time.Time) float64 {
	hoursAgo := time.Since(createdAt).Hours()
	
	// 24小时内的内容获得最高加分
	if hoursAgo <= 24 {
		return 1.0
	}
	
	// 一周内的内容获得递减加分
	if hoursAgo <= 168 { // 7 * 24
		return 1.0 - ((hoursAgo - 24) / 144) // 144 = 168 - 24
	}
	
	return 0.0
}

// 简化的协同过滤辅助函数（实际应用中需要更复杂的实现）
func (r *Recommender) getUserInteractions(userID string) []string {
	// 这里应该从数据库或缓存中获取用户交互数据
	// 目前返回模拟数据
	return []string{"doc1", "doc2", "doc3"}
}

func (r *Recommender) findSimilarUsers(userID string, userInteractions []string) []string {
	// 简化实现：返回模拟的相似用户
	return []string{"user2", "user3"}
}

func (r *Recommender) collectRecommendationsFromSimilarUsers(similarUsers []string) map[string]float64 {
	// 简化实现：返回模拟的推荐文档及其分数
	return map[string]float64{
		"doc4": 0.8,
		"doc5": 0.7,
		"doc6": 0.6,
	}
}

func (r *Recommender) addHybridExplanation(explanation *RecommendationExplanation, strategy string, weight float64) *RecommendationExplanation {
	if explanation == nil {
		explanation = &RecommendationExplanation{
			FactorBreakdown: make(map[string]float64),
		}
	}
	
	explanation.Reason = fmt.Sprintf("Hybrid recommendation (strategy: %s, weight: %.1f)", strategy, weight)
	explanation.FactorBreakdown[strategy+"_weight"] = weight
	
	return explanation
}

func (r *Recommender) mergeAndDeduplicateRecommendations(recommendations []*RecommendationItem) []*RecommendationItem {
	uniqueMap := make(map[string]*RecommendationItem)
	
	for _, rec := range recommendations {
		if existing, exists := uniqueMap[rec.DocumentID]; exists {
			// 合并分数（取最高分）
			if rec.RecommendationScore > existing.RecommendationScore {
				uniqueMap[rec.DocumentID] = rec
			}
		} else {
			uniqueMap[rec.DocumentID] = rec
		}
	}
	
	result := make([]*RecommendationItem, 0, len(uniqueMap))
	for _, rec := range uniqueMap {
		result = append(result, rec)
	}
	
	return result
}

func (r *Recommender) applyFiltering(recommendations []*RecommendationItem, req *RecommendationRequest) []*RecommendationItem {
	if len(req.ExcludeDocuments) == 0 {
		return recommendations
	}
	
	excludeMap := make(map[string]bool)
	for _, docID := range req.ExcludeDocuments {
		excludeMap[docID] = true
	}
	
	filtered := make([]*RecommendationItem, 0)
	for _, rec := range recommendations {
		if !excludeMap[rec.DocumentID] {
			filtered = append(filtered, rec)
		}
	}
	
	return filtered
}

func (r *Recommender) applyDiversity(recommendations []*RecommendationItem, req *RecommendationRequest) []*RecommendationItem {
	// 简化的多样性实现
	if len(recommendations) <= 3 {
		return recommendations
	}
	
	diverse := make([]*RecommendationItem, 0)
	contentTypeCounts := make(map[string]int)
	
	for _, rec := range recommendations {
		contentType := "unknown"
		if ct, exists := rec.Metadata["content_type"]; exists {
			if ctStr, ok := ct.(string); ok {
				contentType = ctStr
			}
		}
		
		// 限制每种内容类型的数量
		if contentTypeCounts[contentType] < 2 {
			diverse = append(diverse, rec)
			contentTypeCounts[contentType]++
		}
		
		if len(diverse) >= req.MaxRecommendations {
			break
		}
	}
	
	return diverse
}

// HealthCheck 健康检查
func (r *Recommender) HealthCheck(ctx context.Context) error {
	return r.searchEngine.HealthCheck(ctx)
}

// Close 关闭推荐系统
func (r *Recommender) Close() error {
	r.logger.Info("Closing recommender system")
	return r.searchEngine.Close()
}