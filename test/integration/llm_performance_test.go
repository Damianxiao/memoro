package integration

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"memoro/internal/config"
	"memoro/internal/models"
	"memoro/internal/services/llm"
)

// TestLLMPerformance 测试LLM性能
func TestLLMPerformance(t *testing.T) {
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

	t.Run("基础响应时间测试", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// 测试简单请求的响应时间
		startTime := time.Now()
		
		messages := []llm.ChatMessage{
			{Role: "user", Content: "你好"},
		}

		response, err := client.ChatCompletion(ctx, messages)
		duration := time.Since(startTime)

		require.NoError(t, err, "基础请求应该成功")
		require.NotNil(t, response, "响应不应为空")

		// 基础性能断言（根据third-part-ai.md的API性能预期）
		assert.Less(t, duration, 30*time.Second, "简单请求应在30秒内完成")
		assert.Greater(t, response.Usage.TotalTokens, 0, "应该有token使用统计")

		t.Logf("✅ 基础响应时间: %v", duration)
		t.Logf("📊 Token使用: %d total", response.Usage.TotalTokens)
	})

	t.Run("摘要生成性能测试", func(t *testing.T) {
		summarizer, err := llm.NewSummarizer(client)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()

		testContent := `人工智能（Artificial Intelligence，AI）是研究、开发用于模拟、延伸和扩展人的智能的理论、方法、技术及应用系统的一门新的技术科学。人工智能是计算机科学的一个分支，它企图了解智能的实质，并生产出一种新的能以人类智能相似的方式做出反应的智能机器。

该领域的研究包括机器人、语言识别、图像识别、自然语言处理和专家系统等。人工智能从诞生以来，理论和技术日益成熟，应用领域也不断扩大，可以设想，未来人工智能带来的科技产品，将会是人类智慧的"容器"。

近年来，深度学习技术的突破推动了人工智能的快速发展。从语音助手到自动驾驶，从推荐系统到智能客服，AI正在改变我们的生活方式。大语言模型的出现更是让AI在理解和生成自然语言方面达到了新的高度。`

		startTime := time.Now()

		request := llm.SummaryRequest{
			Content:     testContent,
			ContentType: models.ContentTypeText,
		}

		result, err := summarizer.GenerateSummary(ctx, request)
		duration := time.Since(startTime)

		require.NoError(t, err, "摘要生成应该成功")
		require.NotNil(t, result, "摘要结果不应为空")

		// 性能断言
		assert.Less(t, duration, 90*time.Second, "摘要生成应在90秒内完成")
		assert.NotEmpty(t, result.OneLine, "一句话摘要不应为空")
		assert.NotEmpty(t, result.Paragraph, "段落摘要不应为空")
		assert.NotEmpty(t, result.Detailed, "详细摘要不应为空")

		t.Logf("✅ 摘要生成性能: %v", duration)
		t.Logf("📝 生成结果长度: 一句话=%d, 段落=%d, 详细=%d", 
			len(result.OneLine), len(result.Paragraph), len(result.Detailed))
	})

	t.Run("标签生成性能测试", func(t *testing.T) {
		tagger, err := llm.NewTagger(client)
		require.NoError(t, err)

		ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
		defer cancel()

		testContent := `机器学习是人工智能的核心技术之一。它通过算法让计算机从数据中学习模式，无需明确编程就能做出预测或决策。深度学习作为机器学习的子集，使用多层神经网络处理复杂数据。这些技术在图像识别、自然语言处理、推荐系统等领域都有广泛应用。`

		startTime := time.Now()

		request := llm.TagRequest{
			Content:     testContent,
			ContentType: models.ContentTypeText,
			MaxTags:     10,
		}

		result, err := tagger.GenerateTags(ctx, request)
		duration := time.Since(startTime)

		require.NoError(t, err, "标签生成应该成功")
		require.NotNil(t, result, "标签结果不应为空")

		// 性能和质量断言
		assert.Less(t, duration, 60*time.Second, "标签生成应在60秒内完成")
		assert.NotEmpty(t, result.Tags, "标签列表不应为空")
		assert.LessOrEqual(t, len(result.Tags), 10, "标签数量不应超过限制")

		t.Logf("✅ 标签生成性能: %v", duration)
		t.Logf("🏷️ 生成结果: 标签=%d个, 分类=%d个, 关键词=%d个", 
			len(result.Tags), len(result.Categories), len(result.Keywords))
	})
}

