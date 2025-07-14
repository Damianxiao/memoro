package vector

import (
	"math"
	"sort"
	"strings"
	"time"

	"memoro/internal/logger"
	"memoro/internal/models"
)

// Ranker 结果排序器
type Ranker struct {
	logger *logger.Logger
}

// RankingStrategy 排序策略
type RankingStrategy string

const (
	RankingStrategySimilarity   RankingStrategy = "similarity"    // 按相似度排序
	RankingStrategyRelevance    RankingStrategy = "relevance"     // 按综合相关性排序
	RankingStrategyTime         RankingStrategy = "time"          // 按时间排序
	RankingStrategyImportance   RankingStrategy = "importance"    // 按重要性排序
	RankingStrategyHybrid       RankingStrategy = "hybrid"        // 混合排序
	RankingStrategyPersonalized RankingStrategy = "personalized" // 个性化排序
)

// RankingOptions 排序选项
type RankingOptions struct {
	Strategy           RankingStrategy         `json:"strategy"`                     // 排序策略
	Weights            *RankingWeights         `json:"weights,omitempty"`           // 权重配置
	PersonalizationCtx *PersonalizationContext `json:"personalization,omitempty"`   // 个性化上下文
	BoostFactors       *BoostFactors           `json:"boost_factors,omitempty"`     // 增强因子
	TimeDecay          *TimeDecayConfig        `json:"time_decay,omitempty"`        // 时间衰减配置
	DiversitySettings  *DiversitySettings      `json:"diversity,omitempty"`         // 多样性设置
}

// RankingWeights 排序权重
type RankingWeights struct {
	Similarity      float64 `json:"similarity"`       // 相似度权重
	KeywordMatch    float64 `json:"keyword_match"`    // 关键词匹配权重
	Importance      float64 `json:"importance"`       // 重要性权重
	Freshness       float64 `json:"freshness"`        // 新鲜度权重
	UserPreference  float64 `json:"user_preference"`  // 用户偏好权重
	ContentType     float64 `json:"content_type"`     // 内容类型权重
	TagRelevance    float64 `json:"tag_relevance"`    // 标签相关性权重
}

// PersonalizationContext 个性化上下文
type PersonalizationContext struct {
	UserID              string                 `json:"user_id"`                        // 用户ID
	UserPreferences     map[string]float64     `json:"user_preferences"`               // 用户偏好
	RecentInteractions  []string               `json:"recent_interactions"`            // 最近交互的内容ID
	PreferredContentTypes []models.ContentType `json:"preferred_content_types"`        // 偏好的内容类型
	PreferredTags       []string               `json:"preferred_tags"`                 // 偏好的标签
	InteractionHistory  map[string]float64     `json:"interaction_history"`            // 交互历史权重
}

// BoostFactors 增强因子
type BoostFactors struct {
	HighImportanceBoost float64            `json:"high_importance_boost"`  // 高重要性增强
	RecentContentBoost  float64            `json:"recent_content_boost"`   // 最新内容增强
	ContentTypeBoosts   map[string]float64 `json:"content_type_boosts"`    // 内容类型增强
	TagBoosts           map[string]float64 `json:"tag_boosts"`             // 标签增强
	UserInteractionBoost float64           `json:"user_interaction_boost"` // 用户交互增强
}

// TimeDecayConfig 时间衰减配置
type TimeDecayConfig struct {
	Enabled         bool    `json:"enabled"`          // 是否启用时间衰减
	DecayRate       float64 `json:"decay_rate"`       // 衰减率 (0-1)
	HalfLifeDays    float64 `json:"half_life_days"`   // 半衰期（天）
	MinScore        float64 `json:"min_score"`        // 最小分数
	RecentBoostDays float64 `json:"recent_boost_days"` // 最近内容增强天数
}

// DiversitySettings 多样性设置
type DiversitySettings struct {
	Enabled              bool    `json:"enabled"`                // 是否启用多样性
	ContentTypeDiversity bool    `json:"content_type_diversity"` // 内容类型多样性
	TagDiversity         bool    `json:"tag_diversity"`          // 标签多样性
	TimeDiversity        bool    `json:"time_diversity"`         // 时间多样性
	MaxSimilarResults    int     `json:"max_similar_results"`    // 最大相似结果数
	DiversityThreshold   float64 `json:"diversity_threshold"`    // 多样性阈值
}

