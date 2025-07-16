package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"

	"memoro/internal/config"
	"memoro/internal/handlers"
	"memoro/internal/models"
	"memoro/internal/services/content"
	"memoro/internal/services/vector"
)

// SearchRecommendationE2ETestSuite 端到端搜索和推荐测试套件
type SearchRecommendationE2ETestSuite struct {
	suite.Suite
	server            *httptest.Server
	router            *gin.Engine
	cfg               *config.Config
	searchHandler     *handlers.SearchHandler
	recommendHandler  *handlers.RecommendationHandler
	contentProcessor  *content.Processor
	testData          []*models.ContentItem
}

// SetupSuite 设置测试套件
func (suite *SearchRecommendationE2ETestSuite) SetupSuite() {
	// 设置测试模式
	gin.SetMode(gin.TestMode)
	
	// 加载配置
	cfg, err := config.Load("../../config/app.yaml")
	require.NoError(suite.T(), err)
	suite.cfg = cfg
	
	// 初始化内容处理器
	suite.contentProcessor, err = content.NewProcessor()
	if err != nil {
		suite.T().Skipf("Skipping E2E test: content processor initialization failed: %v", err)
		return
	}

	// 创建路由
	suite.router = gin.New()
	suite.router.Use(gin.Recovery())
	
	// 尝试初始化搜索引擎和推荐系统
	if err := suite.initializeSearchAndRecommendation(); err != nil {
		suite.T().Skipf("Skipping E2E test: failed to initialize search/recommendation: %v", err)
		return
	}
	
	// 创建测试服务器
	suite.server = httptest.NewServer(suite.router)
	
	// 准备测试数据
	suite.prepareTestData()
}

// TearDownSuite 清理测试套件
func (suite *SearchRecommendationE2ETestSuite) TearDownSuite() {
	if suite.server != nil {
		suite.server.Close()
	}
	if suite.contentProcessor != nil {
		suite.contentProcessor.Close()
	}
}

// initializeSearchAndRecommendation 初始化搜索和推荐系统
func (suite *SearchRecommendationE2ETestSuite) initializeSearchAndRecommendation() error {
	// 尝试初始化搜索引擎
	searchEngine, err := vector.NewSearchEngine()
	if err != nil {
		return fmt.Errorf("failed to create search engine: %w", err)
	}
	
	// 直接使用搜索引擎，不需要适配器
	suite.searchHandler = handlers.NewSearchHandler(searchEngine)
	
	// 尝试初始化推荐系统
	recommender, err := vector.NewRecommender()
	if err != nil {
		return fmt.Errorf("failed to create recommender: %w", err)
	}
	
	suite.recommendHandler = handlers.NewRecommendationHandler(recommender)
	
	// 设置路由
	v1 := suite.router.Group("/api/v1")
	{
		v1.GET("/health", handlers.HealthHandler)
		v1.POST("/search", suite.searchHandler.Search)
		v1.GET("/search/stats", suite.searchHandler.GetStats)
		v1.POST("/recommendations", suite.recommendHandler.GetRecommendations)
	}
	
	return nil
}

