package config

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/viper"
	"memoro/internal/errors"
	"memoro/internal/logger"
)

// Config 应用配置结构
type Config struct {
	Server     ServerConfig     `mapstructure:"server"`
	WeChat     WeChatConfig     `mapstructure:"wechat"`
	Database   DatabaseConfig   `mapstructure:"database"`
	VectorDB   VectorDBConfig   `mapstructure:"vector_db"`
	LLM        LLMConfig        `mapstructure:"llm"`
	Storage    StorageConfig    `mapstructure:"storage"`
	Logging    LoggingConfig    `mapstructure:"logging"`
	Processing ProcessingConfig `mapstructure:"processing"`
	Cache      CacheConfig      `mapstructure:"cache"`
	Security   SecurityConfig   `mapstructure:"security"`
	Monitoring MonitoringConfig `mapstructure:"monitoring"`
}

// ServerConfig 服务器配置
type ServerConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

// WeChatConfig 微信配置
type WeChatConfig struct {
	PadURL            string        `mapstructure:"pad_url"`
	WebSocketURL      string        `mapstructure:"websocket_url"`
	AdminKey          string        `mapstructure:"admin_key"`
	Token             string        `mapstructure:"token"`
	WXID              string        `mapstructure:"wxid"`
	RetryTimes        int           `mapstructure:"retry_times"`
	RetryDelay        time.Duration `mapstructure:"retry_delay"`
	ConnectionTimeout time.Duration `mapstructure:"connection_timeout"`
}

