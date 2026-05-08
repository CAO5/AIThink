.PHONY: help build run clean test deps

help: ## 显示帮助信息
	@echo "Available commands:"
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-15s\033[0m %s\n", $$1, $$2}'

build: ## 编译程序
	@echo "编译程序..."
	go build -o bin/aithink.exe cmd/server/main.go
	@echo "编译完成: bin/aithink.exe"

run: ## 运行程序
	@echo "启动服务..."
	go run cmd/server/main.go

clean: ## 清理编译产物
	@echo "清理编译产物..."
	go clean
	rm -rf bin/
	@echo "清理完成"

deps: ## 安装依赖
	@echo "安装依赖..."
	go mod download
	go mod tidy
	@echo "依赖安装完成"

test: ## 运行测试
	@echo "运行测试..."
	go test ./...

dev: ## 开发模式（热重载需要安装air）
	@echo "开发模式..."
	air

install-tools: ## 安装开发工具
	go install github.com/cosmtrek/air@latest
	go install github.com/swaggo/swag/cmd/swag@latest
