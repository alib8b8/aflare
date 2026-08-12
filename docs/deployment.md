# 部署指南

本指南介绍如何将 aflare 部署到生产环境。

---

## 前置要求

- Go 1.25+（仅源码构建需要）
- Docker（容器部署）
- 至少一个 LLM 提供商的 API Key（OpenAI、Anthropic、Ollama 等）

---

## Docker 部署

### 拉取官方镜像

```bash
docker pull ghcr.io/alib8b8/aflare:latest
```

### 运行单次工作流

```bash
docker run --rm \
  -v $(pwd)/workflows:/workflows \
  -v $(pwd)/output:/output \
  -e ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY} \
  ghcr.io/alib8b8/aflare:latest \
  run /workflows/my-workflow.yaml
```

### 使用本地 Ollama

```bash
docker run --rm \
  -v $(pwd)/workflows:/workflows \
  -e OLLAMA_BASE_URL=http://host.docker.internal:11434 \
  ghcr.io/alib8b8/aflare:latest \
  run /workflows/my-workflow.yaml
```

Linux 用户需添加 `--add-host=host.docker.internal:host-gateway`。

### 本地构建镜像

```bash
git clone https://github.com/alib8b8/aflare.git
cd aflare
docker build -t aflare:latest .
```

多平台构建：

```bash
docker buildx create --use
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t aflare:latest \
  --push \
  .
```

---

## Docker Compose

### 基础配置

创建 `docker-compose.yml`：

```yaml
services:
  aflare:
    image: ghcr.io/alib8b8/aflare:latest
    container_name: aflare
    restart: unless-stopped
    volumes:
      - ./workflows:/workflows
      - ./output:/output
      - ./nodes:/nodes
      - aflare-data:/home/aflare/.aflare
    environment:
      - AFLARE_LOG_LEVEL=info
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
      - OPENAI_API_KEY=${OPENAI_API_KEY}
    command: ["serve", "--port", "8080"]

volumes:
  aflare-data:
```

### 带 Ollama + API 服务

```yaml
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

  aflare-api:
    image: ghcr.io/alib8b8/aflare:latest
    container_name: aflare-api
    restart: unless-stopped
    depends_on:
      - ollama
    ports:
      - "8080:8080"
    volumes:
      - ./workflows:/workflows
      - ./output:/output
      - aflare-data:/home/aflare/.aflare
    environment:
      - AFLARE_LOG_LEVEL=info
      - AFLARE_PPROF=0
      - AFLARE_METRICS=0
      - OLLAMA_BASE_URL=http://ollama:11434
    command: ["serve", "--port", "8080", "--api-key", "${AFLARE_API_KEY}"]

volumes:
  ollama-data:
  aflare-data:
```

### 定时执行（Cron 模式）

```yaml
services:
  aflare-scheduler:
    image: ghcr.io/alib8b8/aflare:latest
    container_name: aflare-scheduler
    restart: unless-stopped
    volumes:
      - ./workflows:/workflows
      - ./output:/output
      - aflare-data:/home/aflare/.aflare
    environment:
      - AFLARE_LOG_LEVEL=info
      - ANTHROPIC_API_KEY=${ANTHROPIC_API_KEY}
    command:
      - /bin/sh
      - -c
      - |
        echo "0 9 * * * /usr/local/bin/aflare run /workflows/daily-report.yaml" | crontab -
        echo "*/30 * * * * /usr/local/bin/aflare run /workflows/btc-monitor.yaml" | crontab -
        crond -f -l 2

volumes:
  aflare-data:
```

---

## Systemd 服务

### API 服务

创建 `/etc/systemd/system/aflare-api.service`：

```ini
[Unit]
Description=aflare Workflow Execution API
After=network.target
Documentation=https://github.com/alib8b8/aflare

[Service]
Type=simple
User=aflare
Group=aflare
ExecStart=/usr/local/bin/aflare serve \
  --host 127.0.0.1 \
  --port 8080 \
  --api-key ${AFLARE_API_KEY} \
  --dir /var/lib/aflare/workflows

Restart=on-failure
RestartSec=5

# 安全加固
NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/aflare
PrivateTmp=yes
PrivateDevices=yes
ProtectKernelTunables=yes
ProtectKernelModules=yes
ProtectControlGroups=yes

# 环境变量
EnvironmentFile=/etc/aflare/api.env
Environment=AFLARE_LOG_LEVEL=info
Environment=AFLARE_LOG_FORMAT=json
Environment=AFLARE_LOG_FILE=/var/log/aflare/api.log

# 资源限制
MemoryMax=512M
CPUQuota=50%

[Install]
WantedBy=multi-user.target
```

