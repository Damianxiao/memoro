package integration

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"memoro/internal/config"
	"memoro/internal/models"
	"memoro/internal/services/content"
	"memoro/internal/services/wechat"
)

// TestWeChatIntegration_CompleteMessageFlow 测试完整的微信消息处理流程
func TestWeChatIntegration_CompleteMessageFlow(t *testing.T) {
	// 这是一个端到端测试，验证从微信消息到处理完成的整个流程
	t.Run("完整的文本消息处理流程", func(t *testing.T) {
		// 准备测试数据
		testMessage := WeChatMessage{
			Type:    "text",
			Content: "人工智能在医疗领域的应用",
			UserID:  "test-user-001",
			Time:    time.Now(),
		}

		// 创建集成测试环境
		integrationManager, err := NewIntegrationManager()
		require.NoError(t, err)
		defer integrationManager.Close()

		// 执行完整的消息处理流程
		result, err := integrationManager.ProcessWeChatMessage(context.Background(), testMessage)
		require.NoError(t, err)

		// 验证处理结果
		assert.Equal(t, content.StatusCompleted, result.Status)
		assert.NotNil(t, result.ContentItem)
		assert.Equal(t, models.ContentTypeText, result.ContentItem.Type)
		assert.NotEmpty(t, result.ContentItem.ID)
		assert.Equal(t, testMessage.UserID, result.ContentItem.UserID)
		
		// 验证内容处理结果
		assert.NotNil(t, result.Summary)
		assert.NotEmpty(t, result.Summary.OneLine)
		assert.NotEmpty(t, result.Summary.Paragraph)
		
		// 验证标签生成 - 检查是否包含相关医疗主题的标签
		assert.NotNil(t, result.Tags)
		assert.Greater(t, len(result.Tags.Tags), 0)
		
		// 检查是否包含与原内容相关的标签
		hasRelevantTag := false
		for _, tag := range result.Tags.Tags {
			if strings.Contains(tag, "人工智能") || strings.Contains(tag, "医疗") || strings.Contains(tag, "AI") {
				hasRelevantTag = true
				break
			}
		}
		assert.True(t, hasRelevantTag, "应该包含与内容相关的标签")
		
		assert.Greater(t, result.ImportanceScore, 0.0)
		assert.LessOrEqual(t, result.ImportanceScore, 1.0)
		
		// 验证向量化结果
		assert.NotNil(t, result.VectorResult)
		assert.True(t, result.VectorResult.Indexed)
		assert.NotEmpty(t, result.VectorResult.DocumentID)
		assert.Greater(t, result.VectorResult.VectorDimension, 0)
		
		// 验证处理时间合理
		assert.Greater(t, result.ProcessingTime, time.Duration(0))
		assert.Less(t, result.ProcessingTime, 30*time.Second)
	})

	t.Run("处理后可以通过搜索找到内容", func(t *testing.T) {
		// 准备测试数据
		testMessage := WeChatMessage{
			Type:    "text",
			Content: "量子计算技术发展",
			UserID:  "test-user-002",
			Time:    time.Now(),
		}

		// 创建集成测试环境
		integrationManager, err := NewIntegrationManager()
		require.NoError(t, err)
		defer integrationManager.Close()

		// 处理消息
		result, err := integrationManager.ProcessWeChatMessage(context.Background(), testMessage)
		require.NoError(t, err)
		require.Equal(t, content.StatusCompleted, result.Status)

		// 等待索引完成
		time.Sleep(100 * time.Millisecond)

		// 执行搜索
		searchRequest := &content.SearchRequest{
			Query:         "量子",
			UserID:        testMessage.UserID,
			TopK:          10,
			MinSimilarity: 0.5,
		}

		searchResults, err := integrationManager.SearchContent(context.Background(), searchRequest)
		require.NoError(t, err)

		// 验证搜索结果
		assert.Greater(t, len(searchResults.Results), 0)
		assert.Equal(t, result.ContentItem.ID, searchResults.Results[0].DocumentID)
		assert.Greater(t, searchResults.Results[0].Similarity, 0.5)
		assert.Contains(t, searchResults.Results[0].Content, "量子")
	})

	t.Run("处理后可以获得相关推荐", func(t *testing.T) {
		// 准备测试数据
		testMessage1 := WeChatMessage{
			Type:    "text",
			Content: "机器学习算法",
			UserID:  "test-user-003",
			Time:    time.Now(),
		}

		testMessage2 := WeChatMessage{
			Type:    "text",
			Content: "深度学习网络",
			UserID:  "test-user-003",
			Time:    time.Now().Add(1 * time.Minute),
		}

		// 创建集成测试环境
		integrationManager, err := NewIntegrationManager()
		require.NoError(t, err)
		defer integrationManager.Close()

		// 处理两条消息
		result1, err := integrationManager.ProcessWeChatMessage(context.Background(), testMessage1)
		require.NoError(t, err)
		require.Equal(t, content.StatusCompleted, result1.Status)

		result2, err := integrationManager.ProcessWeChatMessage(context.Background(), testMessage2)
		require.NoError(t, err)
		require.Equal(t, content.StatusCompleted, result2.Status)

		// 等待索引完成
		time.Sleep(200 * time.Millisecond)

		// 获取推荐
		recommendationRequest := &content.RecommendationRequest{
			Type:               "similar",
			UserID:             testMessage1.UserID,
			SourceDocumentID:   result1.ContentItem.ID,
			MaxRecommendations: 5,
			MinSimilarity:      0.3,
		}

		recommendations, err := integrationManager.GetRecommendations(context.Background(), recommendationRequest)
		require.NoError(t, err)

		// 验证推荐结果
		assert.Greater(t, len(recommendations.Recommendations), 0)
		assert.Equal(t, result2.ContentItem.ID, recommendations.Recommendations[0].DocumentID)
		assert.Greater(t, recommendations.Recommendations[0].Similarity, 0.3)
	})
}

