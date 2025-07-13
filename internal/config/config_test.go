package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConfigLoad(t *testing.T) {
	// 保存原始globalConfig并在测试后恢复
	originalConfig := globalConfig
	defer func() {
		globalConfig = originalConfig
	}()

	// 创建临时配置文件
	tempConfig := `
server:
  host: "127.0.0.1"
  port: 9090
  mode: "development"
  read_timeout: "20s"
  write_timeout: "20s"
  shutdown_timeout: "15s"

wechat:
  websocket_url: "ws://localhost:1239/ws"
  admin_key: "test_key"

database:
  type: "sqlite"
  path: "./test.db"
  auto_migrate: true

vector_db:
  type: "chroma"
  host: "localhost"
  port: 8000
  collection: "test_collection"

llm:
  provider: "openai_compatible"
  api_base: "https://api.test.com/v1"
  model: "gpt-4"
  max_tokens: 1000
  temperature: 0.7
  timeout: "30s"
  retry_times: 2
  retry_delay: "1s"
  rate_limit:
    requests_per_minute: 10
    burst_size: 3

storage:
  file_path: "./test_files"
  max_file_size: "10MB"
  allowed_types:
    - "text/plain"

logging:
  level: "debug"
  format: "json"
  output: "stdout"

processing:
  max_workers: 5
  queue_size: 100
  timeout: "60s"
  content_size_limit: "50KB"
  summary_levels:
    one_line_max_length: 100
    paragraph_max_length: 500
    detailed_max_length: 2000
  tag_limits:
    max_tags: 20
    max_tag_length: 50

cache:
  enabled: true
  type: "memory"
  ttl: "30m"
  max_items: 1000
  cleanup_interval: "5m"

security:
  api_key_header: "X-Test-Key"
  cors_enabled: false
  cors_origins: []
  rate_limiting:
    enabled: true
    requests_per_minute: 30
    burst_size: 5

monitoring:
  metrics_enabled: false
  health_check_interval: "15s"
  performance_tracking: false
`

	// 写入临时文件
	tmpFile, err := os.CreateTemp("", "test_config_*.yaml")
	require.NoError(t, err)
	defer os.Remove(tmpFile.Name())

	_, err = tmpFile.WriteString(tempConfig)
	require.NoError(t, err)
	tmpFile.Close()

	// 测试配置加载
	config, err := Load(tmpFile.Name())
	require.NoError(t, err)
	require.NotNil(t, config)

	// 验证服务器配置
	assert.Equal(t, "127.0.0.1", config.Server.Host)
	assert.Equal(t, 9090, config.Server.Port)
	assert.Equal(t, "development", config.Server.Mode)
	assert.Equal(t, 20*time.Second, config.Server.ReadTimeout)
	assert.Equal(t, 20*time.Second, config.Server.WriteTimeout)
	assert.Equal(t, 15*time.Second, config.Server.ShutdownTimeout)

	// 验证微信配置
	assert.Equal(t, "ws://localhost:1239/ws", config.WeChat.WebSocketURL)
	assert.Equal(t, "test_key", config.WeChat.AdminKey)

	// 验证数据库配置
	assert.Equal(t, "sqlite", config.Database.Type)
	assert.Equal(t, "./test.db", config.Database.Path)
	assert.True(t, config.Database.AutoMigrate)

	// 验证LLM配置
	assert.Equal(t, "openai_compatible", config.LLM.Provider)
	assert.Equal(t, "https://api.test.com/v1", config.LLM.APIBase)
	assert.Equal(t, "gpt-4", config.LLM.Model)
	assert.Equal(t, 1000, config.LLM.MaxTokens)
	assert.Equal(t, 0.7, config.LLM.Temperature)
	assert.Equal(t, 30*time.Second, config.LLM.Timeout)
	assert.Equal(t, 2, config.LLM.RetryTimes)
	assert.Equal(t, 10, config.LLM.RateLimit.RequestsPerMinute)

	// 验证存储配置
	assert.Equal(t, "./test_files", config.Storage.FilePath)
	assert.Equal(t, "10MB", config.Storage.MaxFileSize)
	assert.Contains(t, config.Storage.AllowedTypes, "text/plain")

	// 验证日志配置
	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)

	// 验证处理配置
	assert.Equal(t, 5, config.Processing.MaxWorkers)
	assert.Equal(t, 100, config.Processing.QueueSize)
	assert.Equal(t, 60*time.Second, config.Processing.Timeout)
	assert.Equal(t, "50KB", config.Processing.ContentSizeLimit)

	// 验证缓存配置
	assert.True(t, config.Cache.Enabled)
	assert.Equal(t, "memory", config.Cache.Type)
	assert.Equal(t, 30*time.Minute, config.Cache.TTL)

	// 验证安全配置
	assert.Equal(t, "X-Test-Key", config.Security.APIKeyHeader)
	assert.False(t, config.Security.CORSEnabled)

	// 验证监控配置
	assert.False(t, config.Monitoring.MetricsEnabled)
	assert.Equal(t, 15*time.Second, config.Monitoring.HealthCheckInterval)
}