// RankingResult 排序结果
type RankingResult struct {
	OriginalResults  []*SearchResultItem `json:"original_results"`   // 原始结果
	RankedResults    []*SearchResultItem `json:"ranked_results"`     // 排序后结果
	RankingStrategy  RankingStrategy     `json:"ranking_strategy"`   // 使用的排序策略
	ScoreBreakdown   []ScoreBreakdown    `json:"score_breakdown"`    // 分数分解
	DiversityMetrics *DiversityMetrics   `json:"diversity_metrics"`  // 多样性指标
}

// ScoreBreakdown 分数分解
type ScoreBreakdown struct {
	DocumentID       string  `json:"document_id"`       // 文档ID
	FinalScore       float64 `json:"final_score"`       // 最终分数
	SimilarityScore  float64 `json:"similarity_score"`  // 相似度分数
	KeywordScore     float64 `json:"keyword_score"`     // 关键词分数
	ImportanceScore  float64 `json:"importance_score"`  // 重要性分数
	FreshnessScore   float64 `json:"freshness_score"`   // 新鲜度分数
	PersonalizedScore float64 `json:"personalized_score"` // 个性化分数
	BoostScore       float64 `json:"boost_score"`       // 增强分数
}

// DiversityMetrics 多样性指标
type DiversityMetrics struct {
	ContentTypeDistribution map[string]int `json:"content_type_distribution"` // 内容类型分布
	TagDistribution         map[string]int `json:"tag_distribution"`          // 标签分布
	TimeDistribution        map[string]int `json:"time_distribution"`         // 时间分布
	DiversityScore          float64        `json:"diversity_score"`           // 多样性分数
}

// NewRanker 创建排序器
func NewRanker() *Ranker {
	return &Ranker{
		logger: logger.NewLogger("ranker"),
	}
}

// Rank 执行结果排序
func (r *Ranker) Rank(results []*SearchResultItem, options *RankingOptions) (*RankingResult, error) {
	if len(results) == 0 {
		return &RankingResult{
			OriginalResults: results,
			RankedResults:   results,
			RankingStrategy: options.Strategy,
		}, nil
	}

	r.logger.Debug("Starting result ranking", logger.Fields{
		"result_count": len(results),
		"strategy":     string(options.Strategy),
	})

	// 复制原始结果
	originalResults := make([]*SearchResultItem, len(results))
	copy(originalResults, results)

	// 根据策略执行排序
	rankedResults, scoreBreakdown, err := r.executeRankingStrategy(results, options)
	if err != nil {
		return nil, err
	}

	// 应用多样性处理
	if options.DiversitySettings != nil && options.DiversitySettings.Enabled {
		rankedResults = r.applyDiversityFiltering(rankedResults, options.DiversitySettings)
	}

	// 计算多样性指标
	diversityMetrics := r.calculateDiversityMetrics(rankedResults)

	// 更新排名
	for i, result := range rankedResults {
		result.Rank = i + 1
	}

	rankingResult := &RankingResult{
		OriginalResults:  originalResults,
		RankedResults:    rankedResults,
		RankingStrategy:  options.Strategy,
		ScoreBreakdown:   scoreBreakdown,
		DiversityMetrics: diversityMetrics,
	}

	r.logger.Debug("Ranking completed", logger.Fields{
		"original_count": len(originalResults),
		"ranked_count":   len(rankedResults),
		"strategy":       string(options.Strategy),
	})

	return rankingResult, nil
}

// executeRankingStrategy 执行排序策略
func (r *Ranker) executeRankingStrategy(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	switch options.Strategy {
	case RankingStrategySimilarity:
		return r.rankBySimilarity(results, options)
	case RankingStrategyRelevance:
		return r.rankByRelevance(results, options)
	case RankingStrategyTime:
		return r.rankByTime(results, options)
	case RankingStrategyImportance:
		return r.rankByImportance(results, options)
	case RankingStrategyHybrid:
		return r.rankByHybrid(results, options)
	case RankingStrategyPersonalized:
		return r.rankByPersonalized(results, options)
	default:
		// 默认使用相似度排序
		return r.rankBySimilarity(results, options)
	}
}