// DatabaseConfig 数据库配置
type DatabaseConfig struct {
	Type            string        `mapstructure:"type"`
	Path            string        `mapstructure:"path"`
	AutoMigrate     bool          `mapstructure:"auto_migrate"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

// VectorDBConfig 向量数据库配置
type VectorDBConfig struct {
	Type        string                    `mapstructure:"type"`
	Host        string                    `mapstructure:"host"`
	Port        int                       `mapstructure:"port"`
	Collection  string                    `mapstructure:"collection"`
	Timeout     time.Duration             `mapstructure:"timeout"`
	RetryTimes  int                       `mapstructure:"retry_times"`
	BatchSize   int                       `mapstructure:"batch_size"`
	CacheConfig *VectorCacheConfig        `mapstructure:"cache"`
	PoolConfig  *ConnectionPoolConfig     `mapstructure:"connection_pool"`
}

// VectorCacheConfig 向量缓存配置
type VectorCacheConfig struct {
	QueryVectorTTL        time.Duration `mapstructure:"query_vector_ttl"`
	QueryVectorMaxSize    int           `mapstructure:"query_vector_max_size"`
	RecommendationTTL     time.Duration `mapstructure:"recommendation_ttl"`
	RecommendationMaxSize int           `mapstructure:"recommendation_max_size"`
	UserPreferenceTTL     time.Duration `mapstructure:"user_preference_ttl"`
	UserPreferenceMaxSize int           `mapstructure:"user_preference_max_size"`
	CleanupInterval       time.Duration `mapstructure:"cleanup_interval"`
}

// ConnectionPoolConfig 连接池配置
type ConnectionPoolConfig struct {
	MaxConnections int           `mapstructure:"max_connections"`
	MinConnections int           `mapstructure:"min_connections"`
	IdleTimeout    time.Duration `mapstructure:"idle_timeout"`
	HealthCheck    bool          `mapstructure:"health_check"`
	HealthInterval time.Duration `mapstructure:"health_interval"`
}

// LLMConfig LLM API配置
type LLMConfig struct {
	Provider    string          `mapstructure:"provider"`
	APIBase     string          `mapstructure:"api_base"`
	APIKey      string          `mapstructure:"api_key"`
	Model       string          `mapstructure:"model"`
	MaxTokens   int             `mapstructure:"max_tokens"`
	Temperature float64         `mapstructure:"temperature"`
	Timeout     time.Duration   `mapstructure:"timeout"`
	RetryTimes  int             `mapstructure:"retry_times"`
	RetryDelay  time.Duration   `mapstructure:"retry_delay"`
	RateLimit   RateLimitConfig `mapstructure:"rate_limit"`
}

// RateLimitConfig 速率限制配置
type RateLimitConfig struct {
	RequestsPerMinute int `mapstructure:"requests_per_minute"`
	BurstSize         int `mapstructure:"burst_size"`
}

// StorageConfig 存储配置
type StorageConfig struct {
	FilePath     string   `mapstructure:"file_path"`
	MaxFileSize  string   `mapstructure:"max_file_size"`
	AllowedTypes []string `mapstructure:"allowed_types"`
}

// LoggingConfig 日志配置
type LoggingConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	Output     string `mapstructure:"output"`
	FilePath   string `mapstructure:"file_path"`
	MaxSize    string `mapstructure:"max_size"`
	MaxBackups int    `mapstructure:"max_backups"`
	MaxAge     int    `mapstructure:"max_age"`
	Compress   bool   `mapstructure:"compress"`
}

// ProcessingConfig 处理配置
type ProcessingConfig struct {
	MaxWorkers     int                 `mapstructure:"max_workers"`
	QueueSize      int                 `mapstructure:"queue_size"`
	Timeout        time.Duration       `mapstructure:"timeout"`
	MaxContentSize int                 `mapstructure:"max_content_size"` // 最大内容大小(字节)
	SummaryLevels  SummaryLevelsConfig `mapstructure:"summary_levels"`
	TagLimits      TagLimitsConfig     `mapstructure:"tag_limits"`
}

// SummaryLevelsConfig 摘要级别配置
type SummaryLevelsConfig struct {
	OneLineMaxLength   int `mapstructure:"one_line_max_length"`
	ParagraphMaxLength int `mapstructure:"paragraph_max_length"`
	DetailedMaxLength  int `mapstructure:"detailed_max_length"`
}

// TagLimitsConfig 标签限制配置
type TagLimitsConfig struct {
	MaxTags           int     `mapstructure:"max_tags"`
	MaxTagLength      int     `mapstructure:"max_tag_length"`
	DefaultConfidence float64 `mapstructure:"default_confidence"` // 默认置信度
}

// CacheConfig 缓存配置
type CacheConfig struct {
	Enabled         bool          `mapstructure:"enabled"`
	Type            string        `mapstructure:"type"`
	TTL             time.Duration `mapstructure:"ttl"`
	MaxItems        int           `mapstructure:"max_items"`
	CleanupInterval time.Duration `mapstructure:"cleanup_interval"`
}

// SecurityConfig 安全配置
type SecurityConfig struct {
	APIKeyHeader string                  `mapstructure:"api_key_header"`
	CORSEnabled  bool                    `mapstructure:"cors_enabled"`
	CORSOrigins  []string                `mapstructure:"cors_origins"`
	RateLimiting SecurityRateLimitConfig `mapstructure:"rate_limiting"`
}

// SecurityRateLimitConfig 安全速率限制配置
type SecurityRateLimitConfig struct {
	Enabled           bool `mapstructure:"enabled"`
	RequestsPerMinute int  `mapstructure:"requests_per_minute"`
	BurstSize         int  `mapstructure:"burst_size"`
}

// MonitoringConfig 监控配置
type MonitoringConfig struct {
	MetricsEnabled      bool          `mapstructure:"metrics_enabled"`
	HealthCheckInterval time.Duration `mapstructure:"health_check_interval"`
	PerformanceTracking bool          `mapstructure:"performance_tracking"`
}

var (
	globalConfig *Config
	configLogger *logger.Logger
)

// Load 加载配置文件
func Load(configPath string) (*Config, error) {
	configLogger = logger.NewLogger("config")

	viper.SetConfigFile(configPath)
	viper.SetConfigType("yaml")

	// 设置环境变量前缀
	viper.SetEnvPrefix("MEMORO")
	viper.AutomaticEnv()

	// 绑定特定的环境变量
	viper.BindEnv("llm.api_key", "MEMORO_LLM_API_KEY")
	viper.BindEnv("wechat.admin_key", "MEMORO_WECHAT_ADMIN_KEY")
	viper.BindEnv("database.path", "MEMORO_DATABASE_PATH")

	configLogger.Info("Loading configuration", logger.Fields{
		"config_path": configPath,
	})

	// 读取配置文件
	if err := viper.ReadInConfig(); err != nil {
		memoErr := errors.ErrConfigInvalid("config_file", err.Error()).
			WithCause(err).
			WithContext(map[string]interface{}{
				"config_path": configPath,
			})
		configLogger.LogMemoroError(memoErr, "Failed to read configuration file")
		return nil, memoErr
	}

	// 解析配置到结构体
	config := &Config{}
	if err := viper.Unmarshal(config); err != nil {
		memoErr := errors.ErrConfigInvalid("config_unmarshal", err.Error()).
			WithCause(err)
		configLogger.LogMemoroError(memoErr, "Failed to unmarshal configuration")
		return nil, memoErr
	}

	// 验证配置
	if err := validateConfig(config); err != nil {
		configLogger.LogMemoroError(err.(*errors.MemoroError), "Configuration validation failed")
		return nil, err
	}

	// 处理环境变量覆盖
	if err := processEnvironmentOverrides(config); err != nil {
		configLogger.LogMemoroError(err.(*errors.MemoroError), "Failed to process environment overrides")
		return nil, err
	}

	globalConfig = config
	configLogger.Info("Configuration loaded successfully", logger.Fields{
		"server_port":   config.Server.Port,
		"database_type": config.Database.Type,
		"llm_provider":  config.LLM.Provider,
		"cache_enabled": config.Cache.Enabled,
	})

	return config, nil
}

// validateConfig 验证配置的有效性
func validateConfig(config *Config) error {
	// 验证服务器配置
	if config.Server.Port <= 0 || config.Server.Port > 65535 {
		return errors.ErrConfigInvalid("server.port", "must be between 1 and 65535")
	}

	if config.Server.Mode != "development" && config.Server.Mode != "production" {
		return errors.ErrConfigInvalid("server.mode", "must be 'development' or 'production'")
	}

	// 验证微信配置
	if config.WeChat.WebSocketURL == "" {
		return errors.ErrConfigMissing("wechat.websocket_url")
	}

	// 验证数据库配置
	if config.Database.Type != "sqlite" {
		return errors.ErrConfigInvalid("database.type", "only 'sqlite' is supported")
	}

	if config.Database.Path == "" {
		return errors.ErrConfigMissing("database.path")
	}

	// 验证LLM配置
	if config.LLM.APIBase == "" {
		return errors.ErrConfigMissing("llm.api_base")
	}

	if config.LLM.Model == "" {
		return errors.ErrConfigMissing("llm.model")
	}

	if config.LLM.MaxTokens <= 0 {
		return errors.ErrConfigInvalid("llm.max_tokens", "must be greater than 0")
	}

	if config.LLM.Temperature < 0 || config.LLM.Temperature > 2 {
		return errors.ErrConfigInvalid("llm.temperature", "must be between 0 and 2")
	}

	// 验证向量数据库配置
	if config.VectorDB.Type != "chroma" {
		return errors.ErrConfigInvalid("vector_db.type", "only 'chroma' is supported")
	}

	if config.VectorDB.Collection == "" {
		return errors.ErrConfigMissing("vector_db.collection")
	}

	// 验证存储配置
	if config.Storage.FilePath == "" {
		return errors.ErrConfigMissing("storage.file_path")
	}

	// 验证日志配置
	if config.Logging.Level == "" {
		return errors.ErrConfigMissing("logging.level")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	isValidLevel := false
	for _, level := range validLogLevels {
		if config.Logging.Level == level {
			isValidLevel = true
			break
		}
	}
	if !isValidLevel {
		return errors.ErrConfigInvalid("logging.level", "must be one of: debug, info, warn, error")
	}

	return nil
}

// processEnvironmentOverrides 处理环境变量覆盖
func processEnvironmentOverrides(config *Config) error {
	// 处理LLM API Key
	if apiKey := os.Getenv("MEMORO_LLM_API_KEY"); apiKey != "" {
		config.LLM.APIKey = apiKey
		configLogger.Debug("LLM API key loaded from environment variable")
	}

	// 处理微信Admin Key
	if adminKey := os.Getenv("MEMORO_WECHAT_ADMIN_KEY"); adminKey != "" {
		config.WeChat.AdminKey = adminKey
		configLogger.Debug("WeChat admin key loaded from environment variable")
	}

	// 处理数据库路径
	if dbPath := os.Getenv("MEMORO_DATABASE_PATH"); dbPath != "" {
		config.Database.Path = dbPath
		configLogger.Debug("Database path loaded from environment variable")
	}

	// 验证关键配置项
	if config.LLM.APIKey == "" {
		configLogger.Warn("LLM API key is empty - some features may not work")
	}

	return nil
}

// Get 获取全局配置
func Get() *Config {
	if globalConfig == nil {
		configLogger.Error("Configuration not loaded", logger.Fields{
			"error": "globalConfig is nil",
		})
		return nil
	}
	return globalConfig
}

// GetLLMConfig 获取LLM配置
func GetLLMConfig() LLMConfig {
	if globalConfig == nil {
		return LLMConfig{}
	}
	return globalConfig.LLM
}

// GetWeChatConfig 获取微信配置
func GetWeChatConfig() WeChatConfig {
	if globalConfig == nil {
		return WeChatConfig{}
	}
	return globalConfig.WeChat
}

// GetDatabaseConfig 获取数据库配置
func GetDatabaseConfig() DatabaseConfig {
	if globalConfig == nil {
		return DatabaseConfig{}
	}
	return globalConfig.Database
}

// IsProduction 检查是否为生产环境
func IsProduction() bool {
	if globalConfig == nil {
		return false
	}
	return globalConfig.Server.Mode == "production"
}

// GetServerAddress 获取服务器地址
func GetServerAddress() string {
	if globalConfig == nil {
		return ":8080"
	}
	return fmt.Sprintf("%s:%d", globalConfig.Server.Host, globalConfig.Server.Port)
}
