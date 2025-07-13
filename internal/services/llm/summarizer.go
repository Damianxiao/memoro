package llm

import (
	"context"
	"fmt"
	"strings"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// Summarizer LLM内容摘要生成器
type Summarizer struct {
	client *Client
	config config.ProcessingConfig
	logger *logger.Logger
}

// SummaryRequest 摘要请求
type SummaryRequest struct {
	Content     string                  `json:"content"`
	ContentType models.ContentType      `json:"content_type"`
	Context     map[string]interface{}  `json:"context,omitempty"` // 可选的上下文信息
}

// SummaryResult 摘要结果
type SummaryResult struct {
	OneLine   string `json:"one_line"`   // 一句话摘要
	Paragraph string `json:"paragraph"`  // 段落摘要
	Detailed  string `json:"detailed"`   // 详细摘要
}

// NewSummarizer 创建新的摘要生成器
func NewSummarizer(client *Client) (*Summarizer, error) {
	if client == nil {
		return nil, errors.ErrValidationFailed("client", "cannot be nil")
	}

	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("processing config")
	}

	summarizer := &Summarizer{
		client: client,
		config: cfg.Processing,
		logger: logger.NewLogger("llm-summarizer"),
	}

	summarizer.logger.Info("Summarizer initialized", logger.Fields{
		"one_line_max":   cfg.Processing.SummaryLevels.OneLineMaxLength,
		"paragraph_max":  cfg.Processing.SummaryLevels.ParagraphMaxLength,
		"detailed_max":   cfg.Processing.SummaryLevels.DetailedMaxLength,
	})

	return summarizer, nil
}

// GenerateSummary 生成多层次摘要
func (s *Summarizer) GenerateSummary(ctx context.Context, request SummaryRequest) (*SummaryResult, error) {
	if request.Content == "" {
		return nil, errors.ErrValidationFailed("content", "cannot be empty")
	}

	if len(request.Content) > 100000 { // 100KB限制
		return nil, errors.ErrValidationFailed("content", "content too large (max 100KB)")
	}

	s.logger.Debug("Generating summary", logger.Fields{
		"content_type":   string(request.ContentType),
		"content_length": len(request.Content),
		"has_context":    request.Context != nil,
	})

	// 构建系统提示
	systemPrompt := s.buildSystemPrompt(request.ContentType)

	// 生成一句话摘要
	oneLine, err := s.generateOneLineSummary(ctx, systemPrompt, request.Content)
	if err != nil {
		s.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to generate one-line summary")
		return nil, err
	}

	// 生成段落摘要
	paragraph, err := s.generateParagraphSummary(ctx, systemPrompt, request.Content)
	if err != nil {
		s.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to generate paragraph summary")
		return nil, err
	}

	// 生成详细摘要
	detailed, err := s.generateDetailedSummary(ctx, systemPrompt, request.Content)
	if err != nil {
		s.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to generate detailed summary")
		return nil, err
	}

	result := &SummaryResult{
		OneLine:   oneLine,
		Paragraph: paragraph,
		Detailed:  detailed,
	}

	// 验证结果
	if err := s.validateSummaryResult(result); err != nil {
		s.logger.LogMemoroError(err.(*errors.MemoroError), "Summary validation failed")
		return nil, err
	}

	s.logger.Debug("Summary generation completed", logger.Fields{
		"one_line_length":   len(result.OneLine),
		"paragraph_length":  len(result.Paragraph),
		"detailed_length":   len(result.Detailed),
	})

	return result, nil
}

// generateOneLineSummary 生成一句话摘要
func (s *Summarizer) generateOneLineSummary(ctx context.Context, systemPrompt, content string) (string, error) {
	maxLength := s.config.SummaryLevels.OneLineMaxLength

	userPrompt := fmt.Sprintf(`请为以下内容生成一句话摘要，要求：
1. 长度不超过%d个字符
2. 概括核心要点
3. 语言简洁明了
4. 不包含换行符

内容：
%s

一句话摘要：`, maxLength, content)

	response, err := s.client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// 清理和验证响应
	summary := strings.TrimSpace(response)
	summary = strings.ReplaceAll(summary, "\n", " ")
	summary = strings.ReplaceAll(summary, "\r", " ")

	if len(summary) > maxLength {
		// 如果超长，截取并添加省略号
		if maxLength > 3 {
			summary = summary[:maxLength-3] + "..."
		} else {
			summary = summary[:maxLength]
		}
	}

	return summary, nil
}

// generateParagraphSummary 生成段落摘要
func (s *Summarizer) generateParagraphSummary(ctx context.Context, systemPrompt, content string) (string, error) {
	maxLength := s.config.SummaryLevels.ParagraphMaxLength

	userPrompt := fmt.Sprintf(`请为以下内容生成段落摘要，要求：
1. 长度不超过%d个字符
2. 包含主要观点和重要细节
3. 结构清晰，逻辑连贯
4. 可以包含3-5句话

内容：
%s

段落摘要：`, maxLength, content)

	response, err := s.client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// 清理响应
	summary := strings.TrimSpace(response)

	if len(summary) > maxLength {
		// 如果超长，在句号处截断
		truncated := s.truncateAtSentence(summary, maxLength)
		if truncated != "" {
			summary = truncated
		} else {
			// 如果没有找到合适的句号，强制截断
			if maxLength > 3 {
				summary = summary[:maxLength-3] + "..."
			} else {
				summary = summary[:maxLength]
			}
		}
	}

	return summary, nil
}

