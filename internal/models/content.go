package models

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"memoro/internal/errors"
	"memoro/internal/logger"
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
	OneLine   string `json:"one_line" gorm:"column:summary_one_line"`   // 一句话摘要
	Paragraph string `json:"paragraph" gorm:"column:summary_paragraph"` // 段落摘要
	Detailed  string `json:"detailed" gorm:"column:summary_detailed"`   // 详细摘要
}

// Validate 验证摘要数据
func (s *Summary) Validate() error {
	if s.OneLine != "" && len(s.OneLine) > 200 {
		return errors.ErrValidationFailed("summary.one_line", "must not exceed 200 characters")
	}
	if s.Paragraph != "" && len(s.Paragraph) > 1000 {
		return errors.ErrValidationFailed("summary.paragraph", "must not exceed 1000 characters")
	}
	if s.Detailed != "" && len(s.Detailed) > 5000 {
		return errors.ErrValidationFailed("summary.detailed", "must not exceed 5000 characters")
	}
	return nil
}

// ContentItem 内容项数据模型
type ContentItem struct {
	ID              string      `json:"id" gorm:"primaryKey"`
	Type            ContentType `json:"type"`                           // text, link, file, image, audio, video
	RawContent      string      `json:"raw_content"`                    // 原始内容
	ProcessedData   string      `json:"-" gorm:"column:processed_data"` // 处理后的数据JSON字符串
	Summary         Summary     `json:"summary" gorm:"embedded"`        // 多层次摘要
	Tags            string      `json:"-" gorm:"column:tags"`           // 标签列表JSON字符串
	ImportanceScore float64     `json:"importance_score"`               // 重要性评分
	VectorID        string      `json:"vector_id"`                      // 向量数据库ID
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	UserID          string      `json:"user_id"` // 用户ID

	// 内存中的字段，不存储到数据库
	processedDataMap map[string]interface{} `json:"processed_data" gorm:"-"`
	tagsList         []string               `json:"tags" gorm:"-"`
	logger           *logger.Logger         `json:"-" gorm:"-"`
}

// NewContentItem 创建新的内容项
func NewContentItem(contentType ContentType, rawContent, userID string) *ContentItem {
	// 验证输入
	if err := validateContentItemInput(contentType, rawContent, userID); err != nil {
		logger.Error("Failed to create ContentItem due to validation error", logger.Fields{
			"error":        err.Error(),
			"content_type": string(contentType),
			"user_id":      userID,
		})
		return nil
	}

	now := time.Now()
	item := &ContentItem{
		ID:               uuid.New().String(),
		Type:             contentType,
		RawContent:       rawContent,
		UserID:           userID,
		ImportanceScore:  0.0,
		CreatedAt:        now,
		UpdatedAt:        now,
		processedDataMap: make(map[string]interface{}),
		tagsList:         make([]string, 0),
		logger:           logger.NewLogger("content-model"),
	}

	item.logger.Debug("Created new ContentItem", logger.Fields{
		"id":             item.ID,
		"type":           string(item.Type),
		"user_id":        item.UserID,
		"content_length": len(rawContent),
	})

	return item
}

// validateContentItemInput 验证ContentItem输入参数
func validateContentItemInput(contentType ContentType, rawContent, userID string) error {
	if !IsValidContentType(contentType) {
		return errors.ErrValidationFailed("content_type", fmt.Sprintf("invalid content type: %s", contentType))
	}

	if strings.TrimSpace(rawContent) == "" {
		return errors.ErrValidationFailed("raw_content", "cannot be empty")
	}

	if len(rawContent) > 100000 { // 100KB limit
		return errors.ErrValidationFailed("raw_content", "content too large (max 100KB)")
	}

	if strings.TrimSpace(userID) == "" {
		return errors.ErrValidationFailed("user_id", "cannot be empty")
	}

	return nil
}

// Validate 验证ContentItem数据完整性
func (c *ContentItem) Validate() error {
	if c.ID == "" {
		return errors.ErrValidationFailed("id", "cannot be empty")
	}

	if err := validateContentItemInput(c.Type, c.RawContent, c.UserID); err != nil {
		return err
	}

	if c.ImportanceScore < 0.0 || c.ImportanceScore > 1.0 {
		return errors.ErrValidationFailed("importance_score", "must be between 0.0 and 1.0")
	}

	// 验证摘要
	if err := c.Summary.Validate(); err != nil {
		return err
	}

	// 验证标签
	tags := c.GetTags()
	if len(tags) > 50 {
		return errors.ErrValidationFailed("tags", "cannot have more than 50 tags")
	}

	for i, tag := range tags {
		if strings.TrimSpace(tag) == "" {
			return errors.ErrValidationFailed("tags", fmt.Sprintf("tag at index %d cannot be empty", i))
		}
		if len(tag) > 100 {
			return errors.ErrValidationFailed("tags", fmt.Sprintf("tag at index %d exceeds 100 characters", i))
		}
	}

	return nil
}

