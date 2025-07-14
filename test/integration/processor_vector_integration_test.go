package integration

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"memoro/internal/models"
	"memoro/internal/services/content"
)

// TestProcessorVectorIntegration 测试内容处理器与向量服务的集成
func TestProcessorVectorIntegration(t *testing.T) {
	// 跳过集成测试（如果不在集成测试环境中）
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Run("ProcessContentWithVectorization", func(t *testing.T) {
		// 1. 创建处理器
		processor, err := content.NewProcessor()
		require.NoError(t, err, "Failed to create processor")
		defer processor.Close()

		// 2. 准备测试请求
		request := &content.ProcessingRequest{
			ID:          "test-vector-integration-001",
			Content:     "人工智能是一门计算机科学技术，旨在创建能够执行通常需要人类智能的任务的系统。机器学习是人工智能的一个重要分支。",
			ContentType: models.ContentTypeText,
			UserID:      "test-user-123",
			Priority:    5,
			Options: content.ProcessingOptions{
				EnableSummary:         true,
				EnableTags:            true,
				EnableClassification:  true,
				EnableImportanceScore: true,
				EnableVectorization:   true, // 这是我们要添加的新选项
				MaxTags:               10,
			},
			CreatedAt: time.Now(),
		}

		// 3. 执行处理
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := processor.ProcessContent(ctx, request)
		require.NoError(t, err, "Failed to process content")
		require.NotNil(t, result, "Result should not be nil")

		// 4. 验证基本处理结果
		assert.Equal(t, content.StatusCompleted, result.Status, "Status should be completed")
		assert.NotNil(t, result.ContentItem, "ContentItem should not be nil")
		assert.NotNil(t, result.Summary, "Summary should not be nil")
		assert.NotNil(t, result.Tags, "Tags should not be nil")
		assert.Greater(t, result.ImportanceScore, 0.0, "Importance score should be positive")

		// 5. 验证向量化结果
		assert.NotNil(t, result.VectorResult, "VectorResult should not be nil")
		assert.NotEmpty(t, result.VectorResult.DocumentID, "Vector document ID should not be empty")
		assert.Greater(t, result.VectorResult.VectorDimension, 0, "Vector dimension should be positive")
		assert.True(t, result.VectorResult.Indexed, "Document should be indexed")

		// 6. 验证向量搜索功能
		// 这里我们可以测试刚索引的内容是否能被搜索到
		searchRequest := &content.SearchRequest{
			Query:         "人工智能",
			UserID:        "test-user-123",
			TopK:          5,
			MinSimilarity: 0.5,
		}

		searchResult, err := processor.SearchContent(ctx, searchRequest)
		require.NoError(t, err, "Failed to search content")
		require.NotNil(t, searchResult, "Search result should not be nil")

		// 验证搜索结果包含我们刚才索引的文档
		found := false
		for _, item := range searchResult.Results {
			if item.DocumentID == result.VectorResult.DocumentID {
				found = true
				assert.Greater(t, item.Similarity, 0.8, "Similarity should be high for same content")
				break
			}
		}
		assert.True(t, found, "Indexed document should be found in search results")
	})

	t.Run("ProcessContentWithoutVectorization", func(t *testing.T) {
		// 测试不启用向量化的情况
		processor, err := content.NewProcessor()
		require.NoError(t, err, "Failed to create processor")
		defer processor.Close()

		request := &content.ProcessingRequest{
			ID:          "test-no-vector-001",
			Content:     "这是一个测试内容",
			ContentType: models.ContentTypeText,
			UserID:      "test-user-123",
			Options: content.ProcessingOptions{
				EnableSummary:         true,
				EnableTags:            true,
				EnableClassification:  true,
				EnableImportanceScore: true,
				EnableVectorization:   false, // 禁用向量化
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := processor.ProcessContent(ctx, request)
		require.NoError(t, err, "Failed to process content")

		// 验证没有向量化结果
		assert.Nil(t, result.VectorResult, "VectorResult should be nil when vectorization is disabled")
	})

	t.Run("BatchVectorIndexing", func(t *testing.T) {
		// 测试批量向量索引
		processor, err := content.NewProcessor()
		require.NoError(t, err, "Failed to create processor")
		defer processor.Close()

		// 创建多个测试请求
		requests := []*content.ProcessingRequest{
			{
				ID:          "batch-test-001",
				Content:     "机器学习是人工智能的核心技术之一",
				ContentType: models.ContentTypeText,
				UserID:      "batch-user-001",
				Options: content.ProcessingOptions{
					EnableVectorization: true,
				},
			},
			{
				ID:          "batch-test-002",
				Content:     "深度学习使用神经网络来模拟人脑的工作方式",
				ContentType: models.ContentTypeText,
				UserID:      "batch-user-001",
				Options: content.ProcessingOptions{
					EnableVectorization: true,
				},
			},
			{
				ID:          "batch-test-003",
				Content:     "自然语言处理让计算机理解人类语言",
				ContentType: models.ContentTypeText,
				UserID:      "batch-user-001",
				Options: content.ProcessingOptions{
					EnableVectorization: true,
				},
			},
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// 批量处理
		for _, req := range requests {
			result, err := processor.ProcessContent(ctx, req)
			require.NoError(t, err, "Failed to process batch request %s", req.ID)
			assert.NotNil(t, result.VectorResult, "VectorResult should not be nil for %s", req.ID)
		}

		// 测试批量搜索
		searchRequest := &content.SearchRequest{
			Query:         "人工智能 机器学习",
			UserID:        "batch-user-001",
			TopK:          5,
			MinSimilarity: 0.3,
		}

		searchResult, err := processor.SearchContent(ctx, searchRequest)
		require.NoError(t, err, "Failed to search batch content")
		
		// 应该能找到至少2个相关文档
		assert.GreaterOrEqual(t, len(searchResult.Results), 2, "Should find at least 2 related documents")
	})
}

// TestProcessorRecommendationIntegration 测试内容处理器与推荐系统的集成
func TestProcessorRecommendationIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test")
	}

	t.Run("ContentBasedRecommendations", func(t *testing.T) {
		processor, err := content.NewProcessor()
		require.NoError(t, err, "Failed to create processor")
		defer processor.Close()

		// 1. 先索引一些内容
		contents := []string{
			"人工智能技术正在改变世界",
			"机器学习算法的最新进展",
			"深度学习在图像识别中的应用",
			"自然语言处理的发展历程",
			"计算机视觉技术介绍",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		var documentIDs []string
		for i, contentText := range contents {
			request := &content.ProcessingRequest{
				ID:          fmt.Sprintf("rec-test-%03d", i+1),
				Content:     contentText,
				ContentType: models.ContentTypeText,
				UserID:      "rec-user-123",
				Options: content.ProcessingOptions{
					EnableVectorization: true,
				},
			}

			result, err := processor.ProcessContent(ctx, request)
			require.NoError(t, err, "Failed to process content %d", i+1)
			documentIDs = append(documentIDs, result.VectorResult.DocumentID)
		}

		// 2. 获取推荐
		recRequest := &content.RecommendationRequest{
			SourceDocumentID:   documentIDs[0], // 基于第一个文档
			UserID:             "rec-user-123",
			MaxRecommendations: 3,
			Type:               "similar",
		}

		recResult, err := processor.GetRecommendations(ctx, recRequest)
		require.NoError(t, err, "Failed to get recommendations")
		require.NotNil(t, recResult, "Recommendation result should not be nil")

		// 3. 验证推荐结果
		assert.GreaterOrEqual(t, len(recResult.Recommendations), 1, "Should have at least 1 recommendation")
		assert.LessOrEqual(t, len(recResult.Recommendations), 3, "Should not exceed max recommendations")

		// 验证推荐项不包含源文档
		for _, rec := range recResult.Recommendations {
			assert.NotEqual(t, documentIDs[0], rec.DocumentID, "Recommendation should not include source document")
			assert.Greater(t, rec.Similarity, 0.0, "Similarity should be positive")
		}
	})
}