package models

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ContentType 内容类型枚举
type ContentType string

const (
	ContentTypeText  ContentType = "text"
	ContentTypeLink  ContentType = "link"
	ContentTypeFile  ContentType = "file"
	ContentTypeImage ContentType = "image"
	ContentTypeAudio ContentType = "audio"
	ContentTypeVideo ContentType = "video"
)

// IsValidContentType 验证内容类型是否有效
func IsValidContentType(ct ContentType) bool {
	validTypes := []ContentType{
		ContentTypeText, ContentTypeLink, ContentTypeFile,
		ContentTypeImage, ContentTypeAudio, ContentTypeVideo,
	}
	for _, validType := range validTypes {
		if ct == validType {
			return true
		}
	}
	return false
}

// Summary 摘要结构
type Summary struct {
	OneLine   string `json:"one_line" gorm:"column:summary_one_line"`     // 一句话摘要
	Paragraph string `json:"paragraph" gorm:"column:summary_paragraph"`   // 段落摘要
	Detailed  string `json:"detailed" gorm:"column:summary_detailed"`     // 详细摘要
}

// ContentItem 内容项数据模型
type ContentItem struct {
	ID              string                 `json:"id" gorm:"primaryKey"`
	Type            ContentType            `json:"type"`                                                    // text, link, file, image, audio, video
	RawContent      string                 `json:"raw_content"`                                             // 原始内容
	ProcessedData   string                 `json:"-" gorm:"column:processed_data"`                         // 处理后的数据JSON字符串
	Summary         Summary                `json:"summary" gorm:"embedded"`                                 // 多层次摘要
	Tags            string                 `json:"-" gorm:"column:tags"`                                    // 标签列表JSON字符串
	ImportanceScore float64                `json:"importance_score"`                                        // 重要性评分
	VectorID        string                 `json:"vector_id"`                                              // 向量数据库ID
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	UserID          string                 `json:"user_id"`                                                // 用户ID
	
	// 内存中的字段，不存储到数据库
	processedDataMap map[string]interface{} `json:"processed_data" gorm:"-"`
	tagsList         []string               `json:"tags" gorm:"-"`
}

// NewContentItem 创建新的内容项
func NewContentItem(contentType ContentType, rawContent, userID string) *ContentItem {
	// 验证输入
	if !IsValidContentType(contentType) {
		return nil
	}
	if rawContent == "" {
		return nil
	}
	
	now := time.Now()
	return &ContentItem{
		ID:              uuid.New().String(),
		Type:            contentType,
		RawContent:      rawContent,
		UserID:          userID,
		ImportanceScore: 0.0,
		CreatedAt:       now,
		UpdatedAt:       now,
		processedDataMap: make(map[string]interface{}),
		tagsList:        make([]string, 0),
	}
}

// SetSummary 设置摘要
func (c *ContentItem) SetSummary(summary Summary) {
	c.Summary = summary
	c.UpdatedAt = time.Now()
}

// GetSummary 获取摘要
func (c *ContentItem) GetSummary() Summary {
	return c.Summary
}

// SetTags 设置标签列表
func (c *ContentItem) SetTags(tags []string) {
	c.tagsList = make([]string, len(tags))
	copy(c.tagsList, tags)
	c.UpdatedAt = time.Now()
}

// GetTags 获取标签列表
func (c *ContentItem) GetTags() []string {
	if c.tagsList == nil {
		c.tagsList = make([]string, 0)
	}
	return c.tagsList
}

// AddTag 添加单个标签
func (c *ContentItem) AddTag(tag string) {
	if c.tagsList == nil {
		c.tagsList = make([]string, 0)
	}
	
	// 检查标签是否已存在
	for _, existingTag := range c.tagsList {
		if existingTag == tag {
			return // 标签已存在，不重复添加
		}
	}
	
	c.tagsList = append(c.tagsList, tag)
	c.UpdatedAt = time.Now()
}

// SetProcessedData 设置处理后的数据
func (c *ContentItem) SetProcessedData(data map[string]interface{}) {
	if c.processedDataMap == nil {
		c.processedDataMap = make(map[string]interface{})
	}
	
	// 深拷贝数据
	for k, v := range data {
		c.processedDataMap[k] = v
	}
	c.UpdatedAt = time.Now()
}

// GetProcessedData 获取处理后的数据
func (c *ContentItem) GetProcessedData() map[string]interface{} {
	if c.processedDataMap == nil {
		c.processedDataMap = make(map[string]interface{})
	}
	return c.processedDataMap
}

// SetImportanceScore 设置重要性评分
func (c *ContentItem) SetImportanceScore(score float64) error {
	if score < 0.0 || score > 1.0 {
		return fmt.Errorf("importance score must be between 0.0 and 1.0, got %f", score)
	}
	c.ImportanceScore = score
	c.UpdatedAt = time.Now()
	return nil
}

// GetImportanceScore 获取重要性评分
func (c *ContentItem) GetImportanceScore() float64 {
	return c.ImportanceScore
}

// BeforeCreate GORM钩子：创建前执行
func (c *ContentItem) BeforeCreate(tx *gorm.DB) error {
	// 序列化processedDataMap到ProcessedData字段
	if c.processedDataMap != nil && len(c.processedDataMap) > 0 {
		jsonData, err := json.Marshal(c.processedDataMap)
		if err != nil {
			return fmt.Errorf("failed to marshal processed data: %w", err)
		}
		c.ProcessedData = string(jsonData)
	}
	
	// 序列化tagsList到Tags字段
	if c.tagsList != nil && len(c.tagsList) > 0 {
		jsonData, err := json.Marshal(c.tagsList)
		if err != nil {
			return fmt.Errorf("failed to marshal tags: %w", err)
		}
		c.Tags = string(jsonData)
	}
	
	return nil
}

// BeforeUpdate GORM钩子：更新前执行
func (c *ContentItem) BeforeUpdate(tx *gorm.DB) error {
	return c.BeforeCreate(tx) // 使用相同的序列化逻辑
}

// AfterFind GORM钩子：查询后执行
func (c *ContentItem) AfterFind(tx *gorm.DB) error {
	// 反序列化ProcessedData字段到processedDataMap
	if c.ProcessedData != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(c.ProcessedData), &data); err != nil {
			return fmt.Errorf("failed to unmarshal processed data: %w", err)
		}
		c.processedDataMap = data
	} else {
		c.processedDataMap = make(map[string]interface{})
	}
	
	// 反序列化Tags字段到tagsList
	if c.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(c.Tags), &tags); err != nil {
			return fmt.Errorf("failed to unmarshal tags: %w", err)
		}
		c.tagsList = tags
	} else {
		c.tagsList = make([]string, 0)
	}
	
	return nil
}