// TestWeChatIntegration_ErrorHandling 测试错误处理
func TestWeChatIntegration_ErrorHandling(t *testing.T) {
	t.Run("处理无效消息", func(t *testing.T) {
		// 准备无效消息
		invalidMessage := WeChatMessage{
			Type:    "text",
			Content: "", // 空内容
			UserID:  "test-user-004",
			Time:    time.Now(),
		}

		// 创建集成测试环境
		integrationManager, err := NewIntegrationManager()
		require.NoError(t, err)
		defer integrationManager.Close()

		// 处理无效消息
		result, err := integrationManager.ProcessWeChatMessage(context.Background(), invalidMessage)
		
		// 验证错误处理
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "content")
	})

	t.Run("处理超大消息", func(t *testing.T) {
		// 准备超大消息
		largeContent := make([]byte, 200000) // 200KB
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		largeMessage := WeChatMessage{
			Type:    "text",
			Content: string(largeContent),
			UserID:  "test-user-005",
			Time:    time.Now(),
		}

		// 创建集成测试环境
		integrationManager, err := NewIntegrationManager()
		require.NoError(t, err)
		defer integrationManager.Close()

		// 处理超大消息
		result, err := integrationManager.ProcessWeChatMessage(context.Background(), largeMessage)
		
		// 验证错误处理
		assert.Error(t, err)
		assert.Nil(t, result)
		assert.Contains(t, err.Error(), "too large")
	})
}

// WeChatMessage 微信消息结构
type WeChatMessage struct {
	Type    string    `json:"type"`
	Content string    `json:"content"`
	UserID  string    `json:"user_id"`
	Time    time.Time `json:"time"`
}

// IntegrationManager 集成测试管理器
type IntegrationManager struct {
	processor       *content.Processor
	wechatClient    *wechat.WeChatWebSocketClient
	config          *config.Config
}

// NewIntegrationManager 创建集成测试管理器
func NewIntegrationManager() (*IntegrationManager, error) {
	// 加载测试配置
	cfg, err := loadTestConfig()
	if err != nil {
		return nil, err
	}

	// 创建内容处理器
	processor, err := content.NewProcessor()
	if err != nil {
		return nil, err
	}

	// 创建微信客户端
	wechatClient := wechat.NewWeChatWebSocketClient(
		cfg.WeChat.WebSocketURL,
		cfg.WeChat.AdminKey,
	)

	return &IntegrationManager{
		processor:    processor,
		wechatClient: wechatClient,
		config:       cfg,
	}, nil
}