func TestConfigValidation(t *testing.T) {
	tests := []struct {
		name        string
		config      *Config
		expectError bool
		errorField  string
	}{
		{
			name: "Valid config",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "development",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "ws://localhost:1239/ws",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 0.5,
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
			},
			expectError: false,
		},
		{
			name: "Invalid port - too high",
			config: &Config{
				Server: ServerConfig{
					Port: 99999,
					Mode: "development",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "ws://localhost:1239/ws",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 0.5,
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
			},
			expectError: true,
			errorField:  "server.port",
		},
		{
			name: "Invalid mode",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "invalid_mode",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "ws://localhost:1239/ws",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 0.5,
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
			},
			expectError: true,
			errorField:  "server.mode",
		},
		{
			name: "Missing WebSocket URL",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "development",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 0.5,
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
			},
			expectError: true,
			errorField:  "wechat.websocket_url",
		},
		{
			name: "Invalid LLM temperature",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "development",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "ws://localhost:1239/ws",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 3.0, // Invalid
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "info",
				},
			},
			expectError: true,
			errorField:  "llm.temperature",
		},
		{
			name: "Invalid log level",
			config: &Config{
				Server: ServerConfig{
					Port: 8080,
					Mode: "development",
				},
				WeChat: WeChatConfig{
					WebSocketURL: "ws://localhost:1239/ws",
				},
				Database: DatabaseConfig{
					Type: "sqlite",
					Path: "./test.db",
				},
				LLM: LLMConfig{
					APIBase:     "https://api.test.com/v1",
					Model:       "gpt-4",
					MaxTokens:   1000,
					Temperature: 0.5,
				},
				VectorDB: VectorDBConfig{
					Type:       "chroma",
					Collection: "test",
				},
				Storage: StorageConfig{
					FilePath: "./files",
				},
				Logging: LoggingConfig{
					Level: "invalid",
				},
			},
			expectError: true,
			errorField:  "logging.level",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
				if tt.errorField != "" {
					assert.Contains(t, err.Error(), tt.errorField)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEnvironmentOverrides(t *testing.T) {
	// 保存原始环境变量
	originalAPIKey := os.Getenv("MEMORO_LLM_API_KEY")
	originalAdminKey := os.Getenv("MEMORO_WECHAT_ADMIN_KEY")
	originalDBPath := os.Getenv("MEMORO_DATABASE_PATH")

	// 清理环境变量
	defer func() {
		if originalAPIKey != "" {
			os.Setenv("MEMORO_LLM_API_KEY", originalAPIKey)
		} else {
			os.Unsetenv("MEMORO_LLM_API_KEY")
		}
		if originalAdminKey != "" {
			os.Setenv("MEMORO_WECHAT_ADMIN_KEY", originalAdminKey)
		} else {
			os.Unsetenv("MEMORO_WECHAT_ADMIN_KEY")
		}
		if originalDBPath != "" {
			os.Setenv("MEMORO_DATABASE_PATH", originalDBPath)
		} else {
			os.Unsetenv("MEMORO_DATABASE_PATH")
		}
	}()

	// 设置测试环境变量
	os.Setenv("MEMORO_LLM_API_KEY", "test_api_key")
	os.Setenv("MEMORO_WECHAT_ADMIN_KEY", "test_admin_key")
	os.Setenv("MEMORO_DATABASE_PATH", "/test/db/path")

	config := &Config{
		LLM: LLMConfig{
			APIKey: "",
		},
		WeChat: WeChatConfig{
			AdminKey: "",
		},
		Database: DatabaseConfig{
			Path: "",
		},
	}

	err := processEnvironmentOverrides(config)
	require.NoError(t, err)

	assert.Equal(t, "test_api_key", config.LLM.APIKey)
	assert.Equal(t, "test_admin_key", config.WeChat.AdminKey)
	assert.Equal(t, "/test/db/path", config.Database.Path)
}

func TestConfigHelperFunctions(t *testing.T) {
	// 保存原始globalConfig并在测试后恢复
	originalConfig := globalConfig
	defer func() {
		globalConfig = originalConfig
	}()

	// 重置globalConfig为nil以测试空状态
	globalConfig = nil

	// 测试在没有加载配置时的行为
	assert.Nil(t, Get())
	assert.Equal(t, LLMConfig{}, GetLLMConfig())
	assert.Equal(t, WeChatConfig{}, GetWeChatConfig())
	assert.Equal(t, DatabaseConfig{}, GetDatabaseConfig())
	assert.False(t, IsProduction())
	assert.Equal(t, ":8080", GetServerAddress())

	// 设置测试配置
	testConfig := &Config{
		Server: ServerConfig{
			Host: "localhost",
			Port: 9090,
			Mode: "production",
		},
		LLM: LLMConfig{
			Provider: "test",
		},
		WeChat: WeChatConfig{
			AdminKey: "test_key",
		},
		Database: DatabaseConfig{
			Type: "sqlite",
		},
	}

	globalConfig = testConfig

	// 测试辅助函数
	assert.Equal(t, testConfig, Get())
	assert.Equal(t, testConfig.LLM, GetLLMConfig())
	assert.Equal(t, testConfig.WeChat, GetWeChatConfig())
	assert.Equal(t, testConfig.Database, GetDatabaseConfig())
	assert.True(t, IsProduction())
	assert.Equal(t, "localhost:9090", GetServerAddress())
}