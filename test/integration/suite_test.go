package integration

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestSuite 集成测试套件
type TestSuite struct {
	t      *testing.T
	db     *gorm.DB
	dbPath string
}

// NewTestSuite 创建集成测试套件
func NewTestSuite(t *testing.T) *TestSuite {
	return &TestSuite{t: t}
}

// SetupTestDB 设置测试数据库
func (ts *TestSuite) SetupTestDB() {
	// 创建临时数据库文件
	tempDir := os.TempDir()
	ts.dbPath = filepath.Join(tempDir, "memoro_test.db")

	// 如果文件存在则删除
	if _, err := os.Stat(ts.dbPath); err == nil {
		err = os.Remove(ts.dbPath)
		require.NoError(ts.t, err, "Failed to remove existing test database")
	}

	// 连接SQLite数据库
	db, err := gorm.Open(sqlite.Open(ts.dbPath), &gorm.Config{})
	require.NoError(ts.t, err, "Failed to connect to test database")

	ts.db = db
}

// CleanupTestDB 清理测试数据库
func (ts *TestSuite) CleanupTestDB() {
	if ts.db != nil {
		sqlDB, err := ts.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}

	if ts.dbPath != "" {
		os.Remove(ts.dbPath)
	}
}

// GetDB 获取数据库连接
func (ts *TestSuite) GetDB() *gorm.DB {
	return ts.db
}

// WaitForCondition 等待条件满足
func (ts *TestSuite) WaitForCondition(condition func() bool, timeout time.Duration, message string) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			ts.t.Fatalf("Timeout waiting for condition: %s", message)
		case <-ticker.C:
			if condition() {
				return
			}
		}
	}
}

// AssertEventually 断言条件最终成立
func (ts *TestSuite) AssertEventually(condition func() bool, timeout time.Duration, message string) {
	ts.WaitForCondition(condition, timeout, message)
}
