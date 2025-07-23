# YouTube Tools

一个基于 Golang 的 RESTful API 服务，用于包装 yt-dlp 工具的操作，提供视频下载、信息获取等功能。

## 功能特点

- 获取视频信息：标题、上传者、时长、格式等
- 下载视频：支持多种格式和质量选择
- 异步下载：支持后台下载和状态查询
- RESTful API：标准的 HTTP API 接口
- Swagger 文档：自动生成的 API 文档
- 日志记录：结构化日志记录所有操作

## 技术栈

- **编程语言**：Golang 1.24.2
- **Web 框架**：Gin
- **日志框架**：zap
- **API 文档**：Swagger/OpenAPI
- **容器化**：Docker
- **部署**：Kubernetes
- **视频处理工具**：yt-dlp, ffmpeg

## 快速开始

### 前提条件

- Go 1.24.2 或更高版本
- yt-dlp
- ffmpeg

### 安装

1. 克隆仓库

```bash
git clone https://github.com/self-made-boy/youtube-tools.git
cd youtube-tools
```

2. 安装依赖

```bash
go mod download
```

3. 生成 API 文档并构建项目

```bash
make docs
make build
```

4. 运行服务

```bash
make run
```

或者一键生成文档并启动服务：

```bash
make swagger
```

默认情况下，服务将在 `http://localhost:8080` 上运行。

### 使用 Docker

1. 构建 Docker 镜像

```bash
docker build -t youtube-tools .
```

2. 运行容器

```bash
docker run -p 8080:8080 -v /path/to/downloads:/app/downloads youtube-tools
```

### 使用 Kubernetes

```bash
kubectl apply -f deploy/k8s.yml
```

## 开发工具

### Makefile 命令

项目提供了 Makefile 来简化常用操作：

```bash
make help      # 显示所有可用命令
make build     # 构建应用程序
make run       # 运行应用程序
make test      # 运行测试
make clean     # 清理构建产物
make docs      # 生成 Swagger 文档
make swagger   # 生成文档并启动服务
make deps      # 安装依赖
make fmt       # 格式化代码
make vet       # 代码检查
make check     # 完整检查（格式化 + 检查 + 测试）
make dev       # 开发模式（生成文档 + 运行）
```

### 文档生成

项目使用 Swagger 自动生成 API 文档。当你修改了 API 接口或注释后，需要重新生成文档：

```bash
make docs
```

这将自动安装 `swag` 工具（如果未安装）并生成最新的 API 文档。

## API 使用

### API 文档

访问 `http://localhost:8080/swagger/index.html` 查看完整的 API 文档。

### 示例

#### 获取视频信息

```bash
curl -X GET "http://localhost:8080/api/v1/info?url=https://www.youtube.com/watch?v=example"
```

#### 下载视频

```bash
curl -X POST "http://localhost:8080/api/v1/download" \
  -H "Content-Type: application/json" \
  -d '{"url":"https://www.youtube.com/watch?v=example","format":"best"}'
```

#### 获取下载状态

```bash
curl -X GET "http://localhost:8080/api/v1/status/task_id_here"
```

#### 取消下载

```bash
curl -X DELETE "http://localhost:8080/api/v1/download/task_id_here"
```

## 配置

服务可以通过环境变量进行配置：

| 环境变量 | 描述 | 默认值 |
|----------|------|--------|
| PORT | 服务端口 | 8080 |
| LOG_LEVEL | 日志级别 (debug, info, warn, error) | info |
| LOG_FORMAT | 日志格式 (json, console) | json |
| YTDLP_PATH | yt-dlp 可执行文件路径 | /usr/bin/yt-dlp |
| FFMPEG_PATH | ffmpeg 可执行文件路径 | /usr/bin/ffmpeg |
| DOWNLOAD_DIR | 下载目录 | /app/downloads |
| MAX_DOWNLOADS | 最大并发下载数 | 5 |
| MAX_FILE_SIZE | 最大文件大小 (字节) | 1073741824 (1GB) |
| ENV | 环境 (development, production) | development |

## 项目结构

```
/
├── cmd/                # 应用程序入口
│   └── api/            # API 服务入口
├── internal/           # 内部包
│   ├── api/            # API 路由和处理器
│   ├── config/         # 配置管理
│   ├── logger/         # 日志管理
│   ├── middleware/     # 中间件
│   ├── models/         # 数据模型
│   └── ytdlp/          # yt-dlp 命令包装
├── pkg/                # 可重用的公共包
│   ├── utils/          # 工具函数
│   └── errors/         # 错误处理
├── docs/               # 文档
│   └── swagger/        # Swagger 文档
├── scripts/            # 脚本
├── DOCKERFILE          # Docker 构建文件
├── deploy/             # 部署配置
│   └── k8s.yml         # Kubernetes 配置
├── go.mod              # Go 模块文件
└── README.md           # 项目说明
```
