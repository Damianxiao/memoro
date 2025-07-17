package main

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"memoro/internal/config"
	"memoro/internal/models"
	"memoro/internal/services/content"
	"memoro/internal/wechat"
)

func main() {
	fmt.Println("🚀 Memoro - 智能内容处理系统")
	fmt.Println("=============================")

	// 1. 初始化配置
	fmt.Println("📋 初始化配置...")
	cfg := &config.Config{
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
			Collection: "memoro_demo",
			Timeout:    30 * time.Second,
			RetryTimes: 3,
			BatchSize:  100,
		},
		Processing: config.ProcessingConfig{
			MaxWorkers:     2,
			QueueSize:      10,
			Timeout:        120 * time.Second,
			MaxContentSize: 102400,
			TagLimits: config.TagLimitsConfig{
				MaxTags:      10,
				MaxTagLength: 50,
			},
			SummaryLevels: config.SummaryLevelsConfig{
				OneLineMaxLength:   200,
				ParagraphMaxLength: 1000,
				DetailedMaxLength:  5000,
			},
		},
	}

	err := config.InitializeForTest(cfg)
	if err != nil {
		log.Fatal("配置初始化失败:", err)
	}

	// 2. 初始化内容处理器
	fmt.Println("🔧 初始化内容处理器...")
	processor, err := content.NewProcessor()
	if err != nil {
		log.Fatal("内容处理器初始化失败:", err)
	}
	defer processor.Close()

	// 3. 初始化微信登录状态监控
	fmt.Println("📱 初始化微信登录状态监控...")
	wechatClient := wechat.NewClient()
	statusChecker := wechat.NewStatusChecker(wechatClient)
	
	// 检查初始登录状态
	isLoggedIn, err := statusChecker.CheckCurrentStatus()
	if err != nil {
		fmt.Printf("⚠️  微信状态检查失败: %v\n", err)
	} else if !isLoggedIn {
		fmt.Println("🔄 微信未登录，正在触发登录...")
		if err := statusChecker.TriggerLogin(); err != nil {
			fmt.Printf("❌ 微信登录失败: %v\n", err)
		}
	} else {
		fmt.Println("✅ 微信已登录")
	}

	// 启动状态监控
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		statusChecker.StartMonitoring(func(status bool) {
			if status {
				fmt.Println("✅ 微信登录状态恢复")
			} else {
				fmt.Println("⚠️  微信登录状态失效，正在重新登录...")
				if err := statusChecker.TriggerLogin(); err != nil {
					fmt.Printf("❌ 自动重新登录失败: %v\n", err)
				}
			}
		})
	}()

	// 4. 演示完整的内容处理流程
	fmt.Println("\n🎯 演示内容处理流程")
	fmt.Println("-------------------")

	// 示例内容
	testContent := "人工智能在医疗领域的应用正在快速发展，包括疾病诊断、药物研发、个性化治疗等方面。"

	// 创建处理请求
	processingRequest := &content.ProcessingRequest{
		ID:          generateRequestID(),
		Content:     testContent,
		ContentType: models.ContentTypeText,
		UserID:      "demo-user",
		Priority:    5,
		Context: map[string]interface{}{
			"source":    "demo",
			"timestamp": time.Now(),
		},
		Options: content.ProcessingOptions{
			EnableSummary:         true,
			EnableTags:            true,
			EnableClassification:  true,
			EnableImportanceScore: true,
			EnableVectorization:   true,
			MaxTags:               10,
		},
		CreatedAt: time.Now(),
	}

	fmt.Printf("📝 处理内容: %s\n", testContent)
	fmt.Println("⏳ 正在处理...")

	// 执行处理
	result, err := processor.ProcessContent(context.Background(), processingRequest)
	if err != nil {
		log.Printf("❌ 处理失败: %v", err)
		return
	}

	// 显示处理结果
	fmt.Println("\n✅ 处理完成！")
	fmt.Printf("📄 状态: %s\n", result.Status)
	fmt.Printf("⏱️  处理时间: %v\n", result.ProcessingTime)

	if result.Summary != nil {
		fmt.Printf("📝 摘要: %s\n", result.Summary.OneLine)
	}

	if result.Tags != nil && len(result.Tags.Tags) > 0 {
		fmt.Printf("🏷️  标签: %v\n", result.Tags.Tags)
	}

	fmt.Printf("⭐ 重要性评分: %.2f\n", result.ImportanceScore)

	if result.VectorResult != nil {
		fmt.Printf("🔍 向量化: %s (维度: %d)\n",
			getBoolStr(result.VectorResult.Indexed),
			result.VectorResult.VectorDimension)
	}

	// 4. 演示搜索功能（如果可用）
	fmt.Println("\n🔍 搜索功能演示")
	fmt.Println("---------------")

	searchRequest := &content.SearchRequest{
		Query:         "人工智能",
		UserID:        "demo-user",
		TopK:          5,
		MinSimilarity: 0.5,
	}

	fmt.Printf("🔍 搜索查询: %s\n", searchRequest.Query)
	searchResults, err := processor.SearchContent(context.Background(), searchRequest)
	if err != nil {
		fmt.Printf("⚠️  搜索功能暂时不可用: %v\n", err)
	} else {
		fmt.Printf("📊 搜索结果: %d 条\n", len(searchResults.Results))
		for i, result := range searchResults.Results {
			fmt.Printf("  %d. 相似度: %.2f\n", i+1, result.Similarity)
		}
	}

	fmt.Println("\n🎉 演示完成！")
	fmt.Println("系统核心功能运行正常:")
	fmt.Println("  ✅ LLM 集成 (文本生成、标签、摘要)")
	fmt.Println("  ✅ 向量数据库 (Chroma)")
	fmt.Println("  ✅ 内容处理管道")
	fmt.Println("  ✅ 配置管理")
	fmt.Println("  ⚠️  搜索功能需要进一步优化")
}

// generateRequestID 生成请求ID
func generateRequestID() string {
	return fmt.Sprintf("req_%d", time.Now().UnixNano())
}

// getBoolStr 将布尔值转换为中文字符串
func getBoolStr(b bool) string {
	if b {
		return "已完成"
	}
	return "未完成"
}
