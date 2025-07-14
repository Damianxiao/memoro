package content

import (
	"context"
	"fmt"
	"sync"
	"time"

	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
	"memoro/internal/models"
	"memoro/internal/services/llm"
	"memoro/internal/services/vector"
)

// ProcessingStatus 处理状态枚举
type ProcessingStatus string

const (
	StatusPending    ProcessingStatus = "pending"    // 等待处理
	StatusProcessing ProcessingStatus = "processing" // 正在处理
	StatusCompleted  ProcessingStatus = "completed"  // 处理完成
	StatusFailed     ProcessingStatus = "failed"     // 处理失败
	StatusCancelled  ProcessingStatus = "cancelled"  // 已取消
)

// ProcessingRequest 内容处理请求
type ProcessingRequest struct {
	ID          string                 `json:"id"`
	Content     string                 `json:"content"`
	ContentType models.ContentType     `json:"content_type"`
	UserID      string                 `json:"user_id"`
	Priority    int                    `json:"priority"` // 优先级 (1-10, 10最高)
	Context     map[string]interface{} `json:"context"`  // 上下文信息
	Options     ProcessingOptions      `json:"options"`  // 处理选项
	CreatedAt   time.Time              `json:"created_at"`
}

// ProcessingOptions 处理选项
type ProcessingOptions struct {
	EnableSummary         bool     `json:"enable_summary"`          // 是否生成摘要
	EnableTags            bool     `json:"enable_tags"`             // 是否生成标签
	EnableClassification  bool     `json:"enable_classification"`   // 是否进行分类
	EnableImportanceScore bool     `json:"enable_importance_score"` // 是否计算重要性评分
	EnableVectorization   bool     `json:"enable_vectorization"`    // 是否启用向量化
	ExistingTags          []string `json:"existing_tags"`           // 现有标签
	MaxTags               int      `json:"max_tags"`                // 最大标签数
}

// ProcessingResult 处理结果
type ProcessingResult struct {
	RequestID       string              `json:"request_id"`
	Status          ProcessingStatus    `json:"status"`
	ContentItem     *models.ContentItem `json:"content_item"`
	Summary         *llm.SummaryResult  `json:"summary"`
	Tags            *llm.TagResult      `json:"tags"`
	ImportanceScore float64             `json:"importance_score"`
	VectorResult    *VectorResult       `json:"vector_result,omitempty"`    // 向量化结果
	ProcessingTime  time.Duration       `json:"processing_time"`
	Error           string              `json:"error,omitempty"`
	CompletedAt     time.Time           `json:"completed_at"`
}

// VectorResult 向量化结果
type VectorResult struct {
	DocumentID      string    `json:"document_id"`      // 向量文档ID
	VectorDimension int       `json:"vector_dimension"` // 向量维度
	Indexed         bool      `json:"indexed"`          // 是否已索引
	IndexedAt       time.Time `json:"indexed_at"`       // 索引时间
	Error           string    `json:"error,omitempty"`  // 向量化错误
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query         string                `json:"query"`                   // 查询文本
	UserID        string                `json:"user_id"`                 // 用户ID
	ContentTypes  []models.ContentType  `json:"content_types,omitempty"` // 内容类型过滤
	TopK          int                   `json:"top_k"`                   // 返回结果数量
	MinSimilarity float32               `json:"min_similarity"`          // 最小相似度
	TimeRange     *TimeRange            `json:"time_range,omitempty"`    // 时间范围
	Tags          []string              `json:"tags,omitempty"`          // 标签过滤
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Results      []*SearchResultItem `json:"results"`       // 搜索结果
	TotalResults int                 `json:"total_results"` // 总结果数
	QueryTime    time.Duration       `json:"query_time"`    // 查询耗时
}

// SearchResultItem 搜索结果项
type SearchResultItem struct {
	DocumentID      string                 `json:"document_id"`      // 文档ID
	Content         string                 `json:"content"`          // 文档内容
	Similarity      float64                `json:"similarity"`       // 相似度分数
	Rank            int                    `json:"rank"`             // 排名
	Metadata        map[string]interface{} `json:"metadata"`         // 文档元数据
	ContentSummary  string                 `json:"content_summary"`  // 内容摘要
	MatchedKeywords []string               `json:"matched_keywords"` // 匹配的关键词
	CreatedAt       time.Time              `json:"created_at"`       // 创建时间
}