// generateDetailedSummary 生成详细摘要
func (s *Summarizer) generateDetailedSummary(ctx context.Context, systemPrompt, content string) (string, error) {
	maxLength := s.config.SummaryLevels.DetailedMaxLength

	userPrompt := fmt.Sprintf(`请为以下内容生成详细摘要，要求：
1. 长度不超过%d个字符
2. 包含所有重要信息和细节
3. 保持原文的逻辑结构
4. 可以分段组织内容
5. 突出关键观点和数据

内容：
%s

详细摘要：`, maxLength, content)

	response, err := s.client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}

	// 清理响应
	summary := strings.TrimSpace(response)

	if len(summary) > maxLength {
		// 详细摘要允许在段落处截断
		truncated := s.truncateAtParagraph(summary, maxLength)
		if truncated != "" {
			summary = truncated
		} else {
			// 如果没有找到合适的段落，在句号处截断
			truncated = s.truncateAtSentence(summary, maxLength)
			if truncated != "" {
				summary = truncated
			} else {
				// 最后强制截断
				if maxLength > 3 {
					summary = summary[:maxLength-3] + "..."
				} else {
					summary = summary[:maxLength]
				}
			}
		}
	}

	return summary, nil
}

// buildSystemPrompt 构建系统提示
func (s *Summarizer) buildSystemPrompt(contentType models.ContentType) string {
	basePrompt := `你是一个专业的内容摘要助手。你的任务是为用户提供准确、简洁、有用的内容摘要。

摘要原则：
1. 保持客观中立，不添加个人观点
2. 提取核心信息和关键要点
3. 保持原文的语言风格和重要术语
4. 确保摘要的完整性和准确性
5. 根据内容类型调整摘要策略`

	switch contentType {
	case models.ContentTypeText:
		return basePrompt + `

针对文本内容：
- 识别主题和论点
- 提取关键信息和数据
- 保持逻辑结构清晰`

	case models.ContentTypeLink:
		return basePrompt + `

针对链接内容：
- 识别网页标题和主要内容
- 提取核心信息和价值
- 注明内容来源和类型`

	case models.ContentTypeFile:
		return basePrompt + `

针对文件内容：
- 识别文档类型和主要内容
- 提取关键信息和结构
- 保留重要的格式信息`

	case models.ContentTypeImage:
		return basePrompt + `

针对图片内容：
- 描述图片的主要元素
- 识别文字信息（如有）
- 分析图片的用途和含义`

	default:
		return basePrompt
	}
}

// truncateAtSentence 在句号处截断文本
func (s *Summarizer) truncateAtSentence(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// 在最大长度附近寻找句号
	searchStart := maxLength - 100
	if searchStart < 0 {
		searchStart = 0
	}

	searchText := text[searchStart:maxLength]
	lastDot := strings.LastIndex(searchText, "。")
	if lastDot == -1 {
		lastDot = strings.LastIndex(searchText, ".")
	}

	if lastDot != -1 {
		return text[:searchStart+lastDot+1]
	}

	return ""
}

// truncateAtParagraph 在段落处截断文本
func (s *Summarizer) truncateAtParagraph(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}

	// 在最大长度附近寻找段落分隔符
	searchStart := maxLength - 200
	if searchStart < 0 {
		searchStart = 0
	}

	searchText := text[searchStart:maxLength]
	lastParagraph := strings.LastIndex(searchText, "\n\n")
	if lastParagraph != -1 {
		return text[:searchStart+lastParagraph]
	}

	return ""
}

// validateSummaryResult 验证摘要结果
func (s *Summarizer) validateSummaryResult(result *SummaryResult) error {
	if result.OneLine == "" {
		return errors.ErrValidationFailed("one_line_summary", "cannot be empty")
	}

	if len(result.OneLine) > s.config.SummaryLevels.OneLineMaxLength {
		return errors.ErrValidationFailed("one_line_summary", fmt.Sprintf("exceeds maximum length of %d", s.config.SummaryLevels.OneLineMaxLength))
	}

	if result.Paragraph == "" {
		return errors.ErrValidationFailed("paragraph_summary", "cannot be empty")
	}

	if len(result.Paragraph) > s.config.SummaryLevels.ParagraphMaxLength {
		return errors.ErrValidationFailed("paragraph_summary", fmt.Sprintf("exceeds maximum length of %d", s.config.SummaryLevels.ParagraphMaxLength))
	}

	if result.Detailed == "" {
		return errors.ErrValidationFailed("detailed_summary", "cannot be empty")
	}

	if len(result.Detailed) > s.config.SummaryLevels.DetailedMaxLength {
		return errors.ErrValidationFailed("detailed_summary", fmt.Sprintf("exceeds maximum length of %d", s.config.SummaryLevels.DetailedMaxLength))
	}

	return nil
}

// GenerateQuickSummary 生成快速摘要（仅一句话）
func (s *Summarizer) GenerateQuickSummary(ctx context.Context, content string, contentType models.ContentType) (string, error) {
	if content == "" {
		return "", errors.ErrValidationFailed("content", "cannot be empty")
	}

	systemPrompt := s.buildSystemPrompt(contentType)
	return s.generateOneLineSummary(ctx, systemPrompt, content)
}

// Close 关闭摘要生成器
func (s *Summarizer) Close() error {
	s.logger.Info("Closing summarizer")
	return nil
}