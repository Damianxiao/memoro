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

// TestLLMAPIConnection 测试LLM API基础连接
func TestLLMAPIConnection(t *testing.T) {
	// 检查是否有API Key环境变量
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	cfg, err := config.Load("../../config/app.yaml")
	require.NoError(t, err, "Failed to load config")
	require.NotEmpty(t, cfg.LLM.APIKey, "LLM API key should be loaded from environment")

	// 创建LLM客户端
	client, err := llm.NewClient()
	require.NoError(t, err, "Failed to create LLM client")
	require.NotNil(t, client, "LLM client should not be nil")

	// 测试基础聊天功能（使用third-part-ai.md中的示例）
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages := []llm.ChatMessage{
		{Role: "user", Content: "晚上好"},
	}

	response, err := client.ChatCompletion(ctx, messages)
	require.NoError(t, err, "API call should succeed")
	require.NotNil(t, response, "Response should not be nil")

	// 验证响应格式符合third-part-ai.md规范
	assert.NotEmpty(t, response.ID, "Response should have ID")
	assert.Equal(t, "chat.completion", response.Object, "Object should be chat.completion")
	assert.Greater(t, response.Created, int64(0), "Created timestamp should be positive")
	assert.NotEmpty(t, response.Model, "Model should not be empty")
	assert.NotEmpty(t, response.Choices, "Choices should not be empty")
	assert.Greater(t, response.Usage.TotalTokens, 0, "Total tokens should be positive")

	// 验证第一个选择的内容
	choice := response.Choices[0]
	assert.Equal(t, 0, choice.Index, "First choice index should be 0")
	assert.Equal(t, "assistant", choice.Message.Role, "Response role should be assistant")
	assert.NotEmpty(t, choice.Message.Content, "Response content should not be empty")
	assert.Equal(t, "stop", choice.FinishReason, "Finish reason should be stop")

	t.Logf("✅ API连接测试成功")
	t.Logf("📊 Token使用: %d prompt + %d completion = %d total", 
		response.Usage.PromptTokens, 
		response.Usage.CompletionTokens, 
		response.Usage.TotalTokens)
	t.Logf("🤖 响应内容: %s", choice.Message.Content)
}

// TestLLMSimpleCompletion 测试简单完成功能
func TestLLMSimpleCompletion(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置并创建客户端
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	client, err := llm.NewClient()
	require.NoError(t, err)

	// 测试简单完成
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	systemPrompt := "你是一个专业的内容摘要助手。"
	userPrompt := "请为以下内容生成一句话摘要：人工智能技术正在快速发展，特别是在自然语言处理领域取得了重大突破。"

	response, err := client.SimpleCompletion(ctx, systemPrompt, userPrompt)
	require.NoError(t, err, "Simple completion should succeed")
	assert.NotEmpty(t, response, "Response should not be empty")

	t.Logf("✅ 简单完成测试成功")
	t.Logf("📝 生成的摘要: %s", response)
}

// TestLLMSummarizerIntegration 测试摘要生成器集成
func TestLLMSummarizerIntegration(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	// 创建客户端和摘要生成器
	client, err := llm.NewClient()
	require.NoError(t, err)

	summarizer, err := llm.NewSummarizer(client)
	require.NoError(t, err)
	require.NotNil(t, summarizer, "Summarizer should not be nil")

	// 测试摘要生成
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	testContent := `人工智能（AI）是计算机科学的一个分支，它试图创建能够以类似人类智能的方式感知、学习、推理和解决问题的机器和软件。AI的发展历程可以追溯到20世纪50年代，当时科学家们开始探索让机器模拟人类思维的可能性。

近年来，随着深度学习技术的突破，AI在各个领域都取得了显著进展。从语音识别到图像处理，从自然语言处理到自动驾驶，AI技术正在改变我们的生活方式。特别是大语言模型的出现，使得AI在理解和生成人类语言方面达到了前所未有的水平。

然而，AI的快速发展也带来了新的挑战和伦理问题。如何确保AI系统的安全性、公平性和透明度，如何处理AI可能带来的就业影响，这些都是需要我们认真考虑的问题。`

	request := llm.SummaryRequest{
		Content:     testContent,
		ContentType: models.ContentTypeText,
		Context:     map[string]interface{}{"topic": "人工智能"},
	}

	result, err := summarizer.GenerateSummary(ctx, request)
	require.NoError(t, err, "Summary generation should succeed")
	require.NotNil(t, result, "Summary result should not be nil")

	// 验证三层次摘要
	assert.NotEmpty(t, result.OneLine, "一句话摘要不应为空")
	assert.NotEmpty(t, result.Paragraph, "段落摘要不应为空")
	assert.NotEmpty(t, result.Detailed, "详细摘要不应为空")

	// 验证摘要长度限制
	assert.LessOrEqual(t, len(result.OneLine), 200, "一句话摘要应不超过200字符")
	assert.LessOrEqual(t, len(result.Paragraph), 1000, "段落摘要应不超过1000字符")
	assert.LessOrEqual(t, len(result.Detailed), 5000, "详细摘要应不超过5000字符")

	t.Logf("✅ 摘要生成测试成功")
	t.Logf("📄 一句话摘要 (%d字符): %s", len(result.OneLine), result.OneLine)
	t.Logf("📄 段落摘要 (%d字符): %s", len(result.Paragraph), result.Paragraph)
	t.Logf("📄 详细摘要 (%d字符): %s", len(result.Detailed), result.Detailed)
}

