package integration

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memoro/internal/config"
	"memoro/internal/services/content"
)

// TestContentProcessorInitialization 测试内容处理器初始化
func TestContentProcessorInitialization(t *testing.T) {
	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	t.Run("处理器创建和关闭", func(t *testing.T) {
		processor, err := content.NewProcessor()
		if err != nil {
			// 如果没有LLM配置，这是预期的错误
			t.Logf("Expected error without LLM configuration: %v", err)
			return
		}
		
		require.NotNil(t, processor, "处理器不应为空")
		
		// 测试统计信息
		stats := processor.GetStats()
		assert.Contains(t, stats, "active_requests", "统计应包含活跃请求数")
		assert.Contains(t, stats, "total_results", "统计应包含总结果数")
		assert.Contains(t, stats, "status_distribution", "统计应包含状态分布")
		
		t.Logf("📊 处理器统计信息:")
		for key, value := range stats {
			t.Logf("  %s: %v", key, value)
		}
		
		// 关闭处理器
		err = processor.Close()
		assert.NoError(t, err, "处理器关闭应该成功")
		
		t.Logf("✅ 处理器初始化和关闭测试完成")
	})
}

// TestExtractorInitialization 测试提取器初始化
func TestExtractorInitialization(t *testing.T) {
	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	t.Run("提取器管理器创建", func(t *testing.T) {
		extractor, err := content.NewExtractorManager()
		require.NoError(t, err, "提取器管理器创建应该成功")
		require.NotNil(t, extractor, "提取器管理器不应为空")
		
		// 测试支持的类型
		supportedTypes := extractor.GetSupportedTypes()
		assert.NotEmpty(t, supportedTypes, "应该支持至少一种内容类型")
		
		t.Logf("📋 支持的内容类型: %v", supportedTypes)
		
		// 测试类型检查
		for _, contentType := range supportedTypes {
			canHandle := extractor.CanHandle(contentType)
			assert.True(t, canHandle, "应该能处理支持的类型: %s", contentType)
		}
		
		// 关闭提取器
		err = extractor.Close()
		assert.NoError(t, err, "提取器关闭应该成功")
		
		t.Logf("✅ 提取器初始化测试完成")
	})
}

// TestClassifierInitialization 测试分类器初始化
func TestClassifierInitialization(t *testing.T) {
	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	t.Run("分类器创建", func(t *testing.T) {
		classifier, err := content.NewClassifier()
		if err != nil {
			// 如果没有LLM配置，这是预期的错误
			t.Logf("Expected error without LLM configuration: %v", err)
			return
		}
		
		require.NotNil(t, classifier, "分类器不应为空")
		
		// 关闭分类器
		err = classifier.Close()
		assert.NoError(t, err, "分类器关闭应该成功")
		
		t.Logf("✅ 分类器初始化测试完成")
	})
}

// TestProcessingRequestValidation 测试处理请求验证
func TestProcessingRequestValidation(t *testing.T) {
	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	processor, err := content.NewProcessor()
	if err != nil {
		t.Skipf("Skipping validation test due to missing LLM configuration: %v", err)
		return
	}
	defer processor.Close()

	t.Run("空内容验证", func(t *testing.T) {
		request := &content.ProcessingRequest{
			ID:          "test-001",
			Content:     "",
			ContentType: "text",
			UserID:      "test-user",
		}

		err := processor.ProcessContentAsync(request)
		assert.Error(t, err, "空内容应该返回错误")
		t.Logf("✅ 空内容验证错误: %v", err)
	})

	t.Run("无效内容类型验证", func(t *testing.T) {
		request := &content.ProcessingRequest{
			ID:          "test-002",
			Content:     "test content",
			ContentType: "invalid_type",
			UserID:      "test-user",
		}

		err := processor.ProcessContentAsync(request)
		assert.Error(t, err, "无效内容类型应该返回错误")
		t.Logf("✅ 无效内容类型验证错误: %v", err)
	})

	t.Run("超大内容验证", func(t *testing.T) {
		// 创建超过100KB的内容
		largeContent := make([]byte, 100001)
		for i := range largeContent {
			largeContent[i] = 'a'
		}

		request := &content.ProcessingRequest{
			ID:          "test-003",
			Content:     string(largeContent),
			ContentType: "text",
			UserID:      "test-user",
		}

		err := processor.ProcessContentAsync(request)
		assert.Error(t, err, "超大内容应该返回错误")
		t.Logf("✅ 超大内容验证错误: %v", err)
	})

	t.Logf("✅ 所有验证测试完成")
}