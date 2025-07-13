package llm

import (
	"context"
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
	"memoro/internal/config"
	"memoro/internal/errors"
	"memoro/internal/logger"
)

// Client OpenAI兼容的LLM客户端
type Client struct {
	httpClient *resty.Client
	config     config.LLMConfig
	logger     *logger.Logger
}

// ChatMessage 聊天消息结构
type ChatMessage struct {
	Role    string `json:"role"`    // system, user, assistant
	Content string `json:"content"`
}

// ChatCompletionRequest OpenAI兼容的聊天完成请求
type ChatCompletionRequest struct {
	Model       string        `json:"model"`
	Messages    []ChatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens"`
	Temperature float64       `json:"temperature"`
	Stream      bool          `json:"stream"`
}

// ChatCompletionChoice 聊天完成选择
type ChatCompletionChoice struct {
	Index        int         `json:"index"`
	Message      ChatMessage `json:"message"`
	FinishReason string      `json:"finish_reason"`
}

// ChatCompletionUsage 使用情况统计
type ChatCompletionUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// ChatCompletionResponse OpenAI兼容的聊天完成响应
type ChatCompletionResponse struct {
	ID      string                  `json:"id"`
	Object  string                  `json:"object"`
	Created int64                   `json:"created"`
	Model   string                  `json:"model"`
	Choices []ChatCompletionChoice  `json:"choices"`
	Usage   ChatCompletionUsage     `json:"usage"`
}

// NewClient 创建新的LLM客户端
func NewClient() (*Client, error) {
	cfg := config.GetLLMConfig()
	if cfg.APIBase == "" {
		return nil, errors.ErrConfigMissing("llm.api_base")
	}

	if cfg.Model == "" {
		return nil, errors.ErrConfigMissing("llm.model")
	}

	clientLogger := logger.NewLogger("llm-client")

	// 创建HTTP客户端
	httpClient := resty.New()
	httpClient.SetBaseURL(cfg.APIBase)
	httpClient.SetTimeout(cfg.Timeout)
	httpClient.SetHeader("Content-Type", "application/json")
	
	// 设置API密钥
	if cfg.APIKey != "" {
		httpClient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", cfg.APIKey))
	} else {
		clientLogger.Warn("LLM API key is not set - requests may fail")
	}

	// 设置重试策略
	httpClient.SetRetryCount(cfg.RetryTimes)
	httpClient.SetRetryWaitTime(cfg.RetryDelay)
	httpClient.SetRetryMaxWaitTime(cfg.RetryDelay * 3)

	// 添加请求日志
	httpClient.OnBeforeRequest(func(c *resty.Client, req *resty.Request) error {
		clientLogger.Debug("LLM API request", logger.Fields{
			"url":    req.URL,
			"method": req.Method,
			"size":   len(req.Body.([]byte)),
		})
		return nil
	})

	httpClient.OnAfterResponse(func(c *resty.Client, resp *resty.Response) error {
		clientLogger.Debug("LLM API response", logger.Fields{
			"status":    resp.StatusCode(),
			"size":      len(resp.Body()),
			"time":      resp.Time(),
		})
		return nil
	})

	client := &Client{
		httpClient: httpClient,
		config:     cfg,
		logger:     clientLogger,
	}

	clientLogger.Info("LLM client initialized", logger.Fields{
		"provider":    cfg.Provider,
		"model":       cfg.Model,
		"max_tokens":  cfg.MaxTokens,
		"temperature": cfg.Temperature,
		"timeout":     cfg.Timeout,
		"retry_times": cfg.RetryTimes,
	})

	return client, nil
}

