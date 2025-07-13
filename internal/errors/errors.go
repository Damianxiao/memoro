package errors

import (
	"fmt"
	"time"
)

// ErrorType 错误类型枚举
type ErrorType string

const (
	// 系统级错误
	ErrorTypeSystem   ErrorType = "SYSTEM"
	ErrorTypeDatabase ErrorType = "DATABASE"
	ErrorTypeNetwork  ErrorType = "NETWORK"
	ErrorTypeConfig   ErrorType = "CONFIG"

	// 业务级错误
	ErrorTypeBusiness   ErrorType = "BUSINESS"
	ErrorTypeValidation ErrorType = "VALIDATION"
	ErrorTypeAuth       ErrorType = "AUTH"

	// 集成错误
	ErrorTypeWebSocket ErrorType = "WEBSOCKET"
	ErrorTypeLLM       ErrorType = "LLM"
	ErrorTypeVector    ErrorType = "VECTOR"
)

// ErrorCode 错误码
type ErrorCode string

const (
	// 系统错误码 (E1xxx)
	ErrCodeSystemGeneric   ErrorCode = "E1000"
	ErrCodeDatabaseConnect ErrorCode = "E1001"
	ErrCodeDatabaseQuery   ErrorCode = "E1002"
	ErrCodeNetworkTimeout  ErrorCode = "E1003"
	ErrCodeConfigMissing   ErrorCode = "E1004"
	ErrCodeConfigInvalid   ErrorCode = "E1005"

	// 业务错误码 (E2xxx)
	ErrCodeValidationFailed  ErrorCode = "E2001"
	ErrCodeResourceNotFound  ErrorCode = "E2002"
	ErrCodeDuplicateResource ErrorCode = "E2003"
	ErrCodeInvalidInput      ErrorCode = "E2004"

	// 集成错误码 (E3xxx)
	ErrCodeWebSocketConnect ErrorCode = "E3001"
	ErrCodeWebSocketMessage ErrorCode = "E3002"
	ErrCodeLLMAPICall       ErrorCode = "E3003"
	ErrCodeVectorStorage    ErrorCode = "E3004"
)

// MemoroError 统一错误结构
type MemoroError struct {
	Type      ErrorType   `json:"type"`
	Code      ErrorCode   `json:"code"`
	Message   string      `json:"message"`
	Details   string      `json:"details,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Context   interface{} `json:"context,omitempty"`
	Cause     error       `json:"-"` // 原始错误，不序列化
}

// Error 实现error接口
func (e *MemoroError) Error() string {
	if e.Details != "" {
		return fmt.Sprintf("[%s:%s] %s - %s", e.Type, e.Code, e.Message, e.Details)
	}
	return fmt.Sprintf("[%s:%s] %s", e.Type, e.Code, e.Message)
}

// Unwrap 支持错误链
func (e *MemoroError) Unwrap() error {
	return e.Cause
}

// NewMemoroError 创建新的Memoro错误
func NewMemoroError(errorType ErrorType, code ErrorCode, message string) *MemoroError {
	return &MemoroError{
		Type:      errorType,
		Code:      code,
		Message:   message,
		Timestamp: time.Now(),
	}
}

// WithDetails 添加详细信息
func (e *MemoroError) WithDetails(details string) *MemoroError {
	e.Details = details
	return e
}

// WithContext 添加上下文信息
func (e *MemoroError) WithContext(context interface{}) *MemoroError {
	e.Context = context
	return e
}

// WithCause 添加原始错误
func (e *MemoroError) WithCause(cause error) *MemoroError {
	e.Cause = cause
	return e
}

// IsType 检查错误类型
func (e *MemoroError) IsType(errorType ErrorType) bool {
	return e.Type == errorType
}

// IsCode 检查错误码
func (e *MemoroError) IsCode(code ErrorCode) bool {
	return e.Code == code
}

// 预定义常用错误

// ErrDatabaseConnection 数据库连接错误
func ErrDatabaseConnection(details string, cause error) *MemoroError {
	return NewMemoroError(ErrorTypeDatabase, ErrCodeDatabaseConnect, "Failed to connect to database").
		WithDetails(details).
		WithCause(cause)
}

// ErrValidationFailed 验证失败错误
func ErrValidationFailed(field, reason string) *MemoroError {
	return NewMemoroError(ErrorTypeValidation, ErrCodeValidationFailed, "Validation failed").
		WithDetails(fmt.Sprintf("Field '%s': %s", field, reason))
}

// ErrWebSocketConnection WebSocket连接错误
func ErrWebSocketConnection(details string, cause error) *MemoroError {
	return NewMemoroError(ErrorTypeWebSocket, ErrCodeWebSocketConnect, "WebSocket connection failed").
		WithDetails(details).
		WithCause(cause)
}

// ErrConfigMissing 配置缺失错误
func ErrConfigMissing(configKey string) *MemoroError {
	return NewMemoroError(ErrorTypeConfig, ErrCodeConfigMissing, "Required configuration missing").
		WithDetails(fmt.Sprintf("Missing config key: %s", configKey))
}

// ErrConfigInvalid 配置无效错误
func ErrConfigInvalid(configKey, reason string) *MemoroError {
	return NewMemoroError(ErrorTypeConfig, ErrCodeConfigInvalid, "Invalid configuration").
		WithDetails(fmt.Sprintf("Config key '%s': %s", configKey, reason))
}

// ErrResourceNotFound 资源未找到错误
func ErrResourceNotFound(resourceType, resourceID string) *MemoroError {
	return NewMemoroError(ErrorTypeBusiness, ErrCodeResourceNotFound, "Resource not found").
		WithDetails(fmt.Sprintf("%s with ID '%s' not found", resourceType, resourceID))
}
