package handlers

import (
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"memoro/test/unit"
)

func TestHealthHandler(t *testing.T) {
	helper := unit.NewTestHelper(t)

	tests := []struct {
		name           string
		method         string
		expectedCode   int
		expectedFields []string
	}{
		{
			name:           "GET /health should return 200 OK",
			method:         "GET",
			expectedCode:   http.StatusOK,
			expectedFields: []string{"status", "timestamp"},
		},
		{
			name:         "POST /health should return 404 Not Found",
			method:       "POST",
			expectedCode: http.StatusNotFound, // Gin默认返回404给未注册的方法
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 设置测试路由
			r := helper.SetupTestGin()

			// 这里会调用尚未实现的handler
			RegisterHealthRoutes(r) // 这个函数还不存在，测试会失败

			// 执行请求
			req := helper.MakeRequest(tt.method, "/health", nil)
			w := helper.ExecuteRequest(r, req)

			// 断言状态码
			helper.AssertStatusCode(w, tt.expectedCode)

			// 如果是成功响应，检查JSON字段
			if tt.expectedCode == http.StatusOK {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				if err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}

				// 验证必要字段存在
				for _, field := range tt.expectedFields {
					if _, exists := response[field]; !exists {
						t.Errorf("Expected field '%s' not found in response", field)
					}
				}

				// 验证timestamp是最近的时间
				if timestamp, ok := response["timestamp"].(float64); ok {
					now := time.Now().Unix()
					if abs(int64(timestamp)-now) > 5 { // 允许5秒误差
						t.Errorf("Timestamp %d is not recent (now: %d)", int64(timestamp), now)
					}
				}

				// 验证status字段值
				if status, ok := response["status"].(string); ok {
					if status != "ok" {
						t.Errorf("Expected status 'ok', got '%s'", status)
					}
				}
			}
		})
	}
}

// abs 返回整数的绝对值
func abs(x int64) int64 {
	if x < 0 {
		return -x
	}
	return x
}