// RecommendationRequest 推荐请求
type RecommendationRequest struct {
	Type                string                `json:"type"`                          // 推荐类型
	UserID              string                `json:"user_id"`                       // 用户ID
	SourceDocumentID    string                `json:"source_document_id,omitempty"` // 源文档ID
	SourceQuery         string                `json:"source_query,omitempty"`       // 源查询
	MaxRecommendations  int                   `json:"max_recommendations"`          // 最大推荐数量
	ContentTypes        []models.ContentType  `json:"content_types,omitempty"`      // 内容类型过滤
	ExcludeDocuments    []string              `json:"exclude_documents,omitempty"`  // 排除的文档ID
	MinSimilarity       float32               `json:"min_similarity"`               // 最小相似度
}

// RecommendationResponse 推荐响应
type RecommendationResponse struct {
	Recommendations []*RecommendationItem `json:"recommendations"`     // 推荐结果
	TotalFound      int                   `json:"total_found"`         // 总发现数量
	ProcessTime     time.Duration         `json:"process_time"`        // 处理时间
	Type            string                `json:"recommendation_type"` // 推荐类型
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
	RelatedKeywords   []string               `json:"related_keywords"`   // 相关关键词
	CreatedAt         time.Time              `json:"created_at"`         // 创建时间
}

// TimeRange 时间范围
type TimeRange struct {
	StartTime time.Time `json:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time"`   // 结束时间
}

// Processor 中央内容处理器
type Processor struct {
	config     config.ProcessingConfig
	llmClient  *llm.Client
	summarizer *llm.Summarizer
	tagger     *llm.Tagger
	extractor  *ExtractorManager
	classifier *ContentClassifier
	searchEngine *vector.SearchEngine  // 智能搜索引擎
	logger     *logger.Logger

	// 处理状态管理
	activeRequests map[string]*ProcessingRequest
	results        map[string]*ProcessingResult
	mu             sync.RWMutex

	// 控制通道
	requestChan chan *ProcessingRequest
	stopChan    chan struct{}
	workerWg    sync.WaitGroup
}

// NewProcessor 创建新的内容处理器
func NewProcessor() (*Processor, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("processing config")
	}

	processorLogger := logger.NewLogger("content-processor")

	// 初始化LLM客户端
	llmClient, err := llm.NewClient()
	if err != nil {
		processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create LLM client")
		return nil, err
	}

	// 初始化摘要生成器
	summarizer, err := llm.NewSummarizer(llmClient)
	if err != nil {
		processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create summarizer")
		return nil, err
	}

	// 初始化标签生成器
	tagger, err := llm.NewTagger(llmClient)
	if err != nil {
		processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create tagger")
		return nil, err
	}

	// 初始化内容提取器
	extractor, err := NewExtractorManager()
	if err != nil {
		processorLogger.Error("Failed to create extractor", logger.Fields{"error": err.Error()})
		return nil, err
	}

	// 初始化内容分类器
	classifier, err := NewClassifier()
	if err != nil {
		processorLogger.Error("Failed to create classifier", logger.Fields{"error": err.Error()})
		return nil, err
	}

	// 初始化向量搜索引擎
	searchEngine, err := vector.NewSearchEngine()
	if err != nil {
		processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create search engine")
		return nil, err
	}

	processor := &Processor{
		config:         cfg.Processing,
		llmClient:      llmClient,
		summarizer:     summarizer,
		tagger:         tagger,
		extractor:      extractor,
		classifier:     classifier,
		searchEngine:   searchEngine,
		logger:         processorLogger,
		activeRequests: make(map[string]*ProcessingRequest),
		results:        make(map[string]*ProcessingResult),
		requestChan:    make(chan *ProcessingRequest, cfg.Processing.QueueSize),
		stopChan:       make(chan struct{}),
	}

	// 启动工作协程
	processor.startWorkers()

	processorLogger.Info("Content processor initialized", logger.Fields{
		"max_workers": cfg.Processing.MaxWorkers,
		"queue_size":  cfg.Processing.QueueSize,
		"timeout":     cfg.Processing.Timeout,
	})

	return processor, nil
}