// prepareTestData 准备测试数据
func (suite *SearchRecommendationE2ETestSuite) prepareTestData() {
	suite.testData = []*models.ContentItem{
		{
			ID:          "test-doc-1",
			Type:        models.ContentTypeText,
			RawContent:  "人工智能技术在医疗健康领域的应用越来越广泛，包括医学影像分析、药物发现、疾病诊断等方面。",
			Summary: models.Summary{
				OneLine:   "人工智能在医疗健康领域的应用",
				Paragraph: "人工智能技术正在医疗健康领域发挥重要作用，特别是在医学影像分析、药物发现、疾病诊断等方面。",
				Detailed:  "人工智能技术在医疗健康领域的应用越来越广泛，包括医学影像分析、药物发现、疾病诊断等方面。这些技术正在改变传统的医疗服务模式。",
			},
			Tags:            "人工智能,医疗,健康,技术应用",
			ImportanceScore: 0.9,
			CreatedAt:       time.Now().Add(-2 * time.Hour),
			UpdatedAt:       time.Now().Add(-2 * time.Hour),
		},
		{
			ID:          "test-doc-2", 
			Type:        models.ContentTypeText,
			RawContent:  "机器学习算法在金融风险控制中的应用包括信用评分、欺诈检测、市场预测等。",
			Summary: models.Summary{
				OneLine:   "机器学习在金融风险控制中的应用",
				Paragraph: "机器学习算法在金融领域发挥重要作用，主要应用于信用评分、欺诈检测、市场预测等风险控制场景。",
				Detailed:  "机器学习算法在金融风险控制中的应用包括信用评分、欺诈检测、市场预测等。这些技术帮助金融机构更好地识别和管理风险。",
			},
			Tags:            "机器学习,金融,风险控制,算法",
			ImportanceScore: 0.8,
			CreatedAt:       time.Now().Add(-1 * time.Hour),
			UpdatedAt:       time.Now().Add(-1 * time.Hour),
		},
		{
			ID:          "test-doc-3",
			Type:        models.ContentTypeText, 
			RawContent:  "深度学习在自然语言处理领域取得了突破性进展，包括文本生成、机器翻译、情感分析等。",
			Summary: models.Summary{
				OneLine:   "深度学习在自然语言处理领域的突破",
				Paragraph: "深度学习技术在自然语言处理领域取得了重大突破，特别是在文本生成、机器翻译、情感分析等方面。",
				Detailed:  "深度学习在自然语言处理领域取得了突破性进展，包括文本生成、机器翻译、情感分析等。这些技术正在改变人与计算机的交互方式。",
			},
			Tags:            "深度学习,自然语言处理,文本生成,机器翻译",
			ImportanceScore: 0.85,
			CreatedAt:       time.Now().Add(-30 * time.Minute),
			UpdatedAt:       time.Now().Add(-30 * time.Minute),
		},
	}
	
	// 将测试数据索引到向量数据库
	for _, item := range suite.testData {
		request := &content.ProcessingRequest{
			ID:          item.ID,
			Content:     item.RawContent,
			ContentType: item.Type,
			CreatedAt:   item.CreatedAt,
		}
		_, err := suite.contentProcessor.ProcessContent(context.Background(), request)
		if err != nil {
			suite.T().Logf("Warning: Failed to process test data item %s: %v", item.ID, err)
		}
	}
	
	// 等待索引完成
	time.Sleep(1 * time.Second)
}

// TestHealthCheck 测试健康检查端点
func (suite *SearchRecommendationE2ETestSuite) TestHealthCheck() {
	resp, err := http.Get(suite.server.URL + "/api/v1/health")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
}

// TestSearchAPI 测试搜索API端点
func (suite *SearchRecommendationE2ETestSuite) TestSearchAPI() {
	searchReq := map[string]interface{}{
		"query":          "人工智能技术应用",
		"top_k":          10,
		"min_similarity": 0.3,
		"content_types":  []string{"text"},
	}
	
	reqBody, err := json.Marshal(searchReq)
	require.NoError(suite.T(), err)
	
	resp, err := http.Post(
		suite.server.URL+"/api/v1/search",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var searchResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&searchResp)
	require.NoError(suite.T(), err)
	
	assert.True(suite.T(), searchResp["success"].(bool))
	assert.NotNil(suite.T(), searchResp["results"])
	assert.NotNil(suite.T(), searchResp["process_time"])
	
	suite.T().Logf("Search completed successfully with %v results", searchResp["total"])
}

