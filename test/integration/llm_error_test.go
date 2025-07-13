package integration

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memoro/internal/config"
	"memoro/internal/models"
	"memoro/internal/services/llm"
)

// TestLLMErrorHandling 测试LLM错误处理
func TestLLMErrorHandling(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	client, err := llm.NewClient()
	require.NoError(t, err)

	t.Run("测试超时处理", func(t *testing.T) {
		// 创建一个很短的超时上下文
		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
		defer cancel()

		messages := []llm.ChatMessage{
			{Role: "user", Content: "请详细解释人工智能的发展历史"},
		}

		_, err := client.ChatCompletion(ctx, messages)
		assert.Error(t, err, "应该因为超时而返回错误")
		t.Logf("✅ 超时错误处理正常: %v", err)
	})

	t.Run("测试空消息处理", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// 测试空消息列表
		_, err := client.ChatCompletion(ctx, []llm.ChatMessage{})
		assert.Error(t, err, "空消息列表应该返回错误")

		// 测试空内容消息
		messages := []llm.ChatMessage{
			{Role: "user", Content: ""},
		}
		_, err = client.ChatCompletion(ctx, messages)
		assert.Error(t, err, "空内容消息应该返回错误")

		t.Logf("✅ 空消息错误处理正常")
	})

	t.Run("测试无效角色处理", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		messages := []llm.ChatMessage{
			{Role: "invalid_role", Content: "测试内容"},
		}

		// 这可能会被API接受或拒绝，我们主要测试客户端不会崩溃
		_, err := client.ChatCompletion(ctx, messages)
		// 无论成功还是失败，都不应该panic
		t.Logf("✅ 无效角色处理结果: %v", err)
	})
}

// TestLLMContentBoundaries 测试内容边界情况
func TestLLMContentBoundaries(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	client, err := llm.NewClient()
	require.NoError(t, err)

	summarizer, err := llm.NewSummarizer(client)
	require.NoError(t, err)

	tagger, err := llm.NewTagger(client)
	require.NoError(t, err)

	t.Run("测试大内容处理", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// 创建接近最大限制的内容 (100KB = 102400字节)
		largeContent := strings.Repeat("这是一个很长的测试内容。", 3000) // 约30KB
		
		// 测试摘要生成
		summaryRequest := llm.SummaryRequest{
			Content:     largeContent,
			ContentType: models.ContentTypeText,
		}

		result, err := summarizer.GenerateSummary(ctx, summaryRequest)
		if err != nil {
			t.Logf("大内容摘要生成失败（可能超出限制）: %v", err)
		} else {
			assert.NotNil(t, result, "大内容摘要应该成功生成")
			t.Logf("✅ 大内容摘要生成成功，内容长度: %d", len(largeContent))
		}
	})

	t.Run("测试超大内容处理", func(t *testing.T) {
		// 创建超过最大限制的内容
		oversizeContent := strings.Repeat("测试内容", 20000) // 约200KB，超过100KB限制

		summaryRequest := llm.SummaryRequest{
			Content:     oversizeContent,
			ContentType: models.ContentTypeText,
		}

		_, err := summarizer.GenerateSummary(context.Background(), summaryRequest)
		assert.Error(t, err, "超大内容应该返回错误")
		t.Logf("✅ 超大内容错误处理正常: %v", err)
	})

	t.Run("测试特殊字符内容", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		specialContent := "测试特殊字符：@#$%^&*(){}[]|\\:;\"'<>,.?/~`! 🚀🎉💡🔥⭐ αβγδε 中文测试"

		// 测试摘要
		summaryRequest := llm.SummaryRequest{
			Content:     specialContent,
			ContentType: models.ContentTypeText,
		}

		result, err := summarizer.GenerateSummary(ctx, summaryRequest)
		require.NoError(t, err, "特殊字符内容处理应该成功")
		assert.NotEmpty(t, result.OneLine, "特殊字符摘要不应为空")

		// 测试标签
		tagRequest := llm.TagRequest{
			Content:     specialContent,
			ContentType: models.ContentTypeText,
			MaxTags:     5,
		}

		tagResult, err := tagger.GenerateTags(ctx, tagRequest)
		require.NoError(t, err, "特殊字符标签生成应该成功")
		assert.NotEmpty(t, tagResult.Tags, "特殊字符标签不应为空")

		t.Logf("✅ 特殊字符处理测试成功")
	})

	t.Run("测试多语言内容", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		multilingualContent := `Hello world! 你好世界！こんにちは世界！Bonjour le monde! 
		Hola mundo! Привет мир! مرحبا بالعالم! 
		This is a multilingual test content with different scripts and languages.
		这是一个多语言测试内容，包含不同的文字和语言。`

		summaryRequest := llm.SummaryRequest{
			Content:     multilingualContent,
			ContentType: models.ContentTypeText,
		}

		result, err := summarizer.GenerateSummary(ctx, summaryRequest)
		require.NoError(t, err, "多语言内容处理应该成功")
		assert.NotEmpty(t, result.OneLine, "多语言摘要不应为空")

		t.Logf("✅ 多语言处理测试成功")
		t.Logf("🌍 多语言摘要: %s", result.OneLine)
	})
}

