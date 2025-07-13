package unit

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestHelper 提供测试辅助函数
type TestHelper struct {
	t *testing.T
}

// NewTestHelper 创建测试辅助实例
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// SetupTestGin 设置测试用的Gin引擎
func (h *TestHelper) SetupTestGin() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	return r
}

// MakeRequest 创建HTTP测试请求
func (h *TestHelper) MakeRequest(method, url string, body interface{}) *http.Request {
	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		require.NoError(h.t, err, "Failed to marshal request body")
	}

	req, err := http.NewRequest(method, url, bytes.NewBuffer(reqBody))
	require.NoError(h.t, err, "Failed to create request")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return req
}

// ExecuteRequest 执行HTTP测试请求
func (h *TestHelper) ExecuteRequest(r *gin.Engine, req *http.Request) *httptest.ResponseRecorder {
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// AssertJSONResponse 断言JSON响应
func (h *TestHelper) AssertJSONResponse(w *httptest.ResponseRecorder, expectedCode int, expectedBody interface{}) {
	assert.Equal(h.t, expectedCode, w.Code, "Response code should match")
	assert.Equal(h.t, "application/json; charset=utf-8", w.Header().Get("Content-Type"), "Content-Type should be JSON")

	if expectedBody != nil {
		var actualBody interface{}
		err := json.Unmarshal(w.Body.Bytes(), &actualBody)
		require.NoError(h.t, err, "Response body should be valid JSON")

		expectedJSON, err := json.Marshal(expectedBody)
		require.NoError(h.t, err, "Expected body should be valid JSON")

		var expectedBodyNormalized interface{}
		err = json.Unmarshal(expectedJSON, &expectedBodyNormalized)
		require.NoError(h.t, err, "Expected body should unmarshal correctly")

		assert.Equal(h.t, expectedBodyNormalized, actualBody, "Response body should match expected")
	}
}

// AssertStatusCode 断言状态码
func (h *TestHelper) AssertStatusCode(w *httptest.ResponseRecorder, expectedCode int) {
	assert.Equal(h.t, expectedCode, w.Code, "Response code should match")
}