// ChatCompletion 执行聊天完成请求
func (c *Client) ChatCompletion(ctx context.Context, messages []ChatMessage) (*ChatCompletionResponse, error) {
	if len(messages) == 0 {
		return nil, errors.ErrValidationFailed("messages", "cannot be empty")
	}

	// 验证消息格式
	for i, msg := range messages {
		if msg.Role == "" {
			return nil, errors.ErrValidationFailed("messages", fmt.Sprintf("message %d: role cannot be empty", i))
		}
		if msg.Content == "" {
			return nil, errors.ErrValidationFailed("messages", fmt.Sprintf("message %d: content cannot be empty", i))
		}
		if msg.Role != "system" && msg.Role != "user" && msg.Role != "assistant" {
			return nil, errors.ErrValidationFailed("messages", fmt.Sprintf("message %d: invalid role '%s'", i, msg.Role))
		}
	}

	// 构建请求
	request := ChatCompletionRequest{
		Model:       c.config.Model,
		Messages:    messages,
		MaxTokens:   c.config.MaxTokens,
		Temperature: c.config.Temperature,
		Stream:      false, // 目前只支持非流式
	}

	c.logger.Debug("Sending chat completion request", logger.Fields{
		"model":        request.Model,
		"message_count": len(messages),
		"max_tokens":   request.MaxTokens,
		"temperature":  request.Temperature,
	})

	// 发送请求
	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetBody(request).
		SetResult(&ChatCompletionResponse{}).
		Post("/chat/completions")

	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Failed to call LLM API").
			WithCause(err).
			WithContext(map[string]interface{}{
				"model":    request.Model,
				"messages": len(messages),
			})
		c.logger.LogMemoroError(memoErr, "LLM API call failed")
		return nil, memoErr
	}

	// 检查HTTP状态
	if resp.StatusCode() != 200 {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "LLM API returned error status").
			WithDetails(fmt.Sprintf("Status: %d, Body: %s", resp.StatusCode(), string(resp.Body()))).
			WithContext(map[string]interface{}{
				"status_code": resp.StatusCode(),
				"model":       request.Model,
			})
		c.logger.LogMemoroError(memoErr, "LLM API error response")
		return nil, memoErr
	}

	// 解析响应
	result := resp.Result().(*ChatCompletionResponse)
	if result == nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "Failed to parse LLM API response").
			WithDetails("Response result is nil")
		c.logger.LogMemoroError(memoErr, "LLM API response parsing failed")
		return nil, memoErr
	}

	// 验证响应
	if len(result.Choices) == 0 {
		memoErr := errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "LLM API returned no choices").
			WithContext(map[string]interface{}{
				"response_id": result.ID,
				"model":       result.Model,
			})
		c.logger.LogMemoroError(memoErr, "LLM API empty response")
		return nil, memoErr
	}

	c.logger.Debug("Chat completion successful", logger.Fields{
		"response_id":       result.ID,
		"model":            result.Model,
		"choices":          len(result.Choices),
		"prompt_tokens":    result.Usage.PromptTokens,
		"completion_tokens": result.Usage.CompletionTokens,
		"total_tokens":     result.Usage.TotalTokens,
		"finish_reason":    result.Choices[0].FinishReason,
	})

	return result, nil
}

// SimpleCompletion 简单的单轮对话完成
func (c *Client) SimpleCompletion(ctx context.Context, systemPrompt, userMessage string) (string, error) {
	messages := []ChatMessage{
		{
			Role:    "user",
			Content: userMessage,
		},
	}

	// 如果有系统提示，添加到消息开头
	if systemPrompt != "" {
		messages = append([]ChatMessage{
			{
				Role:    "system",
				Content: systemPrompt,
			},
		}, messages...)
	}

	response, err := c.ChatCompletion(ctx, messages)
	if err != nil {
		return "", err
	}

	if len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return "", errors.NewMemoroError(errors.ErrorTypeLLM, errors.ErrCodeLLMAPICall, "LLM returned empty response")
	}

	return strings.TrimSpace(response.Choices[0].Message.Content), nil
}

// ValidateConnection 验证与LLM服务的连接
func (c *Client) ValidateConnection(ctx context.Context) error {
	c.logger.Info("Validating LLM connection")

	// 发送简单的测试请求
	testMessage := "Hello, please respond with 'OK' to confirm the connection."
	response, err := c.SimpleCompletion(ctx, "", testMessage)
	if err != nil {
		c.logger.LogMemoroError(err.(*errors.MemoroError), "LLM connection validation failed")
		return err
	}

	c.logger.Info("LLM connection validation successful", logger.Fields{
		"test_response": response,
		"response_length": len(response),
	})

	return nil
}

// GetConfig 获取当前LLM配置
func (c *Client) GetConfig() config.LLMConfig {
	return c.config
}

// UpdateConfig 更新LLM配置
func (c *Client) UpdateConfig(newConfig config.LLMConfig) error {
	// 验证新配置
	if newConfig.APIBase == "" {
		return errors.ErrConfigMissing("llm.api_base")
	}
	if newConfig.Model == "" {
		return errors.ErrConfigMissing("llm.model")
	}

	// 更新HTTP客户端配置
	c.httpClient.SetBaseURL(newConfig.APIBase)
	c.httpClient.SetTimeout(newConfig.Timeout)

	if newConfig.APIKey != "" {
		c.httpClient.SetHeader("Authorization", fmt.Sprintf("Bearer %s", newConfig.APIKey))
	}

	// 更新重试策略
	c.httpClient.SetRetryCount(newConfig.RetryTimes)
	c.httpClient.SetRetryWaitTime(newConfig.RetryDelay)

	c.config = newConfig

	c.logger.Info("LLM configuration updated", logger.Fields{
		"provider":    newConfig.Provider,
		"model":       newConfig.Model,
		"max_tokens":  newConfig.MaxTokens,
		"temperature": newConfig.Temperature,
	})

	return nil
}

// Close 关闭客户端并清理资源
func (c *Client) Close() error {
	c.logger.Info("Closing LLM client")
	// HTTP客户端不需要显式关闭，Go的垃圾回收器会处理
	return nil
}