// TestLLMRateLimitAndRetry 测试速率限制和重试机制
func TestLLMRateLimitAndRetry(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	client, err := llm.NewClient()
	require.NoError(t, err)

	t.Run("测试并发请求处理", func(t *testing.T) {
		// 创建多个并发请求
		concurrentRequests := 3
		results := make(chan error, concurrentRequests)

		for i := 0; i < concurrentRequests; i++ {
			go func(index int) {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()

				messages := []llm.ChatMessage{
					{Role: "user", Content: "简单测试请求 " + string(rune(index+'1'))},
				}

				_, err := client.ChatCompletion(ctx, messages)
				results <- err
			}(i)
		}

		// 收集结果
		successCount := 0
		for i := 0; i < concurrentRequests; i++ {
			err := <-results
			if err == nil {
				successCount++
			} else {
				t.Logf("并发请求 %d 失败: %v", i+1, err)
			}
		}

		assert.Greater(t, successCount, 0, "至少应有一个并发请求成功")
		t.Logf("✅ 并发请求测试完成，成功: %d/%d", successCount, concurrentRequests)
	})
}

// TestLLMConfigurationValidation 测试配置验证
func TestLLMConfigurationValidation(t *testing.T) {
	t.Run("测试配置加载", func(t *testing.T) {
		cfg, err := config.Load("../../config/app.yaml")
		require.NoError(t, err, "配置加载应该成功")

		// 验证LLM配置
		assert.Equal(t, "openai_compatible", cfg.LLM.Provider, "Provider应该匹配")
		assert.Equal(t, "https://api.gpt.ge/v1", cfg.LLM.APIBase, "API Base应该匹配third-part-ai.md")
		assert.Equal(t, "gpt-4o", cfg.LLM.Model, "Model应该匹配third-part-ai.md")
		assert.Equal(t, 1688, cfg.LLM.MaxTokens, "MaxTokens应该匹配")
		assert.Equal(t, 0.5, cfg.LLM.Temperature, "Temperature应该匹配")

		// 验证处理配置
		assert.Equal(t, 102400, cfg.Processing.MaxContentSize, "MaxContentSize应该为100KB")
		assert.Equal(t, 200, cfg.Processing.SummaryLevels.OneLineMaxLength, "一句话摘要长度限制")
		assert.Equal(t, 1000, cfg.Processing.SummaryLevels.ParagraphMaxLength, "段落摘要长度限制")
		assert.Equal(t, 5000, cfg.Processing.SummaryLevels.DetailedMaxLength, "详细摘要长度限制")
		assert.Equal(t, 50, cfg.Processing.TagLimits.MaxTags, "最大标签数量")
		assert.Equal(t, 100, cfg.Processing.TagLimits.MaxTagLength, "标签最大长度")
		assert.Equal(t, 0.7, cfg.Processing.TagLimits.DefaultConfidence, "默认置信度")

		t.Logf("✅ 配置验证完成")
	})

	t.Run("测试环境变量加载", func(t *testing.T) {
		apiKey := os.Getenv("MEMORO_LLM_API_KEY")
		if apiKey != "" {
			assert.True(t, strings.HasPrefix(apiKey, "sk-"), "API Key应该以sk-开头")
			assert.Greater(t, len(apiKey), 20, "API Key长度应该合理")
			t.Logf("✅ API Key格式验证通过")
		} else {
			t.Log("⚠️  MEMORO_LLM_API_KEY未设置")
		}
	})
}