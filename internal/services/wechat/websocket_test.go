package wechat

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// MockWebSocketServer 模拟WebSocket服务器
type MockWebSocketServer struct {
	server   *httptest.Server
	upgrader websocket.Upgrader
	messages []string
	clients  []*websocket.Conn
}

// NewMockWebSocketServer 创建模拟WebSocket服务器
func NewMockWebSocketServer() *MockWebSocketServer {
	mock := &MockWebSocketServer{
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
		messages: make([]string, 0),
		clients:  make([]*websocket.Conn, 0),
	}

	mock.server = httptest.NewServer(http.HandlerFunc(mock.handleWebSocket))
	return mock
}

func (m *MockWebSocketServer) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := m.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	m.clients = append(m.clients, conn)

	// 模拟WeChatPad消息
	go func() {
		defer conn.Close()
		for {
			_, message, err := conn.ReadMessage()
			if err != nil {
				break
			}
			m.messages = append(m.messages, string(message))
		}
	}()
}

func (m *MockWebSocketServer) GetURL() string {
	return strings.Replace(m.server.URL, "http://", "ws://", 1)
}

func (m *MockWebSocketServer) SendMessage(message string) error {
	for _, client := range m.clients {
		if err := client.WriteMessage(websocket.TextMessage, []byte(message)); err != nil {
			return err
		}
	}
	return nil
}

func (m *MockWebSocketServer) Close() {
	for _, client := range m.clients {
		client.Close()
	}
	m.server.Close()
}

func TestWeChatWebSocketClient(t *testing.T) {
	// 创建模拟服务器
	mockServer := NewMockWebSocketServer()
	defer mockServer.Close()

	tests := []struct {
		name        string
		serverURL   string
		adminKey    string
		expectError bool
		description string
	}{
		{
			name:        "successful connection",
			serverURL:   mockServer.GetURL(),
			adminKey:    "12345",
			expectError: false,
			description: "应该能够成功连接到WeChatPad WebSocket",
		},
		{
			name:        "invalid URL",
			serverURL:   "ws://invalid-url:9999",
			adminKey:    "12345",
			expectError: true,
			description: "无效URL应该返回连接错误",
		},
		{
			name:        "empty admin key",
			serverURL:   mockServer.GetURL(),
			adminKey:    "",
			expectError: false, // URL构造应该成功，但连接可能失败
			description: "空admin key应该在URL构造时处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 这里会调用尚未实现的WebSocket客户端
			client := NewWeChatWebSocketClient(tt.serverURL, tt.adminKey)
			require.NotNil(t, client, "客户端不应该为nil")

			// 尝试连接
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			err := client.Connect(ctx)

			if tt.expectError {
				assert.Error(t, err, tt.description)
			} else {
				assert.NoError(t, err, tt.description)

				// 如果连接成功，测试基本功能
				if err == nil {
					// 测试连接状态
					assert.True(t, client.IsConnected(), "客户端应该报告已连接状态")

					// 测试断开连接
					err = client.Disconnect()
					assert.NoError(t, err, "断开连接不应该有错误")
					assert.False(t, client.IsConnected(), "断开后应该报告未连接状态")
				}
			}
		})
	}
}

func TestWeChatWebSocketClient_MessageHandling(t *testing.T) {
	mockServer := NewMockWebSocketServer()
	defer mockServer.Close()

	client := NewWeChatWebSocketClient(mockServer.GetURL(), "12345")
	require.NotNil(t, client)

	// 连接到服务器
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err, "连接应该成功")
	defer client.Disconnect()

	// 测试消息处理
	messageReceived := make(chan string, 1)
	client.SetMessageHandler(func(message []byte) error {
		messageReceived <- string(message)
		return nil
	})

	// 启动消息监听
	go client.StartListening()

	// 从服务器发送消息
	testMessage := `{"type":"text","content":"hello world","from":"test_user"}`
	err = mockServer.SendMessage(testMessage)
	require.NoError(t, err)

	// 验证消息接收
	select {
	case received := <-messageReceived:
		assert.Equal(t, testMessage, received, "接收的消息应该匹配发送的消息")
	case <-time.After(1 * time.Second):
		t.Fatal("1秒内没有接收到消息")
	}
}

func TestWeChatWebSocketClient_ErrorHandling(t *testing.T) {
	mockServer := NewMockWebSocketServer()
	defer mockServer.Close()

	client := NewWeChatWebSocketClient(mockServer.GetURL(), "12345")
	require.NotNil(t, client)

	// 测试错误处理
	errorReceived := make(chan error, 1)
	client.SetErrorHandler(func(err error) {
		errorReceived <- err
	})

	// 连接到服务器
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err := client.Connect(ctx)
	require.NoError(t, err)

	// 启动监听
	go client.StartListening()

	// 关闭服务器以触发错误
	mockServer.Close()

	// 验证错误处理
	select {
	case err := <-errorReceived:
		assert.Error(t, err, "应该接收到连接错误")
	case <-time.After(2 * time.Second):
		t.Fatal("2秒内没有接收到错误")
	}
}