// rankBySimilarity 按相似度排序
func (r *Ranker) rankBySimilarity(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 计算分数分解
	for i, result := range results {
		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:      result.DocumentID,
			FinalScore:      result.Similarity,
			SimilarityScore: result.Similarity,
		}
	}

	// 按相似度排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Similarity > results[j].Similarity
	})

	return results, scoreBreakdown, nil
}

// rankByRelevance 按综合相关性排序
func (r *Ranker) rankByRelevance(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 计算分数分解
	for i, result := range results {
		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:        result.DocumentID,
			FinalScore:        result.RelevanceScore,
			SimilarityScore:   result.Similarity,
			KeywordScore:      float64(len(result.MatchedKeywords)),
			ImportanceScore:   r.extractImportanceScore(result.Metadata),
			FreshnessScore:    r.calculateFreshnessScore(result.CreatedAt, options.TimeDecay),
		}
	}

	// 按综合相关性排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results, scoreBreakdown, nil
}

// rankByTime 按时间排序
func (r *Ranker) rankByTime(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 计算分数分解
	for i, result := range results {
		timeScore := float64(result.CreatedAt.Unix()) / float64(time.Now().Unix())
		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:      result.DocumentID,
			FinalScore:      timeScore,
			FreshnessScore:  timeScore,
		}
	}

	// 按时间排序（最新的在前）
	sort.Slice(results, func(i, j int) bool {
		return results[i].CreatedAt.After(results[j].CreatedAt)
	})

	return results, scoreBreakdown, nil
}

// rankByImportance 按重要性排序
func (r *Ranker) rankByImportance(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 计算分数分解
	for i, result := range results {
		importance := r.extractImportanceScore(result.Metadata)
		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:      result.DocumentID,
			FinalScore:      importance,
			ImportanceScore: importance,
		}
	}

	// 按重要性排序
	sort.Slice(results, func(i, j int) bool {
		importanceI := r.extractImportanceScore(results[i].Metadata)
		importanceJ := r.extractImportanceScore(results[j].Metadata)
		return importanceI > importanceJ
	})

	return results, scoreBreakdown, nil
}

// rankByHybrid 混合排序
func (r *Ranker) rankByHybrid(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	weights := options.Weights
	if weights == nil {
		// 默认权重
		weights = &RankingWeights{
			Similarity:     0.4,
			KeywordMatch:   0.2,
			Importance:     0.2,
			Freshness:      0.1,
			UserPreference: 0.05,
			ContentType:    0.03,
			TagRelevance:   0.02,
		}
	}

	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 为每个结果计算混合分数
	for i, result := range results {
		similarityScore := result.Similarity
		keywordScore := r.calculateKeywordScore(result.MatchedKeywords)
		importanceScore := r.extractImportanceScore(result.Metadata)
		freshnessScore := r.calculateFreshnessScore(result.CreatedAt, options.TimeDecay)
		personalizedScore := r.calculatePersonalizedScore(result, options.PersonalizationCtx)
		contentTypeScore := r.calculateContentTypeScore(result.Metadata, options.PersonalizationCtx)
		tagScore := r.calculateTagScore(result.Metadata, options.PersonalizationCtx)

		// 应用增强因子
		boostScore := r.applyBoostFactors(result, options.BoostFactors)

		// 计算最终分数
		finalScore := similarityScore*weights.Similarity +
			keywordScore*weights.KeywordMatch +
			importanceScore*weights.Importance +
			freshnessScore*weights.Freshness +
			personalizedScore*weights.UserPreference +
			contentTypeScore*weights.ContentType +
			tagScore*weights.TagRelevance +
			boostScore

		result.RelevanceScore = finalScore

		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:        result.DocumentID,
			FinalScore:        finalScore,
			SimilarityScore:   similarityScore,
			KeywordScore:      keywordScore,
			ImportanceScore:   importanceScore,
			FreshnessScore:    freshnessScore,
			PersonalizedScore: personalizedScore,
			BoostScore:        boostScore,
		}
	}

	// 按最终分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results, scoreBreakdown, nil
}

