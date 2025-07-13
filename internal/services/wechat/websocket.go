package wechat

import (
	"context"
	"fmt"
	"net/url"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/sirupsen/logrus"
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
	logger         *logrus.Logger
}

// NewWeChatWebSocketClient 创建新的WebSocket客户端
func NewWeChatWebSocketClient(serverURL, adminKey string) *WeChatWebSocketClient {
	logger := logrus.New()
	logger.SetLevel(logrus.InfoLevel)

	return &WeChatWebSocketClient{
		serverURL: serverURL,
		adminKey:  adminKey,
		connected: false,
		stopChan:  make(chan struct{}),
		logger:    logger,
	}
}

// Connect 连接到WebSocket服务器
func (c *WeChatWebSocketClient) Connect(ctx context.Context) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.connected {
		return fmt.Errorf("client is already connected")
	}

	// 构造WebSocket URL
	wsURL, err := c.buildWebSocketURL()
	if err != nil {
		return fmt.Errorf("failed to build WebSocket URL: %w", err)
	}

	// 设置拨号器
	dialer := websocket.DefaultDialer
	dialer.HandshakeTimeout = 10 * time.Second

	// 连接WebSocket
	conn, _, err := dialer.DialContext(ctx, wsURL, nil)
	if err != nil {
		return fmt.Errorf("failed to connect to WebSocket: %w", err)
	}

	c.conn = conn
	c.connected = true
	c.logger.Infof("Connected to WeChatPad WebSocket: %s", wsURL)

	return nil
}

// Disconnect 断开WebSocket连接
func (c *WeChatWebSocketClient) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.connected || c.conn == nil {
		return nil
	}

	// 发送停止信号
	close(c.stopChan)
	c.stopChan = make(chan struct{}) // 重新创建channel供下次使用

	// 关闭连接
	err := c.conn.Close()
	c.conn = nil
	c.connected = false

	c.logger.Info("Disconnected from WeChatPad WebSocket")
	return err
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
}

// SetErrorHandler 设置错误处理器
func (c *WeChatWebSocketClient) SetErrorHandler(handler ErrorHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.errorHandler = handler
}

// StartListening 开始监听消息
func (c *WeChatWebSocketClient) StartListening() {
	if !c.IsConnected() {
		c.handleError(fmt.Errorf("client is not connected"))
		return
	}

	for {
		select {
		case <-c.stopChan:
			c.logger.Info("Stopping message listening")
			return
		default:
			// 读取消息
			messageType, message, err := c.conn.ReadMessage()
			if err != nil {
				c.handleError(fmt.Errorf("failed to read message: %w", err))
				return
			}

			// 只处理文本消息
			if messageType == websocket.TextMessage {
				c.handleMessage(message)
			}
		}
	}
}

// SendMessage 发送消息到WebSocket
func (c *WeChatWebSocketClient) SendMessage(message []byte) error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if !c.connected || c.conn == nil {
		return fmt.Errorf("client is not connected")
	}

	return c.conn.WriteMessage(websocket.TextMessage, message)
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
			c.logger.Errorf("Message handler error: %v", err)
		}
	}
}

// handleError 处理错误
func (c *WeChatWebSocketClient) handleError(err error) {
	c.mu.RLock()
	handler := c.errorHandler
	c.mu.RUnlock()

	c.logger.Errorf("WebSocket error: %v", err)

	if handler != nil {
		handler(err)
	}

	// 错误发生时断开连接
	c.Disconnect()
}