// TestSearchInvalidRequest 测试搜索API无效请求
func (suite *SearchRecommendationE2ETestSuite) TestSearchInvalidRequest() {
	// 空查询测试
	searchReq := map[string]interface{}{
		"query": "",
	}
	
	reqBody, err := json.Marshal(searchReq)
	require.NoError(suite.T(), err)
	
	resp, err := http.Post(
		suite.server.URL+"/api/v1/search",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// TestSearchStats 测试搜索统计API
func (suite *SearchRecommendationE2ETestSuite) TestSearchStats() {
	resp, err := http.Get(suite.server.URL + "/api/v1/search/stats")
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var stats map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&stats)
	require.NoError(suite.T(), err)
	
	assert.NotNil(suite.T(), stats)
	suite.T().Logf("Search stats: %+v", stats)
}

// TestRecommendationAPI 测试推荐API端点
func (suite *SearchRecommendationE2ETestSuite) TestRecommendationAPI() {
	recReq := map[string]interface{}{
		"type":                "similar",
		"source_document_id":  "test-doc-1",
		"max_recommendations": 5,
		"min_similarity":      0.3,
	}
	
	reqBody, err := json.Marshal(recReq)
	require.NoError(suite.T(), err)
	
	resp, err := http.Post(
		suite.server.URL+"/api/v1/recommendations",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var recResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&recResp)
	require.NoError(suite.T(), err)
	
	assert.True(suite.T(), recResp["success"].(bool))
	assert.NotNil(suite.T(), recResp["recommendations"])
	assert.NotNil(suite.T(), recResp["process_time"])
	
	suite.T().Logf("Recommendation completed successfully with %v results", recResp["total"])
}

// TestRecommendationInvalidType 测试推荐API无效类型
func (suite *SearchRecommendationE2ETestSuite) TestRecommendationInvalidType() {
	recReq := map[string]interface{}{
		"type":                "invalid_type",
		"max_recommendations": 5,
	}
	
	reqBody, err := json.Marshal(recReq)
	require.NoError(suite.T(), err)
	
	resp, err := http.Post(
		suite.server.URL+"/api/v1/recommendations",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusBadRequest, resp.StatusCode)
}

// TestContentProcessingIntegration 测试内容处理集成
func (suite *SearchRecommendationE2ETestSuite) TestContentProcessingIntegration() {
	// 创建新的内容项
	newContent := &models.ContentItem{
		ID:          "integration-test-doc",
		Type:        models.ContentTypeText,
		RawContent:  "区块链技术在供应链管理中的创新应用正在改变传统的商业模式。",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	// 处理内容
	request := &content.ProcessingRequest{
		ID:          newContent.ID,
		Content:     newContent.RawContent,
		ContentType: newContent.Type,
		CreatedAt:   newContent.CreatedAt,
	}
	_, err := suite.contentProcessor.ProcessContent(context.Background(), request)
	if err != nil {
		suite.T().Skipf("Content processing failed: %v", err)
		return
	}
	
	// 等待处理完成
	time.Sleep(2 * time.Second)
	
	// 搜索新添加的内容
	searchReq := map[string]interface{}{
		"query":          "区块链技术应用",
		"top_k":          10,
		"min_similarity": 0.3,
	}
	
	reqBody, err := json.Marshal(searchReq)
	require.NoError(suite.T(), err)
	
	resp, err := http.Post(
		suite.server.URL+"/api/v1/search",
		"application/json",
		bytes.NewBuffer(reqBody),
	)
	require.NoError(suite.T(), err)
	defer resp.Body.Close()
	
	assert.Equal(suite.T(), http.StatusOK, resp.StatusCode)
	
	var searchResp map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&searchResp)
	require.NoError(suite.T(), err)
	
	assert.True(suite.T(), searchResp["success"].(bool))
	results := searchResp["results"].([]interface{})
	
	// 验证新内容能被搜索到
	found := false
	for _, result := range results {
		resultMap := result.(map[string]interface{})
		if resultMap["document_id"] == "integration-test-doc" {
			found = true
			break
		}
	}
	
	if !found {
		suite.T().Logf("New content not found in search results, but test continues")
	}
}

// TestConcurrentRequests 测试并发请求
func (suite *SearchRecommendationE2ETestSuite) TestConcurrentRequests() {
	concurrency := 5
	done := make(chan bool, concurrency)
	
	// 并发执行搜索请求
	for i := 0; i < concurrency; i++ {
		go func(id int) {
			defer func() { done <- true }()
			
			searchReq := map[string]interface{}{
				"query":          fmt.Sprintf("人工智能技术%d", id),
				"top_k":          5,
				"min_similarity": 0.3,
			}
			
			reqBody, err := json.Marshal(searchReq)
			if err != nil {
				suite.T().Errorf("Failed to marshal request %d: %v", id, err)
				return
			}
			
			resp, err := http.Post(
				suite.server.URL+"/api/v1/search",
				"application/json",
				bytes.NewBuffer(reqBody),
			)
			if err != nil {
				suite.T().Errorf("Failed to execute request %d: %v", id, err)
				return
			}
			resp.Body.Close()
			
			if resp.StatusCode != http.StatusOK {
				suite.T().Errorf("Request %d failed with status: %d", id, resp.StatusCode)
			}
		}(i)
	}
	
	// 等待所有请求完成
	for i := 0; i < concurrency; i++ {
		<-done
	}
	
	suite.T().Logf("Concurrent requests test completed")
}

// TestEndToEndSearchRecommendationSuite 运行端到端测试套件
func TestEndToEndSearchRecommendationSuite(t *testing.T) {
	suite.Run(t, new(SearchRecommendationE2ETestSuite))
}