### WebUI 服务

创建 `/etc/systemd/system/aflare-webui.service`：

```ini
[Unit]
Description=aflare WebUI Editor
After=network.target
Documentation=https://github.com/alib8b8/aflare

[Service]
Type=simple
User=aflare
Group=aflare
ExecStart=/usr/local/bin/aflare webui \
  --host 127.0.0.1 \
  --port 8081 \
  --dir /var/lib/aflare/workflows

Restart=on-failure
RestartSec=5

NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/aflare
PrivateTmp=yes

EnvironmentFile=/etc/aflare/api.env
Environment=AFLARE_LOG_LEVEL=info
Environment=AFLARE_LOG_FILE=/var/log/aflare/webui.log

MemoryMax=256M

[Install]
WantedBy=multi-user.target
```

### 调度器服务

创建 `/etc/systemd/system/aflare-scheduler.service`：

```ini
[Unit]
Description=aflare Cron Scheduler
After=network.target
Documentation=https://github.com/alib8b8/aflare

[Service]
Type=simple
User=aflare
Group=aflare
ExecStart=/usr/local/bin/aflare schedule start

Restart=on-failure
RestartSec=10

NoNewPrivileges=yes
ProtectSystem=strict
ProtectHome=yes
ReadWritePaths=/var/lib/aflare
PrivateTmp=yes

EnvironmentFile=/etc/aflare/api.env
Environment=AFLARE_LOG_LEVEL=info
Environment=AFLARE_LOG_FILE=/var/log/aflare/scheduler.log

MemoryMax=256M

[Install]
WantedBy=multi-user.target
```

### 启用服务

```bash
# 创建用户和目录
sudo useradd -r -s /bin/false aflare
sudo mkdir -p /var/lib/aflare/workflows /var/log/aflare
sudo chown -R aflare:aflare /var/lib/aflare /var/log/aflare

# 创建环境变量文件 /etc/aflare/api.env
sudo tee /etc/aflare/api.env << 'EOF'
ANTHROPIC_API_KEY=sk-ant-xxx
OPENAI_API_KEY=sk-xxx
AFLARE_API_KEY=your-secret-api-key
AFLARE_AUDIT_HMAC_KEY=your-audit-hmac-key
EOF
sudo chmod 600 /etc/aflare/api.env

# 启用
sudo systemctl daemon-reload
sudo systemctl enable --now aflare-api
sudo systemctl enable --now aflare-scheduler
```

---

## Nginx 反向代理

生产环境中建议使用 Nginx 作为反向代理，提供 TLS 终止和访问控制：

```nginx
# /etc/nginx/sites-available/aflare
server {
    listen 443 ssl;
    server_name aflare.example.com;

    ssl_certificate     /etc/ssl/certs/aflare.pem;
    ssl_certificate_key /etc/ssl/private/aflare.key;

    # API 服务
    location /api/v1/ {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        # 限流
        limit_req zone=api burst=20 nodelay;
        limit_req_status 429;
    }

    # WebUI
    location /webui/ {
        proxy_pass http://127.0.0.1:8081/;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;

        # 仅允许内网访问
        allow 10.0.0.0/8;
        allow 172.16.0.0/12;
        allow 192.168.0.0/16;
        deny all;
    }
}

# 限流区域
limit_req_zone $binary_remote_addr zone=api:10m rate=10r/s;
```

---

## 环境变量参考

### 核心路径

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AFLARE_HOME` | `~/.aflare` | 程序主目录（模板、技能、二进制文件） |
| `AFLARE_DATA` | `~/.aflare` | 数据目录（配置、日志、缓存、工作区） |
| `AFLARE_CONFIG` | 自动查找 | 配置文件路径（覆盖自动发现） |

### 日志

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AFLARE_LOG_LEVEL` | `info` | 日志级别：`debug` / `info` / `warn` / `error` |
| `AFLARE_LOG_FORMAT` | `text` | 日志格式：`text` / `json` |
| `AFLARE_LOG_FILE` | 无（stdout） | 日志文件路径 |
| `AFLARE_LOG_MAX_MB` | `100` | 日志文件轮转阈值（MB） |
| `AFLARE_LOG_MAX_BACKUPS` | `3` | 保留的旧日志文件数量 |

