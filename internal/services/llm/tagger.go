package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// Tagger LLM智能标签生成器
type Tagger struct {
	client *Client
	config config.ProcessingConfig
	logger *logger.Logger
}

// TagRequest 标签生成请求
type TagRequest struct {
	Content     string                  `json:"content"`
	ContentType models.ContentType      `json:"content_type"`
	Context     map[string]interface{}  `json:"context,omitempty"`     // 可选的上下文信息
	ExistingTags []string               `json:"existing_tags,omitempty"` // 已有标签，用于参考
	MaxTags     int                     `json:"max_tags,omitempty"`    // 最大标签数量
}

// TagResult 标签生成结果
type TagResult struct {
	Tags       []string               `json:"tags"`        // 生成的标签列表
	Categories []string               `json:"categories"`  // 内容分类
	Keywords   []string               `json:"keywords"`    // 关键词
	Confidence map[string]float64     `json:"confidence"`  // 各标签的置信度
}

// TagResponse LLM标签响应结构（用于解析LLM返回的JSON）
type TagResponse struct {
	Tags       []string               `json:"tags"`
	Categories []string               `json:"categories"`
	Keywords   []string               `json:"keywords"`
	Confidence map[string]float64     `json:"confidence"`
}

// NewTagger 创建新的标签生成器
func NewTagger(client *Client) (*Tagger, error) {
	if client == nil {
		return nil, errors.ErrValidationFailed("client", "cannot be nil")
	}

	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("processing config")
	}

	tagger := &Tagger{
		client: client,
		config: cfg.Processing,
		logger: logger.NewLogger("llm-tagger"),
	}

	tagger.logger.Info("Tagger initialized", logger.Fields{
		"max_tags":        cfg.Processing.TagLimits.MaxTags,
		"max_tag_length":  cfg.Processing.TagLimits.MaxTagLength,
	})

	return tagger, nil
}

// GenerateTags 生成智能标签
func (t *Tagger) GenerateTags(ctx context.Context, request TagRequest) (*TagResult, error) {
	if request.Content == "" {
		return nil, errors.ErrValidationFailed("content", "cannot be empty")
	}

	if len(request.Content) > 100000 { // 100KB限制
		return nil, errors.ErrValidationFailed("content", "content too large (max 100KB)")
	}

	// 设置默认最大标签数
	if request.MaxTags <= 0 {
		request.MaxTags = t.config.TagLimits.MaxTags
	}
	if request.MaxTags > t.config.TagLimits.MaxTags {
		request.MaxTags = t.config.TagLimits.MaxTags
	}

	t.logger.Debug("Generating tags", logger.Fields{
		"content_type":     string(request.ContentType),
		"content_length":   len(request.Content),
		"max_tags":         request.MaxTags,
		"existing_tags":    len(request.ExistingTags),
		"has_context":      request.Context != nil,
	})

	// 构建系统提示和用户请求
	systemPrompt := t.buildSystemPrompt(request.ContentType)
	userPrompt := t.buildUserPrompt(request)

	// 调用LLM生成标签
	response, err := t.client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	if err != nil {
		t.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to generate tags")
		return nil, err
	}

	// 解析LLM响应
	result, err := t.parseTagResponse(response)
	if err != nil {
		t.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to parse tag response")
		return nil, err
	}

	// 验证和清理结果
	if err := t.validateAndCleanResult(result, request.MaxTags); err != nil {
		t.logger.LogMemoroError(err.(*errors.MemoroError), "Tag validation failed")
		return nil, err
	}

	t.logger.Debug("Tag generation completed", logger.Fields{
		"tags_count":       len(result.Tags),
		"categories_count": len(result.Categories),
		"keywords_count":   len(result.Keywords),
	})

	return result, nil
}

// GenerateSimpleTags 生成简单标签（仅标签，不包含分类和关键词）
func (t *Tagger) GenerateSimpleTags(ctx context.Context, content string, contentType models.ContentType, maxTags int) ([]string, error) {
	if content == "" {
		return nil, errors.ErrValidationFailed("content", "cannot be empty")
	}

	if maxTags <= 0 {
		maxTags = 10 // 默认最大10个标签
	}

	systemPrompt := `你是一个专业的内容标签生成器。请为给定内容生成最相关的标签。

要求：
1. 每个标签不超过10个字符
2. 标签要准确反映内容主题
3. 优先选择常用的、有意义的标签
4. 避免重复和冗余
5. 只返回标签列表，用逗号分隔`

	userPrompt := fmt.Sprintf(`请为以下%s内容生成%d个最相关的标签：

内容：
%s

标签（用逗号分隔）：`, t.getContentTypeDisplay(contentType), maxTags, content)

	response, err := t.client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	if err != nil {
		return nil, err
	}

	// 解析简单标签响应
	tags := t.parseSimpleTagResponse(response)

	// 验证和清理标签
	cleanTags := t.cleanTags(tags)
	if len(cleanTags) > maxTags {
		cleanTags = cleanTags[:maxTags]
	}

	return cleanTags, nil
}

