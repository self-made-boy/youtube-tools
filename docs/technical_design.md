# YouTube Tools 技术方案

## 1. 项目概述

本项目旨在开发一个基于 Golang 的 RESTful API 服务，用于包装 yt-dlp 工具的操作，提供视频下载、信息获取等功能。

## 2. 技术栈

- **编程语言**：Golang 1.24.2
- **Web 框架**：Gin
- **日志框架**：zap
- **API 文档**：Swagger/OpenAPI
- **容器化**：Docker
- **部署**：Kubernetes
- **视频处理工具**：yt-dlp, ffmpeg

## 3. 系统架构

```
+----------------+     +----------------+     +----------------+
|                |     |                |     |                |
|  客户端请求    | --> |  RESTful API   | --> |  yt-dlp 命令   |
|                |     |                |     |                |
+----------------+     +----------------+     +----------------+
                                |
                                v
                       +----------------+
                       |                |
                       |  日志系统      |
                       |                |
                       +----------------+
```

## 4. 项目结构

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

## 5. API 设计

### 5.1 基础路径

```
/api/v1
```

### 5.2 端点设计

#### 5.2.1 获取视频信息

```
GET /api/v1/info
```

**参数**：
- `url` (必填): 视频 URL
- `format` (可选): 输出格式 (json, simple)

**响应**：
```json
{
  "status": "success",
  "data": {
    "id": "video_id",
    "title": "视频标题",
    "uploader": "上传者",
    "duration": 123,
    "upload_date": "20230101",
    "formats": [
      {
        "format_id": "22",
        "ext": "mp4",
        "resolution": "720p",
        "filesize": 12345678
      }
    ]
  }
}
```

#### 5.2.2 下载视频

```
POST /api/v1/download
```

**请求体**：
```json
{
  "url": "https://www.youtube.com/watch?v=example",
  "format": "best",
  "output_dir": "/downloads",
  "filename": "custom_name"
}
```

**响应**：
```json
{
  "status": "success",
  "data": {
    "task_id": "unique_task_id",
    "filename": "downloaded_file.mp4",
    "path": "/downloads/downloaded_file.mp4",
    "size": 12345678
  }
}
```

#### 5.2.3 获取下载状态

```
GET /api/v1/status/:task_id
```

**响应**：
```json
{
  "status": "success",
  "data": {
    "task_id": "unique_task_id",
    "state": "completed", // pending, downloading, completed, failed
    "progress": 100,
    "speed": "1.2 MB/s",
    "eta": "00:00",
    "filename": "downloaded_file.mp4",
    "error": null
  }
}
```

#### 5.2.4 取消下载

```
DELETE /api/v1/download/:task_id
```

**响应**：
```json
{
  "status": "success",
  "message": "Download cancelled successfully"
}
```

#### 5.2.5 健康检查

```
GET /api/v1/health
```

**响应**：
```json
{
  "status": "success",
  "data": {
    "version": "1.0.0",
    "uptime": "1h 23m 45s"
  }
}
```

## 6. 日志系统

使用 zap 日志库实现结构化日志记录，包括：

- 访问日志：记录所有 API 请求和响应
- 应用日志：记录应用程序运行状态和错误
- yt-dlp 日志：记录 yt-dlp 命令的输出和错误

日志格式：

```json
{
  "level": "info",
  "timestamp": "2023-01-01T12:00:00.000Z",
  "caller": "api/handler.go:42",
  "message": "Request received",
  "request_id": "req-123",
  "method": "GET",
  "path": "/api/v1/info",
  "ip": "127.0.0.1",
  "user_agent": "Mozilla/5.0...",
  "latency": "42ms"
}
```

## 7. 错误处理

统一的错误响应格式：

```json
{
  "status": "error",
  "error": {
    "code": "INVALID_URL",
    "message": "The provided URL is invalid or unsupported",
    "details": {}
  }
}
```

错误码分类：

- 400-499: 客户端错误
- 500-599: 服务器错误

## 8. 安全考虑

- 输入验证：验证所有用户输入，特别是 URL 和命令参数
- 资源限制：限制下载大小、并发下载数量和速率
- 日志脱敏：确保敏感信息不会记录到日志中

## 9. 部署方案

### 9.1 Docker 部署

使用多阶段构建的 Dockerfile，包含：

1. 基础镜像：Ubuntu 24.04
2. 开发工具安装
3. yt-dlp 和 ffmpeg 安装
4. Golang 应用构建
5. 最终运行镜像

### 9.2 Kubernetes 部署

单节点部署，使用 Kubernetes 配置：

- Deployment: 1 个副本
- Service: 暴露 8080 端口
- PersistentVolumeClaim: 用于存储下载的视频文件

## 10. 监控和维护

- 健康检查端点用于监控服务状态
- 资源使用情况监控（CPU、内存、磁盘）
- 定期清理临时文件和过期下载

## 11. 后续优化方向

- 添加用户认证和授权
- 实现异步任务队列
- 添加缓存机制
- 支持更多视频源和格式
- 添加视频处理功能（如转码、裁剪等）