// ProcessContent 处理内容请求
func (p *Processor) ProcessContent(ctx context.Context, request *ProcessingRequest) (*ProcessingResult, error) {
	if request == nil {
		return nil, errors.ErrValidationFailed("request", "cannot be nil")
	}

	// 验证请求
	if err := p.validateRequest(request); err != nil {
		p.logger.LogMemoroError(err.(*errors.MemoroError), "Invalid processing request")
		return nil, err
	}

	// 设置默认值
	p.setDefaultOptions(request)

	p.logger.Debug("Processing content request", logger.Fields{
		"request_id":   request.ID,
		"content_type": string(request.ContentType),
		"content_size": len(request.Content),
		"user_id":      request.UserID,
		"priority":     request.Priority,
	})

	// 创建处理结果
	result := &ProcessingResult{
		RequestID: request.ID,
		Status:    StatusPending,
	}

	// 保存请求和初始结果
	p.mu.Lock()
	p.activeRequests[request.ID] = request
	p.results[request.ID] = result
	p.mu.Unlock()

	// 提交到处理队列
	select {
	case p.requestChan <- request:
		p.logger.Debug("Request queued for processing", logger.Fields{
			"request_id": request.ID,
		})
	case <-ctx.Done():
		p.removeRequest(request.ID)
		return nil, errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeSystemGeneric, "Request cancelled").
			WithCause(ctx.Err())
	default:
		p.removeRequest(request.ID)
		return nil, errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeSystemGeneric, "Processing queue full")
	}

	// 等待处理完成
	return p.waitForResult(ctx, request.ID)
}

// ProcessContentAsync 异步处理内容
func (p *Processor) ProcessContentAsync(request *ProcessingRequest) error {
	if request == nil {
		return errors.ErrValidationFailed("request", "cannot be nil")
	}

	// 验证请求
	if err := p.validateRequest(request); err != nil {
		return err
	}

	// 设置默认值
	p.setDefaultOptions(request)

	// 创建处理结果
	result := &ProcessingResult{
		RequestID: request.ID,
		Status:    StatusPending,
	}

	// 保存请求和初始结果
	p.mu.Lock()
	p.activeRequests[request.ID] = request
	p.results[request.ID] = result
	p.mu.Unlock()

	// 提交到处理队列
	select {
	case p.requestChan <- request:
		p.logger.Debug("Request queued for async processing", logger.Fields{
			"request_id": request.ID,
		})
		return nil
	default:
		p.removeRequest(request.ID)
		return errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeSystemGeneric, "Processing queue full")
	}
}

// GetResult 获取处理结果
func (p *Processor) GetResult(requestID string) (*ProcessingResult, error) {
	p.mu.RLock()
	result, exists := p.results[requestID]
	p.mu.RUnlock()

	if !exists {
		return nil, errors.ErrResourceNotFound("processing_result", requestID)
	}

	// 返回结果的副本
	resultCopy := *result
	return &resultCopy, nil
}

// GetStatus 获取处理状态
func (p *Processor) GetStatus(requestID string) (ProcessingStatus, error) {
	p.mu.RLock()
	result, exists := p.results[requestID]
	p.mu.RUnlock()

	if !exists {
		return "", errors.ErrResourceNotFound("processing_request", requestID)
	}

	return result.Status, nil
}

// CancelRequest 取消处理请求
func (p *Processor) CancelRequest(requestID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	result, exists := p.results[requestID]
	if !exists {
		return errors.ErrResourceNotFound("processing_request", requestID)
	}

	if result.Status == StatusCompleted || result.Status == StatusFailed {
		return errors.ErrValidationFailed("status", "request already completed")
	}

	result.Status = StatusCancelled
	result.CompletedAt = time.Now()

	p.logger.Debug("Processing request cancelled", logger.Fields{
		"request_id": requestID,
	})

	return nil
}

// startWorkers 启动工作协程
func (p *Processor) startWorkers() {
	for i := 0; i < p.config.MaxWorkers; i++ {
		p.workerWg.Add(1)
		go p.worker(i)
	}
}