### 安全

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AFLARE_SAFE_MODE` | `false` | 安全模式：禁用命令执行节点 |
| `AFLARE_SECURITY_LEVEL` | `L1` | 安全等级：`L0` / `L1` / `L2` / `L3` |
| `AFLARE_SECRETS_PASSWORD` | 无 | 密钥加密主密码 |
| `AFLARE_AUDIT_HMAC_KEY` | 无 | 审计日志 HMAC 密钥（若未设置则使用 `AFLARE_SECRETS_PASSWORD` 派生） |
| `AFLARE_AUDIT_DIR` | 自动 | 审计日志目录 |
| `AFLARE_EXECUTE_ALLOWLIST` | `0` | 设为 `1` 启用命令执行白名单 |
| `AFLARE_WEBUI_AUTH_TOKEN` | 无 | WebUI API 认证 Token |

### 运行时

| 变量 | 默认值 | 说明 |
|------|--------|------|
| `AFLARE_LANG` | 系统语言 | 界面语言：`en` / `zh` / `ru` |
| `AFLARE_METRICS` | `0` | 设为 `1` 启用 Prometheus `/metrics` 端点 |
| `AFLARE_PPROF` | `0` | 设为 `1` 启用 `/debug/pprof/` 性能分析端点 |
| `AFLARE_PRICING_FILE` | 无 | LLM 定价覆盖文件（JSON 格式） |
| `AFLARE_TRACE_NO_REDACT` | `0` | 需与 `AFLARE_DEBUG_MODE=1` 同时启用，禁用 LLM trace 脱敏 |
| `AFLARE_DEBUG_MODE` | `0` | 调试模式（与 `AFLARE_TRACE_NO_REDACT` 配合使用） |
| `AFLARE_AUTOUPGRADE_CONFIG` | 自动 | 自动升级配置文件路径 |

### LLM 提供商

aflare 通过标准环境变量配置 LLM 提供商 API 密钥：

| 变量 | 说明 |
|------|------|
| `ANTHROPIC_API_KEY` | Anthropic Claude API 密钥 |
| `OPENAI_API_KEY` | OpenAI API 密钥 |
| `DEEPSEEK_API_KEY` | DeepSeek API 密钥 |
| `OLLAMA_BASE_URL` | Ollama 服务地址（默认 `http://localhost:11434`） |
| `GOOGLE_API_KEY` | Google Gemini API 密钥 |

也可以使用 `aflare.yaml` 配置文件管理提供商设置，详见 [入门指南](getting-started.md)。

---

## 安全注意事项

### 生产环境清单

- [ ] **使用反向代理**：Nginx/Caddy 提供 TLS 终止
- [ ] **设置 API Key**：`aflare serve --api-key` 保护 API 端点
- [ ] **设置审计密钥**：`AFLARE_AUDIT_HMAC_KEY` 启用防篡改审计日志
- [ ] **禁用 pprof**：生产环境不要设置 `AFLARE_PPROF=1`
- [ ] **谨慎启用 Metrics**：`AFLARE_METRICS=1` 仅在受信网络或反向代理后启用
- [ ] **限制 bind 地址**：WebUI 使用 `127.0.0.1`，不要绑定 `0.0.0.0`
- [ ] **使用非 root 用户**：Docker 镜像和 Systemd 服务均应以非 root 用户运行
- [ ] **保护环境变量文件**：`/etc/aflare/api.env` 权限设为 `600`
- [ ] **开启安全模式**：生产环境推荐 `AFLARE_SAFE_MODE=1` 或 `AFLARE_SECURITY_LEVEL=L3`
- [ ] **定期更新**：使用 `aflare self-update` 或 Docker 镜像更新

### Docker 安全加固

```bash
docker run --rm \
  --cap-drop=ALL \
  --security-opt=no-new-privileges \
  --read-only \
  --tmpfs /tmp \
  ghcr.io/alib8b8/aflare:latest \
  run /workflows/my-workflow.yaml
```

### 密钥管理

- 不要在镜像或工作流 YAML 中硬编码 API 密钥
- 使用 `aflare secrets add` 管理加密密钥
- 通过环境变量注入敏感信息
- 审计日志使用 HMAC 哈希链防止篡改

---

## 相关文档

- [README](../README.md) — 项目概览与快速开始
- [API 参考](api.md) — REST API 详细文档
- [WebUI 编辑器](webui.md) — 可视化编辑器
- [入门指南](getting-started.md) — 工作流 YAML 语法
- [安全策略](../SECURITY.md) — 安全漏洞报告与最佳实践
- [故障排查](troubleshooting.md) — 常见问题