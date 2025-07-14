package content

import (
	"context"
	"regexp"
	"strings"
	"time"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
	"memoro/internal/services/llm"
)

// ClassificationResult 分类结果
type ClassificationResult struct {
	Categories      []string               `json:"categories"`       // 主要分类
	Tags            []string               `json:"tags"`             // 自动生成的标签
	Keywords        []string               `json:"keywords"`         // 关键词
	ImportanceScore float64                `json:"importance_score"` // 重要性评分 (0.0-1.0)
	Confidence      float64                `json:"confidence"`       // 置信度
	Metadata        map[string]interface{} `json:"metadata"`         // 额外元数据
}

// Classifier 内容分类器接口
type Classifier interface {
	// Classify 对内容进行分类
	Classify(ctx context.Context, content *ExtractedContent) (*ClassificationResult, error)
	
	// CalculateImportance 计算内容重要性
	CalculateImportance(ctx context.Context, content *ExtractedContent) (float64, error)
	
	// Close 关闭分类器
	Close() error
}

// ContentClassifier 内容分类器实现
type ContentClassifier struct {
	config     config.ProcessingConfig
	llmClient  *llm.Client
	tagger     *llm.Tagger
	logger     *logger.Logger
}

// NewClassifier 创建内容分类器
func NewClassifier() (*ContentClassifier, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("processing config")
	}

	classifierLogger := logger.NewLogger("content-classifier")

	// 创建LLM客户端
	llmClient, err := llm.NewClient()
	if err != nil {
		classifierLogger.Error("Failed to create LLM client", logger.Fields{
			"error": err.Error(),
		})
		return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create LLM client").
			WithCause(err)
	}

	// 创建标签生成器
	tagger, err := llm.NewTagger(llmClient)
	if err != nil {
		classifierLogger.Error("Failed to create tagger", logger.Fields{
			"error": err.Error(),
		})
		return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create tagger").
			WithCause(err)
	}

	classifier := &ContentClassifier{
		config:    cfg.Processing,
		llmClient: llmClient,
		tagger:    tagger,
		logger:    classifierLogger,
	}

	classifierLogger.Info("Content classifier initialized")

	return classifier, nil
}

// Classify 对内容进行分类
func (cc *ContentClassifier) Classify(ctx context.Context, content *ExtractedContent) (*ClassificationResult, error) {
	if content == nil {
		return nil, errors.ErrValidationFailed("content", "cannot be nil")
	}

	if content.Content == "" {
		return nil, errors.ErrValidationFailed("content.content", "cannot be empty")
	}

	cc.logger.Debug("Starting content classification", logger.Fields{
		"content_type":   string(content.Type),
		"content_length": len(content.Content),
		"has_title":      content.Title != "",
	})

	// 使用LLM生成标签和分类
	tagRequest := llm.TagRequest{
		Content:     content.Content,
		ContentType: content.Type,
		MaxTags:     20, // 增加标签数量限制
	}

	tagResult, err := cc.tagger.GenerateTags(ctx, tagRequest)
	if err != nil {
		cc.logger.Error("Failed to generate tags", logger.Fields{
			"error":        err.Error(),
			"content_type": string(content.Type),
		})
		return nil, err
	}

	// 计算重要性评分
	importanceScore, err := cc.CalculateImportance(ctx, content)
	if err != nil {
		cc.logger.Warn("Failed to calculate importance score, using default", logger.Fields{
			"error": err.Error(),
		})
		importanceScore = 0.5 // 默认中等重要性
	}

	// 提取关键词
	keywords := cc.extractKeywords(content.Content)

	// 构建分类结果
	result := &ClassificationResult{
		Categories:      tagResult.Categories,
		Tags:            tagResult.Tags,
		Keywords:        keywords,
		ImportanceScore: importanceScore,
		Confidence:      0.7, // 使用默认置信度
		Metadata: map[string]interface{}{
			"classification_time": time.Now(),
			"content_type":        string(content.Type),
			"content_length":      len(content.Content),
			"language":            content.Language,
			"has_title":           content.Title != "",
			"tag_count":           len(tagResult.Tags),
			"category_count":      len(tagResult.Categories),
			"keyword_count":       len(keywords),
			"tag_confidence":      tagResult.Confidence,
		},
	}

	// 验证分类结果
	if err := cc.validateClassificationResult(result); err != nil {
		return nil, err
	}

	cc.logger.Debug("Content classification completed", logger.Fields{
		"categories":       len(result.Categories),
		"tags":            len(result.Tags),
		"keywords":        len(result.Keywords),
		"importance_score": result.ImportanceScore,
		"confidence":      result.Confidence,
	})

	return result, nil
}

