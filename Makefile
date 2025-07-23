# YouTube Tools Makefile

.PHONY: help build run test clean docs swagger

# 默认目标
help:
	@echo "Available commands:"
	@echo "  build    - Build the application"
	@echo "  run      - Run the application"
	@echo "  test     - Run tests"
	@echo "  clean    - Clean build artifacts"
	@echo "  docs     - Generate Swagger documentation"
	@echo "  swagger  - Generate and serve Swagger docs"

# 构建应用
build:
	mkdir -p build
	go build -o build/youtube-tools cmd/api/main.go

# 运行应用
run: build
	./build/youtube-tools

# 运行测试
test:
	go test ./...

# 清理构建产物
clean:
	rm -rf build/
	rm -f docs/docs.go docs/swagger.json docs/swagger.yaml

# 生成 Swagger 文档
docs:
	@echo "Generating Swagger documentation..."
	@if ! command -v swag >/dev/null 2>&1; then \
		echo "Installing swag..."; \
		go install github.com/swaggo/swag/cmd/swag@latest; \
	fi
	mkdir -p docs
	$(shell go env GOPATH)/bin/swag init -g cmd/api/main.go -o docs
	@echo "Swagger documentation generated successfully!"

# 生成文档并启动服务
swagger: docs build
	@echo "Starting server with Swagger documentation..."
	@echo "Swagger UI will be available at: http://localhost:8080/swagger/index.html"
	./build/youtube-tools

# 安装依赖
deps:
	go mod download
	go mod tidy

# 格式化代码
fmt:
	go fmt ./...

# 代码检查
vet:
	go vet ./...

# 完整检查（格式化 + 检查 + 测试）
check: fmt vet test

# 开发模式（生成文档 + 运行）
dev: docs run