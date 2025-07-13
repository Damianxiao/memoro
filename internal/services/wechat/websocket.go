package wechat

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"memoro/internal/errors"
	"memoro/internal/logger"
)

// MessageHandler 消息处理函数类型
type MessageHandler func(message []byte) error

// ErrorHandler 错误处理函数类型
type ErrorHandler func(err error)

// WeChatWebSocketClient WebSocket客户端
type WeChatWebSocketClient struct {
	serverURL      string
	adminKey       string
	conn           *websocket.Conn
	connected      bool
	mu             sync.RWMutex
	messageHandler MessageHandler
	errorHandler   ErrorHandler
	stopChan       chan struct{}
	logger         *logger.Logger
}

// NewWeChatWebSocketClient 创建新的WebSocket客户端
func NewWeChatWebSocketClient(serverURL, adminKey string) *WeChatWebSocketClient {
	return &WeChatWebSocketClient{
		serverURL: serverURL,
		adminKey:  adminKey,
		connected: false,
		stopChan:  make(chan struct{}),
		logger:    logger.NewLogger("wechat-websocket"),
	}
}

// Connect 连接到WebSocket服务器
func (c *WeChatWebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketConnect, "Client is already connected")
	}

	// 构造WebSocket URL
	wsURL, err := c.buildWebSocketURL()
	if err != nil {
		memoErr := errors.ErrWebSocketConnection("Failed to build WebSocket URL", err)
		c.logger.LogMemoroError(memoErr, "WebSocket URL construction failed")
		return memoErr
	}

	// 设置拨号器
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	c.logger.Info("Attempting to connect to WebSocket", logger.Fields{
		"url":     wsURL,
		"timeout": "10s",
	})

	// 连接WebSocket
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		memoErr := errors.ErrWebSocketConnection("Failed to establish WebSocket connection", err).
			WithContext(map[string]interface{}{
				"url":     wsURL,
				"timeout": "10s",
			})
		c.logger.LogMemoroError(memoErr, "WebSocket connection failed")
		return memoErr
	}

	c.conn = conn
	c.connected = true

	c.logger.Info("Successfully connected to WebSocket", logger.Fields{
		"url":           wsURL,
		"connection_id": fmt.Sprintf("%p", conn),
	})

	return nil
}

// Disconnect 断开WebSocket连接
func (c *WeChatWebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		c.logger.Debug("Disconnect called on already disconnected client")
		return nil
	}

	c.logger.Info("Disconnecting WebSocket client", logger.Fields{
		"connection_id": fmt.Sprintf("%p", c.conn),
	})

	// 发送停止信号
	close(c.stopChan)
	c.stopChan = make(chan struct{}) // 重新创建channel供下次使用

	// 关闭连接
	err := c.conn.Close()
	c.conn = nil
	c.connected = false

	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketConnect, "Failed to close WebSocket connection").
			WithCause(err)
		c.logger.LogMemoroError(memoErr, "Error during WebSocket disconnection")
		return memoErr
	}

	c.logger.Info("WebSocket client disconnected successfully")
	return nil
}

// IsConnected 检查连接状态
func (c *WeChatWebSocketClient) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.connected
}

// SetMessageHandler 设置消息处理器
func (c *WeChatWebSocketClient) SetMessageHandler(handler MessageHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.messageHandler = handler
	c.logger.Debug("Message handler set")
}

// SetErrorHandler 设置错误处理器
func (c *WeChatWebSocketClient) SetErrorHandler(handler ErrorHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorHandler = handler
	c.logger.Debug("Error handler set")
}

// StartListening 开始监听消息
func (c *WeChatWebSocketClient) StartListening() {
	if !c.IsConnected() {
		err := errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketConnect, "Cannot start listening: client is not connected")
		c.logger.LogMemoroError(err, "Failed to start message listening")
		c.handleError(err)
		return
	}

	c.logger.Info("Starting WebSocket message listening")

	for {
		select {
		case <-c.stopChan:
			c.logger.Info("Stopping message listening due to stop signal")
			return
		default:
			// 读取消息
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				memoErr := errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketMessage, "Failed to read WebSocket message").
					WithCause(err).
					WithContext(map[string]interface{}{
						"connection_id": fmt.Sprintf("%p", c.conn),
					})
				c.logger.LogMemoroError(memoErr, "WebSocket message read error")
				c.handleError(memoErr)
				return
			}

			// 只处理文本消息
			if messageType == websocket.TextMessage {
				c.logger.Debug("Received WebSocket message", logger.Fields{
					"message_type": "text",
					"message_size": len(message),
				})
				c.handleMessage(message)
			} else {
				c.logger.Debug("Ignored non-text WebSocket message", logger.Fields{
					"message_type": messageType,
				})
			}
		}
	}
}

// SendMessage 发送消息到WebSocket
func (c *WeChatWebSocketClient) SendMessage(message []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketMessage, "Cannot send message: client is not connected")
	}

	c.logger.Debug("Sending WebSocket message", logger.Fields{
		"message_size": len(message),
	})

	err := c.conn.WriteMessage(websocket.TextMessage, message)
	if err != nil {
		memoErr := errors.NewMemoroError(errors.ErrorTypeWebSocket, errors.ErrCodeWebSocketMessage, "Failed to send WebSocket message").
			WithCause(err).
			WithContext(map[string]interface{}{
				"message_size": len(message),
			})
		c.logger.LogMemoroError(memoErr, "WebSocket message send error")
		return memoErr
	}

	return nil
}

// buildWebSocketURL 构造WebSocket URL
func (c *WeChatWebSocketClient) buildWebSocketURL() (string, error) {
	u, err := url.Parse(c.serverURL)
	if err != nil {
		return "", err
	}

	// 添加查询参数
	q := u.Query()
	if c.adminKey != "" {
		q.Set("key", c.adminKey)
	}
	u.RawQuery = q.Encode()

	// 确保是WebSocket协议
	if u.Scheme == "http" {
		u.Scheme = "ws"
	} else if u.Scheme == "https" {
		u.Scheme = "wss"
	}

	return u.String(), nil
}

// handleMessage 处理接收到的消息
func (c *WeChatWebSocketClient) handleMessage(message []byte) {
	c.mu.RLock()
	handler := c.messageHandler
	c.mu.RUnlock()

	if handler != nil {
		if err := handler(message); err != nil {
			c.logger.Error("Message handler error", logger.Fields{
				"error":        err.Error(),
				"message_size": len(message),
			})
		}
	} else {
		c.logger.Debug("No message handler set, ignoring message")
	}
}

// handleError 处理错误
func (c *WeChatWebSocketClient) handleError(err error) {
	c.mu.RLock()
	handler := c.errorHandler
	c.mu.RUnlock()

	if memoErr, ok := err.(*errors.MemoroError); ok {
		c.logger.LogMemoroError(memoErr, "WebSocket error occurred")
	} else {
		c.logger.Error("WebSocket error occurred", logger.Fields{
			"error": err.Error(),
		})
	}

	if handler != nil {
		handler(err)
	}

	// 错误发生时断开连接
	c.Disconnect()
}