// rankByPersonalized 个性化排序
func (r *Ranker) rankByPersonalized(results []*SearchResultItem, options *RankingOptions) ([]*SearchResultItem, []ScoreBreakdown, error) {
	if options.PersonalizationCtx == nil {
		// 如果没有个性化上下文，退回到混合排序
		return r.rankByHybrid(results, options)
	}

	scoreBreakdown := make([]ScoreBreakdown, len(results))

	// 为每个结果计算个性化分数
	for i, result := range results {
		baseScore := result.RelevanceScore
		personalizedScore := r.calculatePersonalizedScore(result, options.PersonalizationCtx)
		
		// 个性化权重更高
		finalScore := baseScore*0.6 + personalizedScore*0.4

		result.RelevanceScore = finalScore

		scoreBreakdown[i] = ScoreBreakdown{
			DocumentID:        result.DocumentID,
			FinalScore:        finalScore,
			SimilarityScore:   result.Similarity,
			PersonalizedScore: personalizedScore,
		}
	}

	// 按个性化分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].RelevanceScore > results[j].RelevanceScore
	})

	return results, scoreBreakdown, nil
}

// calculateKeywordScore 计算关键词分数
func (r *Ranker) calculateKeywordScore(matchedKeywords []string) float64 {
	if len(matchedKeywords) == 0 {
		return 0.0
	}
	
	// 简单的关键词分数计算
	return math.Min(float64(len(matchedKeywords))/5.0, 1.0)
}

// extractImportanceScore 提取重要性分数
func (r *Ranker) extractImportanceScore(metadata map[string]interface{}) float64 {
	if importance, exists := metadata["importance_score"]; exists {
		if score, ok := importance.(float64); ok {
			return score / 10.0 // 假设重要性分数0-10，规范化到0-1
		}
	}
	return 0.5 // 默认中等重要性
}

// calculateFreshnessScore 计算新鲜度分数
func (r *Ranker) calculateFreshnessScore(createdAt time.Time, timeDecay *TimeDecayConfig) float64 {
	if timeDecay == nil || !timeDecay.Enabled {
		return 1.0
	}

	daysSinceCreation := time.Since(createdAt).Hours() / 24.0

	// 时间衰减计算
	if timeDecay.HalfLifeDays > 0 {
		decayFactor := math.Pow(0.5, daysSinceCreation/timeDecay.HalfLifeDays)
		score := decayFactor
		
		// 应用最小分数限制
		if score < timeDecay.MinScore {
			score = timeDecay.MinScore
		}
		
		// 最近内容增强
		if timeDecay.RecentBoostDays > 0 && daysSinceCreation <= timeDecay.RecentBoostDays {
			boost := 1.0 + (timeDecay.RecentBoostDays-daysSinceCreation)/timeDecay.RecentBoostDays*0.5
			score *= boost
		}
		
		return math.Min(score, 1.0)
	}

	// 简单的线性衰减
	score := 1.0 - (daysSinceCreation/365.0)*timeDecay.DecayRate
	return math.Max(score, timeDecay.MinScore)
}

// calculatePersonalizedScore 计算个性化分数
func (r *Ranker) calculatePersonalizedScore(result *SearchResultItem, ctx *PersonalizationContext) float64 {
	if ctx == nil {
		return 0.0
	}

	score := 0.0

	// 内容类型偏好
	if contentType, exists := result.Metadata["content_type"]; exists {
		if contentTypeStr, ok := contentType.(string); ok {
			for _, preferredType := range ctx.PreferredContentTypes {
				if string(preferredType) == contentTypeStr {
					score += 0.3
					break
				}
			}
		}
	}

	// 标签偏好
	if tags, exists := result.Metadata["tags"]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					for _, preferredTag := range ctx.PreferredTags {
						if tagStr == preferredTag {
							score += 0.2
							break
						}
					}
				}
			}
		}
	}

	// 历史交互
	if weight, exists := ctx.InteractionHistory[result.DocumentID]; exists {
		score += weight * 0.5
	}

	// 用户偏好
	if userPref, exists := ctx.UserPreferences[result.DocumentID]; exists {
		score += userPref * 0.3
	}

	return math.Min(score, 1.0)
}