// worker 工作协程
func (p *Processor) worker(workerID int) {
	defer p.workerWg.Done()

	workerLogger := logger.NewLogger(fmt.Sprintf("content-worker-%d", workerID))
	workerLogger.Debug("Content worker started")

	for {
		select {
		case request := <-p.requestChan:
			p.processRequest(workerLogger, request)
		case <-p.stopChan:
			workerLogger.Debug("Content worker stopping")
			return
		}
	}
}

// processRequest 处理具体请求
func (p *Processor) processRequest(workerLogger *logger.Logger, request *ProcessingRequest) {
	startTime := time.Now()

	// 更新状态为处理中
	p.updateRequestStatus(request.ID, StatusProcessing)

	workerLogger.Debug("Processing request", logger.Fields{
		"request_id":   request.ID,
		"content_type": string(request.ContentType),
		"user_id":      request.UserID,
	})

	// 创建处理上下文
	ctx, cancel := context.WithTimeout(context.Background(), p.config.Timeout)
	defer cancel()

	// 执行实际处理
	result, err := p.doProcessing(ctx, request)
	if err != nil {
		workerLogger.LogMemoroError(err.(*errors.MemoroError), "Processing failed")
		p.updateRequestResult(request.ID, &ProcessingResult{
			RequestID:      request.ID,
			Status:         StatusFailed,
			Error:          err.Error(),
			ProcessingTime: time.Since(startTime),
			CompletedAt:    time.Now(),
		})
		return
	}

	// 设置处理时间和完成时间
	result.ProcessingTime = time.Since(startTime)
	result.CompletedAt = time.Now()
	result.Status = StatusCompleted

	p.updateRequestResult(request.ID, result)

	workerLogger.Debug("Processing completed", logger.Fields{
		"request_id":       request.ID,
		"processing_time":  result.ProcessingTime,
		"has_summary":      result.Summary != nil,
		"has_tags":         result.Tags != nil,
		"importance_score": result.ImportanceScore,
	})
}

// doProcessing 执行实际的内容处理
func (p *Processor) doProcessing(ctx context.Context, request *ProcessingRequest) (*ProcessingResult, error) {
	result := &ProcessingResult{
		RequestID: request.ID,
	}

	// 1. 内容提取和清理
	extractedContent, err := p.extractor.Extract(ctx, request.Content, request.ContentType)
	if err != nil {
		p.logger.Error("Content extraction failed", logger.Fields{
			"request_id": request.ID,
			"error":      err.Error(),
		})
		return nil, err
	}

	// 2. 创建内容项
	contentItem := models.NewContentItem(request.ContentType, extractedContent.Content, request.UserID)
	if contentItem == nil {
		return nil, errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeValidationFailed, "Failed to create content item")
	}

	// 设置提取的元数据
	processedData := contentItem.GetProcessedData()
	if extractedContent.Title != "" {
		processedData["title"] = extractedContent.Title
	}
	if extractedContent.Description != "" {
		processedData["description"] = extractedContent.Description
	}
	if len(extractedContent.Metadata) > 0 {
		processedData["extraction_metadata"] = extractedContent.Metadata
	}
	contentItem.SetProcessedData(processedData)

	// 3. 内容分类和重要性评分
	if request.Options.EnableClassification || request.Options.EnableImportanceScore {
		classificationResult, err := p.classifier.Classify(ctx, extractedContent)
		if err != nil {
			p.logger.Error("Content classification failed", logger.Fields{
				"request_id": request.ID,
				"error":      err.Error(),
			})
			// 不中断处理，使用默认值
			if request.Options.EnableImportanceScore {
				contentItem.ImportanceScore = 0.5
				result.ImportanceScore = 0.5
			}
		} else {
			// 应用分类结果
			if request.Options.EnableClassification {
				// 将分类信息存储到ProcessedData中
				processedData := contentItem.GetProcessedData()
				processedData["categories"] = classificationResult.Categories
				processedData["keywords"] = classificationResult.Keywords
				processedData["classification_confidence"] = classificationResult.Confidence
				processedData["classification_metadata"] = classificationResult.Metadata
				contentItem.SetProcessedData(processedData)
			}

			if request.Options.EnableImportanceScore {
				result.ImportanceScore = classificationResult.ImportanceScore
				contentItem.ImportanceScore = classificationResult.ImportanceScore
			}
		}
	}

	// 4. 生成摘要
	if request.Options.EnableSummary {
		summaryRequest := llm.SummaryRequest{
			Content:     extractedContent.Content,
			ContentType: request.ContentType,
			Context:     request.Context,
		}

		summary, err := p.summarizer.GenerateSummary(ctx, summaryRequest)
		if err != nil {
			return nil, err
		}

		result.Summary = summary

		// 设置内容项的摘要
		modelSummary := models.Summary{
			OneLine:   summary.OneLine,
			Paragraph: summary.Paragraph,
			Detailed:  summary.Detailed,
		}
		contentItem.SetSummary(modelSummary)
	}

	// 5. 生成标签
	if request.Options.EnableTags {
		tagRequest := llm.TagRequest{
			Content:      extractedContent.Content,
			ContentType:  request.ContentType,
			Context:      request.Context,
			ExistingTags: request.Options.ExistingTags,
			MaxTags:      request.Options.MaxTags,
		}

		tags, err := p.tagger.GenerateTags(ctx, tagRequest)
		if err != nil {
			return nil, err
		}

		result.Tags = tags

		// 设置内容项的标签
		contentItem.SetTags(tags.Tags)
	}

	// 6. 向量化和索引
	if request.Options.EnableVectorization {
		vectorResult := &VectorResult{
			DocumentID: contentItem.ID,
		}

		err := p.searchEngine.IndexDocument(ctx, contentItem)
		if err != nil {
			p.logger.Error("Content vectorization failed", logger.Fields{
				"request_id":  request.ID,
				"content_id":  contentItem.ID,
				"error":       err.Error(),
			})
			// 向量化失败不中断处理，记录错误
			vectorResult.Error = err.Error()
			vectorResult.Indexed = false
		} else {
			vectorResult.Indexed = true
			vectorResult.IndexedAt = time.Now()
			
			p.logger.Debug("Content indexed successfully", logger.Fields{
				"request_id": request.ID,
				"content_id": contentItem.ID,
			})
		}

		// 获取向量维度信息（如果索引成功）
		if vectorResult.Indexed {
			// 这里可以从搜索引擎获取向量维度信息
			// 暂时设置一个默认值，后续可以从embedding service获取
			vectorResult.VectorDimension = 1536 // OpenAI embedding维度
		}

		result.VectorResult = vectorResult
	}

	result.ContentItem = contentItem
	return result, nil
}