// CalculateImportance 计算内容重要性评分
func (cc *ContentClassifier) CalculateImportance(ctx context.Context, content *ExtractedContent) (float64, error) {
	if content == nil {
		return 0.0, errors.ErrValidationFailed("content", "cannot be nil")
	}

	cc.logger.Debug("Calculating importance score", logger.Fields{
		"content_type":   string(content.Type),
		"content_length": len(content.Content),
	})

	// 基础分数计算
	var baseScore float64 = 0.5

	// 1. 内容长度因子 (0.0-0.3)
	lengthScore := cc.calculateLengthScore(content.Content)
	
	// 2. 内容类型因子 (0.0-0.2)
	typeScore := cc.calculateTypeScore(content.Type)
	
	// 3. 内容特征因子 (0.0-0.3)
	featureScore := cc.calculateFeatureScore(content)
	
	// 4. 语言质量因子 (0.0-0.2)
	qualityScore := cc.calculateQualityScore(content.Content)

	// 计算综合分数
	totalScore := baseScore + lengthScore + typeScore + featureScore + qualityScore

	// 确保分数在 0.0-1.0 范围内
	if totalScore > 1.0 {
		totalScore = 1.0
	} else if totalScore < 0.0 {
		totalScore = 0.0
	}

	cc.logger.Debug("Importance score calculated", logger.Fields{
		"base_score":    baseScore,
		"length_score":  lengthScore,
		"type_score":    typeScore,
		"feature_score": featureScore,
		"quality_score": qualityScore,
		"total_score":   totalScore,
	})

	return totalScore, nil
}

// calculateLengthScore 计算长度得分
func (cc *ContentClassifier) calculateLengthScore(content string) float64 {
	length := len(content)
	
	// 短内容 (<100字符) - 较低分数
	if length < 100 {
		return 0.05
	}
	
	// 中等内容 (100-1000字符) - 递增分数
	if length < 1000 {
		return 0.05 + (float64(length-100)/900)*0.15
	}
	
	// 长内容 (1000-5000字符) - 高分数
	if length < 5000 {
		return 0.20 + (float64(length-1000)/4000)*0.10
	}
	
	// 超长内容 (>5000字符) - 最高分数
	return 0.30
}

// calculateTypeScore 根据内容类型计算得分
func (cc *ContentClassifier) calculateTypeScore(contentType models.ContentType) float64 {
	switch contentType {
	case models.ContentTypeText:
		return 0.15 // 文本内容通常重要
	case models.ContentTypeLink:
		return 0.20 // 链接通常包含有价值的信息
	case models.ContentTypeFile:
		return 0.18 // 文件通常包含重要文档
	case models.ContentTypeImage:
		return 0.10 // 图片重要性相对较低
	default:
		return 0.10
	}
}

// calculateFeatureScore 根据内容特征计算得分
func (cc *ContentClassifier) calculateFeatureScore(content *ExtractedContent) float64 {
	var score float64

	// 有标题加分
	if content.Title != "" {
		score += 0.05
	}

	// 有描述加分
	if content.Description != "" {
		score += 0.03
	}

	// 包含URL加分 (对于学习资源)
	if cc.containsURLs(content.Content) {
		score += 0.04
	}

	// 包含技术关键词加分
	if cc.containsTechnicalKeywords(content.Content) {
		score += 0.06
	}

	// 包含问题标识符加分
	if cc.containsQuestionIndicators(content.Content) {
		score += 0.05
	}

	// 包含代码加分
	if cc.containsCode(content.Content) {
		score += 0.07
	}

	// 确保不超过最大值
	if score > 0.30 {
		score = 0.30
	}

	return score
}

