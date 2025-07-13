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
	ProcessingTime  time.Duration       `json:"processing_time"`
	Error           string              `json:"error,omitempty"`
	CompletedAt     time.Time           `json:"completed_at"`
}

// Processor 中央内容处理器
type Processor struct {
	config     config.ProcessingConfig
	llmClient  *llm.Client
	summarizer *llm.Summarizer
	tagger     *llm.Tagger
	// TODO: Enable when extractor.go is implemented
	// extractor   *Extractor
	// classifier  *Classifier
	logger *logger.Logger

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

	// TODO: Enable when extractor.go is implemented
	// 初始化内容提取器
	// extractor, err := NewExtractor()
	// if err != nil {
	//	processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create extractor")
	//	return nil, err
	// }

	// TODO: Enable when classifier.go is implemented
	// 初始化内容分类器
	// classifier, err := NewClassifier()
	// if err != nil {
	//	processorLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to create classifier")
	//	return nil, err
	// }

	processor := &Processor{
		config:     cfg.Processing,
		llmClient:  llmClient,
		summarizer: summarizer,
		tagger:     tagger,
		// TODO: Enable when extractor.go and classifier.go are implemented
		// extractor:      extractor,
		// classifier:     classifier,
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

	// 1. 内容提取和清理 - TODO: Enable when extractor.go is implemented
	// extractedContent, err := p.extractor.ExtractContent(ctx, request.Content, request.ContentType)
	// if err != nil {
	//	return nil, err
	// }
	// 临时使用原始内容
	extractedContent := request.Content

	// 2. 创建内容项
	contentItem := models.NewContentItem(request.ContentType, extractedContent, request.UserID)
	if contentItem == nil {
		return nil, errors.NewMemoroError(errors.ErrorTypeBusiness, errors.ErrCodeValidationFailed, "Failed to create content item")
	}

	// 3. 内容分类和重要性评分 - TODO: Enable when classifier.go is implemented
	if request.Options.EnableClassification || request.Options.EnableImportanceScore {
		// classificationResult, err := p.classifier.ClassifyContent(ctx, extractedContent, request.ContentType)
		// if err != nil {
		//	return nil, err
		// }
		// 临时设置：分类功能将在classifier.go实现后启用
		if request.Options.EnableImportanceScore {
			contentItem.ImportanceScore = 0.5 // 默认重要性分数
		}

		if request.Options.EnableImportanceScore {
			// result.ImportanceScore = classificationResult.ImportanceScore
			// contentItem.SetImportanceScore(classificationResult.ImportanceScore)
			// 临时使用默认值
			result.ImportanceScore = 0.5
			contentItem.ImportanceScore = 0.5
		}
	}

	// 4. 生成摘要
	if request.Options.EnableSummary {
		summaryRequest := llm.SummaryRequest{
			Content:     extractedContent,
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
			Content:      extractedContent,
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
	if !options.EnableSummary && !options.EnableTags && !options.EnableClassification && !options.EnableImportanceScore {
		options.EnableSummary = true
		options.EnableTags = true
		options.EnableClassification = true
		options.EnableImportanceScore = true
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
	// TODO: Enable when extractor.go and classifier.go are implemented
	// if p.extractor != nil {
	//	p.extractor.Close()
	// }
	// if p.classifier != nil {
	//	p.classifier.Close()
	// }

	p.logger.Info("Content processor shut down completed")
	return nil
}