// validateRequest 验证处理请求
func (p *Processor) validateRequest(request *ProcessingRequest) error {
	if request.ID == "" {
		return errors.ErrValidationFailed("id", "cannot be empty")
	}

	if request.Content == "" {
		return errors.ErrValidationFailed("content", "cannot be empty")
	}

	if len(request.Content) > 100000 { // 100KB限制
		return errors.ErrValidationFailed("content", "content too large (max 100KB)")
	}

	if request.UserID == "" {
		return errors.ErrValidationFailed("user_id", "cannot be empty")
	}

	if !models.IsValidContentType(request.ContentType) {
		return errors.ErrValidationFailed("content_type", fmt.Sprintf("invalid content type: %s", request.ContentType))
	}

	return nil
}

// setDefaultOptions 设置默认选项
func (p *Processor) setDefaultOptions(request *ProcessingRequest) {
	if request.Priority <= 0 {
		request.Priority = 5 // 默认优先级
	}

	if request.CreatedAt.IsZero() {
		request.CreatedAt = time.Now()
	}

	options := &request.Options

	// 如果没有明确设置，默认启用所有功能
	if !options.EnableSummary && !options.EnableTags && !options.EnableClassification && !options.EnableImportanceScore && !options.EnableVectorization {
		options.EnableSummary = true
		options.EnableTags = true
		options.EnableClassification = true
		options.EnableImportanceScore = true
		options.EnableVectorization = true
	}

	if options.MaxTags <= 0 {
		options.MaxTags = p.config.TagLimits.MaxTags
	}

	if options.MaxTags > p.config.TagLimits.MaxTags {
		options.MaxTags = p.config.TagLimits.MaxTags
	}
}

// updateRequestStatus 更新请求状态
func (p *Processor) updateRequestStatus(requestID string, status ProcessingStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if result, exists := p.results[requestID]; exists {
		result.Status = status
	}
}

