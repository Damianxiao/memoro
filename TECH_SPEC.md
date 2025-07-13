# Memoro 技术方案文档

## 项目概述

**项目名称**: Memoro (HandyNote Bot)  
**项目定位**: 个人知识管理助手 + 智能信息检索系统  
**目标用户**: 知识工作者、创业者、研究人员、终身学习者  
**核心价值**: 通过微信接口实现智能的信息收集、总结、归档和检索

## 核心功能

### MVP 阶段功能
1. **多媒体内容收集**: 文本、链接、文档、图片、音频、视频的自动收集和处理
2. **智能内容处理**: 自动总结、分类、标签生成、关键词提取
3. **语义搜索**: 基于向量数据库的智能检索和相关推荐

### 用户交互流程
```
用户发送 → 内容识别 → AI处理 → 存储归档 → 确认反馈
用户查询 → 语义搜索 → 结果排序 → AI整理 → 智能回复
```

## 技术架构

### 核心技术栈
- **编程语言**: Go 1.23+ (支持最新特性如range-over-func)
- **Web框架**: Gin (轻量高性能) + gorilla/websocket
- **数据库**: SQLite (轻量级，适合个人使用)
- **向量数据库**: Chroma (开源，支持本地部署)
- **LLM集成**: 通过配置文件支持第三方API
- **文件存储**: 本地文件系统
- **部署方式**: 原生二进制 + 外部数据库 (开发阶段)

### 系统架构图
```
┌─────────────────┐    ┌──────────────────┐    ┌─────────────────┐
│   微信接口层     │ -> │   消息处理中心   │ -> │   数据存储层    │
│ WeChatPadPro    │    │   Gin + WebSocket│    │ SQLite +        │
│ WebSocket       │    │   Go Routines    │    │ Chroma Vector   │
└─────────────────┘    └──────────────────┘    └─────────────────┘
                                │
                       ┌────────▼────────┐
                       │   AI处理层      │
                       │ 第三方LLM API   │
                       │ 内容分析引擎    │
                       └─────────────────┘
```

## 项目结构

```
/home/damian/memoro/
├── README.md                    # 项目说明文档
├── TECH_SPEC.md                 # 技术方案文档 (本文档)
├── .gitignore                   # Git忽略文件
├── go.mod                       # Go模块定义 (go 1.23)
├── go.sum                       # 依赖锁定文件
├── Makefile                     # 构建脚本
├── docker-compose.yml           # 生产环境容器编排
├── config/
│   ├── app.yaml                 # 应用配置文件
│   └── llm-api.md              # LLM API调用文档
├── cmd/
│   └── memoro/
│       └── main.go             # 应用程序入口
├── internal/                    # 内部代码 (不对外导出)
│   ├── config/                 # 配置管理
│   │   ├── config.go           # 配置结构定义
│   │   └── loader.go           # 配置加载器
│   ├── models/                 # 数据模型
│   │   ├── content.go          # 内容模型
│   │   ├── user.go             # 用户模型
│   │   └── search.go           # 搜索结果模型
│   ├── services/               # 业务逻辑服务
│   │   ├── wechat/             # 微信消息处理
│   │   │   ├── client.go       # WeChatPad客户端
│   │   │   ├── handler.go      # 消息处理器
│   │   │   └── websocket.go    # WebSocket连接管理
│   │   ├── content/            # 内容处理服务
│   │   │   ├── processor.go    # 内容处理器
│   │   │   ├── extractor.go    # 内容提取器
│   │   │   └── classifier.go   # 内容分类器
│   │   ├── llm/                # LLM调用服务
│   │   │   ├── client.go       # LLM客户端
│   │   │   ├── summarizer.go   # 内容总结
│   │   │   └── tagger.go       # 标签生成
│   │   ├── vector/             # 向量存储服务
│   │   │   ├── chroma.go       # Chroma数据库客户端
│   │   │   ├── embedding.go    # 向量化处理
│   │   │   └── similarity.go   # 相似度计算
│   │   └── search/             # 搜索服务
│   │       ├── engine.go       # 搜索引擎
│   │       ├── ranker.go       # 结果排序
│   │       └── recommender.go  # 推荐系统
│   ├── handlers/               # HTTP请求处理器
│   │   ├── health.go           # 健康检查
│   │   ├── webhook.go          # 微信Webhook
│   │   └── api.go              # REST API
│   ├── middleware/             # 中间件
│   │   ├── logging.go          # 日志中间件
│   │   ├── auth.go             # 认证中间件
│   │   └── cors.go             # 跨域中间件
│   └── storage/                # 存储层
│       ├── sqlite.go           # SQLite数据库操作
│       ├── file.go             # 文件系统操作
│       └── migration.go        # 数据库迁移
├── pkg/                        # 可导出的包
│   ├── wechatpad/              # WeChatPad客户端包
│   │   ├── client.go           # 客户端实现
│   │   ├── types.go            # 类型定义
│   │   └── utils.go            # 工具函数
│   └── utils/                  # 通用工具函数
│       ├── crypto.go           # 加密工具
│       ├── file.go             # 文件操作
│       └── text.go             # 文本处理
├── api/                        # API定义
│   └── v1/                     # API版本1
│       ├── content.go          # 内容相关API
│       └── search.go           # 搜索相关API
├── scripts/                    # 部署和工具脚本
│   ├── build.sh               # 构建脚本
│   ├── deploy.sh              # 部署脚本
│   └── migrate.sh             # 数据迁移脚本
├── test/                       # 测试文件
│   ├── integration/           # 集成测试
│   └── unit/                  # 单元测试
├── data/                       # 数据存储目录
│   ├── sqlite/                # SQLite数据库文件
│   ├── files/                 # 用户文件存储
│   ├── chroma/                # Chroma向量数据库
│   └── logs/                  # 应用日志
└── docs/                      # 项目文档
    ├── api.md                 # API文档
    ├── deployment.md          # 部署文档
    └── development.md         # 开发文档
```