// calculateQualityScore 计算内容质量得分
func (cc *ContentClassifier) calculateQualityScore(content string) float64 {
	var score float64

	// 检查文本结构化程度
	if cc.hasGoodStructure(content) {
		score += 0.05
	}

	// 检查是否包含完整句子
	if cc.hasCompleteSentences(content) {
		score += 0.05
	}

	// 检查词汇丰富度
	if cc.hasRichVocabulary(content) {
		score += 0.05
	}

	// 检查逻辑连贯性 (简单版本)
	if cc.hasLogicalCoherence(content) {
		score += 0.05
	}

	return score
}

// extractKeywords 提取关键词
func (cc *ContentClassifier) extractKeywords(content string) []string {
	// 移除标点符号并转换为小写
	cleanContent := strings.ToLower(content)
	
	// 定义停用词
	stopWords := map[string]bool{
		"的": true, "了": true, "在": true, "是": true, "我": true, "有": true, "和": true,
		"就": true, "不": true, "人": true, "都": true, "一": true, "个": true, "上": true,
		"也": true, "很": true, "到": true, "说": true, "要": true, "去": true, "你": true,
		"会": true, "着": true, "没": true, "看": true, "好": true, "自": true, "己": true,
		"可以": true, "这个": true, "那个": true, "什么": true, "怎么": true, "为什么": true,
		"the": true, "and": true, "for": true, "are": true, "but": true, "not": true,
		"you": true, "all": true, "can": true, "had": true, "her": true, "was": true,
		"one": true, "our": true, "out": true, "day": true, "get": true, "has": true,
	}

	// 简单的关键词提取 - 查找重要的词汇模式
	keywordPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[一-龠]{2,}`),     // 中文词汇
		regexp.MustCompile(`[a-zA-Z]{3,}`),   // 英文词汇
		regexp.MustCompile(`\d+`),            // 数字
	}

	keywords := make(map[string]int)
	
	for _, pattern := range keywordPatterns {
		matches := pattern.FindAllString(cleanContent, -1)
		for _, match := range matches {
			if len(match) > 2 && !stopWords[match] {
				keywords[match]++
			}
		}
	}

	// 按频率排序并返回前20个关键词
	result := make([]string, 0, 20)
	for keyword, freq := range keywords {
		if freq >= 2 && len(result) < 20 { // 至少出现2次
			result = append(result, keyword)
		}
	}

	return result
}

// 辅助函数

func (cc *ContentClassifier) containsURLs(content string) bool {
	urlPattern := regexp.MustCompile(`https?://[^\s]+`)
	return urlPattern.MatchString(content)
}

func (cc *ContentClassifier) containsTechnicalKeywords(content string) bool {
	technicalKeywords := []string{
		"算法", "数据", "编程", "代码", "开发", "技术", "系统", "软件", "硬件",
		"网络", "数据库", "服务器", "API", "框架", "库", "工具", "平台",
		"algorithm", "data", "programming", "code", "development", "technology",
		"system", "software", "hardware", "network", "database", "server",
		"api", "framework", "library", "tool", "platform",
	}
	
	contentLower := strings.ToLower(content)
	for _, keyword := range technicalKeywords {
		if strings.Contains(contentLower, keyword) {
			return true
		}
	}
	return false
}

