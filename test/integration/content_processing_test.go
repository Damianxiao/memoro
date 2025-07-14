package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memoro/internal/config"
	"memoro/internal/models"
	"memoro/internal/services/content"
)

// TestEndToEndContentProcessing 端到端内容处理流程测试
func TestEndToEndContentProcessing(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping content processing test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	processor, err := content.NewProcessor()
	require.NoError(t, err)
	defer processor.Close()

	t.Run("文本内容完整处理流程", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		testContent := `人工智能技术发展现状与展望

人工智能（AI）技术正在以前所未有的速度发展，深刻改变着各个行业和社会生活的方方面面。从机器学习、深度学习到大语言模型，AI技术的进步为人类带来了巨大的机遇和挑战。

## 技术发展现状

1. **深度学习突破**：卷积神经网络（CNN）在图像识别领域取得重大突破，准确率已超过人类水平。
2. **自然语言处理**：大语言模型如GPT系列、BERT等在文本理解和生成方面表现出色。
3. **强化学习应用**：在游戏、机器人控制、自动驾驶等领域展现出强大潜力。

## 应用场景

- 医疗诊断：AI辅助影像诊断、药物发现
- 金融科技：智能风控、算法交易
- 智能制造：预测性维护、质量检测
- 教育领域：个性化学习、智能辅导

## 发展趋势

未来AI技术将朝着更加通用化、高效化、可解释化的方向发展。多模态AI、联邦学习、边缘计算等新兴技术将进一步推动AI的普及和应用。

总结：人工智能技术正处于快速发展期，其影响将越来越深远。`

		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     testContent,
			ContentType: models.ContentTypeText,
			UserID:      "test-user-001",
			Priority:    8,
			Options: content.ProcessingOptions{
				EnableSummary:         true,
				EnableTags:            true,
				EnableClassification:  true,
				EnableImportanceScore: true,
				MaxTags:               15,
			},
			Context: map[string]interface{}{
				"source": "integration_test",
				"test":   true,
			},
		}

		result, err := processor.ProcessContent(ctx, request)
		require.NoError(t, err, "内容处理应该成功")
		require.NotNil(t, result, "处理结果不应为空")

		// 验证处理状态
		assert.Equal(t, content.StatusCompleted, result.Status, "处理状态应为已完成")
		assert.Equal(t, request.ID, result.RequestID, "请求ID应匹配")
		assert.NotNil(t, result.ContentItem, "内容项不应为空")
		assert.Greater(t, result.ProcessingTime, time.Duration(0), "处理时间应大于0")
		assert.False(t, result.CompletedAt.IsZero(), "完成时间应已设置")

		// 验证内容项基本属性
		item := result.ContentItem
		assert.Equal(t, models.ContentTypeText, item.Type, "内容类型应为文本")
		assert.NotEmpty(t, item.ID, "内容ID不应为空")
		assert.Equal(t, "test-user-001", item.UserID, "用户ID应匹配")
		assert.Greater(t, item.ImportanceScore, 0.0, "重要性评分应大于0")
		assert.LessOrEqual(t, item.ImportanceScore, 1.0, "重要性评分应小于等于1")

		// 验证提取的内容和元数据
		assert.NotEmpty(t, item.RawContent, "原始内容不应为空")
		assert.Contains(t, item.RawContent, "人工智能", "应包含关键词")
		
		// 验证提取器生成的元数据
		processedData := item.GetProcessedData()
		if title, exists := processedData["title"]; exists {
			t.Logf("📝 提取的标题: %v", title)
		}
		if desc, exists := processedData["description"]; exists {
			t.Logf("📝 提取的描述: %v", desc)
		}

		// 验证摘要生成
		if result.Summary != nil {
			assert.NotEmpty(t, result.Summary.OneLine, "一句话摘要不应为空")
			assert.NotEmpty(t, result.Summary.Paragraph, "段落摘要不应为空")
			assert.NotEmpty(t, result.Summary.Detailed, "详细摘要不应为空")
			
			// 验证摘要质量
			assert.Contains(t, result.Summary.OneLine, "人工智能", "一句话摘要应包含关键词")
			assert.Less(t, len(result.Summary.OneLine), 300, "一句话摘要应相对简短")
			assert.Greater(t, len(result.Summary.Detailed), len(result.Summary.Paragraph), "详细摘要应比段落摘要更长")

			t.Logf("📄 摘要生成结果:")
			t.Logf("  一句话: %s", result.Summary.OneLine)
			t.Logf("  段落: %s", result.Summary.Paragraph[:min(100, len(result.Summary.Paragraph))])
			t.Logf("  详细: %s", result.Summary.Detailed[:min(200, len(result.Summary.Detailed))])

			// 验证模型中的摘要设置
			assert.Equal(t, result.Summary.OneLine, item.Summary.OneLine, "模型摘要应与结果一致")
		}

		// 验证标签生成
		if result.Tags != nil {
			assert.NotEmpty(t, result.Tags.Tags, "标签列表不应为空")
			assert.LessOrEqual(t, len(result.Tags.Tags), 15, "标签数量不应超过限制")
			assert.NotEmpty(t, result.Tags.Categories, "分类列表不应为空")
			
			// 验证标签质量
			foundRelevantTag := false
			for _, tag := range result.Tags.Tags {
				if tag == "人工智能" || tag == "机器学习" || tag == "深度学习" || tag == "AI" {
					foundRelevantTag = true
					break
				}
			}
			assert.True(t, foundRelevantTag, "应包含相关的技术标签")

			t.Logf("🏷️ 标签生成结果:")
			t.Logf("  标签: %v", result.Tags.Tags)
			t.Logf("  分类: %v", result.Tags.Categories)
			t.Logf("  关键词: %v", result.Tags.Keywords)

			// 验证模型中的标签设置
			assert.Equal(t, result.Tags.Tags, item.Tags, "模型标签应与结果一致")
		}

		// 验证分类信息
		if categories, exists := processedData["categories"]; exists {
			categoriesList, ok := categories.([]string)
			assert.True(t, ok, "分类信息应为字符串数组")
			assert.NotEmpty(t, categoriesList, "分类列表不应为空")
			t.Logf("📊 分类结果: %v", categoriesList)
		}

		// 验证关键词提取
		if keywords, exists := processedData["keywords"]; exists {
			keywordsList, ok := keywords.([]string)
			assert.True(t, ok, "关键词应为字符串数组")
			assert.NotEmpty(t, keywordsList, "关键词列表不应为空")
			t.Logf("🔑 关键词: %v", keywordsList)
		}

		// 验证重要性评分
		assert.Equal(t, result.ImportanceScore, item.ImportanceScore, "重要性评分应一致")
		assert.Greater(t, result.ImportanceScore, 0.5, "技术内容重要性评分应较高")

		t.Logf("✅ 端到端文本处理测试完成")
		t.Logf("📈 重要性评分: %.3f", result.ImportanceScore)
		t.Logf("⏱️ 处理时间: %v", result.ProcessingTime)
	})

	t.Run("链接内容完整处理流程", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		testURL := "https://example.com" // 使用示例URL进行测试

		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     testURL,
			ContentType: models.ContentTypeLink,
			UserID:      "test-user-002",
			Priority:    6,
			Options: content.ProcessingOptions{
				EnableSummary:         true,
				EnableTags:            true,
				EnableClassification:  true,
				EnableImportanceScore: true,
				MaxTags:               10,
			},
		}

		result, err := processor.ProcessContent(ctx, request)
		require.NoError(t, err, "链接处理应该成功")
		require.NotNil(t, result, "处理结果不应为空")

		// 验证基本处理结果
		assert.Equal(t, content.StatusCompleted, result.Status, "处理状态应为已完成")
		assert.NotNil(t, result.ContentItem, "内容项不应为空")

		// 验证链接特定的元数据
		item := result.ContentItem
		assert.Equal(t, models.ContentTypeLink, item.Type, "内容类型应为链接")
		
		processedData := item.GetProcessedData()
		if metadata, exists := processedData["extraction_metadata"]; exists {
			metadataMap, ok := metadata.(map[string]interface{})
			assert.True(t, ok, "提取元数据应为map")
			
			if url, exists := metadataMap["url"]; exists {
				assert.Equal(t, testURL, url, "URL应匹配")
				t.Logf("🔗 提取的URL: %v", url)
			}
			
			if domain, exists := metadataMap["domain"]; exists {
				t.Logf("🌐 域名: %v", domain)
			}
		}

		t.Logf("✅ 端到端链接处理测试完成")
		t.Logf("📈 重要性评分: %.3f", result.ImportanceScore)
		t.Logf("⏱️ 处理时间: %v", result.ProcessingTime)
	})

	t.Run("异步处理测试", func(t *testing.T) {
		testContent := "这是一个简单的测试文本，用于验证异步处理功能。"

		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     testContent,
			ContentType: models.ContentTypeText,
			UserID:      "test-user-async",
			Priority:    3,
			Options: content.ProcessingOptions{
				EnableSummary:         true,
				EnableTags:            false,
				EnableClassification:  false,
				EnableImportanceScore: true,
				MaxTags:               5,
			},
		}

		// 提交异步处理
		err := processor.ProcessContentAsync(request)
		require.NoError(t, err, "异步处理提交应该成功")

		// 轮询检查状态
		var result *content.ProcessingResult
		maxWaitTime := 90 * time.Second
		startTime := time.Now()

		for time.Since(startTime) < maxWaitTime {
			status, err := processor.GetStatus(request.ID)
			require.NoError(t, err, "获取状态应该成功")

			if status == content.StatusCompleted || status == content.StatusFailed {
				result, err = processor.GetResult(request.ID)
				require.NoError(t, err, "获取结果应该成功")
				break
			}

			time.Sleep(1 * time.Second)
		}

		require.NotNil(t, result, "异步处理应该完成")
		assert.Equal(t, content.StatusCompleted, result.Status, "异步处理应该成功")
		assert.NotNil(t, result.Summary, "应该生成摘要")

		t.Logf("✅ 异步处理测试完成")
		t.Logf("⏱️ 处理时间: %v", result.ProcessingTime)
	})

	t.Run("处理器性能统计", func(t *testing.T) {
		stats := processor.GetStats()
		
		assert.Contains(t, stats, "active_requests", "统计应包含活跃请求数")
		assert.Contains(t, stats, "total_results", "统计应包含总结果数")
		assert.Contains(t, stats, "status_distribution", "统计应包含状态分布")
		
		t.Logf("📊 处理器统计信息:")
		for key, value := range stats {
			t.Logf("  %s: %v", key, value)
		}
	})
}