## 核心依赖包

### go.mod 主要依赖
```go
module memoro

go 1.23

require (
    github.com/gin-gonic/gin v1.9.1              // Web框架
    github.com/gorilla/websocket v1.5.0          // WebSocket支持
    gorm.io/gorm v1.25.4                         // ORM框架
    gorm.io/driver/sqlite v1.5.3                 // SQLite驱动
    github.com/go-resty/resty/v2 v2.7.0          // HTTP客户端
    github.com/spf13/viper v1.16.0               // 配置管理
    github.com/sirupsen/logrus v1.9.3            // 结构化日志
    github.com/chromem-go/chromem v0.4.0         // Chroma Go客户端
    github.com/google/uuid v1.3.0                // UUID生成
    github.com/stretchr/testify v1.8.4           // 测试框架
)
```

## 数据模型设计

### 核心数据结构
```go
// ContentItem 内容项数据模型
type ContentItem struct {
    ID              string                 `json:"id" gorm:"primaryKey"`
    Type            ContentType            `json:"type"`                    // text, link, file, image, audio, video
    RawContent      string                 `json:"raw_content"`             // 原始内容
    ProcessedData   map[string]interface{} `json:"processed_data" gorm:"serializer:json"` // 处理后的数据
    Summary         Summary                `json:"summary" gorm:"embedded"` // 多层次摘要
    Tags            []string               `json:"tags" gorm:"serializer:json"` // 标签列表
    ImportanceScore float64                `json:"importance_score"`        // 重要性评分
    VectorID        string                 `json:"vector_id"`              // 向量数据库ID
    CreatedAt       time.Time              `json:"created_at"`
    UpdatedAt       time.Time              `json:"updated_at"`
    UserID          string                 `json:"user_id"`                // 用户ID (未来扩展)
}

// Summary 摘要结构
type Summary struct {
    OneLine   string `json:"one_line"`   // 一句话摘要
    Paragraph string `json:"paragraph"`  // 段落摘要
    Detailed  string `json:"detailed"`   // 详细摘要
}
```

## MVP开发计划

### Phase 1: 基础架构搭建 (Week 1)
**目标**: 建立项目基础框架和开发环境

#### 任务清单:
- [x] 检查和安装Go 1.23+环境
- [x] 创建项目结构和初始化Go模块
- [ ] 配置基础的Gin Web服务
- [ ] 实现WeChatPadPro WebSocket连接
- [ ] 设置SQLite数据库和基础数据模型
- [ ] 实现基础的日志和配置管理

#### 验收标准:
- Web服务可以启动并响应健康检查
- 可以接收WeChatPad的WebSocket消息
- 数据库可以正常读写
- 日志输出格式正确

### Phase 2: 核心功能实现 (Week 2)
**目标**: 实现内容收集、处理和存储功能

