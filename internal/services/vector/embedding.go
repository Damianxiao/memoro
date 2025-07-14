package vector

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
)

// EmbeddingService 向量化服务
type EmbeddingService struct {
	httpClient *resty.Client
	config     config.LLMConfig
	logger     *logger.Logger
}

// EmbeddingRequest 向量化请求
type EmbeddingRequest struct {
	Text        string                 `json:"text"`                  // 要向量化的文本
	ContentType models.ContentType     `json:"content_type"`          // 内容类型
	MaxTokens   int                    `json:"max_tokens,omitempty"`  // 最大token数量
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // 额外元数据
}

// EmbeddingResponse LLM API的embedding响应
type EmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
}

// EmbeddingResult 向量化结果
type EmbeddingResult struct {
	Vector       []float32     `json:"vector"`         // 生成的向量
	Dimension    int           `json:"dimension"`      // 向量维度
	TokensUsed   int           `json:"tokens_used"`    // 使用的token数量
	ProcessTime  time.Duration `json:"process_time"`   // 处理时间
	Model        string        `json:"model"`          // 使用的模型
	TextLength   int           `json:"text_length"`    // 原文长度
}

// BatchEmbeddingRequest 批量向量化请求
type BatchEmbeddingRequest struct {
	Texts       []string               `json:"texts"`                 // 要向量化的文本列表
	ContentType models.ContentType     `json:"content_type"`          // 内容类型
	MaxTokens   int                    `json:"max_tokens,omitempty"`  // 最大token数量
	Metadata    map[string]interface{} `json:"metadata,omitempty"`    // 额外元数据
}

// BatchEmbeddingResult 批量向量化结果
type BatchEmbeddingResult struct {
	Results      []*EmbeddingResult `json:"results"`       // 向量化结果列表
	TotalTokens  int                `json:"total_tokens"`  // 总token使用量
	ProcessTime  time.Duration      `json:"process_time"`  // 总处理时间
	SuccessCount int                `json:"success_count"` // 成功处理数量
	FailureCount int                `json:"failure_count"` // 失败处理数量
}

// NewEmbeddingService 创建向量化服务
func NewEmbeddingService() (*EmbeddingService, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("embedding service config")
	}

	embeddingLogger := logger.NewLogger("embedding-service")

	// 创建HTTP客户端用于embedding API调用
	httpClient := resty.New()
	httpClient.SetBaseURL(cfg.LLM.APIBase)
	httpClient.SetTimeout(cfg.LLM.Timeout)
	httpClient.SetHeader("Content-Type", "application/json")

	// 设置API密钥
	if cfg.LLM.APIKey != "" {
		httpClient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", cfg.LLM.APIKey))
	} else {
		embeddingLogger.Warn("LLM API key is not set - embedding requests may fail")
	}

	// 设置重试策略
	httpClient.SetRetryCount(cfg.LLM.RetryTimes)
	httpClient.SetRetryWaitTime(cfg.LLM.RetryDelay)

	service := &EmbeddingService{
		httpClient: httpClient,
		config:     cfg.LLM,
		logger:     embeddingLogger,
	}

	embeddingLogger.Info("Embedding service initialized", logger.Fields{
		"model":       cfg.LLM.Model,
		"api_base":    cfg.LLM.APIBase,
		"max_tokens":  cfg.LLM.MaxTokens,
	})

	return service, nil
}

// GenerateEmbedding 生成单个文本的向量
func (es *EmbeddingService) GenerateEmbedding(ctx context.Context, req *EmbeddingRequest) (*EmbeddingResult, error) {
	if req == nil {
		return nil, errors.ErrValidationFailed("request", "cannot be nil")
	}

	if strings.TrimSpace(req.Text) == "" {
		return nil, errors.ErrValidationFailed("text", "cannot be empty")
	}

	startTime := time.Now()

	es.logger.Debug("Generating embedding", logger.Fields{
		"text_length":  len(req.Text),
		"content_type": string(req.ContentType),
		"max_tokens":   req.MaxTokens,
	})

	// 预处理文本
	processedText := es.preprocessText(req.Text, req.ContentType)

	// 限制文本长度
	if req.MaxTokens > 0 {
		processedText = es.truncateText(processedText, req.MaxTokens)
	}

	// 调用LLM API生成embedding
	embedding, tokensUsed, err := es.callEmbeddingAPI(ctx, processedText)
	if err != nil {
		return nil, err
	}

	processTime := time.Since(startTime)

	result := &EmbeddingResult{
		Vector:      embedding,
		Dimension:   len(embedding),
		TokensUsed:  tokensUsed,
		ProcessTime: processTime,
		Model:       es.config.Model,
		TextLength:  len(req.Text),
	}

	es.logger.Debug("Embedding generated successfully", logger.Fields{
		"dimension":    result.Dimension,
		"tokens_used":  result.TokensUsed,
		"process_time": result.ProcessTime,
		"text_length":  result.TextLength,
	})

	return result, nil
}