// TestContentProcessingEdgeCases 内容处理边界案例测试
func TestContentProcessingEdgeCases(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping edge case test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	processor, err := content.NewProcessor()
	require.NoError(t, err)
	defer processor.Close()

	t.Run("空内容处理", func(t *testing.T) {
		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     "",
			ContentType: models.ContentTypeText,
			UserID:      "test-user",
		}

		_, err := processor.ProcessContent(context.Background(), request)
		assert.Error(t, err, "空内容应该返回错误")
	})

	t.Run("超大内容处理", func(t *testing.T) {
		// 创建超过100KB的内容
		largeContent := make([]byte, 100001)
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     string(largeContent),
			ContentType: models.ContentTypeText,
			UserID:      "test-user",
		}

		_, err := processor.ProcessContent(context.Background(), request)
		assert.Error(t, err, "超大内容应该返回错误")
	})

	t.Run("无效内容类型", func(t *testing.T) {
		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     "test content",
			ContentType: "invalid_type",
			UserID:      "test-user",
		}

		_, err := processor.ProcessContent(context.Background(), request)
		assert.Error(t, err, "无效内容类型应该返回错误")
	})

	t.Run("取消处理请求", func(t *testing.T) {
		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     "test content for cancellation",
			ContentType: models.ContentTypeText,
			UserID:      "test-user",
		}

		// 异步提交请求
		err := processor.ProcessContentAsync(request)
		require.NoError(t, err)

		// 立即取消
		err = processor.CancelRequest(request.ID)
		require.NoError(t, err)

		// 验证状态
		status, err := processor.GetStatus(request.ID)
		require.NoError(t, err)
		assert.Equal(t, content.StatusCancelled, status, "状态应为已取消")
	})

	t.Run("超时处理", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		request := &content.ProcessingRequest{
			ID:          uuid.New().String(),
			Content:     "test content",
			ContentType: models.ContentTypeText,
			UserID:      "test-user",
		}

		_, err := processor.ProcessContent(ctx, request)
		assert.Error(t, err, "超时应该返回错误")
	})
}

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}