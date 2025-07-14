package content

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// ExtractedContent 提取的内容结构
type ExtractedContent struct {
	Content     string                 `json:"content"`      // 提取的主要内容
	Title       string                 `json:"title"`        // 标题
	Description string                 `json:"description"`  // 描述
	Metadata    map[string]interface{} `json:"metadata"`     // 元数据
	Type        models.ContentType     `json:"type"`         // 内容类型
	Size        int64                  `json:"size"`         // 内容大小
	Language    string                 `json:"language"`     // 语言
}

// Extractor 内容提取器接口
type Extractor interface {
	// Extract 提取内容
	Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error)
	
	// CanHandle 检查是否能处理指定类型的内容
	CanHandle(contentType models.ContentType) bool
	
	// GetSupportedTypes 获取支持的内容类型
	GetSupportedTypes() []models.ContentType
	
	// Close 关闭提取器
	Close() error
}

// ExtractorManager 提取器管理器
type ExtractorManager struct {
	extractors map[models.ContentType]Extractor
	config     config.ProcessingConfig
	logger     *logger.Logger
}

// NewExtractorManager 创建提取器管理器
func NewExtractorManager() (*ExtractorManager, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("processing config")
	}

	manager := &ExtractorManager{
		extractors: make(map[models.ContentType]Extractor),
		config:     cfg.Processing,
		logger:     logger.NewLogger("extractor-manager"),
	}

	// 注册各类型提取器
	if err := manager.registerExtractors(); err != nil {
		return nil, err
	}

	manager.logger.Info("Extractor manager initialized", logger.Fields{
		"registered_types": len(manager.extractors),
	})

	return manager, nil
}

// registerExtractors 注册提取器
func (em *ExtractorManager) registerExtractors() error {
	// 注册文本提取器
	textExtractor := &TextExtractor{
		config: em.config,
		logger: logger.NewLogger("text-extractor"),
	}
	em.extractors[models.ContentTypeText] = textExtractor

	// 注册链接提取器
	linkExtractor := &LinkExtractor{
		config:     em.config,
		logger:     logger.NewLogger("link-extractor"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
	em.extractors[models.ContentTypeLink] = linkExtractor

	// 注册文件提取器
	fileExtractor := &FileExtractor{
		config: em.config,
		logger: logger.NewLogger("file-extractor"),
	}
	em.extractors[models.ContentTypeFile] = fileExtractor

	// 注册图片提取器
	imageExtractor := &ImageExtractor{
		config: em.config,
		logger: logger.NewLogger("image-extractor"),
	}
	em.extractors[models.ContentTypeImage] = imageExtractor

	em.logger.Debug("All extractors registered", logger.Fields{
		"text_extractor":  textExtractor != nil,
		"link_extractor":  linkExtractor != nil,
		"file_extractor":  fileExtractor != nil,
		"image_extractor": imageExtractor != nil,
	})

	return nil
}

// Extract 提取内容
func (em *ExtractorManager) Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error) {
	if rawContent == "" {
		return nil, errors.ErrValidationFailed("raw_content", "cannot be empty")
	}

	// 查找对应的提取器
	extractor, exists := em.extractors[contentType]
	if !exists {
		return nil, errors.ErrValidationFailed("content_type", fmt.Sprintf("unsupported content type: %s", contentType))
	}

	em.logger.Debug("Extracting content", logger.Fields{
		"content_type":   string(contentType),
		"content_length": len(rawContent),
	})

	// 调用具体提取器
	result, err := extractor.Extract(ctx, rawContent, contentType)
	if err != nil {
		em.logger.Error("Content extraction failed", logger.Fields{
			"content_type": string(contentType),
			"error":        err.Error(),
		})
		return nil, err
	}

	// 验证提取结果
	if err := em.validateExtractedContent(result); err != nil {
		return nil, err
	}

	em.logger.Debug("Content extraction completed", logger.Fields{
		"content_type":   string(contentType),
		"extracted_size": len(result.Content),
		"has_title":      result.Title != "",
		"has_metadata":   len(result.Metadata) > 0,
	})

	return result, nil
}

// GetSupportedTypes 获取所有支持的内容类型
func (em *ExtractorManager) GetSupportedTypes() []models.ContentType {
	types := make([]models.ContentType, 0, len(em.extractors))
	for contentType := range em.extractors {
		types = append(types, contentType)
	}
	return types
}

// CanHandle 检查是否能处理指定类型
func (em *ExtractorManager) CanHandle(contentType models.ContentType) bool {
	_, exists := em.extractors[contentType]
	return exists
}

// validateExtractedContent 验证提取的内容
func (em *ExtractorManager) validateExtractedContent(content *ExtractedContent) error {
	if content == nil {
		return errors.ErrValidationFailed("extracted_content", "cannot be nil")
	}

	if content.Content == "" {
		return errors.ErrValidationFailed("extracted_content.content", "cannot be empty")
	}

	if len(content.Content) > em.config.MaxContentSize {
		return errors.ErrValidationFailed("extracted_content.content", 
			fmt.Sprintf("extracted content too large (max %d bytes)", em.config.MaxContentSize))
	}

	if !models.IsValidContentType(content.Type) {
		return errors.ErrValidationFailed("extracted_content.type", "invalid content type")
	}

	return nil
}

// Close 关闭提取器管理器
func (em *ExtractorManager) Close() error {
	for contentType, extractor := range em.extractors {
		if err := extractor.Close(); err != nil {
			em.logger.Error("Failed to close extractor", logger.Fields{
				"content_type": string(contentType),
				"error":        err.Error(),
			})
		}
	}

	em.logger.Info("Extractor manager closed")
	return nil
}

// TextExtractor 文本内容提取器
type TextExtractor struct {
	config config.ProcessingConfig
	logger *logger.Logger
}

// Extract 提取文本内容
func (te *TextExtractor) Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error) {
	// 文本内容直接返回，进行基本清理
	cleanContent := strings.TrimSpace(rawContent)
	
	// 检测语言（简单实现）
	language := te.detectLanguage(cleanContent)
	
	// 尝试提取标题（如果内容包含明显的标题格式）
	title := te.extractTitle(cleanContent)
	
	result := &ExtractedContent{
		Content:     cleanContent,
		Title:       title,
		Description: te.generateDescription(cleanContent),
		Type:        models.ContentTypeText,
		Size:        int64(len(cleanContent)),
		Language:    language,
		Metadata: map[string]interface{}{
			"word_count":      te.countWords(cleanContent),
			"line_count":      strings.Count(cleanContent, "\n") + 1,
			"has_urls":        te.containsURLs(cleanContent),
			"estimated_read_time": te.estimateReadTime(cleanContent),
		},
	}

	te.logger.Debug("Text extraction completed", logger.Fields{
		"content_length": len(cleanContent),
		"word_count":     result.Metadata["word_count"],
		"language":       language,
		"has_title":      title != "",
	})

	return result, nil
}

