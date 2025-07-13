.PHONY: build run test clean deps fmt lint vet

# Go参数
GOCMD=go
GOBUILD=$(GOCMD) build
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
GOMOD=$(GOCMD) mod
BINARY_NAME=memoro
BINARY_UNIX=memoro-unix

# 构建项目
build:
	$(GOBUILD) -o bin/$(BINARY_NAME) -v ./cmd/memoro

# 运行项目
run:
	$(GOBUILD) -o bin/$(BINARY_NAME) -v ./cmd/memoro
	./bin/$(BINARY_NAME)

# 运行测试
test:
	$(GOTEST) -v ./...

# 清理构建文件
clean:
	$(GOCLEAN)
	rm -f bin/$(BINARY_NAME)
	rm -f bin/$(BINARY_UNIX)

# 下载依赖
deps:
	$(GOMOD) download
	$(GOMOD) tidy

# 格式化代码
fmt:
	$(GOCMD) fmt ./...

# 代码检查
lint:
	golangci-lint run

# 代码检查
vet:
	$(GOCMD) vet ./...

# 构建Linux版本
build-linux:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOBUILD) -o bin/$(BINARY_UNIX) -v ./cmd/memoro

# 安装开发工具
install-tools:
	$(GOGET) -u github.com/golangci/golangci-lint/cmd/golangci-lint

# 创建目录
setup:
	mkdir -p data/sqlite data/files data/logs data/chroma bin