func (cc *ContentClassifier) containsQuestionIndicators(content string) bool {
	questionPatterns := []*regexp.Regexp{
		regexp.MustCompile(`[？?]`),
		regexp.MustCompile(`(如何|怎么|为什么|什么是|how\s+to|what\s+is|why)`),
	}
	
	for _, pattern := range questionPatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func (cc *ContentClassifier) containsCode(content string) bool {
	codePatterns := []*regexp.Regexp{
		regexp.MustCompile(`\{[^}]*\}`),           // 代码块
		regexp.MustCompile("`[^`]+`"),             // 行内代码
		regexp.MustCompile(`(function|class|def|import|include|#include)`), // 关键词
	}
	
	for _, pattern := range codePatterns {
		if pattern.MatchString(content) {
			return true
		}
	}
	return false
}

func (cc *ContentClassifier) hasGoodStructure(content string) bool {
	// 检查是否有标题、列表、段落等结构
	structurePatterns := []*regexp.Regexp{
		regexp.MustCompile(`^#+\s`),          // Markdown标题
		regexp.MustCompile(`^\d+\.`),         // 编号列表
		regexp.MustCompile(`^[*-]\s`),        // 无序列表
		regexp.MustCompile(`\n\s*\n`),        // 段落分隔
	}
	
	structureCount := 0
	for _, pattern := range structurePatterns {
		if pattern.MatchString(content) {
			structureCount++
		}
	}
	
	return structureCount >= 2
}

func (cc *ContentClassifier) hasCompleteSentences(content string) bool {
	// 检查是否包含完整句子 (以句号、问号、感叹号结尾)
	sentencePattern := regexp.MustCompile(`[。！？.!?]`)
	matches := sentencePattern.FindAllString(content, -1)
	return len(matches) >= 2
}

func (cc *ContentClassifier) hasRichVocabulary(content string) bool {
	// 简单的词汇丰富度检查
	words := strings.Fields(content)
	uniqueWords := make(map[string]bool)
	
	for _, word := range words {
		if len(word) > 2 {
			uniqueWords[strings.ToLower(word)] = true
		}
	}
	
	// 词汇多样性比例
	if len(words) == 0 {
		return false
	}
	
	diversity := float64(len(uniqueWords)) / float64(len(words))
	return diversity > 0.5
}

func (cc *ContentClassifier) hasLogicalCoherence(content string) bool {
	// 简单的逻辑连贯性检查 - 查找连接词
	coherenceIndicators := []string{
		"因此", "所以", "但是", "然而", "而且", "另外", "首先", "其次", "最后",
		"therefore", "however", "moreover", "furthermore", "first", "second", "finally",
	}
	
	contentLower := strings.ToLower(content)
	coherenceCount := 0
	
	for _, indicator := range coherenceIndicators {
		if strings.Contains(contentLower, indicator) {
			coherenceCount++
		}
	}
	
	return coherenceCount >= 2
}

// validateClassificationResult 验证分类结果
func (cc *ContentClassifier) validateClassificationResult(result *ClassificationResult) error {
	if result == nil {
		return errors.ErrValidationFailed("classification_result", "cannot be nil")
	}

	if result.ImportanceScore < 0.0 || result.ImportanceScore > 1.0 {
		return errors.ErrValidationFailed("importance_score", "must be between 0.0 and 1.0")
	}

	if result.Confidence < 0.0 || result.Confidence > 1.0 {
		return errors.ErrValidationFailed("confidence", "must be between 0.0 and 1.0")
	}

	// 限制标签数量
	if len(result.Tags) > 50 {
		result.Tags = result.Tags[:50]
	}

	// 限制关键词数量
	if len(result.Keywords) > 30 {
		result.Keywords = result.Keywords[:30]
	}

	return nil
}

// Close 关闭分类器
func (cc *ContentClassifier) Close() error {
	cc.logger.Info("Closing content classifier")
	
	if cc.llmClient != nil {
		if err := cc.llmClient.Close(); err != nil {
			cc.logger.Error("Failed to close LLM client", logger.Fields{
				"error": err.Error(),
			})
		}
	}
	
	return nil
}

// NewContentClassifier 创建内容分类器（向后兼容的工厂函数）
func NewContentClassifier() (*ContentClassifier, error) {
	return NewClassifier()
}