#### 任务清单:
- [ ] 实现多种内容类型的识别和提取
- [ ] 集成第三方LLM API进行内容总结
- [ ] 实现自动标签生成和分类
- [ ] 建立内容存储和管理机制
- [ ] 实现基础的微信交互逻辑

#### 验收标准:
- 能够处理文本、链接、文件等多种内容
- LLM可以正确生成摘要和标签
- 内容可以正确存储到数据库
- 微信可以收到处理确认消息

### Phase 3: 智能检索系统 (Week 3)
**目标**: 实现语义搜索和智能推荐功能

#### 任务清单:
- [ ] 部署和配置Chroma向量数据库
- [ ] 实现内容向量化和相似度搜索
- [ ] 建立智能问答和推荐系统
- [ ] 优化搜索结果排序算法
- [ ] 完善用户交互体验

#### 验收标准:
- 语义搜索可以返回相关内容
- 推荐系统可以主动发现关联信息
- 搜索响应时间在可接受范围内
- 用户可以通过微信进行自然交互

## 数据流设计

### 内容收集流程
```
微信消息输入 → 消息类型识别 → 内容提取 → LLM处理 → 向量化 → 双重存储
                                                              ↓
                                                    SQLite + Chroma
```

### 智能检索流程
```
用户查询 → 查询理解 → 向量搜索 → 结果筛选 → LLM整理 → 微信回复
            ↓           ↓          ↓         ↓
        语义分析   相似度计算   排序算法   内容生成
```

## 配置管理

### 应用配置文件结构 (config/app.yaml)
```yaml
server:
  host: "0.0.0.0"
  port: 8080
  mode: "development"

wechat:
  pad_url: "http://localhost:1239"
  websocket_url: "ws://localhost:1239/ws"
  admin_key: "12345"
  token: ""
  wxid: "wxid_w3a18zqallvs12"

database:
  type: "sqlite"
  path: "./data/sqlite/memoro.db"
  auto_migrate: true

vector_db:
  type: "chroma"
  host: "localhost"
  port: 8000
  collection: "memoro_content"

llm:
  provider: "custom"
  api_base: ""
  api_key: ""
  model: ""
  max_tokens: 4000

storage:
  file_path: "./data/files"
  max_file_size: "50MB"

logging:
  level: "info"
  format: "json"
  output: "./data/logs/app.log"
```

## 部署方案

### 开发环境 (原生二进制)
1. **安装依赖**: Go 1.23+, SQLite, Chroma
2. **编译运行**: `make build && ./bin/memoro`
3. **配置文件**: 复制并修改 `config/app.yaml`
4. **数据目录**: 自动创建 `data/` 目录结构

### 生产环境 (Docker容器)
```yaml
# docker-compose.yml (生产环境)
version: '3.8'
services:
  memoro:
    build: .
    ports:
      - "8080:8080"
    volumes:
      - ./data:/app/data
      - ./config:/app/config
    depends_on:
      - chroma
    environment:
      - APP_ENV=production

  chroma:
    image: chromadb/chroma:latest
    ports:
      - "8000:8000"
    volumes:
      - chroma_data:/chroma/data

volumes:
  chroma_data:
```

## 性能优化策略

### 并发处理
- 使用Go routines处理WebSocket消息
- 异步处理LLM API调用
- 批量向量化减少API调用次数

### 缓存策略
- LLM响应结果缓存
- 向量搜索结果缓存
- 文件内容提取结果缓存

### 存储优化
- SQLite WAL模式提高并发性
- 文件分片存储大型文档
- 定期清理过期的临时文件

## 扩展规划

### 短期扩展 (1-3个月)
- 支持更多文件格式 (Excel, PowerPoint等)
- 实现内容定期回顾和提醒功能
- 添加用户自定义标签体系
- 支持多用户和权限管理

### 长期扩展 (3-12个月)
- 支持多平台集成 (QQ, 钉钉, Slack等)
- 实现知识图谱可视化
- 添加团队协作功能
- 开放API生态和插件系统

## 风险评估和预案

### 技术风险
- **LLM API限流**: 实现重试机制和降级策略
- **向量数据库性能**: 优化索引和分片策略
- **微信接口稳定性**: 实现自动重连和状态监控

### 业务风险
- **数据安全**: 实现数据加密和备份机制
- **隐私保护**: 本地存储减少隐私泄露风险
- **用户体验**: 持续收集反馈并快速迭代

---

**文档版本**: v1.0  
**创建日期**: 2025-01-13  
**最后更新**: 2025-01-13  
**维护者**: Claude & User