// CanHandle 检查是否能处理文本类型
func (te *TextExtractor) CanHandle(contentType models.ContentType) bool {
	return contentType == models.ContentTypeText
}

// GetSupportedTypes 获取支持的类型
func (te *TextExtractor) GetSupportedTypes() []models.ContentType {
	return []models.ContentType{models.ContentTypeText}
}

// detectLanguage 检测语言
func (te *TextExtractor) detectLanguage(content string) string {
	// 简单的中英文检测
	chinesePattern := regexp.MustCompile(`[\p{Han}]`)
	englishPattern := regexp.MustCompile(`[a-zA-Z]`)

	chineseCount := len(chinesePattern.FindAllString(content, -1))
	englishCount := len(englishPattern.FindAllString(content, -1))

	if chineseCount > englishCount {
		return "zh"
	} else if englishCount > 0 {
		return "en"
	}
	return "unknown"
}

// extractTitle 提取标题
func (te *TextExtractor) extractTitle(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) == 0 {
		return ""
	}

	firstLine := strings.TrimSpace(lines[0])
	
	// 如果第一行较短且不包含句号，可能是标题
	if len(firstLine) > 0 && len(firstLine) < 100 && !strings.Contains(firstLine, "。") && !strings.Contains(firstLine, ".") {
		return firstLine
	}

	// 查找markdown风格的标题
	titlePattern := regexp.MustCompile(`^#+\s+(.+)$`)
	for _, line := range lines {
		if matches := titlePattern.FindStringSubmatch(strings.TrimSpace(line)); len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}

	return ""
}

// generateDescription 生成描述
func (te *TextExtractor) generateDescription(content string) string {
	// 取前200字符作为描述
	maxLen := 200
	if len(content) <= maxLen {
		return content
	}
	
	// 尝试在句号处截断
	truncated := content[:maxLen]
	if lastDot := strings.LastIndex(truncated, "。"); lastDot > 50 {
		return content[:lastDot+3] // 包含句号
	}
	if lastDot := strings.LastIndex(truncated, "."); lastDot > 50 {
		return content[:lastDot+1]
	}
	
	return truncated + "..."
}

