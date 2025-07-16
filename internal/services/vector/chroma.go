package vector

import (
	"context"
	"fmt"
	"time"

	chroma "github.com/amikos-tech/chroma-go"
	"github.com/amikos-tech/chroma-go/types"
	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
)

// ChromaClient Chroma向量数据库客户端
type ChromaClient struct {
	client     *chroma.Client
	collection *chroma.Collection
	config     config.VectorDBConfig
	logger     *logger.Logger
}

// VectorDocument 向量文档结构
type VectorDocument struct {
	ID        string                 `json:"id"`         // 文档ID
	Content   string                 `json:"content"`    // 文档内容
	Embedding []float32              `json:"embedding"`  // 向量表示
	Metadata  map[string]interface{} `json:"metadata"`   // 元数据
	Distance  float32                `json:"distance"`   // 相似度距离（查询时使用）
	CreatedAt time.Time              `json:"created_at"` // 创建时间
}

// SearchQuery 搜索查询结构
type SearchQuery struct {
	QueryText     string                 `json:"query_text"`             // 查询文本
	QueryVector   []float32              `json:"query_vector,omitempty"` // 查询向量（可选）
	TopK          int                    `json:"top_k"`                  // 返回结果数量
	Filter        map[string]interface{} `json:"filter,omitempty"`       // 过滤条件
	IncludeText   bool                   `json:"include_text"`           // 是否包含原文
	MinSimilarity float32                `json:"min_similarity"`         // 最小相似度阈值
}

// SearchResult 搜索结果
type SearchResult struct {
	Documents    []*VectorDocument `json:"documents"`     // 匹配的文档
	QueryTime    time.Duration     `json:"query_time"`    // 查询耗时
	TotalResults int               `json:"total_results"` // 总结果数
}

// NewChromaClient 创建新的Chroma客户端
func NewChromaClient() (*ChromaClient, error) {
	cfg := config.Get()
	if cfg == nil {
		return nil, errors.ErrConfigMissing("vector database config")
	}

	chromaLogger := logger.NewLogger("chroma-client")

	// 构建Chroma服务器URL
	serverURL := fmt.Sprintf("http://%s:%d", cfg.VectorDB.Host, cfg.VectorDB.Port)

	// 创建Chroma客户端
	client, err := chroma.NewClient(chroma.WithBasePath(serverURL))
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create Chroma client").
			WithCause(err).
			WithContext(map[string]interface{}{
				"server_url": serverURL,
			})
		chromaLogger.LogMemoroError(memoErr, "Chroma client creation failed")
		return nil, memoErr
	}

	chromaClient := &ChromaClient{
		client: client,
		config: cfg.VectorDB,
		logger: chromaLogger,
	}

	// 初始化集合
	if err := chromaClient.initializeCollection(); err != nil {
		return nil, err
	}

	chromaLogger.Info("Chroma client initialized", logger.Fields{
		"server_url":  serverURL,
		"collection":  cfg.VectorDB.Collection,
		"batch_size":  cfg.VectorDB.BatchSize,
		"retry_times": cfg.VectorDB.RetryTimes,
		"timeout":     cfg.VectorDB.Timeout,
	})

	return chromaClient, nil
}

// initializeCollection 初始化或获取集合
func (cc *ChromaClient) initializeCollection() error {
	ctx, cancel := context.WithTimeout(context.Background(), cc.config.Timeout)
	defer cancel()

	// 尝试获取现有集合
	collection, err := cc.client.GetCollection(ctx, cc.config.Collection, nil)
	if err != nil {
		// 如果集合不存在，创建新集合
		cc.logger.Info("Collection not found, creating new collection", logger.Fields{
			"collection": cc.config.Collection,
		})

		metadata := map[string]interface{}{
			"description": "Memoro content vectors",
			"created_at":  time.Now().Unix(),
		}

		collection, err = cc.client.CreateCollection(ctx, cc.config.Collection, metadata, true, nil, types.L2)
		if err != nil {
			memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create Chroma collection").
				WithCause(err).
				WithContext(map[string]interface{}{
					"collection": cc.config.Collection,
				})
			cc.logger.LogMemoroError(memoErr, "Collection creation failed")
			return memoErr
		}

		cc.logger.Info("Created new Chroma collection", logger.Fields{
			"collection": cc.config.Collection,
		})
	} else {
		cc.logger.Info("Using existing Chroma collection", logger.Fields{
			"collection": cc.config.Collection,
		})
	}

	cc.collection = collection
	return nil
}