// GenerateBatchEmbeddings 批量生成向量
func (es *EmbeddingService) GenerateBatchEmbeddings(ctx context.Context, req *BatchEmbeddingRequest) (*BatchEmbeddingResult, error) {
	if req == nil {
		return nil, errors.ErrValidationFailed("request", "cannot be nil")
	}

	if len(req.Texts) == 0 {
		return nil, errors.ErrValidationFailed("texts", "cannot be empty")
	}

	startTime := time.Now()

	es.logger.Info("Generating batch embeddings", logger.Fields{
		"batch_size":   len(req.Texts),
		"content_type": string(req.ContentType),
		"max_tokens":   req.MaxTokens,
	})

	results := make([]*EmbeddingResult, 0, len(req.Texts))
	totalTokens := 0
	successCount := 0
	failureCount := 0

	// 批量处理文本
	for i, text := range req.Texts {
		if strings.TrimSpace(text) == "" {
			es.logger.Warn("Skipping empty text", logger.Fields{
				"index": i,
			})
			failureCount++
			continue
		}

		// 创建单个请求
		singleReq := &EmbeddingRequest{
			Text:        text,
			ContentType: req.ContentType,
			MaxTokens:   req.MaxTokens,
			Metadata:    req.Metadata,
		}

		// 生成单个embedding
		result, err := es.GenerateEmbedding(ctx, singleReq)
		if err != nil {
			es.logger.Error("Failed to generate embedding for text", logger.Fields{
				"index": i,
				"error": err.Error(),
			})
			failureCount++
			continue
		}

		results = append(results, result)
		totalTokens += result.TokensUsed
		successCount++
	}

	processTime := time.Since(startTime)

	batchResult := &BatchEmbeddingResult{
		Results:      results,
		TotalTokens:  totalTokens,
		ProcessTime:  processTime,
		SuccessCount: successCount,
		FailureCount: failureCount,
	}

	es.logger.Info("Batch embeddings generated", logger.Fields{
		"success_count": successCount,
		"failure_count": failureCount,
		"total_tokens":  totalTokens,
		"process_time":  processTime,
	})

	return batchResult, nil
}

// preprocessText 预处理文本
func (es *EmbeddingService) preprocessText(text string, contentType models.ContentType) string {
	// 移除多余的空白字符
	processed := strings.TrimSpace(text)
	
	// 替换多个连续空格为单个空格
	processed = strings.ReplaceAll(processed, "\n", " ")
	processed = strings.ReplaceAll(processed, "\t", " ")
	
	// 移除重复空格
	for strings.Contains(processed, "  ") {
		processed = strings.ReplaceAll(processed, "  ", " ")
	}

	// 根据内容类型进行特殊处理
	switch contentType {
	case models.ContentTypeLink:
		// 对于链接内容，添加前缀以改善embedding质量
		processed = "Web content: " + processed
	case models.ContentTypeFile:
		// 对于文件内容，添加前缀
		processed = "Document content: " + processed
	case models.ContentTypeImage:
		// 对于图片内容（OCR结果），添加前缀
		processed = "Image text: " + processed
	}

	return processed
}

// truncateText 截断文本到指定token数量
func (es *EmbeddingService) truncateText(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return text
	}

	// 简单的token估算：平均每个token约4个字符
	estimatedTokens := len(text) / 4
	if estimatedTokens <= maxTokens {
		return text
	}

	// 截断文本
	maxChars := maxTokens * 4
	if len(text) > maxChars {
		return text[:maxChars] + "..."
	}

	return text
}