// SetSummary 设置摘要
func (c *ContentItem) SetSummary(summary Summary) error {
	if err := summary.Validate(); err != nil {
		c.logger.LogMemoroError(err.(*errors.MemoroError), "Failed to set summary due to validation error")
		return err
	}

	c.Summary = summary
	c.UpdatedAt = time.Now()

	c.logger.Debug("Summary updated", logger.Fields{
		"content_id":    c.ID,
		"has_one_line":  summary.OneLine != "",
		"has_paragraph": summary.Paragraph != "",
		"has_detailed":  summary.Detailed != "",
	})

	return nil
}

// GetSummary 获取摘要
func (c *ContentItem) GetSummary() Summary {
	return c.Summary
}

// SetTags 设置标签列表
func (c *ContentItem) SetTags(tags []string) error {
	// 验证标签
	if len(tags) > 50 {
		err := errors.ErrValidationFailed("tags", "cannot have more than 50 tags")
		c.logger.LogMemoroError(err, "Failed to set tags")
		return err
	}

	// 清理和验证每个标签
	cleanTags := make([]string, 0, len(tags))
	for i, tag := range tags {
		cleanTag := strings.TrimSpace(tag)
		if cleanTag == "" {
			continue // 跳过空标签
		}
		if len(cleanTag) > 100 {
			err := errors.ErrValidationFailed("tags", fmt.Sprintf("tag at index %d exceeds 100 characters", i))
			c.logger.LogMemoroError(err, "Failed to set tags")
			return err
		}
		cleanTags = append(cleanTags, cleanTag)
	}

	c.tagsList = cleanTags
	c.UpdatedAt = time.Now()

	c.logger.Debug("Tags updated", logger.Fields{
		"content_id": c.ID,
		"tag_count":  len(cleanTags),
		"tags":       cleanTags,
	})

	return nil
}

// GetTags 获取标签列表
func (c *ContentItem) GetTags() []string {
	if c.tagsList == nil {
		c.tagsList = make([]string, 0)
	}
	return c.tagsList
}

// AddTag 添加单个标签
func (c *ContentItem) AddTag(tag string) error {
	cleanTag := strings.TrimSpace(tag)
	if cleanTag == "" {
		return errors.ErrValidationFailed("tag", "cannot be empty")
	}

	if len(cleanTag) > 100 {
		return errors.ErrValidationFailed("tag", "exceeds 100 characters")
	}

	if c.tagsList == nil {
		c.tagsList = make([]string, 0)
	}

	// 检查标签是否已存在
	for _, existingTag := range c.tagsList {
		if existingTag == cleanTag {
			c.logger.Debug("Tag already exists, skipping", logger.Fields{
				"content_id": c.ID,
				"tag":        cleanTag,
			})
			return nil // 标签已存在，不重复添加
		}
	}

	// 检查标签数量限制
	if len(c.tagsList) >= 50 {
		return errors.ErrValidationFailed("tags", "cannot have more than 50 tags")
	}

	c.tagsList = append(c.tagsList, cleanTag)
	c.UpdatedAt = time.Now()

	c.logger.Debug("Tag added", logger.Fields{
		"content_id": c.ID,
		"tag":        cleanTag,
		"total_tags": len(c.tagsList),
	})

	return nil
}

// SetProcessedData 设置处理后的数据
func (c *ContentItem) SetProcessedData(data map[string]interface{}) error {
	if data == nil {
		data = make(map[string]interface{})
	}

	// 验证数据大小（序列化后不超过1MB）
	jsonData, err := json.Marshal(data)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to serialize processed data").
			WithCause(err).
			WithContext(map[string]interface{}{
				"content_id": c.ID,
				"data_keys":  getMapKeys(data),
			})
		c.logger.LogMemoroError(memoErr, "ProcessedData serialization failed")
		return memoErr
	}

	if len(jsonData) > 1024*1024 { // 1MB limit
		err := errors.ErrValidationFailed("processed_data", "serialized data exceeds 1MB limit")
		c.logger.LogMemoroError(err, "ProcessedData too large")
		return err
	}

	if c.processedDataMap == nil {
		c.processedDataMap = make(map[string]interface{})
	}

	// 深拷贝数据
	for k, v := range data {
		c.processedDataMap[k] = v
	}
	c.UpdatedAt = time.Now()

	c.logger.Debug("ProcessedData updated", logger.Fields{
		"content_id": c.ID,
		"data_size":  len(jsonData),
		"keys":       getMapKeys(data),
	})

	return nil
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
		return errors.ErrValidationFailed("importance_score", fmt.Sprintf("must be between 0.0 and 1.0, got %f", score))
	}

	oldScore := c.ImportanceScore
	c.ImportanceScore = score
	c.UpdatedAt = time.Now()

	c.logger.Debug("Importance score updated", logger.Fields{
		"content_id": c.ID,
		"old_score":  oldScore,
		"new_score":  score,
	})

	return nil
}