// countWords 计算词数
func (te *TextExtractor) countWords(content string) int {
	// 中文按字符计算，英文按单词计算
	chinesePattern := regexp.MustCompile(`[\p{Han}]`)
	englishPattern := regexp.MustCompile(`\b[a-zA-Z]+\b`)

	chineseCount := len(chinesePattern.FindAllString(content, -1))
	englishWords := englishPattern.FindAllString(content, -1)
	
	return chineseCount + len(englishWords)
}

// containsURLs 检查是否包含URL
func (te *TextExtractor) containsURLs(content string) bool {
	urlPattern := regexp.MustCompile(`https?://[^\s]+`)
	return urlPattern.MatchString(content)
}

// estimateReadTime 估算阅读时间（分钟）
func (te *TextExtractor) estimateReadTime(content string) int {
	wordCount := te.countWords(content)
	// 假设每分钟阅读200个词/字
	minutes := wordCount / 200
	if minutes < 1 {
		return 1
	}
	return minutes
}

// Close 关闭文本提取器
func (te *TextExtractor) Close() error {
	te.logger.Debug("Text extractor closed")
	return nil
}

// LinkExtractor 链接内容提取器
type LinkExtractor struct {
	config     config.ProcessingConfig
	logger     *logger.Logger
	httpClient *http.Client
}

// Extract 提取链接内容
func (le *LinkExtractor) Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error) {
	// 验证URL格式
	parsedURL, err := url.Parse(strings.TrimSpace(rawContent))
	if err != nil || parsedURL.Scheme == "" {
		return nil, errors.ErrValidationFailed("url", "invalid URL format")
	}

	le.logger.Debug("Extracting link content", logger.Fields{
		"url":    parsedURL.String(),
		"scheme": parsedURL.Scheme,
		"host":   parsedURL.Host,
	})

	// 创建HTTP请求
	req, err := http.NewRequestWithContext(ctx, "GET", parsedURL.String(), nil)
	if err != nil {
		return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create HTTP request").
			WithCause(err).
			WithContext(map[string]interface{}{"url": parsedURL.String()})
	}

	// 设置User-Agent
	req.Header.Set("User-Agent", "Memoro/1.0 (Knowledge Management Bot)")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	// 发送请求
	resp, err := le.httpClient.Do(req)
	if err != nil {
		return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to fetch URL").
			WithCause(err).
			WithContext(map[string]interface{}{"url": parsedURL.String()})
	}
	defer resp.Body.Close()

	// 检查响应状态
	if resp.StatusCode >= 400 {
		return nil, errors.ErrValidationFailed("url", fmt.Sprintf("HTTP error: %d %s", resp.StatusCode, resp.Status))
	}

	// 读取响应内容
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to read response body").
			WithCause(err)
	}

	htmlContent := string(bodyBytes)

	// 提取网页信息
	title := le.extractTitle(htmlContent)
	description := le.extractDescription(htmlContent)
	content := le.extractMainContent(htmlContent)

	result := &ExtractedContent{
		Content:     content,
		Title:       title,
		Description: description,
		Type:        models.ContentTypeLink,
		Size:        int64(len(content)),
		Language:    le.detectLanguage(content),
		Metadata: map[string]interface{}{
			"url":             parsedURL.String(),
			"domain":          parsedURL.Host,
			"status_code":     resp.StatusCode,
			"content_type":    resp.Header.Get("Content-Type"),
			"content_length":  len(bodyBytes),
			"response_time":   time.Now(),
		},
	}

	le.logger.Debug("Link extraction completed", logger.Fields{
		"url":            parsedURL.String(),
		"title":          title,
		"content_length": len(content),
		"status_code":    resp.StatusCode,
	})

	return result, nil
}