// calculateContentTypeScore 计算内容类型分数
func (r *Ranker) calculateContentTypeScore(metadata map[string]interface{}, ctx *PersonalizationContext) float64 {
	if ctx == nil {
		return 0.0
	}

	if contentType, exists := metadata["content_type"]; exists {
		if contentTypeStr, ok := contentType.(string); ok {
			for _, preferredType := range ctx.PreferredContentTypes {
				if string(preferredType) == contentTypeStr {
					return 1.0
				}
			}
		}
	}

	return 0.0
}

// calculateTagScore 计算标签分数
func (r *Ranker) calculateTagScore(metadata map[string]interface{}, ctx *PersonalizationContext) float64 {
	if ctx == nil {
		return 0.0
	}

	score := 0.0
	if tags, exists := metadata["tags"]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			matchCount := 0
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					for _, preferredTag := range ctx.PreferredTags {
						if strings.EqualFold(tagStr, preferredTag) {
							matchCount++
							break
						}
					}
				}
			}
			if len(ctx.PreferredTags) > 0 {
				score = float64(matchCount) / float64(len(ctx.PreferredTags))
			}
		}
	}

	return math.Min(score, 1.0)
}

// applyBoostFactors 应用增强因子
func (r *Ranker) applyBoostFactors(result *SearchResultItem, boostFactors *BoostFactors) float64 {
	if boostFactors == nil {
		return 0.0
	}

	boost := 0.0

	// 高重要性增强
	importance := r.extractImportanceScore(result.Metadata)
	if importance > 0.8 && boostFactors.HighImportanceBoost > 0 {
		boost += boostFactors.HighImportanceBoost
	}

	// 最新内容增强
	daysSinceCreation := time.Since(result.CreatedAt).Hours() / 24.0
	if daysSinceCreation <= 7 && boostFactors.RecentContentBoost > 0 {
		boost += boostFactors.RecentContentBoost * (7.0 - daysSinceCreation) / 7.0
	}

	// 内容类型增强
	if contentType, exists := result.Metadata["content_type"]; exists {
		if contentTypeStr, ok := contentType.(string); ok {
			if typeBoost, exists := boostFactors.ContentTypeBoosts[contentTypeStr]; exists {
				boost += typeBoost
			}
		}
	}

	// 标签增强
	if tags, exists := result.Metadata["tags"]; exists {
		if tagList, ok := tags.([]interface{}); ok {
			for _, tag := range tagList {
				if tagStr, ok := tag.(string); ok {
					if tagBoost, exists := boostFactors.TagBoosts[tagStr]; exists {
						boost += tagBoost
					}
				}
			}
		}
	}

	return boost
}

// applyDiversityFiltering 应用多样性过滤
func (r *Ranker) applyDiversityFiltering(results []*SearchResultItem, settings *DiversitySettings) []*SearchResultItem {
	if !settings.Enabled || len(results) <= 1 {
		return results
	}

	diverseResults := make([]*SearchResultItem, 0, len(results))
	contentTypeCounts := make(map[string]int)
	tagCounts := make(map[string]int)
	timeBuckets := make(map[string]int)

	for _, result := range results {
		shouldInclude := true

		// 内容类型多样性检查
		if settings.ContentTypeDiversity {
			if contentType, exists := result.Metadata["content_type"]; exists {
				if contentTypeStr, ok := contentType.(string); ok {
					if contentTypeCounts[contentTypeStr] >= settings.MaxSimilarResults {
						shouldInclude = false
					}
				}
			}
		}

		// 标签多样性检查
		if shouldInclude && settings.TagDiversity {
			if tags, exists := result.Metadata["tags"]; exists {
				if tagList, ok := tags.([]interface{}); ok {
					for _, tag := range tagList {
						if tagStr, ok := tag.(string); ok {
							if tagCounts[tagStr] >= settings.MaxSimilarResults {
								shouldInclude = false
								break
							}
						}
					}
				}
			}
		}

		// 时间多样性检查
		if shouldInclude && settings.TimeDiversity {
			timeBucket := result.CreatedAt.Format("2006-01-02") // 按天分组
			if timeBuckets[timeBucket] >= settings.MaxSimilarResults {
				shouldInclude = false
			}
		}

		if shouldInclude {
			diverseResults = append(diverseResults, result)

			// 更新计数器
			if contentType, exists := result.Metadata["content_type"]; exists {
				if contentTypeStr, ok := contentType.(string); ok {
					contentTypeCounts[contentTypeStr]++
				}
			}

			if tags, exists := result.Metadata["tags"]; exists {
				if tagList, ok := tags.([]interface{}); ok {
					for _, tag := range tagList {
						if tagStr, ok := tag.(string); ok {
							tagCounts[tagStr]++
						}
					}
				}
			}

			timeBucket := result.CreatedAt.Format("2006-01-02")
			timeBuckets[timeBucket]++
		}
	}

	return diverseResults
}