// updateRequestResult 更新请求结果
func (p *Processor) updateRequestResult(requestID string, result *ProcessingResult) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.results[requestID] = result
}

// waitForResult 等待处理结果
func (p *Processor) waitForResult(ctx context.Context, requestID string) (*ProcessingResult, error) {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			p.CancelRequest(requestID)
			return nil, errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeSystemGeneric, "Request timeout").
				WithCause(ctx.Err())
		case <-ticker.C:
			result, err := p.GetResult(requestID)
			if err != nil {
				return nil, err
			}

			if result.Status == StatusCompleted || result.Status == StatusFailed || result.Status == StatusCancelled {
				return result, nil
			}
		}
	}
}

// removeRequest 移除请求
func (p *Processor) removeRequest(requestID string) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.activeRequests, requestID)
	delete(p.results, requestID)
}

// GetStats 获取处理器统计信息
func (p *Processor) GetStats() map[string]interface{} {
	p.mu.RLock()
	defer p.mu.RUnlock()

	stats := map[string]interface{}{
		"active_requests": len(p.activeRequests),
		"total_results":   len(p.results),
		"queue_size":      len(p.requestChan),
		"max_workers":     p.config.MaxWorkers,
		"max_queue_size":  p.config.QueueSize,
	}

	// 统计状态分布
	statusCounts := make(map[ProcessingStatus]int)
	for _, result := range p.results {
		statusCounts[result.Status]++
	}
	stats["status_distribution"] = statusCounts

	return stats
}

// Close 关闭处理器
func (p *Processor) Close() error {
	p.logger.Info("Shutting down content processor")

	// 停止接收新请求
	close(p.stopChan)

	// 等待所有工作协程完成
	p.workerWg.Wait()

	// 关闭依赖组件
	if p.llmClient != nil {
		p.llmClient.Close()
	}
	if p.summarizer != nil {
		p.summarizer.Close()
	}
	if p.tagger != nil {
		p.tagger.Close()
	}
	if p.extractor != nil {
		p.extractor.Close()
	}
	if p.classifier != nil {
		p.classifier.Close()
	}
	if p.searchEngine != nil {
		p.searchEngine.Close()
	}

	p.logger.Info("Content processor shut down completed")
	return nil
}

// SearchContent 搜索内容
func (p *Processor) SearchContent(ctx context.Context, request *SearchRequest) (*SearchResponse, error) {
	if request == nil {
		return nil, errors.ErrValidationFailed("search_request", "cannot be nil")
	}

	if request.Query == "" {
		return nil, errors.ErrValidationFailed("query", "cannot be empty")
	}

	if request.TopK <= 0 {
		request.TopK = 10 // 默认返回10个结果
	}

	p.logger.Debug("Searching content", logger.Fields{
		"query":          request.Query,
		"user_id":        request.UserID,
		"top_k":          request.TopK,
		"min_similarity": request.MinSimilarity,
	})

	startTime := time.Now()

	// 构建搜索选项
	searchOptions := &vector.SearchOptions{
		Query:               request.Query,
		ContentTypes:        request.ContentTypes,
		UserID:              request.UserID,
		TopK:                request.TopK,
		MinSimilarity:       request.MinSimilarity,
		IncludeContent:      true,
		SimilarityType:      vector.SimilarityTypeCosine,
		TimeRange:           (*vector.TimeRange)(request.TimeRange),
		Tags:                request.Tags,
		EnableReranking:     true,
		MaxResults:          request.TopK * 2, // 获取更多结果用于重排序
	}

	// 执行搜索
	searchResponse, err := p.searchEngine.Search(ctx, searchOptions)
	if err != nil {
		p.logger.Error("Search failed", logger.Fields{
			"query":   request.Query,
			"user_id": request.UserID,
			"error":   err.Error(),
		})
		return nil, err
	}

	// 转换结果格式
	results := make([]*SearchResultItem, len(searchResponse.Results))
	for i, item := range searchResponse.Results {
		results[i] = &SearchResultItem{
			DocumentID:      item.DocumentID,
			Content:         item.Content,
			Similarity:      item.Similarity,
			Rank:            item.Rank,
			Metadata:        item.Metadata,
			ContentSummary:  item.ContentSummary,
			MatchedKeywords: item.MatchedKeywords,
			CreatedAt:       item.CreatedAt,
		}
	}

	queryTime := time.Since(startTime)

	response := &SearchResponse{
		Results:      results,
		TotalResults: len(results),
		QueryTime:    queryTime,
	}

	p.logger.Debug("Search completed", logger.Fields{
		"query":          request.Query,
		"results_count":  len(results),
		"query_time":     queryTime,
	})

	return response, nil
}