// AddDocument 添加文档到向量数据库
func (cc *ChromaClient) AddDocument(ctx context.Context, doc *VectorDocument) error {
	if doc == nil {
		return errors.ErrValidationFailed("document", "cannot be nil")
	}

	if doc.ID == "" {
		return errors.ErrValidationFailed("document.id", "cannot be empty")
	}

	if len(doc.Embedding) == 0 {
		return errors.ErrValidationFailed("document.embedding", "cannot be empty")
	}

	cc.logger.Debug("Adding document to vector database", logger.Fields{
		"document_id":    doc.ID,
		"content_length": len(doc.Content),
		"embedding_size": len(doc.Embedding),
		"metadata_keys":  getMetadataKeys(doc.Metadata),
	})

	// 准备数据
	ids := []string{doc.ID}
	// Convert float32 slice to interface slice for Chroma API
	embeddingData := make([]interface{}, len(doc.Embedding))
	for i, v := range doc.Embedding {
		embeddingData[i] = v
	}
	embedding, err := types.NewEmbedding(embeddingData)
	if err != nil {
		return errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create embedding").
			WithCause(err).
			WithContext(map[string]interface{}{
				"document_id": doc.ID,
			})
	}
	embeddings := []*types.Embedding{embedding}
	documents := []string{doc.Content}
	metadatas := []map[string]interface{}{doc.Metadata}

	// 添加创建时间到元数据
	if doc.Metadata == nil {
		doc.Metadata = make(map[string]interface{})
	}
	doc.Metadata["created_at"] = doc.CreatedAt.Unix()
	doc.Metadata["content_length"] = len(doc.Content)

	// 添加到集合
	_, err = cc.collection.Add(ctx, embeddings, metadatas, documents, ids)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to add document to Chroma").
			WithCause(err).
			WithContext(map[string]interface{}{
				"document_id": doc.ID,
				"collection":  cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Document addition failed")
		return memoErr
	}

	cc.logger.Debug("Document added successfully", logger.Fields{
		"document_id": doc.ID,
	})

	return nil
}

// AddDocuments 批量添加文档
func (cc *ChromaClient) AddDocuments(ctx context.Context, docs []*VectorDocument) error {
	if len(docs) == 0 {
		return errors.ErrValidationFailed("documents", "cannot be empty")
	}

	cc.logger.Info("Adding documents batch to vector database", logger.Fields{
		"batch_size": len(docs),
	})

	// 准备批量数据
	ids := make([]string, len(docs))
	embeddings := make([]*types.Embedding, len(docs))
	documents := make([]string, len(docs))
	metadatas := make([]map[string]interface{}, len(docs))

	for i, doc := range docs {
		if doc.ID == "" {
			return errors.ErrValidationFailed("document.id", fmt.Sprintf("document at index %d has empty ID", i))
		}
		if len(doc.Embedding) == 0 {
			return errors.ErrValidationFailed("document.embedding", fmt.Sprintf("document at index %d has empty embedding", i))
		}

		ids[i] = doc.ID
		// Convert float32 slice to interface slice for Chroma API
		embeddingData := make([]interface{}, len(doc.Embedding))
		for j, v := range doc.Embedding {
			embeddingData[j] = v
		}
		embedding, err := types.NewEmbedding(embeddingData)
		if err != nil {
			return errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create embedding").
				WithCause(err).
				WithContext(map[string]interface{}{
					"document_index": i,
					"document_id":    doc.ID,
				})
		}
		embeddings[i] = embedding
		documents[i] = doc.Content
		metadatas[i] = doc.Metadata

		// 添加创建时间到元数据
		if metadatas[i] == nil {
			metadatas[i] = make(map[string]interface{})
		}
		metadatas[i]["created_at"] = doc.CreatedAt.Unix()
		metadatas[i]["content_length"] = len(doc.Content)
	}

	// 批量添加到集合
	_, err := cc.collection.Add(ctx, embeddings, metadatas, documents, ids)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to add documents batch to Chroma").
			WithCause(err).
			WithContext(map[string]interface{}{
				"batch_size": len(docs),
				"collection": cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Batch addition failed")
		return memoErr
	}

	cc.logger.Info("Documents batch added successfully", logger.Fields{
		"batch_size": len(docs),
	})

	return nil
}

// Search 执行相似度搜索
func (cc *ChromaClient) Search(ctx context.Context, query *SearchQuery) (*SearchResult, error) {
	if query == nil {
		return nil, errors.ErrValidationFailed("query", "cannot be nil")
	}

	if len(query.QueryVector) == 0 && query.QueryText == "" {
		return nil, errors.ErrValidationFailed("query", "either query_vector or query_text must be provided")
	}

	if query.TopK <= 0 {
		query.TopK = 10 // 默认返回10个结果
	}

	startTime := time.Now()

	cc.logger.Debug("Executing vector similarity search", logger.Fields{
		"query_text":      query.QueryText,
		"has_vector":      len(query.QueryVector) > 0,
		"vector_length":   len(query.QueryVector),
		"top_k":           query.TopK,
		"min_similarity":  query.MinSimilarity,
		"has_filter":      len(query.Filter) > 0,
	})

	var queryEmbedding *types.Embedding
	if len(query.QueryVector) > 0 {
		// Convert float32 slice to interface slice for Chroma API
		embeddingData := make([]interface{}, len(query.QueryVector))
		for i, v := range query.QueryVector {
			embeddingData[i] = v
		}
		embedding, err := types.NewEmbedding(embeddingData)
		if err != nil {
			return nil, errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create query embedding").
				WithCause(err)
		}
		queryEmbedding = embedding
	}

	// 构建查询选项
	var queryOptions []types.CollectionQueryOption

	if queryEmbedding != nil {
		// 向量查询
		queryOptions = append(queryOptions,
			types.WithQueryEmbedding(queryEmbedding),
			types.WithNResults(int32(query.TopK)),
			types.WithInclude(types.IDocuments, types.IEmbeddings, types.IMetadatas, types.IDistances),
		)
	} else {
		// 如果没有向量，返回错误，因为Chroma v0.4.24不支持纯文本搜索
		return nil, errors.ErrValidationFailed("query", "query vector is required for search")
	}

	// 添加过滤条件
	if len(query.Filter) > 0 {
		queryOptions = append(queryOptions, types.WithWhereMap(query.Filter))
	}

	// 执行查询
	queryResult, err := cc.collection.QueryWithOptions(ctx, queryOptions...)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to execute Chroma query").
			WithCause(err).
			WithContext(map[string]interface{}{
				"query_text": query.QueryText,
				"top_k":      query.TopK,
				"collection": cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Vector search failed")
		return nil, memoErr
	}

	// 处理查询结果
	documents := make([]*VectorDocument, 0)

	if queryResult != nil && len(queryResult.Ids) > 0 {
		for i := 0; i < len(queryResult.Ids[0]); i++ {
			// 计算相似度（距离转换为相似度）
			distance := float32(0.0)
			if len(queryResult.Distances) > 0 && len(queryResult.Distances[0]) > i {
				distance = float32(queryResult.Distances[0][i])
			}

			similarity := 1.0 - distance // L2距离转换为相似度

			// 应用最小相似度过滤
			if similarity < query.MinSimilarity {
				continue
			}

			doc := &VectorDocument{
				Distance: distance,
			}

			// 设置文档ID
			if len(queryResult.Ids[0]) > i {
				doc.ID = queryResult.Ids[0][i]
			}

			// 设置文档内容
			if query.IncludeText && len(queryResult.Documents) > 0 && len(queryResult.Documents[0]) > i {
				doc.Content = queryResult.Documents[0][i]
			}

			// 设置向量 - QueryResults doesn't include embeddings in v0.2.3
			// We'll leave embedding empty for search results as they're not returned by default

			// 设置元数据
			if len(queryResult.Metadatas) > 0 && len(queryResult.Metadatas[0]) > i {
				doc.Metadata = queryResult.Metadatas[0][i]

				// 从元数据中恢复创建时间
				if createdAtVal, exists := doc.Metadata["created_at"]; exists {
					if createdAtFloat, ok := createdAtVal.(float64); ok {
						doc.CreatedAt = time.Unix(int64(createdAtFloat), 0)
					} else if createdAtInt, ok := createdAtVal.(int64); ok {
						doc.CreatedAt = time.Unix(createdAtInt, 0)
					}
				}
			}

			documents = append(documents, doc)
		}
	}

	queryTime := time.Since(startTime)
	result := &SearchResult{
		Documents:    documents,
		QueryTime:    queryTime,
		TotalResults: len(documents),
	}

	cc.logger.Debug("Vector search completed", logger.Fields{
		"query_time":       queryTime,
		"total_results":    len(documents),
		"filtered_results": result.TotalResults,
	})

	return result, nil
}

// GetDocument 根据ID获取文档
func (cc *ChromaClient) GetDocument(ctx context.Context, id string) (*VectorDocument, error) {
	if id == "" {
		return nil, errors.ErrValidationFailed("id", "cannot be empty")
	}

	cc.logger.Debug("Getting document by ID", logger.Fields{
		"document_id": id,
	})

	// 通过ID查询文档
	getResult, err := cc.collection.GetWithOptions(ctx,
		types.WithIds([]string{id}),
		types.WithInclude(types.IDocuments, types.IEmbeddings, types.IMetadatas),
	)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to get document from Chroma").
			WithCause(err).
			WithContext(map[string]interface{}{
				"document_id": id,
				"collection":  cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Document retrieval failed")
		return nil, memoErr
	}

	if getResult == nil || len(getResult.Ids) == 0 {
		return nil, errors.ErrResourceNotFound("document", id)
	}

	// 构建文档对象
	doc := &VectorDocument{
		ID: id,
	}

	// 设置内容
	if len(getResult.Documents) > 0 && len(getResult.Documents) > 0 {
		doc.Content = getResult.Documents[0]
	}

	// 设置向量
	if len(getResult.Embeddings) > 0 && getResult.Embeddings[0] != nil {
		if getResult.Embeddings[0].ArrayOfFloat32 != nil {
			doc.Embedding = *getResult.Embeddings[0].ArrayOfFloat32
		}
	}

	// 设置元数据
	if len(getResult.Metadatas) > 0 {
		doc.Metadata = getResult.Metadatas[0]

		// 从元数据中恢复创建时间
		if createdAtVal, exists := doc.Metadata["created_at"]; exists {
			if createdAtFloat, ok := createdAtVal.(float64); ok {
				doc.CreatedAt = time.Unix(int64(createdAtFloat), 0)
			} else if createdAtInt, ok := createdAtVal.(int64); ok {
				doc.CreatedAt = time.Unix(createdAtInt, 0)
			}
		}
	}

	cc.logger.Debug("Document retrieved successfully", logger.Fields{
		"document_id":    id,
		"content_length": len(doc.Content),
		"has_embedding":  len(doc.Embedding) > 0,
	})

	return doc, nil
}

// DeleteDocument 删除文档
func (cc *ChromaClient) DeleteDocument(ctx context.Context, id string) error {
	if id == "" {
		return errors.ErrValidationFailed("id", "cannot be empty")
	}

	cc.logger.Debug("Deleting document", logger.Fields{
		"document_id": id,
	})

	// 从集合中删除文档
	_, err := cc.collection.Delete(ctx, []string{id}, nil, nil)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to delete document from Chroma").
			WithCause(err).
			WithContext(map[string]interface{}{
				"document_id": id,
				"collection":  cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Document deletion failed")
		return memoErr
	}

	cc.logger.Debug("Document deleted successfully", logger.Fields{
		"document_id": id,
	})

	return nil
}

// UpdateDocument 更新文档
func (cc *ChromaClient) UpdateDocument(ctx context.Context, doc *VectorDocument) error {
	if doc == nil {
		return errors.ErrValidationFailed("document", "cannot be nil")
	}

	if doc.ID == "" {
		return errors.ErrValidationFailed("document.id", "cannot be empty")
	}

	cc.logger.Debug("Updating document", logger.Fields{
		"document_id":    doc.ID,
		"content_length": len(doc.Content),
		"has_embedding":  len(doc.Embedding) > 0,
	})

	// 准备更新数据
	ids := []string{doc.ID}

	var embeddings []*types.Embedding
	if len(doc.Embedding) > 0 {
		// Convert float32 slice to interface slice for Chroma API
		embeddingData := make([]interface{}, len(doc.Embedding))
		for i, v := range doc.Embedding {
			embeddingData[i] = v
		}
		embedding, err := types.NewEmbedding(embeddingData)
		if err != nil {
			return errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to create embedding for update").
				WithCause(err).
				WithContext(map[string]interface{}{
					"document_id": doc.ID,
				})
		}
		embeddings = []*types.Embedding{embedding}
	}

	var documents []string
	if doc.Content != "" {
		documents = []string{doc.Content}
	}

	var metadatas []map[string]interface{}
	if doc.Metadata != nil {
		// 添加更新时间到元数据
		doc.Metadata["updated_at"] = time.Now().Unix()
		doc.Metadata["content_length"] = len(doc.Content)
		metadatas = []map[string]interface{}{doc.Metadata}
	}

	// 更新文档 - 使用Modify方法
	_, err := cc.collection.Modify(ctx, embeddings, metadatas, documents, ids)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to update document in Chroma").
			WithCause(err).
			WithContext(map[string]interface{}{
				"document_id": doc.ID,
				"collection":  cc.config.Collection,
			})
		cc.logger.LogMemoroError(memoErr, "Document update failed")
		return memoErr
	}

	cc.logger.Debug("Document updated successfully", logger.Fields{
		"document_id": doc.ID,
	})

	return nil
}

// GetCollectionInfo 获取集合信息
func (cc *ChromaClient) GetCollectionInfo(ctx context.Context) (map[string]interface{}, error) {
	cc.logger.Debug("Getting collection information")

	// 获取集合计数
	count, err := cc.collection.Count(ctx)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to get collection count").
			WithCause(err)
		cc.logger.LogMemoroError(memoErr, "Collection count failed")
		return nil, memoErr
	}

	info := map[string]interface{}{
		"collection_name": cc.config.Collection,
		"document_count":  count,
		"server_url":      fmt.Sprintf("http://%s:%d", cc.config.Host, cc.config.Port),
		"batch_size":      cc.config.BatchSize,
		"timeout":         cc.config.Timeout,
	}

	cc.logger.Debug("Collection information retrieved", logger.Fields{
		"document_count": count,
	})

	return info, nil
}

// HealthCheck 健康检查
func (cc *ChromaClient) HealthCheck(ctx context.Context) error {
	cc.logger.Debug("Performing health check")

	// 尝试获取集合信息作为健康检查
	_, err := cc.GetCollectionInfo(ctx)
	if err != nil {
		cc.logger.Error("Health check failed", logger.Fields{
			"error": err.Error(),
		})
		return err
	}

	cc.logger.Debug("Health check passed")
	return nil
}

// Close 关闭客户端连接
func (cc *ChromaClient) Close() error {
	cc.logger.Info("Closing Chroma client")

	// Chroma客户端不需要显式关闭连接
	// 这里主要是为了符合接口规范

	cc.logger.Info("Chroma client closed")
	return nil
}

// getMetadataKeys 获取元数据键列表（辅助函数）
func getMetadataKeys(metadata map[string]interface{}) []string {
	if metadata == nil {
		return []string{}
	}

	keys := make([]string, 0, len(metadata))
	for k := range metadata {
		keys = append(keys, k)
	}
	return keys
}