// callEmbeddingAPI 调用LLM API生成embedding
func (es *EmbeddingService) callEmbeddingAPI(ctx context.Context, text string) ([]float32, int, error) {
	es.logger.Debug("Calling LLM API for embedding", logger.Fields{
		"text_length": len(text),
		"api_base":    es.config.APIBase,
		"model":       es.config.Model,
	})

	// 构建embedding请求
	requestBody := map[string]interface{}{
		"model": es.config.Model,
		"input": text,
	}

	requestJSON, err := json.Marshal(requestBody)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to marshal embedding request").
			WithCause(err)
		es.logger.LogMemoroError(memoErr, "Request marshaling failed")
		return nil, 0, memoErr
	}

	// 发送HTTP请求
	resp, err := es.httpClient.R().
		SetContext(ctx).
		SetHeader("Content-Type", "application/json").
		SetBody(requestJSON).
		SetResult(&EmbeddingResponse{}).
		Post("/embeddings")

	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Failed to call embedding API").
			WithCause(err).
			WithContext(map[string]interface{}{
				"text_length": len(text),
			})
		es.logger.LogMemoroError(memoErr, "Embedding API call failed")
		return nil, 0, memoErr
	}

	// 检查HTTP状态
	if resp.StatusCode() != 200 {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Embedding API returned error status").
			WithDetails(fmt.Sprintf("Status: %d, Body: %s", resp.StatusCode(), string(resp.Body()))).
			WithContext(map[string]interface{}{
				"status_code": resp.StatusCode(),
			})
		es.logger.LogMemoroError(memoErr, "Embedding API error response")
		return nil, 0, memoErr
	}

	// 解析响应
	result := resp.Result().(*EmbeddingResponse)
	if result == nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Failed to parse embedding API response").
			WithDetails("Response result is nil")
		es.logger.LogMemoroError(memoErr, "Embedding API response parsing failed")
		return nil, 0, memoErr
	}

	// 验证响应
	if len(result.Data) == 0 {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Embedding API returned no data")
		es.logger.LogMemoroError(memoErr, "Embedding API empty response")
		return nil, 0, memoErr
	}

	embedding := result.Data[0].Embedding
	tokensUsed := result.Usage.TotalTokens

	es.logger.Debug("Embedding API call successful", logger.Fields{
		"dimension":   len(embedding),
		"tokens_used": tokensUsed,
		"model":       result.Model,
	})

	return embedding, tokensUsed, nil
}

// GetHTTPClient 获取内部HTTP客户端
func (es *EmbeddingService) GetHTTPClient() *resty.Client {
	return es.httpClient
}

// CreateContentVector 为内容项创建向量文档
func (es *EmbeddingService) CreateContentVector(ctx context.Context, contentItem *models.ContentItem) (*VectorDocument, error) {
	if contentItem == nil {
		return nil, errors.ErrValidationFailed("content_item", "cannot be nil")
	}

	es.logger.Debug("Creating vector for content item", logger.Fields{
		"content_id":   contentItem.ID,
		"content_type": string(contentItem.Type),
		"content_size": len(contentItem.RawContent),
	})

	// 准备embedding请求
	embeddingReq := &EmbeddingRequest{
		Text:        contentItem.RawContent,
		ContentType: contentItem.Type,
		MaxTokens:   es.config.MaxTokens / 2, // 为embedding预留一半token
		Metadata: map[string]interface{}{
			"content_id": contentItem.ID,
			"user_id":    contentItem.UserID,
		},
	}

	// 生成embedding
	embeddingResult, err := es.GenerateEmbedding(ctx, embeddingReq)
	if err != nil {
		return nil, err
	}

	// 构建元数据
	metadata := map[string]interface{}{
		"content_id":       contentItem.ID,
		"content_type":     string(contentItem.Type),
		"user_id":          contentItem.UserID,
		"importance_score": contentItem.ImportanceScore,
		"tokens_used":      embeddingResult.TokensUsed,
		"vector_dimension": embeddingResult.Dimension,
		"model":            embeddingResult.Model,
	}

	// 添加标签信息
	if tags := contentItem.GetTags(); len(tags) > 0 {
		metadata["tags"] = tags
	}

	// 添加摘要信息
	summary := contentItem.GetSummary()
	if summary.OneLine != "" {
		metadata["summary_oneline"] = summary.OneLine
	}

	// 添加处理后的数据
	if processedData := contentItem.GetProcessedData(); len(processedData) > 0 {
		// 只添加重要的元数据，避免向量数据库元数据过大
		if categories, exists := processedData["categories"]; exists {
			metadata["categories"] = categories
		}
		if keywords, exists := processedData["keywords"]; exists {
			metadata["keywords"] = keywords
		}
	}

	// 创建向量文档
	vectorDoc := &VectorDocument{
		ID:        contentItem.ID,
		Content:   contentItem.RawContent,
		Embedding: embeddingResult.Vector,
		Metadata:  metadata,
		CreatedAt: contentItem.CreatedAt,
	}

	es.logger.Debug("Vector document created", logger.Fields{
		"content_id":       contentItem.ID,
		"vector_dimension": len(vectorDoc.Embedding),
		"metadata_keys":    getMetadataKeys(vectorDoc.Metadata),
	})

	return vectorDoc, nil
}

// Close 关闭embedding服务
func (es *EmbeddingService) Close() error {
	es.logger.Info("Closing embedding service")
	
	// HTTP客户端不需要显式关闭
	
	es.logger.Info("Embedding service closed")
	return nil
}