// GetRecommendations 获取推荐内容
func (p *Processor) GetRecommendations(ctx context.Context, request *RecommendationRequest) (*RecommendationResponse, error) {
	if request == nil {
		return nil, errors.ErrValidationFailed("recommendation_request", "cannot be nil")
	}

	if request.Type == "" {
		request.Type = "similar" // 默认相似推荐
	}

	if request.MaxRecommendations <= 0 {
		request.MaxRecommendations = 5 // 默认返回5个推荐
	}

	p.logger.Debug("Getting recommendations", logger.Fields{
		"type":                request.Type,
		"user_id":             request.UserID,
		"source_document_id":  request.SourceDocumentID,
		"max_recommendations": request.MaxRecommendations,
	})

	startTime := time.Now()

	// 构建推荐请求
	recRequest := &vector.RecommendationRequest{
		Type:                vector.RecommendationType(request.Type),
		UserID:              request.UserID,
		SourceDocumentID:    request.SourceDocumentID,
		SourceQuery:         request.SourceQuery,
		MaxRecommendations:  request.MaxRecommendations,
		ContentTypes:        request.ContentTypes,
		ExcludeDocuments:    request.ExcludeDocuments,
		MinSimilarity:       request.MinSimilarity,
		DiversityEnabled:    true,
		IncludeExplanations: false, // 简化版本不包含解释
	}

	// 执行推荐
	recResponse, err := p.searchEngine.GetRecommendations(ctx, recRequest)
	if err != nil {
		p.logger.Error("Recommendation failed", logger.Fields{
			"type":    request.Type,
			"user_id": request.UserID,
			"error":   err.Error(),
		})
		return nil, err
	}

	// 转换结果格式
	recommendations := make([]*RecommendationItem, len(recResponse.Recommendations))
	for i, item := range recResponse.Recommendations {
		recommendations[i] = &RecommendationItem{
			DocumentID:          item.DocumentID,
			Content:             item.Content,
			Similarity:          item.Similarity,
			Confidence:          item.Confidence,
			Rank:                item.Rank,
			Metadata:            item.Metadata,
			RecommendationScore: item.RecommendationScore,
			RelatedKeywords:     item.RelatedKeywords,
			CreatedAt:           item.CreatedAt,
		}
	}

	processTime := time.Since(startTime)

	response := &RecommendationResponse{
		Recommendations: recommendations,
		TotalFound:      len(recommendations),
		ProcessTime:     processTime,
		Type:            request.Type,
	}

	p.logger.Debug("Recommendations generated", logger.Fields{
		"type":              request.Type,
		"recommendations":   len(recommendations),
		"process_time":      processTime,
	})

	return response, nil
}

// BatchIndexContent 批量索引内容
func (p *Processor) BatchIndexContent(ctx context.Context, contentItems []*models.ContentItem) error {
	if len(contentItems) == 0 {
		return errors.ErrValidationFailed("content_items", "cannot be empty")
	}

	p.logger.Info("Batch indexing content", logger.Fields{
		"batch_size": len(contentItems),
	})

	return p.searchEngine.BatchIndexDocuments(ctx, contentItems)
}

// DeleteFromIndex 从索引中删除内容
func (p *Processor) DeleteFromIndex(ctx context.Context, documentID string) error {
	if documentID == "" {
		return errors.ErrValidationFailed("document_id", "cannot be empty")
	}

	p.logger.Debug("Deleting document from index", logger.Fields{
		"document_id": documentID,
	})

	return p.searchEngine.DeleteDocument(ctx, documentID)
}

// GetVectorStats 获取向量数据库统计信息
func (p *Processor) GetVectorStats(ctx context.Context) (map[string]interface{}, error) {
	return p.searchEngine.GetSearchStats(ctx)
}