// GetImportanceScore 获取重要性评分
func (c *ContentItem) GetImportanceScore() float64 {
	return c.ImportanceScore
}

// BeforeCreate GORM钩子：创建前执行
func (c *ContentItem) BeforeCreate(tx *gorm.DB) error {
	if c.logger == nil {
		c.logger = logger.NewLogger("content-model")
	}

	// 验证数据
	if err := c.Validate(); err != nil {
		c.logger.LogMemoroError(err.(*errors.MemoroError), "Validation failed before create")
		return err
	}

	// 序列化processedDataMap到ProcessedData字段
	if err := c.serializeProcessedData(); err != nil {
		return err
	}

	// 序列化tagsList到Tags字段
	if err := c.serializeTags(); err != nil {
		return err
	}

	c.logger.Debug("ContentItem ready for database creation", logger.Fields{
		"content_id": c.ID,
		"type":       string(c.Type),
	})

	return nil
}

// BeforeUpdate GORM钩子：更新前执行
func (c *ContentItem) BeforeUpdate(tx *gorm.DB) error {
	if c.logger == nil {
		c.logger = logger.NewLogger("content-model")
	}

	// 验证数据
	if err := c.Validate(); err != nil {
		c.logger.LogMemoroError(err.(*errors.MemoroError), "Validation failed before update")
		return err
	}

	// 序列化数据
	if err := c.serializeProcessedData(); err != nil {
		return err
	}

	if err := c.serializeTags(); err != nil {
		return err
	}

	c.logger.Debug("ContentItem ready for database update", logger.Fields{
		"content_id": c.ID,
		"updated_at": c.UpdatedAt,
	})

	return nil
}

// AfterFind GORM钩子：查询后执行
func (c *ContentItem) AfterFind(tx *gorm.DB) error {
	if c.logger == nil {
		c.logger = logger.NewLogger("content-model")
	}

	// 反序列化ProcessedData字段到processedDataMap
	if err := c.deserializeProcessedData(); err != nil {
		return err
	}

	// 反序列化Tags字段到tagsList
	if err := c.deserializeTags(); err != nil {
		return err
	}

	c.logger.Debug("ContentItem loaded from database", logger.Fields{
		"content_id": c.ID,
		"type":       string(c.Type),
		"tag_count":  len(c.tagsList),
	})

	return nil
}

// serializeProcessedData 序列化处理数据
func (c *ContentItem) serializeProcessedData() error {
	if c.processedDataMap != nil && len(c.processedDataMap) > 0 {
		jsonData, err := json.Marshal(c.processedDataMap)
		if err != nil {
			memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to marshal processed data").
				WithCause(err).
				WithContext(map[string]interface{}{
					"content_id": c.ID,
					"data_keys":  getMapKeys(c.processedDataMap),
				})
			c.logger.LogMemoroError(memoErr, "ProcessedData marshaling failed")
			return memoErr
		}
		c.ProcessedData = string(jsonData)
	}
	return nil
}

// deserializeProcessedData 反序列化处理数据
func (c *ContentItem) deserializeProcessedData() error {
	if c.ProcessedData != "" {
		var data map[string]interface{}
		if err := json.Unmarshal([]byte(c.ProcessedData), &data); err != nil {
			memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to unmarshal processed data").
				WithCause(err).
				WithContext(map[string]interface{}{
					"content_id":  c.ID,
					"data_length": len(c.ProcessedData),
				})
			c.logger.LogMemoroError(memoErr, "ProcessedData unmarshaling failed")
			return memoErr
		}
		c.processedDataMap = data
	} else {
		c.processedDataMap = make(map[string]interface{})
	}
	return nil
}

// serializeTags 序列化标签
func (c *ContentItem) serializeTags() error {
	if c.tagsList != nil && len(c.tagsList) > 0 {
		jsonData, err := json.Marshal(c.tagsList)
		if err != nil {
			memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to marshal tags").
				WithCause(err).
				WithContext(map[string]interface{}{
					"content_id": c.ID,
					"tag_count":  len(c.tagsList),
				})
			c.logger.LogMemoroError(memoErr, "Tags marshaling failed")
			return memoErr
		}
		c.Tags = string(jsonData)
	}
	return nil
}

// deserializeTags 反序列化标签
func (c *ContentItem) deserializeTags() error {
	if c.Tags != "" {
		var tags []string
		if err := json.Unmarshal([]byte(c.Tags), &tags); err != nil {
			memoErr := errors.NewMemoroError(errors.ErrorTypeSystem, errors.ErrCodeSystemGeneric, "Failed to unmarshal tags").
				WithCause(err).
				WithContext(map[string]interface{}{
					"content_id":  c.ID,
					"tags_length": len(c.Tags),
				})
			c.logger.LogMemoroError(memoErr, "Tags unmarshaling failed")
			return memoErr
		}
		c.tagsList = tags
	} else {
		c.tagsList = make([]string, 0)
	}
	return nil
}

// getMapKeys 获取map的键列表（辅助函数）
func getMapKeys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
