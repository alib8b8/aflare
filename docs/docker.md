# Docker 部署指南

本指南介绍如何使用 Docker 部署 llm-box。

## 快速开始

### 使用预构建镜像

```bash
# 拉取最新镜像
docker pull ghcr.io/alib8b8/llm-box:latest

# 运行工作流
docker run --rm \
  -v $(pwd)/workflows:/workflows \
  -v $(pwd)/output:/output \
  -e ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY} \
  ghcr.io/alib8b8/llm-box:latest \
  run /workflows/my-workflow.yaml
```

### 使用特定版本

```bash
docker pull ghcr.io/alib8b8/llm-box:v0.2.10

docker run --rm \
  -v $(pwd)/workflows:/workflows \
  ghcr.io/alib8b8/llm-box:v0.2.10 \
  run /workflows/my-workflow.yaml
```

## 本地构建

### 构建镜像

```bash
# 克隆仓库
git clone https://github.com/alib8b8/llm-box.git
cd llm-box

# 构建镜像
docker build -t llm-box:latest .
```

### 多平台构建

```bash
# 启用 buildx
docker buildx create --use

# 构建 linux/amd64 和 linux/arm64
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t llm-box:latest \
  --push \
  .
```

## Dockerfile

项目使用以下 Dockerfile：

```dockerfile
# Stage 1: Build
FROM golang:1.25-alpine AS builder

WORKDIR /app

# 安装依赖
RUN apk add --no-cache git ca-certificates

# 复制 go.mod 和 go.sum
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 构建
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X main.version=$(git describe --tags --always --dirty)" \
    -o llm-box ./cmd/llm-box

# Stage 2: Runtime
FROM alpine:3.20

WORKDIR /app

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 从构建阶段复制二进制文件
COPY --from=builder /app/llm-box /app/llm-box

# 创建必要目录
RUN mkdir -p /workflows /output /nodes

# 设置环境变量
ENV LLMBOX_CONFIG=/app/config.yaml
ENV LLMBOX_LOG_LEVEL=info
ENV TZ=Asia/Shanghai

# 设置工作目录
WORKDIR /workflows

# 入口点
ENTRYPOINT ["/app/llm-box"]
CMD ["--help"]
```

## 配置

### 环境变量

| 变量 | 描述 | 默认值 |
|------|------|--------|
| `LLMBOX_CONFIG` | 配置文件路径 | `/app/config.yaml` |
| `LLMBOX_LOG_LEVEL` | 日志级别 | `info` |
| `LLMBOX_CACHE_DIR` | 缓存目录 | `/tmp/llm-box-cache` |
| `ANTHROPIC_API_KEY` | Anthropic API 密钥 | - |
| `OPENAI_API_KEY` | OpenAI API 密钥 | - |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 | - |

### 挂载卷

```bash
docker run --rm \
  -v $(pwd)/workflows:/workflows \      # 工作流目录
  -v $(pwd)/output:/output \            # 输出目录
  -v $(pwd)/nodes:/nodes \              # 自定义节点
  -v $(pwd)/config.yaml:/app/config.yaml \  # 配置文件
  -v llmbox-cache:/tmp/llm-box-cache \  # 持久化缓存
  ghcr.io/alib8b8/llm-box:latest \
  run /workflows/my-workflow.yaml
```

### 配置文件示例

创建 `config.yaml`：

```yaml
# llm-box 配置

# 日志设置
log:
  level: info
  format: json

# 提供商设置
providers:
  openai:
    api_key: ${OPENAI_API_KEY}
    base_url: https://api.openai.com/v1
  anthropic:
    api_key: ${ANTHROPIC_API_KEY}
    base_url: https://api.anthropic.com/v1
  ollama:
    base_url: http://host.docker.internal:11434

# 默认模型
default_model: gpt-4o-mini

# 安全设置
security:
  max_file_size: 10MB
  max_steps: 1000
  allowed_hosts:
    - api.openai.com
    - api.anthropic.com
```

## Docker Compose

### 基本配置

创建 `docker-compose.yml`：

```yaml
version: '3.8'

services:
  llm-box:
    image: ghcr.io/alib8b8/llm-box:latest
    container_name: llm-box
    restart: unless-stopped
    volumes:
      - ./workflows:/workflows
      - ./output:/output
      - ./nodes:/nodes
      - llmbox-cache:/tmp/llm-box-cache
    environment:
      - LLMBOX_LOG_LEVEL=info
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    command: ["run", "--watch", "/workflows"]

volumes:
  llmbox-cache:
```