// calculateDiversityMetrics 计算多样性指标
func (r *Ranker) calculateDiversityMetrics(results []*SearchResultItem) *DiversityMetrics {
	if len(results) == 0 {
		return &DiversityMetrics{}
	}

	contentTypeDistribution := make(map[string]int)
	tagDistribution := make(map[string]int)
	timeDistribution := make(map[string]int)

	for _, result := range results {
		// 内容类型分布
		if contentType, exists := result.Metadata["content_type"]; exists {
			if contentTypeStr, ok := contentType.(string); ok {
				contentTypeDistribution[contentTypeStr]++
			}
		}

		// 标签分布
		if tags, exists := result.Metadata["tags"]; exists {
			if tagList, ok := tags.([]interface{}); ok {
				for _, tag := range tagList {
					if tagStr, ok := tag.(string); ok {
						tagDistribution[tagStr]++
					}
				}
			}
		}

		// 时间分布（按月分组）
		timeBucket := result.CreatedAt.Format("2006-01")
		timeDistribution[timeBucket]++
	}

	// 计算多样性分数（使用香农熵）
	diversityScore := r.calculateShannonEntropy(contentTypeDistribution) * 0.4 +
		r.calculateShannonEntropy(tagDistribution) * 0.4 +
		r.calculateShannonEntropy(timeDistribution) * 0.2

	return &DiversityMetrics{
		ContentTypeDistribution: contentTypeDistribution,
		TagDistribution:         tagDistribution,
		TimeDistribution:        timeDistribution,
		DiversityScore:          diversityScore,
	}
}

// calculateShannonEntropy 计算香农熵
func (r *Ranker) calculateShannonEntropy(distribution map[string]int) float64 {
	if len(distribution) <= 1 {
		return 0.0
	}

	total := 0
	for _, count := range distribution {
		total += count
	}

	if total == 0 {
		return 0.0
	}

	entropy := 0.0
	for _, count := range distribution {
		if count > 0 {
			probability := float64(count) / float64(total)
			entropy -= probability * math.Log2(probability)
		}
	}

	// 规范化到0-1范围
	maxEntropy := math.Log2(float64(len(distribution)))
	if maxEntropy > 0 {
		entropy /= maxEntropy
	}

	return entropy
}

// GetDefaultRankingOptions 获取默认排序选项
func (r *Ranker) GetDefaultRankingOptions() *RankingOptions {
	return &RankingOptions{
		Strategy: RankingStrategyHybrid,
		Weights: &RankingWeights{
			Similarity:     0.4,
			KeywordMatch:   0.2,
			Importance:     0.2,
			Freshness:      0.1,
			UserPreference: 0.05,
			ContentType:    0.03,
			TagRelevance:   0.02,
		},
		TimeDecay: &TimeDecayConfig{
			Enabled:         true,
			DecayRate:       0.5,
			HalfLifeDays:    365,
			MinScore:        0.1,
			RecentBoostDays: 7,
		},
		DiversitySettings: &DiversitySettings{
			Enabled:              true,
			ContentTypeDiversity: true,
			TagDiversity:         false,
			TimeDiversity:        false,
			MaxSimilarResults:    3,
			DiversityThreshold:   0.7,
		},
	}
}