// TestLLMTaggerIntegration 测试标签生成器集成
func TestLLMTaggerIntegration(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	// 创建客户端和标签生成器
	client, err := llm.NewClient()
	require.NoError(t, err)

	tagger, err := llm.NewTagger(client)
	require.NoError(t, err)
	require.NotNil(t, tagger, "Tagger should not be nil")

	// 测试标签生成
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	testContent := `机器学习是人工智能的一个重要分支，它让计算机能够在没有明确编程的情况下学习和改进。深度学习作为机器学习的子集，使用多层神经网络来分析数据。这些技术在图像识别、语音处理和自然语言理解等领域取得了突破性进展。`

	request := llm.TagRequest{
		Content:     testContent,
		ContentType: models.ContentTypeText,
		MaxTags:     10,
		Context:     map[string]interface{}{"domain": "technology"},
	}

	result, err := tagger.GenerateTags(ctx, request)
	require.NoError(t, err, "Tag generation should succeed")
	require.NotNil(t, result, "Tag result should not be nil")

	// 验证标签结果
	assert.NotEmpty(t, result.Tags, "标签列表不应为空")
	assert.LessOrEqual(t, len(result.Tags), 10, "标签数量不应超过最大限制")
	assert.NotEmpty(t, result.Categories, "分类列表不应为空")
	assert.NotEmpty(t, result.Keywords, "关键词列表不应为空")
	assert.NotEmpty(t, result.Confidence, "置信度映射不应为空")

	// 验证标签长度
	for _, tag := range result.Tags {
		assert.LessOrEqual(t, len(tag), 100, "标签长度不应超过100字符")
		assert.NotEmpty(t, tag, "标签不应为空")
	}

	// 验证置信度
	for tag, confidence := range result.Confidence {
		assert.GreaterOrEqual(t, confidence, 0.0, "置信度应大于等于0")
		assert.LessOrEqual(t, confidence, 1.0, "置信度应小于等于1")
		assert.Contains(t, result.Tags, tag, "置信度映射中的标签应在标签列表中")
	}

	t.Logf("✅ 标签生成测试成功")
	t.Logf("🏷️ 生成标签 (%d个): %v", len(result.Tags), result.Tags)
	t.Logf("📂 内容分类: %v", result.Categories)
	t.Logf("🔑 关键词: %v", result.Keywords)
}

// TestLLMQuickFunctions 测试快速功能
func TestLLMQuickFunctions(t *testing.T) {
	// 检查API Key
	apiKey := os.Getenv("MEMORO_LLM_API_KEY")
	if apiKey == "" {
		t.Skip("Skipping LLM API test: MEMORO_LLM_API_KEY not set")
	}

	// 加载配置
	_, err := config.Load("../../config/app.yaml")
	require.NoError(t, err)

	// 创建客户端
	client, err := llm.NewClient()
	require.NoError(t, err)

	// 测试快速摘要
	summarizer, err := llm.NewSummarizer(client)
	require.NoError(t, err)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	quickSummary, err := summarizer.GenerateQuickSummary(ctx, "今天天气很好，适合外出活动。", models.ContentTypeText)
	require.NoError(t, err, "Quick summary should succeed")
	assert.NotEmpty(t, quickSummary, "Quick summary should not be empty")

	// 测试简单标签
	tagger, err := llm.NewTagger(client)
	require.NoError(t, err)

	simpleTags, err := tagger.GenerateSimpleTags(ctx, "今天是个好日子，阳光明媚。", models.ContentTypeText, 5)
	require.NoError(t, err, "Simple tags should succeed")
	assert.NotEmpty(t, simpleTags, "Simple tags should not be empty")
	assert.LessOrEqual(t, len(simpleTags), 5, "Tag count should not exceed limit")

	t.Logf("✅ 快速功能测试成功")
	t.Logf("⚡ 快速摘要: %s", quickSummary)
	t.Logf("⚡ 简单标签: %v", simpleTags)
}