// extractTitle 从HTML中提取标题
func (le *LinkExtractor) extractTitle(html string) string {
	titlePattern := regexp.MustCompile(`<title[^>]*>([^<]*)</title>`)
	matches := titlePattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// extractDescription 从HTML中提取描述
func (le *LinkExtractor) extractDescription(html string) string {
	// 尝试提取meta description
	descPattern := regexp.MustCompile(`<meta[^>]*name=["']description["'][^>]*content=["']([^"']*)["']`)
	matches := descPattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// 尝试提取og:description
	ogDescPattern := regexp.MustCompile(`<meta[^>]*property=["']og:description["'][^>]*content=["']([^"']*)["']`)
	matches = ogDescPattern.FindStringSubmatch(html)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	return ""
}

// extractMainContent 从HTML中提取主要内容
func (le *LinkExtractor) extractMainContent(html string) string {
	// 简单的HTML标签清理
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	content := tagPattern.ReplaceAllString(html, " ")
	
	// 清理多余的空白字符
	spacePattern := regexp.MustCompile(`\s+`)
	content = spacePattern.ReplaceAllString(content, " ")
	
	content = strings.TrimSpace(content)
	
	// 限制内容长度
	maxLen := 5000
	if len(content) > maxLen {
		content = content[:maxLen] + "..."
	}
	
	return content
}

// detectLanguage 检测语言
func (le *LinkExtractor) detectLanguage(content string) string {
	// 简单的中英文检测（复用TextExtractor的逻辑）
	chinesePattern := regexp.MustCompile(`[\p{Han}]`)
	englishPattern := regexp.MustCompile(`[a-zA-Z]`)

	chineseCount := len(chinesePattern.FindAllString(content, -1))
	englishCount := len(englishPattern.FindAllString(content, -1))

	if chineseCount > englishCount {
		return "zh"
	} else if englishCount > 0 {
		return "en"
	}
	return "unknown"
}

// CanHandle 检查是否能处理链接类型
func (le *LinkExtractor) CanHandle(contentType models.ContentType) bool {
	return contentType == models.ContentTypeLink
}

// GetSupportedTypes 获取支持的类型
func (le *LinkExtractor) GetSupportedTypes() []models.ContentType {
	return []models.ContentType{models.ContentTypeLink}
}

// Close 关闭链接提取器
func (le *LinkExtractor) Close() error {
	le.logger.Debug("Link extractor closed")
	return nil
}

// FileExtractor 文件内容提取器
type FileExtractor struct {
	config config.ProcessingConfig
	logger *logger.Logger
}

// Extract 提取文件内容
func (fe *FileExtractor) Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error) {
	// 这里应该根据文件路径或文件内容进行提取
	// 目前实现基础版本，后续可以扩展支持PDF、Word等
	
	result := &ExtractedContent{
		Content:     rawContent,
		Title:       "File Content",
		Description: "Extracted file content",
		Type:        models.ContentTypeFile,
		Size:        int64(len(rawContent)),
		Language:    "unknown",
		Metadata: map[string]interface{}{
			"extraction_method": "basic",
			"file_size":         len(rawContent),
		},
	}

	fe.logger.Debug("File extraction completed", logger.Fields{
		"content_length": len(rawContent),
	})

	return result, nil
}

// CanHandle 检查是否能处理文件类型
func (fe *FileExtractor) CanHandle(contentType models.ContentType) bool {
	return contentType == models.ContentTypeFile
}

// GetSupportedTypes 获取支持的类型
func (fe *FileExtractor) GetSupportedTypes() []models.ContentType {
	return []models.ContentType{models.ContentTypeFile}
}

// Close 关闭文件提取器
func (fe *FileExtractor) Close() error {
	fe.logger.Debug("File extractor closed")
	return nil
}

// ImageExtractor 图片内容提取器
type ImageExtractor struct {
	config config.ProcessingConfig
	logger *logger.Logger
}

// Extract 提取图片内容
func (ie *ImageExtractor) Extract(ctx context.Context, rawContent string, contentType models.ContentType) (*ExtractedContent, error) {
	// 这里应该实现OCR或图片分析
	// 目前实现基础版本，返回图片的基本信息
	
	result := &ExtractedContent{
		Content:     fmt.Sprintf("Image content: %s", rawContent),
		Title:       "Image",
		Description: "Image content description",
		Type:        models.ContentTypeImage,
		Size:        int64(len(rawContent)),
		Language:    "unknown",
		Metadata: map[string]interface{}{
			"extraction_method": "basic",
			"image_data_size":   len(rawContent),
		},
	}

	ie.logger.Debug("Image extraction completed", logger.Fields{
		"content_length": len(rawContent),
	})

	return result, nil
}

// CanHandle 检查是否能处理图片类型
func (ie *ImageExtractor) CanHandle(contentType models.ContentType) bool {
	return contentType == models.ContentTypeImage
}

// GetSupportedTypes 获取支持的类型
func (ie *ImageExtractor) GetSupportedTypes() []models.ContentType {
	return []models.ContentType{models.ContentTypeImage}
}

// Close 关闭图片提取器
func (ie *ImageExtractor) Close() error {
	ie.logger.Debug("Image extractor closed")
	return nil
}

// NewExtractor 创建内容提取器（向后兼容的工厂函数）
func NewExtractor() (*ExtractorManager, error) {
	return NewExtractorManager()
}