// TestLLMConcurrentPerformance 测试并发性能
func TestLLMConcurrentPerformance(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	t.Run("并发摘要生成测试", func(t *testing.T) {
		concurrentRequests := 3 // 避免超出API速率限制
		results := make(chan TestResult, concurrentRequests)

		testContents := []string{
			"今天天气很好，适合户外活动。",
			"人工智能技术正在快速发展，改变着我们的生活。",
			"深度学习是机器学习的重要分支，具有强大的数据处理能力。",
		}

		startTime := time.Now()

		// 启动并发请求
		for i := 0; i < concurrentRequests; i++ {
			go func(index int) {
				client, err := llm.NewClient()
				if err != nil {
					results <- TestResult{Error: err, Duration: 0}
					return
				}

				summarizer, err := llm.NewSummarizer(client)
				if err != nil {
					results <- TestResult{Error: err, Duration: 0}
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				requestStart := time.Now()

				request := llm.SummaryRequest{
					Content:     testContents[index%len(testContents)],
					ContentType: models.ContentTypeText,
				}

				_, err = summarizer.GenerateSummary(ctx, request)
				duration := time.Since(requestStart)

				results <- TestResult{Error: err, Duration: duration}
			}(i)
		}

		// 收集结果
		successCount := 0
		var totalDuration time.Duration
		for i := 0; i < concurrentRequests; i++ {
			result := <-results
			if result.Error == nil {
				successCount++
				totalDuration += result.Duration
			} else {
				t.Logf("并发请求 %d 失败: %v", i+1, result.Error)
			}
		}

		overallDuration := time.Since(startTime)

		// 并发性能断言
		assert.Greater(t, successCount, 0, "至少应有一个请求成功")
		
		if successCount > 0 {
			avgDuration := totalDuration / time.Duration(successCount)
			t.Logf("✅ 并发摘要测试完成")
			t.Logf("📊 成功率: %d/%d", successCount, concurrentRequests)
			t.Logf("⏱️ 总耗时: %v", overallDuration)
			t.Logf("⏱️ 平均单请求耗时: %v", avgDuration)
		}
	})

	t.Run("并发标签生成测试", func(t *testing.T) {
		concurrentRequests := 2 // 保持较低并发以避免API限制
		results := make(chan TestResult, concurrentRequests)

		testContents := []string{
			"机器学习算法在数据科学中发挥重要作用。",
			"云计算为企业提供了灵活的IT基础设施。",
		}

		for i := 0; i < concurrentRequests; i++ {
			go func(index int) {
				client, err := llm.NewClient()
				if err != nil {
					results <- TestResult{Error: err, Duration: 0}
					return
				}

				tagger, err := llm.NewTagger(client)
				if err != nil {
					results <- TestResult{Error: err, Duration: 0}
					return
				}

				ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
				defer cancel()

				requestStart := time.Now()

				request := llm.TagRequest{
					Content:     testContents[index%len(testContents)],
					ContentType: models.ContentTypeText,
					MaxTags:     5,
				}

				_, err = tagger.GenerateTags(ctx, request)
				duration := time.Since(requestStart)

				results <- TestResult{Error: err, Duration: duration}
			}(i)
		}

		// 收集结果
		successCount := 0
		for i := 0; i < concurrentRequests; i++ {
			result := <-results
			if result.Error == nil {
				successCount++
			}
		}

		assert.Greater(t, successCount, 0, "至少应有一个标签生成请求成功")
		t.Logf("✅ 并发标签测试完成，成功率: %d/%d", successCount, concurrentRequests)
	})
}

// TestLLMTokenUsageAccuracy 测试Token使用统计准确性
func TestLLMTokenUsageAccuracy(t *testing.T) {
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

	t.Run("Token统计准确性测试", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		testCases := []struct {
			name     string
			content  string
			expected TokenExpectation
		}{
			{
				name:    "短消息",
				content: "你好",
				expected: TokenExpectation{MinPrompt: 1, MaxPrompt: 10, MinCompletion: 1, MaxCompletion: 20},
			},
			{
				name:    "中等长度消息",
				content: "请解释一下人工智能的基本概念",
				expected: TokenExpectation{MinPrompt: 5, MaxPrompt: 30, MinCompletion: 10, MaxCompletion: 100},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				messages := []llm.ChatMessage{
					{Role: "user", Content: tc.content},
				}

				response, err := client.ChatCompletion(ctx, messages)
				require.NoError(t, err, "请求应该成功")

				usage := response.Usage

				// 验证Token统计的合理性
				assert.GreaterOrEqual(t, usage.PromptTokens, tc.expected.MinPrompt, "Prompt tokens应该在合理范围内")
				assert.LessOrEqual(t, usage.PromptTokens, tc.expected.MaxPrompt, "Prompt tokens应该在合理范围内")
				assert.GreaterOrEqual(t, usage.CompletionTokens, tc.expected.MinCompletion, "Completion tokens应该在合理范围内")
				assert.LessOrEqual(t, usage.CompletionTokens, tc.expected.MaxCompletion, "Completion tokens应该在合理范围内")
				assert.Equal(t, usage.TotalTokens, usage.PromptTokens+usage.CompletionTokens, "Total tokens应该等于prompt + completion")

				t.Logf("📊 %s Token使用: %d prompt + %d completion = %d total", 
					tc.name, usage.PromptTokens, usage.CompletionTokens, usage.TotalTokens)
			})
		}
	})
}

// TestResult 测试结果结构
type TestResult struct {
	Error    error
	Duration time.Duration
}

// TokenExpectation Token使用预期
type TokenExpectation struct {
	MinPrompt     int
	MaxPrompt     int
	MinCompletion int
	MaxCompletion int
}