// buildSystemPrompt 构建系统提示
func (t *Tagger) buildSystemPrompt(contentType models.ContentType) string {
	basePrompt := `你是一个专业的内容分析和标签生成专家。你的任务是为给定内容生成准确、有用的标签、分类和关键词。

分析原则：
1. 深度理解内容主题和要点
2. 提取最具代表性的标签
3. 识别内容的分类和类型
4. 找出关键词和核心概念
5. 为每个标签提供置信度评分（0-1之间）

输出格式要求：
请以JSON格式返回结果，包含以下字段：
{
  "tags": ["标签1", "标签2", "标签3"],
  "categories": ["分类1", "分类2"],
  "keywords": ["关键词1", "关键词2", "关键词3"],
  "confidence": {"标签1": 0.9, "标签2": 0.8}
}`

	switch contentType {
	case models.ContentTypeText:
		return basePrompt + `

针对文本内容：
- 分析文本主题和论点
- 识别专业术语和概念
- 提取行业或领域标签
- 分析语言风格和类型`

	case models.ContentTypeLink:
		return basePrompt + `

针对链接内容：
- 分析网页标题和描述
- 识别网站类型和用途
- 提取主要信息标签
- 标注内容来源特征`

	case models.ContentTypeFile:
		return basePrompt + `

针对文件内容：
- 识别文件类型和格式
- 分析文档主要内容
- 提取专业领域标签
- 标注文档用途和特征`

	case models.ContentTypeImage:
		return basePrompt + `

针对图片内容：
- 描述图片主要对象
- 识别场景和环境
- 分析图片用途和类型
- 提取视觉元素标签`

	default:
		return basePrompt
	}
}

// buildUserPrompt 构建用户请求提示
func (t *Tagger) buildUserPrompt(request TagRequest) string {
	contentTypeDisplay := t.getContentTypeDisplay(request.ContentType)

	prompt := fmt.Sprintf(`请为以下%s内容生成标签分析：

内容：
%s

要求：
1. 生成最多%d个最相关的标签
2. 每个标签不超过%d个字符
3. 提供2-3个主要分类
4. 提取5-8个关键词
5. 为每个标签提供置信度（0-1之间）`,
		contentTypeDisplay, request.Content, request.MaxTags, t.config.TagLimits.MaxTagLength)

	// 如果有已有标签，提供参考
	if len(request.ExistingTags) > 0 {
		prompt += fmt.Sprintf(`

参考已有标签：%s
请生成与现有标签相关但不重复的新标签。`, strings.Join(request.ExistingTags, ", "))
	}

	// 如果有上下文信息，添加到提示中
	if request.Context != nil && len(request.Context) > 0 {
		prompt += "\n\n附加上下文信息："
		for key, value := range request.Context {
			prompt += fmt.Sprintf("\n- %s: %v", key, value)
		}
	}

	prompt += "\n\n请以JSON格式返回结果："

	return prompt
}

// parseTagResponse 解析LLM的标签响应
func (t *Tagger) parseTagResponse(response string) (*TagResult, error) {
	// 清理响应，移除可能的markdown格式
	response = strings.TrimSpace(response)
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimPrefix(response, "```")
	response = strings.TrimSuffix(response, "```")

	// 尝试解析JSON
	var tagResponse TagResponse
	if err := json.Unmarshal([]byte(response), &tagResponse); err != nil {
		// 如果JSON解析失败，尝试从文本中提取标签
		t.logger.Warn("Failed to parse JSON response, attempting text extraction", logger.Fields{
			"error": err.Error(),
			"response": response,
		})
		return t.fallbackParseResponse(response)
	}

	result := &TagResult{
		Tags:       tagResponse.Tags,
		Categories: tagResponse.Categories,
		Keywords:   tagResponse.Keywords,
		Confidence: tagResponse.Confidence,
	}

	return result, nil
}