// loadTestConfig 加载测试配置
func loadTestConfig() (*config.Config, error) {
	// 为测试环境创建默认配置
	cfg := &config.Config{
		WeChat: config.WeChatConfig{
			WebSocketURL: "ws://localhost:1239/ws",
			AdminKey:     "test-admin-key",
		},
		LLM: config.LLMConfig{
			Provider:    "openai_compatible",
			APIBase:     "https://api.gpt.ge/v1",
			APIKey:      "sk-hPO0u6WuP3LGDKts94742609166644FdB1Aa8c5149A6D5Bc",
			Model:       "gpt-4o",
			MaxTokens:   1000,
			Temperature: 0.5,
			Timeout:     120 * time.Second,
			RetryTimes:  3,
			RetryDelay:  5 * time.Second,
		},
		Database: config.DatabaseConfig{
			Type:        "sqlite",
			Path:        ":memory:",
			AutoMigrate: true,
		},
		VectorDB: config.VectorDBConfig{
			Type:       "chroma",
			Host:       "localhost",
			Port:       8000,
			Collection: "test_memoro_content",
			Timeout:    30 * time.Second,
			RetryTimes: 3,
			BatchSize:  100,
		},
		Processing: config.ProcessingConfig{
			MaxWorkers:     2,
			QueueSize:      10,
			Timeout:        120 * time.Second,
			MaxContentSize: 102400, // 100KB
			TagLimits: config.TagLimitsConfig{
				MaxTags:      10,
				MaxTagLength: 50,
			},
			SummaryLevels: config.SummaryLevelsConfig{
				OneLineMaxLength:    200,
				ParagraphMaxLength:  1000,
				DetailedMaxLength:   5000,
			},
		},
	}
	
	// 初始化全局配置
	return cfg, config.InitializeForTest(cfg)
}

// ProcessWeChatMessage 处理微信消息
func (im *IntegrationManager) ProcessWeChatMessage(ctx context.Context, message WeChatMessage) (*content.ProcessingResult, error) {
	// 转换消息格式
	contentType := models.ContentTypeText
	switch message.Type {
	case "text":
		contentType = models.ContentTypeText
	case "link":
		contentType = models.ContentTypeLink
	case "file":
		contentType = models.ContentTypeFile
	case "image":
		contentType = models.ContentTypeImage
	case "audio":
		contentType = models.ContentTypeAudio
	case "video":
		contentType = models.ContentTypeVideo
	default:
		contentType = models.ContentTypeText
	}

	// 创建处理请求
	processingRequest := &content.ProcessingRequest{
		ID:          generateRequestID(),
		Content:     message.Content,
		ContentType: contentType,
		UserID:      message.UserID,
		Priority:    5,
		Context:     map[string]interface{}{
			"source":    "wechat",
			"timestamp": message.Time,
		},
		Options: content.ProcessingOptions{
			EnableSummary:         true,
			EnableTags:            true,
			EnableClassification:  true,
			EnableImportanceScore: true,
			EnableVectorization:   true,
			MaxTags:              10,
		},
		CreatedAt: time.Now(),
	}

	// 执行处理
	return im.processor.ProcessContent(ctx, processingRequest)
}

// SearchContent 搜索内容
func (im *IntegrationManager) SearchContent(ctx context.Context, request *content.SearchRequest) (*content.SearchResponse, error) {
	return im.processor.SearchContent(ctx, request)
}

// GetRecommendations 获取推荐
func (im *IntegrationManager) GetRecommendations(ctx context.Context, request *content.RecommendationRequest) (*content.RecommendationResponse, error) {
	return im.processor.GetRecommendations(ctx, request)
}

// Close 关闭集成管理器
func (im *IntegrationManager) Close() error {
	if im.processor != nil {
		im.processor.Close()
	}
	if im.wechatClient != nil {
		im.wechatClient.Disconnect()
	}
	return nil
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return "req_" + time.Now().Format("20060102_150405_000000")
}