package logger

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/sirupsen/logrus"
	"memoro/internal/errors"
)

// Logger 标准化日志器
type Logger struct {
	*logrus.Logger
	component string
}

// Fields 日志字段类型
type Fields map[string]interface{}

var (
	// 全局默认日志器
	defaultLogger *Logger
)

// InitLogger 初始化日志系统
func InitLogger(level string, format string, output string, component string) (*Logger, error) {
	logger := logrus.New()
	
	// 设置日志级别
	logLevel, err := logrus.ParseLevel(level)
	if err != nil {
		logLevel = logrus.InfoLevel
	}
	logger.SetLevel(logLevel)
	
	// 设置输出格式
	switch format {
	case "json":
		logger.SetFormatter(&logrus.JSONFormatter{
			TimestampFormat: time.RFC3339,
			FieldMap: logrus.FieldMap{
				logrus.FieldKeyTime:  "timestamp",
				logrus.FieldKeyLevel: "level",
				logrus.FieldKeyMsg:   "message",
			},
		})
	default:
		logger.SetFormatter(&logrus.TextFormatter{
			FullTimestamp:   true,
			TimestampFormat: "2006-01-02 15:04:05",
		})
	}
	
	// 设置输出目标
	if output != "" && output != "stdout" {
		// 确保日志目录存在
		logDir := filepath.Dir(output)
		if err := os.MkdirAll(logDir, 0755); err != nil {
			return nil, errors.ErrConfigInvalid("log directory", err.Error()).WithCause(err)
		}
		
		file, err := os.OpenFile(output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return nil, errors.ErrConfigInvalid("log file", err.Error()).WithCause(err)
		}
		logger.SetOutput(file)
	}
	
	memoLogger := &Logger{
		Logger:    logger,
		component: component,
	}
	
	// 设置为默认日志器
	defaultLogger = memoLogger
	
	return memoLogger, nil
}

// GetDefaultLogger 获取默认日志器
func GetDefaultLogger() *Logger {
	if defaultLogger == nil {
		// 如果没有初始化，创建一个基本的日志器
		logger := logrus.New()
		logger.SetLevel(logrus.InfoLevel)
		defaultLogger = &Logger{Logger: logger, component: "default"}
	}
	return defaultLogger
}

// NewLogger 创建新的组件日志器
func NewLogger(component string) *Logger {
	base := GetDefaultLogger()
	return &Logger{
		Logger:    base.Logger,
		component: component,
	}
}

// WithFields 添加字段
func (l *Logger) WithFields(fields Fields) *logrus.Entry {
	return l.Logger.WithFields(logrus.Fields(fields)).WithField("component", l.component)
}

// WithError 添加错误信息
func (l *Logger) WithError(err error) *logrus.Entry {
	entry := l.Logger.WithField("component", l.component)
	
	if memoErr, ok := err.(*errors.MemoroError); ok {
		return entry.WithFields(logrus.Fields{
			"error_type":    memoErr.Type,
			"error_code":    memoErr.Code,
			"error_details": memoErr.Details,
			"error_context": memoErr.Context,
		})
	}
	
	return entry.WithError(err)
}

// WithContext 从上下文中提取信息
func (l *Logger) WithContext(ctx context.Context) *logrus.Entry {
	entry := l.Logger.WithField("component", l.component)
	
	// 提取请求ID（如果存在）
	if requestID := ctx.Value("request_id"); requestID != nil {
		entry = entry.WithField("request_id", requestID)
	}
	
	// 提取用户ID（如果存在）
	if userID := ctx.Value("user_id"); userID != nil {
		entry = entry.WithField("user_id", userID)
	}
	
	return entry
}

// LogMemoroError 记录Memoro错误
func (l *Logger) LogMemoroError(err *errors.MemoroError, message string) {
	entry := l.WithError(err)
	
	switch err.Type {
	case errors.ErrorTypeSystem, errors.ErrorTypeDatabase, errors.ErrorTypeNetwork:
		entry.Error(message)
	case errors.ErrorTypeValidation, errors.ErrorTypeBusiness:
		entry.Warn(message)
	default:
		entry.Error(message)
	}
}

// 便捷方法

// Debug 调试日志
func (l *Logger) Debug(msg string, fields ...Fields) {
	entry := l.Logger.WithField("component", l.component)
	if len(fields) > 0 {
		entry = entry.WithFields(logrus.Fields(fields[0]))
	}
	entry.Debug(msg)
}

// Info 信息日志
func (l *Logger) Info(msg string, fields ...Fields) {
	entry := l.Logger.WithField("component", l.component)
	if len(fields) > 0 {
		entry = entry.WithFields(logrus.Fields(fields[0]))
	}
	entry.Info(msg)
}

// Warn 警告日志
func (l *Logger) Warn(msg string, fields ...Fields) {
	entry := l.Logger.WithField("component", l.component)
	if len(fields) > 0 {
		entry = entry.WithFields(logrus.Fields(fields[0]))
	}
	entry.Warn(msg)
}

// Error 错误日志
func (l *Logger) Error(msg string, fields ...Fields) {
	entry := l.Logger.WithField("component", l.component)
	if len(fields) > 0 {
		entry = entry.WithFields(logrus.Fields(fields[0]))
	}
	entry.Error(msg)
}

// Fatal 致命错误日志
func (l *Logger) Fatal(msg string, fields ...Fields) {
	entry := l.Logger.WithField("component", l.component)
	if len(fields) > 0 {
		entry = entry.WithFields(logrus.Fields(fields[0]))
	}
	entry.Fatal(msg)
}

// 全局便捷函数

// Debug 全局调试日志
func Debug(msg string, fields ...Fields) {
	GetDefaultLogger().Debug(msg, fields...)
}

// Info 全局信息日志
func Info(msg string, fields ...Fields) {
	GetDefaultLogger().Info(msg, fields...)
}

// Warn 全局警告日志
func Warn(msg string, fields ...Fields) {
	GetDefaultLogger().Warn(msg, fields...)
}

// Error 全局错误日志
func Error(msg string, fields ...Fields) {
	GetDefaultLogger().Error(msg, fields...)
}

// Fatal 全局致命错误日志
func Fatal(msg string, fields ...Fields) {
	GetDefaultLogger().Fatal(msg, fields...)
}

// LogError 记录错误
func LogError(err error, msg string, fields ...Fields) {
	logger := GetDefaultLogger()
	if memoErr, ok := err.(*errors.MemoroError); ok {
		logger.LogMemoroError(memoErr, msg)
	} else {
		entry := logger.WithError(err)
		if len(fields) > 0 {
			entry = entry.WithFields(logrus.Fields(fields[0]))
		}
		entry.Error(msg)
	}
}