// fallbackParseResponse 备用响应解析方法
func (t *Tagger) fallbackParseResponse(response string) (*TagResult, error) {
	lines := strings.Split(response, "\n")
	result := &TagResult{
		Tags:       make([]string, 0),
		Categories: make([]string, 0),
		Keywords:   make([]string, 0),
		Confidence: make(map[string]float64),
	}

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "标签") || strings.Contains(line, "tags") {
			// 提取标签
			tags := t.extractTagsFromLine(line)
			result.Tags = append(result.Tags, tags...)
		}
	}

	// 如果仍然没有提取到标签，返回错误
	if len(result.Tags) == 0 {
		return nil, errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Failed to extract tags from LLM response").
			WithDetails("Response does not contain valid tag information").
			WithContext(map[string]interface{}{
				"response": response,
			})
	}

	return result, nil
}

// extractTagsFromLine 从文本行中提取标签
func (t *Tagger) extractTagsFromLine(line string) []string {
	// 移除标签标识符
	line = strings.ReplaceAll(line, "标签：", "")
	line = strings.ReplaceAll(line, "tags:", "")
	line = strings.ReplaceAll(line, "Tags:", "")

	// 按逗号分割
	tags := strings.Split(line, ",")
	var cleanTags []string

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag = strings.Trim(tag, "\"'")
		if tag != "" && len(tag) <= t.config.TagLimits.MaxTagLength {
			cleanTags = append(cleanTags, tag)
		}
	}

	return cleanTags
}

// parseSimpleTagResponse 解析简单标签响应
func (t *Tagger) parseSimpleTagResponse(response string) []string {
	// 清理响应
	response = strings.TrimSpace(response)
	
	// 按逗号分割
	tags := strings.Split(response, ",")
	var cleanTags []string

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag = strings.Trim(tag, "\"'")
		if tag != "" {
			cleanTags = append(cleanTags, tag)
		}
	}

	return cleanTags
}

// validateAndCleanResult 验证和清理结果
func (t *Tagger) validateAndCleanResult(result *TagResult, maxTags int) error {
	if result == nil {
		return errors.ErrValidationFailed("result", "cannot be nil")
	}

	// 清理和验证标签
	result.Tags = t.cleanTags(result.Tags)
	if len(result.Tags) > maxTags {
		result.Tags = result.Tags[:maxTags]
	}

	// 清理分类
	result.Categories = t.cleanTags(result.Categories)
	if len(result.Categories) > 5 { // 最多5个分类
		result.Categories = result.Categories[:5]
	}

	// 清理关键词
	result.Keywords = t.cleanTags(result.Keywords)
	if len(result.Keywords) > 10 { // 最多10个关键词
		result.Keywords = result.Keywords[:10]
	}

	// 验证置信度
	if result.Confidence == nil {
		result.Confidence = make(map[string]float64)
	}

	// 为没有置信度的标签设置默认值
	for _, tag := range result.Tags {
		if _, exists := result.Confidence[tag]; !exists {
			result.Confidence[tag] = 0.7 // 默认置信度
		}
	}

	return nil
}

// cleanTags 清理标签列表
func (t *Tagger) cleanTags(tags []string) []string {
	seen := make(map[string]bool)
	var cleaned []string

	for _, tag := range tags {
		tag = strings.TrimSpace(tag)
		tag = strings.Trim(tag, "\"'")
		
		// 跳过空标签和过长标签
		if tag == "" || len(tag) > t.config.TagLimits.MaxTagLength {
			continue
		}

		// 跳过重复标签
		if seen[tag] {
			continue
		}

		seen[tag] = true
		cleaned = append(cleaned, tag)
	}

	return cleaned
}

// getContentTypeDisplay 获取内容类型的显示名称
func (t *Tagger) getContentTypeDisplay(contentType models.ContentType) string {
	switch contentType {
	case models.ContentTypeText:
		return "文本"
	case models.ContentTypeLink:
		return "链接"
	case models.ContentTypeFile:
		return "文件"
	case models.ContentTypeImage:
		return "图片"
	case models.ContentTypeAudio:
		return "音频"
	case models.ContentTypeVideo:
		return "视频"
	default:
		return "内容"
	}
}

// Close 关闭标签生成器
func (t *Tagger) Close() error {
	t.logger.Info("Closing tagger")
	return nil
}