### 带本地 Ollama

```yaml
version: '3.8'

services:
  ollama:
    image: ollama/ollama:latest
    container_name: ollama
    restart: unless-stopped
    ports:
      - "11434:11434"
    volumes:
      - ollama-data:/root/.ollama
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]

  llm-box:
    image: ghcr.io/alib8b8/llm-box:latest
    container_name: llm-box
    restart: unless-stopped
    depends_on:
      - ollama
    volumes:
      - ./workflows:/workflows
      - ./output:/output
    environment:
      - OLLAMA_BASE_URL=http://ollama:11434
    command: ["run", "/workflows/my-workflow.yaml"]

volumes:
  ollama-data:
```

### 定时执行

使用 cron 定时运行工作流：

```yaml
version: '3.8'

services:
  llm-box-cron:
    image: ghcr.io/alib8b8/llm-box:latest
    container_name: llm-box-cron
    restart: unless-stopped
    volumes:
      - ./workflows:/workflows
      - ./output:/output
    environment:
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    entrypoint: |
      sh -c "
        echo '0 9 * * * cd /workflows && /app/llm-box run daily-report.yaml' | crontab -
        crond -f
      "
```

## Kubernetes 部署

### Deployment

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: llm-box
  labels:
    app: llm-box
spec:
  replicas: 1
  selector:
    matchLabels:
      app: llm-box
  template:
    metadata:
      labels:
        app: llm-box
    spec:
      containers:
      - name: llm-box
        image: ghcr.io/alib8b8/llm-box:latest
        resources:
          requests:
            memory: "256Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
        env:
        - name: ANTHROPIC_API_KEY
          valueFrom:
            secretKeyRef:
              name: llmbox-secrets
              key: anthropic-api-key
        volumeMounts:
        - name: workflows
          mountPath: /workflows
        - name: output
          mountPath: /output
      volumes:
      - name: workflows
        configMap:
          name: llmbox-workflows
      - name: output
        emptyDir: {}
```

### CronJob

```yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: llm-box-daily
spec:
  schedule: "0 9 * * *"
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: llm-box
            image: ghcr.io/alib8b8/llm-box:latest
            args:
            - run
            - /workflows/daily-report.yaml
            env:
            - name: ANTHROPIC_API_KEY
              valueFrom:
                secretKeyRef:
                  name: llmbox-secrets
                  key: anthropic-api-key
          restartPolicy: OnFailure
```

## 常见问题

### 如何访问本地 Ollama？

在 Docker 中使用 `host.docker.internal` 访问宿主机：

```bash
docker run --rm \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/alib8b8/llm-box:latest \
  run my-workflow.yaml
```

Linux 用户可能需要添加：

```bash
--add-host=host.docker.internal:host-gateway
```

### 如何持久化缓存？

```bash
docker run --rm \
  -v llmbox-cache:/tmp/llm-box-cache \
  ghcr.io/alib8b8/llm-box:latest \
  run my-workflow.yaml
```

### 如何调试？

```bash
# 进入容器
docker run --rm -it \
  --entrypoint sh \
  ghcr.io/alib8b8/llm-box:latest

# 查看日志
docker logs llm-box

# 详细日志
docker run --rm \
  -e LLMBOX_LOG_LEVEL=debug \
  ghcr.io/alib8b8/llm-box:latest \
  run my-workflow.yaml
```

### 如何限制资源？

```bash
docker run --rm \
  --memory="512m" \
  --cpus="0.5" \
  ghcr.io/alib8b8/llm-box:latest \
  run my-workflow.yaml
```

## 安全建议

1. **不要在镜像中硬编码 API 密钥**
2. **使用 Docker secrets 或环境变量**
3. **限制容器权限**：

```bash
docker run --rm \
  --cap-drop=ALL \
  --read-only \
  --tmpfs /tmp \
  ghcr.io/alib8b8/llm-box:latest \
  run my-workflow.yaml
```

4. **使用非 root 用户**（已在镜像中配置）

5. **定期更新基础镜像**

```bash
docker pull alpine:3.20
docker build --no-cache -t llm-box:latest .
```

## 更多资源

- [Docker Hub](https://hub.docker.com/r/alib8b8/llm-box)
- [GitHub Container Registry](https://github.com/alib8b8/llm-box/pkgs/container/llm-box)
- [Kubernetes Helm Chart](./helm/)