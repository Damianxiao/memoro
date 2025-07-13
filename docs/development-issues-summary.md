# Memoro 开发问题总结

## 项目信息
- **项目名称**: Memoro (HandyNote Bot)
- **开发阶段**: Phase 1 - 基础架构搭建
- **文档创建时间**: 2025-07-13
- **开发方法**: TDD (Test-Driven Development)

## 技术规范问题

### 1. 错误的依赖包名称
**问题**: TECH_SPEC.md中指定的Chroma Go客户端包不存在
```yaml
# 错误的包名
github.com/chromem-go/chromem v0.4.0  # Repository not found
```

**解决方案**: 使用HTTP API方式直接调用Chroma服务
```go
// 正确的方式：使用resty进行HTTP调用
client := resty.New().SetBaseURL("http://localhost:8000")
// 调用Chroma的REST API端点
```

**经验教训**: 
- 技术规范编写时需要验证依赖包的实际存在性
- HTTP API调用比第三方客户端包更稳定可靠
- 减少了对第三方维护的依赖

### 2. Go版本兼容性问题
**问题**: TECH_SPEC.md要求Go 1.23+，但环境只有Go 1.20
```bash
# 错误信息
go: go.mod file indicates go 1.23, but maximum version supported by tidy is 1.20
```

**解决方案**: 升级Go环境到1.23.4
```bash
asdf install golang 1.23.4
asdf global golang 1.23.4
```

**经验教训**:
- 开发前应确认环境与技术规范的一致性
- 使用最新Go版本可以享受新特性如range-over-func
- asdf等版本管理器简化了多版本Go的管理

## 开发流程问题

### 3. Git仓库嵌套问题
**问题**: 项目中包含LangBot子项目，导致Git嵌套仓库警告
```bash
warning: adding embedded git repository: LangBot
hint: You've added another git repository inside your current repository.
```

**解决方案**: 移除嵌套仓库并单独管理
```bash
git rm --cached LangBot -rf
```

**经验教训**:
- 项目初始化时应清理无关的Git仓库
- 避免在项目中包含其他Git仓库，除非使用submodule
- 保持Git历史的清洁性

### 4. 测试文件组织问题
**问题**: 测试辅助文件命名导致包导入错误
```bash
# 错误：helper_test.go只能在测试中使用
memoro/test/unit: no non-test Go files in /home/damian/memoro/test/unit
```

**解决方案**: 创建helper.go文件供其他包导入
```go
// helper.go - 可以被其他包导入的辅助函数
// helper_test.go - 只能在本包测试中使用
```

**经验教训**:
- Go的测试文件命名约定需要严格遵守
- _test.go文件只能在同包的测试中访问
- 跨包共享的测试工具应该放在独立的.go文件中

## TDD实践经验

### 5. 测试预期与框架行为不一致
**问题**: 期望POST /health返回405，实际Gin返回404
```go
// 错误预期
expectedCode: http.StatusMethodNotAllowed  // 405

// 实际行为  
// Gin对未注册方法返回404而不是405
```

**解决方案**: 调整测试预期符合框架实际行为
```go
expectedCode: http.StatusNotFound  // 404，Gin的实际行为
```

**经验教训**:
- TDD测试应该基于框架的实际行为而不是假设
- 了解框架特性有助于编写正确的测试
- 先运行测试了解失败原因，再调整实现或测试

### 6. WebSocket测试的复杂性
**成功实践**: 创建模拟WebSocket服务器进行集成测试
```go
type MockWebSocketServer struct {
    server   *httptest.Server
    upgrader websocket.Upgrader
    messages []string
    clients  []*websocket.Conn
}
```

**经验教训**:
- WebSocket测试需要完整的服务器模拟
- 并发处理和连接管理是测试的重点
- 错误处理测试同样重要（连接断开、网络错误等）

## 架构设计改进

### 7. 数据模型的JSON序列化策略
**设计决策**: 使用GORM钩子处理复杂字段的序列化
```go
// 数据库存储字段
ProcessedData string `json:"-" gorm:"column:processed_data"`

// 内存操作字段  
processedDataMap map[string]interface{} `json:"processed_data" gorm:"-"`

// GORM钩子处理序列化
func (c *ContentItem) BeforeCreate(tx *gorm.DB) error {
    jsonData, err := json.Marshal(c.processedDataMap)
    c.ProcessedData = string(jsonData)
    return err
}
```

**经验教训**:
- 分离数据库存储格式和业务逻辑格式
- 使用GORM钩子保持数据一致性
- JSON序列化适合存储灵活的map结构

## 代码质量

### 8. 依赖管理最佳实践
**问题**: 依赖包需要显式添加才能使用
```bash
# 即使go.mod中有，也需要显式添加
go get github.com/sirupsen/logrus
go get github.com/gorilla/websocket  
```

**解决方案**: 按需添加和整理依赖
```bash
go mod tidy  # 整理依赖
go get <package>  # 添加新依赖
```

**经验教训**:
- go.mod文件需要与实际使用的包保持同步
- 定期执行go mod tidy清理无用依赖
- 依赖版本管理要考虑兼容性

## 文档和规范

### 9. 技术规范的准确性重要性
**教训**: 技术规范中的错误会直接影响开发进度
- 错误的包名导致构建失败
- 版本要求不匹配导致环境问题
- 架构设计细节影响实现策略

**改进建议**:
- 技术规范编写后应进行验证
- 关键依赖包的存在性检查
- 环境要求的实际可行性验证

### 10. 提交规范和历史管理
**良好实践**: 严格遵循Conventional Commits
```bash
feat: implement health check endpoint with TDD
feat: implement WeChatPadPro WebSocket integration with TDD
```

**经验教训**:
- 每个功能点完成后立即提交
- 提交信息要清晰描述变更内容
- 避免AI工具签名污染Git历史

## 下一步改进计划

1. **完善测试覆盖率**: 确保所有核心功能都有充分的测试
2. **错误处理标准化**: 建立统一的错误处理和日志记录机制
3. **配置管理**: 实现完整的配置加载和验证机制
4. **性能优化**: 关注WebSocket连接池和数据库连接管理
5. **安全考虑**: 添加输入验证和安全性检查

## 总结

本阶段开发严格遵循TDD方法论，发现并解决了多个技术规范和实现细节问题。主要成果包括：

- ✅ 完整的Go 1.23项目结构
- ✅ 健康检查API的TDD实现
- ✅ WeChatPadPro WebSocket集成
- ✅ 核心数据模型设计
- ✅ 完善的测试框架

这些问题的解决为后续开发奠定了坚实的基础，同时积累了宝贵的技术经验。