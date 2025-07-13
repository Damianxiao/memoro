package models

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func TestContentItem_BasicFields(t *testing.T) {
	tests := []struct {
		name        string
		contentType ContentType
		rawContent  string
		userID      string
		expectValid bool
	}{
		{
			name:        "valid text content",
			contentType: ContentTypeText,
			rawContent:  "这是一段测试文本",
			userID:      "test_user_001",
			expectValid: true,
		},
		{
			name:        "valid link content",
			contentType: ContentTypeLink,
			rawContent:  "https://example.com/article",
			userID:      "test_user_001",
			expectValid: true,
		},
		{
			name:        "valid file content",
			contentType: ContentTypeFile,
			rawContent:  "/path/to/document.pdf",
			userID:      "test_user_001",
			expectValid: true,
		},
		{
			name:        "empty raw content should be invalid",
			contentType: ContentTypeText,
			rawContent:  "",
			userID:      "test_user_001",
			expectValid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建ContentItem实例
			item := NewContentItem(tt.contentType, tt.rawContent, tt.userID)
			
			if tt.expectValid {
				require.NotNil(t, item, "ContentItem should not be nil")
				assert.NotEmpty(t, item.ID, "ID should be generated")
				assert.Equal(t, tt.contentType, item.Type, "Content type should match")
				assert.Equal(t, tt.rawContent, item.RawContent, "Raw content should match")
				assert.Equal(t, tt.userID, item.UserID, "User ID should match")
				assert.NotZero(t, item.CreatedAt, "CreatedAt should be set")
				assert.NotZero(t, item.UpdatedAt, "UpdatedAt should be set")
			} else {
				assert.Nil(t, item, "Invalid content should return nil")
			}
		})
	}
}

func TestContentItem_Summary(t *testing.T) {
	item := NewContentItem(ContentTypeText, "测试内容", "test_user")
	require.NotNil(t, item)

	// 测试Summary结构
	summary := Summary{
		OneLine:   "一句话摘要",
		Paragraph: "段落摘要，包含更多细节信息",
		Detailed:  "详细摘要，包含完整的上下文和关键信息点",
	}

	item.SetSummary(summary)
	
	retrievedSummary := item.GetSummary()
	assert.Equal(t, summary.OneLine, retrievedSummary.OneLine)
	assert.Equal(t, summary.Paragraph, retrievedSummary.Paragraph)
	assert.Equal(t, summary.Detailed, retrievedSummary.Detailed)
}

func TestContentItem_Tags(t *testing.T) {
	item := NewContentItem(ContentTypeText, "测试内容", "test_user")
	require.NotNil(t, item)

	// 测试标签操作
	tags := []string{"技术", "AI", "知识管理", "效率"}
	item.SetTags(tags)

	retrievedTags := item.GetTags()
	assert.Equal(t, len(tags), len(retrievedTags))
	for i, tag := range tags {
		assert.Equal(t, tag, retrievedTags[i])
	}

	// 测试添加单个标签
	item.AddTag("新标签")
	updatedTags := item.GetTags()
	assert.Contains(t, updatedTags, "新标签")
	assert.Equal(t, len(tags)+1, len(updatedTags))
}

func TestContentItem_ProcessedData(t *testing.T) {
	item := NewContentItem(ContentTypeLink, "https://example.com", "test_user")
	require.NotNil(t, item)

	// 测试处理后的数据存储
	processedData := map[string]interface{}{
		"title":       "示例网站",
		"description": "这是一个示例网站的描述",
		"keywords":    []string{"示例", "网站", "测试"},
		"extract_time": time.Now().Unix(),
	}

	item.SetProcessedData(processedData)
	
	retrievedData := item.GetProcessedData()
	assert.Equal(t, processedData["title"], retrievedData["title"])
	assert.Equal(t, processedData["description"], retrievedData["description"])
	
	// 验证JSON序列化/反序列化
	jsonData, err := json.Marshal(retrievedData)
	require.NoError(t, err)
	
	var unmarshaled map[string]interface{}
	err = json.Unmarshal(jsonData, &unmarshaled)
	require.NoError(t, err)
	assert.Equal(t, processedData["title"], unmarshaled["title"])
}

func TestContentItem_ImportanceScore(t *testing.T) {
	item := NewContentItem(ContentTypeText, "重要文档内容", "test_user")
	require.NotNil(t, item)

	// 测试重要性评分
	scores := []float64{0.0, 0.5, 0.8, 1.0}
	
	for _, score := range scores {
		item.SetImportanceScore(score)
		assert.Equal(t, score, item.GetImportanceScore())
	}

	// 测试无效评分
	invalidScores := []float64{-0.1, 1.1, 2.0}
	for _, score := range invalidScores {
		err := item.SetImportanceScore(score)
		assert.Error(t, err, "Invalid scores should return error")
	}
}

func TestContentItem_DatabaseOperations(t *testing.T) {
	// 设置内存SQLite数据库
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// 自动迁移
	err = db.AutoMigrate(&ContentItem{})
	require.NoError(t, err)

	// 创建测试数据
	item := NewContentItem(ContentTypeText, "数据库测试内容", "test_user")
	require.NotNil(t, item)

	// 设置完整信息
	item.SetSummary(Summary{
		OneLine:   "测试摘要",
		Paragraph: "测试段落摘要",
		Detailed:  "详细测试摘要",
	})
	item.SetTags([]string{"测试", "数据库"})
	item.SetImportanceScore(0.8)

	// 保存到数据库
	result := db.Create(item)
	require.NoError(t, result.Error)
	assert.Equal(t, int64(1), result.RowsAffected)

	// 从数据库读取
	var retrievedItem ContentItem
	result = db.First(&retrievedItem, "id = ?", item.ID)
	require.NoError(t, result.Error)

	// 验证数据完整性
	assert.Equal(t, item.ID, retrievedItem.ID)
	assert.Equal(t, item.Type, retrievedItem.Type)
	assert.Equal(t, item.RawContent, retrievedItem.RawContent)
	assert.Equal(t, item.UserID, retrievedItem.UserID)

	// 验证嵌入的Summary
	summary := retrievedItem.GetSummary()
	assert.Equal(t, "测试摘要", summary.OneLine)
	assert.Equal(t, "测试段落摘要", summary.Paragraph)
	assert.Equal(t, "详细测试摘要", summary.Detailed)

	// 验证标签
	tags := retrievedItem.GetTags()
	assert.Contains(t, tags, "测试")
	assert.Contains(t, tags, "数据库")

	// 验证重要性评分
	assert.Equal(t, 0.8, retrievedItem.GetImportanceScore())
}

func TestContentType_Validation(t *testing.T) {
	validTypes := []ContentType{
		ContentTypeText,
		ContentTypeLink,
		ContentTypeFile,
		ContentTypeImage,
		ContentTypeAudio,
		ContentTypeVideo,
	}

	for _, contentType := range validTypes {
		assert.True(t, IsValidContentType(contentType), "Content type %s should be valid", contentType)
	}

	// 测试无效类型
	invalidType := ContentType("invalid_type")
	assert.False(t, IsValidContentType(invalidType), "Invalid content type should not be valid")
}